//go:build windows

package win

import (
	"math"
	"unsafe"
)

var (
	pCreateCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	pCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	pSelectObject           = gdi32.NewProc("SelectObject")
	pDeleteObject           = gdi32.NewProc("DeleteObject")
	pDeleteDC               = gdi32.NewProc("DeleteDC")
	pBitBlt                 = gdi32.NewProc("BitBlt")
	pCreateFontW            = gdi32.NewProc("CreateFontW")
	pSetTextColor           = gdi32.NewProc("SetTextColor")
	pSetBkMode              = gdi32.NewProc("SetBkMode")
	pGetTextExtentPoint32   = gdi32.NewProc("GetTextExtentPoint32W")
	pCreateSolidBrush       = gdi32.NewProc("CreateSolidBrush")
	pGetDeviceCaps          = gdi32.NewProc("GetDeviceCaps")
	pIntersectClipRect      = gdi32.NewProc("IntersectClipRect")
	pSelectClipRgn          = gdi32.NewProc("SelectClipRgn")
	pFillRect               = user32.NewProc("FillRect")
	pDrawText               = user32.NewProc("DrawTextW")
	pGetDC                  = user32.NewProc("GetDC")
	pReleaseDC              = user32.NewProc("ReleaseDC")
)

const (
	DT_LEFT       = 0x0
	DT_CENTER     = 0x1
	DT_RIGHT      = 0x2
	DT_VCENTER    = 0x4
	DT_SINGLELINE = 0x20
	DT_NOPREFIX   = 0x800
	DT_END_ELLIPSIS = 0x8000
	DT_WORDBREAK  = 0x10
	SRCCOPY       = 0x00CC0020
)

func CreateCompatibleDC(hdc uintptr) uintptr            { return call(pCreateCompatibleDC, hdc) }
func CreateCompatibleBitmap(hdc uintptr, w, h int32) uintptr {
	return call(pCreateCompatibleBitmap, hdc, uintptr(w), uintptr(h))
}
func SelectObject(hdc, obj uintptr) uintptr { return call(pSelectObject, hdc, obj) }
func DeleteObject(obj uintptr)              { call(pDeleteObject, obj) }
func DeleteDC(hdc uintptr)                  { call(pDeleteDC, hdc) }
func BitBlt(dst uintptr, x, y, w, h int32, src uintptr, sx, sy int32) {
	call(pBitBlt, dst, uintptr(x), uintptr(y), uintptr(w), uintptr(h), src, uintptr(sx), uintptr(sy), SRCCOPY)
}
func CreateFont(height int32, weight int32, italic bool, face string) uintptr {
	// CLEARTYPE_QUALITY = 5
	return call(pCreateFontW, uintptr(-height), 0, 0, 0, uintptr(weight), B(italic), 0, 0, 1, 0, 0, 5, 0, Str(face))
}
func SetTextColor(hdc uintptr, rgb uint32) { call(pSetTextColor, hdc, uintptr(rgb)) }
func SetBkTransparent(hdc uintptr)        { call(pSetBkMode, hdc, 1) }
func DrawText(hdc uintptr, s string, r *RECT, flags uint32) {
	u := StrPtr(s)
	call(pDrawText, hdc, uintptr(unsafe.Pointer(u)), ^uintptr(0), uintptr(unsafe.Pointer(r)), uintptr(flags))
}
func TextExtent(hdc uintptr, s string) (int32, int32) {
	if s == "" {
		return 0, 0
	}
	u := StrPtr(s)
	n := 0
	for p := unsafe.Pointer(u); *(*uint16)(p) != 0; p = unsafe.Add(p, 2) {
		n++
	}
	var sz struct{ CX, CY int32 }
	call(pGetTextExtentPoint32, hdc, uintptr(unsafe.Pointer(u)), uintptr(n), uintptr(unsafe.Pointer(&sz)))
	return sz.CX, sz.CY
}
func CreateSolidBrush(rgb uint32) uintptr { return call(pCreateSolidBrush, uintptr(rgb)) }
func FillRect(hdc uintptr, r *RECT, brush uintptr) {
	call(pFillRect, hdc, uintptr(unsafe.Pointer(r)), brush)
}
func GetDC(h uintptr) uintptr       { return call(pGetDC, h) }
func ReleaseDC(h, hdc uintptr)      { call(pReleaseDC, h, hdc) }
func ClipRect(hdc uintptr, r RECT)  { call(pIntersectClipRect, hdc, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom)) }
func ResetClip(hdc uintptr)         { call(pSelectClipRgn, hdc, 0) }

