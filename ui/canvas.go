//go:build windows

// Package ui is a tiny immediate-mode drawing layer on GDI (text) + GDI+ (shapes).
// All coordinates are logical pixels (96 DPI); the canvas scales them.
package ui

import (
	"math"
	"unicode/utf8"

	"pulse/win"
)

type Font struct {
	Size   float32
	Weight int32 // 400 normal, 600 semibold, 700 bold
	Mono   bool
	Icon   bool
}

var (
	FontBody   = Font{Size: 13, Weight: 400}
	FontSmall  = Font{Size: 11.5, Weight: 400}
	FontSemi   = Font{Size: 13, Weight: 600}
	FontTitle  = Font{Size: 20, Weight: 600}
	FontH2     = Font{Size: 15, Weight: 600}
	FontBig    = Font{Size: 26, Weight: 600}
	FontMono   = Font{Size: 12.5, Weight: 400, Mono: true}
	FontMonoB  = Font{Size: 12.5, Weight: 600, Mono: true}
	FontIcon   = Font{Size: 15, Icon: true}
	FontIconSm = Font{Size: 11, Icon: true}
)

// IconFace is "Segoe Fluent Icons" on Windows 11, "Segoe MDL2 Assets" on Windows 10.
var IconFace = "Segoe Fluent Icons"
var UIFace = "Segoe UI Variable Text"
var MonoFace = "Cascadia Mono"

type Canvas struct {
	HDC   uintptr
	G     uintptr
	S     float32 // DPI scale
	W, H  float32 // logical size
	fonts map[Font]uintptr
	cur   Font
	clips []rect
}

type rect struct{ x, y, w, h float32 }

func NewCanvas() *Canvas { return &Canvas{fonts: map[Font]uintptr{}, S: 1} }

// Begin starts a frame on a memory DC.
func (c *Canvas) Begin(hdc uintptr, wpx, hpx int32, scale float32) {
	if scale != c.S {
		c.ResetFonts()
	}
	c.HDC, c.S = hdc, scale
	c.W, c.H = float32(wpx)/scale, float32(hpx)/scale
	c.G = win.GdipFromHDC(hdc)
	win.SetBkTransparent(hdc)
	c.cur = Font{}
	c.clips = c.clips[:0]
}

func (c *Canvas) End() {
	win.GdipDelete(c.G)
	c.G = 0
}

func (c *Canvas) ResetFonts() {
	for _, f := range c.fonts {
		win.DeleteObject(f)
	}
	c.fonts = map[Font]uintptr{}
}

func (c *Canvas) font(f Font) {
	if f == c.cur && c.cur.Size != 0 {
		return
	}
	h, ok := c.fonts[f]
	if !ok {
		face := UIFace
		if f.Mono {
			face = MonoFace
		}
		if f.Icon {
			face = IconFace
		}
		w := f.Weight
		if w == 0 {
			w = 400
		}
		h = win.CreateFont(int32(math.Round(float64(f.Size*c.S))), w, false, face)
		c.fonts[f] = h
	}
	win.SelectObject(c.HDC, h)
	c.cur = f
}

func (c *Canvas) p(v float32) float32 { return v * c.S }
func (c *Canvas) i(v float32) int32   { return int32(math.Round(float64(v * c.S))) }

// ---- color helpers: colors are 0xRRGGBB ----

func argb(rgb uint32, a uint8) uint32 { return uint32(a)<<24 | rgb&0xFFFFFF }
func bgr(rgb uint32) uint32            { return (rgb&0xFF)<<16 | rgb&0xFF00 | (rgb>>16)&0xFF }

// Mix blends a toward b by t (0..1).
func Mix(a, b uint32, t float32) uint32 {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	ch := func(s uint) uint32 {
		x := float32((a>>s)&0xFF)*(1-t) + float32((b>>s)&0xFF)*t
		return uint32(x+0.5) << s
	}
	return ch(16) | ch(8) | ch(0)
}

// ---- shapes ----

func (c *Canvas) Fill(x, y, w, h float32, rgb uint32) {
	win.GFillRect(c.G, argb(rgb, 255), c.p(x), c.p(y), c.p(w), c.p(h))
}
func (c *Canvas) FillA(x, y, w, h float32, rgb uint32, a uint8) {
	win.GFillRect(c.G, argb(rgb, a), c.p(x), c.p(y), c.p(w), c.p(h))
}
func (c *Canvas) Round(x, y, w, h, r float32, rgb uint32) {
	win.GFillRound(c.G, argb(rgb, 255), c.p(x), c.p(y), c.p(w), c.p(h), c.p(r))
}
func (c *Canvas) RoundA(x, y, w, h, r float32, rgb uint32, a uint8) {
	win.GFillRound(c.G, argb(rgb, a), c.p(x), c.p(y), c.p(w), c.p(h), c.p(r))
}
func (c *Canvas) Border(x, y, w, h, r float32, rgb uint32, width float32) {
	o := width / 2
	win.GStrokeRound(c.G, argb(rgb, 255), c.p(width), c.p(x+o), c.p(y+o), c.p(w-width), c.p(h-width), c.p(r))
}
func (c *Canvas) Line(x1, y1, x2, y2 float32, rgb uint32, width float32) {
	win.GLine(c.G, argb(rgb, 255), c.p(width), c.p(x1), c.p(y1), c.p(x2), c.p(y2))
}
func (c *Canvas) LineA(x1, y1, x2, y2 float32, rgb uint32, a uint8, width float32) {
	win.GLine(c.G, argb(rgb, a), c.p(width), c.p(x1), c.p(y1), c.p(x2), c.p(y2))
}
func (c *Canvas) Circle(cx, cy, r float32, rgb uint32) {
	win.GFillEllipse(c.G, argb(rgb, 255), c.p(cx-r), c.p(cy-r), c.p(2*r), c.p(2*r))
}
func (c *Canvas) Arc(x, y, w, h, start, sweep float32, rgb uint32, width float32) {
	win.GArc(c.G, argb(rgb, 255), c.p(width), c.p(x), c.p(y), c.p(w), c.p(h), start, sweep)
}

