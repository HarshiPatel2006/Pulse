//go:build windows

package app

import (
	"fmt"
	"math"

	"pulse/ui"
)

// btn draws a rounded button; style 0 = normal, 1 = accent, 2 = danger, 3 = ghost.
func (a *App) btn(c *ui.Canvas, x, y, w, h float32, id, label string, glyph rune, style int, enabled bool, click func()) {
	hov := a.hover == id && enabled
	prs := a.pressed == id && enabled
	bg, fg, border := uint32(ColPanel2), uint32(ColText), uint32(ColLine2)
	switch style {
	case 1:
		bg, fg, border = 0x2A2350, 0xE6E1FF, 0x4A3FA0
	case 2:
		bg, fg, border = 0x3A1A1E, 0xFFD6DA, 0x6E2A32
	case 3:
		bg, border = ColBG, ColBG
	}
	if hov {
		bg = ui.Mix(bg, 0xFFFFFF, 0.06)
		if style == 3 {
			bg = ColHover
		}
	}
	if prs {
		bg = ui.Mix(bg, 0x000000, 0.25)
	}
	if !enabled {
		fg = ColFaint
	}
	c.Round(x, y, w, h, 6, bg)
	if style != 3 {
		c.Border(x, y, w, h, 6, border, 1)
	}
	tx := x
	tw := w
	if glyph != 0 && label != "" {
		lw := c.TextW(label, ui.FontBody) + 16 + 8
		tx = x + (w-lw)/2
		c.Icon(tx, y, 16, h, glyph, 13, fg)
		c.Text(tx+24, y, lw, h, label, ui.FontBody, fg, ui.AlignLeft)
	} else if glyph != 0 {
		c.Icon(x, y, w, h, glyph, 13, fg)
	} else {
		c.Text(tx, y, tw, h, label, ui.FontBody, fg, ui.AlignCenter)
	}
	if enabled && click != nil {
		a.addZone(Zone{X: x, Y: y, W: w, H: h, ID: id, Click: click})
	}
}

// toggle draws an on/off switch.
func (a *App) toggle(c *ui.Canvas, x, y float32, id string, on bool, click func()) {
	w, h := float32(40), float32(20)
	bg := uint32(ColPanel2)
	if on {
		bg = ColAccent
	}
	if a.hover == id {
		bg = ui.Mix(bg, 0xFFFFFF, 0.08)
	}
	c.Round(x, y, w, h, 10, bg)
	if !on {
		c.Border(x, y, w, h, 10, ColLine2, 1)
	}
	kx := x + 4
	if on {
		kx = x + w - 16
	}
	knob := uint32(ColDim)
	if on {
		knob = 0xFFFFFF
	}
	c.Circle(kx+6, y+h/2, 6, knob)
	a.addZone(Zone{X: x, Y: y, W: w, H: h, ID: id, Click: click})
}

// segmented draws a choice group; returns nothing, calls pick(i).
func (a *App) segmented(c *ui.Canvas, x, y, h float32, id string, items []string, sel int, pick func(int)) float32 {
	widths := make([]float32, len(items))
	total := float32(0)
	for i, s := range items {
		widths[i] = c.TextW(s, ui.FontBody) + 24
		total += widths[i]
	}
	c.Round(x, y, total+4, h, 7, ColPanel2)
	c.Border(x, y, total+4, h, 7, ColLine, 1)
	cx := x + 2
	for i, s := range items {
		zid := fmt.Sprintf("%s%d", id, i)
		col := uint32(ColDim)
		if i == sel {
			c.Round(cx, y+2, widths[i], h-4, 5, 0x2A2350)
			col = 0xE6E1FF
		} else if a.hover == zid {
			c.Round(cx, y+2, widths[i], h-4, 5, ColHover)
			col = ColText
		}
		c.Text(cx, y, widths[i], h, s, ui.FontBody, col, ui.AlignCenter)
		ii := i
		a.addZone(Zone{X: cx, Y: y, W: widths[i], H: h, ID: zid, Click: func() { pick(ii) }})
		cx += widths[i]
	}
	return total + 4
}

// bar draws a horizontal meter.
func bar(c *ui.Canvas, x, y, w, h float32, frac float64, col uint32) {
	c.Round(x, y, w, h, h/2, 0x1D1D25)
	if frac > 0 {
		fw := float32(math.Min(1, frac)) * w
		if fw < h {
			fw = h
		}
		c.Round(x, y, fw, h, h/2, col)
	}
}

// card draws a panel background.
func card(c *ui.Canvas, x, y, w, h float32) {
	c.Round(x, y, w, h, 10, ColPanel)
	c.Border(x, y, w, h, 10, ColLine, 1)
}

// kv draws a label/value pair stacked (Task Manager style stats).
func kv(c *ui.Canvas, x, y, w float32, label, value string) {
	c.Text(x, y, w, 16, label, ui.FontSmall, ColDim, ui.AlignLeft)
	c.Text(x, y+16, w, 24, value, ui.Font{Size: 17, Weight: 600}, ColText, ui.AlignLeft)
}

func pct(v float64) string {
	if v < 0.05 {
		return "0%"
	}
	if v < 10 {
		return fmt.Sprintf("%.1f%%", v)
	}
	return fmt.Sprintf("%.0f%%", v)
}

func mb(v float64) string {
	switch {
	case v >= 1<<30:
		return fmt.Sprintf("%.1f GB", v/(1<<30))
	case v >= 1<<20:
		return fmt.Sprintf("%.1f MB", v/(1<<20))
	case v > 0:
		return fmt.Sprintf("%.0f KB", v/(1<<10))
	}
	return "0 MB"
}

func rate(v float64) string { // bytes/s
	switch {
	case v >= 1<<30:
		return fmt.Sprintf("%.1f GB/s", v/(1<<30))
	case v >= 1<<20:
		return fmt.Sprintf("%.1f MB/s", v/(1<<20))
	case v >= 1<<10:
		return fmt.Sprintf("%.0f KB/s", v/(1<<10))
	case v > 0:
		return "0.1 KB/s"
	}
	return "0 MB/s"
}

func bits(v float64) string { // bits/s
	switch {
	case v >= 1e9:
		return fmt.Sprintf("%.1f Gbps", v/1e9)
	case v >= 1e6:
		return fmt.Sprintf("%.1f Mbps", v/1e6)
	case v >= 1e3:
		return fmt.Sprintf("%.0f Kbps", v/1e3)
	}
	return fmt.Sprintf("%.0f bps", v)
}

func dur(ms uint64) string {
	s := ms / 1000
	return fmt.Sprintf("%d:%02d:%02d:%02d", s/86400, s/3600%24, s/60%60, s%60)
}

func cpuTime(t uint64) string { // 100ns
	s := t / 1e7
	return fmt.Sprintf("%d:%02d:%02d", s/3600, s/60%60, s%60)
}

// ringSeries adapts a value slice for charts.
type sliceSeries []float64

func (s sliceSeries) Len() int           { return len(s) }
func (s sliceSeries) At(i int) float64   { return s[i] }