// ---------------- GDI+ (flat API) for anti-aliased shapes ----------------

var (
	pGdiplusStartup         = gdiplus.NewProc("GdiplusStartup")
	pGdipCreateFromHDC      = gdiplus.NewProc("GdipCreateFromHDC")
	pGdipDeleteGraphics     = gdiplus.NewProc("GdipDeleteGraphics")
	pGdipSetSmoothingMode   = gdiplus.NewProc("GdipSetSmoothingMode")
	pGdipSetPixelOffsetMode = gdiplus.NewProc("GdipSetPixelOffsetMode")
	pGdipCreateSolidFill    = gdiplus.NewProc("GdipCreateSolidFill")
	pGdipDeleteBrush        = gdiplus.NewProc("GdipDeleteBrush")
	pGdipCreatePen1         = gdiplus.NewProc("GdipCreatePen1")
	pGdipDeletePen          = gdiplus.NewProc("GdipDeletePen")
	pGdipSetPenLineJoin     = gdiplus.NewProc("GdipSetPenLineJoin")
	pGdipFillRectangle      = gdiplus.NewProc("GdipFillRectangle")
	pGdipDrawLine           = gdiplus.NewProc("GdipDrawLine")
	pGdipDrawLines          = gdiplus.NewProc("GdipDrawLines")
	pGdipFillPolygon        = gdiplus.NewProc("GdipFillPolygon")
	pGdipFillEllipse        = gdiplus.NewProc("GdipFillEllipse")
	pGdipDrawEllipse        = gdiplus.NewProc("GdipDrawEllipse")
	pGdipCreatePath         = gdiplus.NewProc("GdipCreatePath")
	pGdipDeletePath         = gdiplus.NewProc("GdipDeletePath")
	pGdipAddPathArc         = gdiplus.NewProc("GdipAddPathArc")
	pGdipAddPathLine        = gdiplus.NewProc("GdipAddPathLine")
	pGdipClosePathFigure    = gdiplus.NewProc("GdipClosePathFigure")
	pGdipFillPath           = gdiplus.NewProc("GdipFillPath")
	pGdipDrawPath           = gdiplus.NewProc("GdipDrawPath")
	pGdipCreateLineBrush    = gdiplus.NewProc("GdipCreateLineBrush")
	pGdipDrawArc            = gdiplus.NewProc("GdipDrawArc")
	pGdipSetClipRectI       = gdiplus.NewProc("GdipSetClipRectI")
	pGdipResetClip          = gdiplus.NewProc("GdipResetClip")
	pGdipSetPenStartCap     = gdiplus.NewProc("GdipSetPenStartCap")
	pGdipSetPenEndCap       = gdiplus.NewProc("GdipSetPenEndCap")
)

func f(v float32) uintptr { return uintptr(math.Float32bits(v)) }

type PointF struct{ X, Y float32 }

func GdiplusStartup() {
	in := struct {
		Version  uint32
		Callback uintptr
		NoThread int32
		NoCodecs int32
	}{Version: 1}
	var token uintptr
	call(pGdiplusStartup, uintptr(unsafe.Pointer(&token)), uintptr(unsafe.Pointer(&in)), 0)
}

func GdipFromHDC(hdc uintptr) uintptr {
	var g uintptr
	call(pGdipCreateFromHDC, hdc, uintptr(unsafe.Pointer(&g)))
	if g != 0 {
		call(pGdipSetSmoothingMode, g, 4) // AntiAlias
		call(pGdipSetPixelOffsetMode, g, 4) // Half
	}
	return g
}
func GdipDelete(g uintptr) { call(pGdipDeleteGraphics, g) }
func GdipClip(g uintptr, x, y, w, h int32) {
	call(pGdipSetClipRectI, g, uintptr(x), uintptr(y), uintptr(w), uintptr(h), 0)
}
func GdipResetClip(g uintptr) { call(pGdipResetClip, g) }

func brush(argb uint32) uintptr {
	var b uintptr
	call(pGdipCreateSolidFill, uintptr(argb), uintptr(unsafe.Pointer(&b)))
	return b
}
func pen(argb uint32, w float32) uintptr {
	var p uintptr
	call(pGdipCreatePen1, uintptr(argb), f(w), 2 /*UnitPixel*/, uintptr(unsafe.Pointer(&p)))
	call(pGdipSetPenLineJoin, p, 2) // round
	call(pGdipSetPenStartCap, p, 2)
	call(pGdipSetPenEndCap, p, 2)
	return p
}