// ---- clipping ----

func (c *Canvas) Clip(x, y, w, h float32) {
	c.clips = append(c.clips, rect{x, y, w, h})
	c.applyClip()
}
func (c *Canvas) Unclip() {
	if len(c.clips) > 0 {
		c.clips = c.clips[:len(c.clips)-1]
	}
	c.applyClip()
}
func (c *Canvas) applyClip() {
	win.ResetClip(c.HDC)
	win.GdipResetClip(c.G)
	if len(c.clips) == 0 {
		return
	}
	r := c.clips[len(c.clips)-1]
	win.ClipRect(c.HDC, win.RECT{Left: c.i(r.x), Top: c.i(r.y), Right: c.i(r.x + r.w), Bottom: c.i(r.y + r.h)})
	win.GdipClip(c.G, c.i(r.x), c.i(r.y), c.i(r.w), c.i(r.h))
}

// ---- text ----

const (
	AlignLeft   = 0
	AlignCenter = 1
	AlignRight  = 2
)

func (c *Canvas) Text(x, y, w, h float32, s string, f Font, rgb uint32, align int) {
	if s == "" {
		return
	}
	c.font(f)
	win.SetTextColor(c.HDC, bgr(rgb))
	r := win.RECT{Left: c.i(x), Top: c.i(y), Right: c.i(x + w), Bottom: c.i(y + h)}
	flags := uint32(win.DT_SINGLELINE | win.DT_VCENTER | win.DT_NOPREFIX | win.DT_END_ELLIPSIS)
	switch align {
	case AlignCenter:
		flags |= win.DT_CENTER
	case AlignRight:
		flags |= win.DT_RIGHT
	}
	win.DrawText(c.HDC, s, &r, flags)
}

// TextWrap draws multi-line wrapped text.
func (c *Canvas) TextWrap(x, y, w, h float32, s string, f Font, rgb uint32) {
	c.font(f)
	win.SetTextColor(c.HDC, bgr(rgb))
	r := win.RECT{Left: c.i(x), Top: c.i(y), Right: c.i(x + w), Bottom: c.i(y + h)}
	win.DrawText(c.HDC, s, &r, win.DT_WORDBREAK|win.DT_NOPREFIX|win.DT_END_ELLIPSIS)
}

func (c *Canvas) TextW(s string, f Font) float32 {
	c.font(f)
	w, _ := win.TextExtent(c.HDC, s)
	return float32(w) / c.S
}

// Icon draws one glyph from the icon font.
func (c *Canvas) Icon(x, y, w, h float32, glyph rune, size float32, rgb uint32) {
	if glyph == 0 {
		return
	}
	buf := make([]byte, 4)
	n := utf8.EncodeRune(buf, glyph)
	c.Text(x, y, w, h, string(buf[:n]), Font{Size: size, Icon: true}, rgb, AlignCenter)
}

// DrawIcon draws a Windows HICON.
func (c *Canvas) DrawHIcon(x, y, size float32, h uintptr) {
	if h != 0 {
		win.DrawIcon(c.HDC, h, c.i(x), c.i(y), c.i(size))
	}
}

// ---- charts ----

// Series returns n values (oldest first) and the value at i.
type Series interface {
	Len() int
	At(i int) float64
}

// Chart draws an area chart of s scaled to max over capacity points.
func (c *Canvas) Chart(x, y, w, h float32, s Series, capacity int, max float64, rgb uint32, grid bool, fillAlpha uint8) {
	if grid {
		for i := 1; i < 4; i++ {
			gy := y + h*float32(i)/4
			c.LineA(x, gy, x+w, gy, 0xFFFFFF, 14, 1)
		}
		for i := 1; i < 6; i++ {
			gx := x + w*float32(i)/6
			c.LineA(gx, y, gx, y+h, 0xFFFFFF, 10, 1)
		}
	}
	n := s.Len()
	if n < 2 || max <= 0 {
		return
	}
	step := w / float32(capacity-1)
	off := float32(capacity-n) * step
	pts := make([]win.PointF, 0, n+2)
	for i := 0; i < n; i++ {
		v := s.At(i) / max
		if v > 1 {
			v = 1
		}
		if v < 0 {
			v = 0
		}
		pts = append(pts, win.PointF{X: c.p(x + off + float32(i)*step), Y: c.p(y + h - 1 - float32(v)*(h-2))})
	}
	if fillAlpha > 0 {
		poly := append(append([]win.PointF{}, pts...), win.PointF{X: pts[len(pts)-1].X, Y: c.p(y + h)}, win.PointF{X: pts[0].X, Y: c.p(y + h)})
		win.GFillPolyGradient(c.G, argb(rgb, fillAlpha), argb(rgb, fillAlpha/8), c.p(y), c.p(y+h), poly)
	}
	win.GPolyline(c.G, argb(rgb, 255), c.p(1.6), pts)
}