func GFillRect(g uintptr, argb uint32, x, y, w, h float32) {
	b := brush(argb)
	call(pGdipFillRectangle, g, b, f(x), f(y), f(w), f(h))
	call(pGdipDeleteBrush, b)
}

func roundPath(x, y, w, h, r float32) uintptr {
	var p uintptr
	call(pGdipCreatePath, 0, uintptr(unsafe.Pointer(&p)))
	if r*2 > h {
		r = h / 2
	}
	if r*2 > w {
		r = w / 2
	}
	if r < 0.5 {
		call(pGdipAddPathLine, p, f(x), f(y), f(x+w), f(y))
		call(pGdipAddPathLine, p, f(x+w), f(y), f(x+w), f(y+h))
		call(pGdipAddPathLine, p, f(x+w), f(y+h), f(x), f(y+h))
		call(pGdipClosePathFigure, p)
		return p
	}
	d := r * 2
	call(pGdipAddPathArc, p, f(x), f(y), f(d), f(d), f(180), f(90))
	call(pGdipAddPathArc, p, f(x+w-d), f(y), f(d), f(d), f(270), f(90))
	call(pGdipAddPathArc, p, f(x+w-d), f(y+h-d), f(d), f(d), f(0), f(90))
	call(pGdipAddPathArc, p, f(x), f(y+h-d), f(d), f(d), f(90), f(90))
	call(pGdipClosePathFigure, p)
	return p
}

func GFillRound(g uintptr, argb uint32, x, y, w, h, r float32) {
	if w <= 0 || h <= 0 {
		return
	}
	p := roundPath(x, y, w, h, r)
	b := brush(argb)
	call(pGdipFillPath, g, b, p)
	call(pGdipDeleteBrush, b)
	call(pGdipDeletePath, p)
}

func GStrokeRound(g uintptr, argb uint32, width, x, y, w, h, r float32) {
	if w <= 0 || h <= 0 {
		return
	}
	p := roundPath(x, y, w, h, r)
	pn := pen(argb, width)
	call(pGdipDrawPath, g, pn, p)
	call(pGdipDeletePen, pn)
	call(pGdipDeletePath, p)
}

func GLine(g uintptr, argb uint32, width, x1, y1, x2, y2 float32) {
	pn := pen(argb, width)
	call(pGdipDrawLine, g, pn, f(x1), f(y1), f(x2), f(y2))
	call(pGdipDeletePen, pn)
}

func GPolyline(g uintptr, argb uint32, width float32, pts []PointF) {
	if len(pts) < 2 {
		return
	}
	pn := pen(argb, width)
	call(pGdipDrawLines, g, pn, uintptr(unsafe.Pointer(&pts[0])), uintptr(len(pts)))
	call(pGdipDeletePen, pn)
}

// GFillPolyGradient fills a polygon with a vertical gradient top->bottom.
func GFillPolyGradient(g uintptr, top, bottom uint32, y0, y1 float32, pts []PointF) {
	if len(pts) < 3 {
		return
	}
	var b uintptr
	p1 := PointF{0, y0 - 1}
	p2 := PointF{0, y1 + 1}
	call(pGdipCreateLineBrush, uintptr(unsafe.Pointer(&p1)), uintptr(unsafe.Pointer(&p2)), uintptr(top), uintptr(bottom), 0, uintptr(unsafe.Pointer(&b)))
	if b == 0 {
		b = brush(top)
	}
	call(pGdipFillPolygon, g, b, uintptr(unsafe.Pointer(&pts[0])), uintptr(len(pts)), 0)
	call(pGdipDeleteBrush, b)
}

func GFillPoly(g uintptr, argb uint32, pts []PointF) {
	if len(pts) < 3 {
		return
	}
	b := brush(argb)
	call(pGdipFillPolygon, g, b, uintptr(unsafe.Pointer(&pts[0])), uintptr(len(pts)), 0)
	call(pGdipDeleteBrush, b)
}

func GFillEllipse(g uintptr, argb uint32, x, y, w, h float32) {
	b := brush(argb)
	call(pGdipFillEllipse, g, b, f(x), f(y), f(w), f(h))
	call(pGdipDeleteBrush, b)
}

func GArc(g uintptr, argb uint32, width, x, y, w, h, start, sweep float32) {
	pn := pen(argb, width)
	call(pGdipDrawArc, g, pn, f(x), f(y), f(w), f(h), f(start), f(sweep))
	call(pGdipDeletePen, pn)
}
