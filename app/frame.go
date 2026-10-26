//go:build windows

package app

import (
	"fmt"
	"time"

	"pulse/ui"
	"pulse/win"
)

const (
	titleH  = 48
	statusH = 28
)

var pageColors = []uint32{ColCPU, ColGPU, ColNet, ColDisk, ColMem, ColPower, 0x5C9DFF, ColAccent, ColDim}

func (a *App) navW() float32 {
	if a.set.NavCollapsed {
		return 56
	}
	return 212
}

func (a *App) drawFrame(c *ui.Canvas) {
	W, H := c.W, c.H
	c.Fill(0, 0, W, H, ColBG)
	nav := a.navW()

	// ---- nav rail ----
	c.Fill(0, titleH, nav, H-titleH-statusH, ColBG)
	y := float32(titleH + 6)
	for i := range pageNames {
		if i == PSettings {
			continue
		}
		a.navItem(c, i, 6, y, nav-12)
		y += 42
	}
	a.navItem(c, PSettings, 6, H-statusH-48, nav-12)

	// ---- content ----
	cx := nav
	c.Round(cx, titleH, W-cx-6, H-titleH-statusH, 12, 0x0E0E12)
	c.Border(cx, titleH, W-cx-6, H-titleH-statusH, 12, ColLine, 1)
	px, py, pw, ph := cx+20, float32(titleH+14), W-cx-46, H-titleH-statusH-26
	c.Clip(cx+1, titleH+1, W-cx-8, H-titleH-statusH-2)
	if p := a.pages[a.set.Page]; p != nil {
		p.draw(a, c, px, py, pw, ph)
	}
	c.Unclip()

	// ---- title bar (drawn last so it sits on top) ----
	a.drawTitle(c, W)

	// ---- status bar ----
	a.drawStatus(c, W, H)

	// ---- toast ----
	if a.toast != "" && time.Since(a.toastAt) < 3*time.Second {
		tw := c.TextW(a.toast, ui.FontBody) + 32
		tx, ty := (W-tw)/2, H-statusH-56
		c.Round(tx, ty, tw, 34, 17, 0x232336)
		c.Border(tx, ty, tw, 34, 17, 0x4A3FA0, 1)
		c.Text(tx, ty, tw, 34, a.toast, ui.FontBody, ColText, ui.AlignCenter)
	}
}

func (a *App) navItem(c *ui.Canvas, i int, x, y, w float32) {
	id := fmt.Sprintf("nav%d", i)
	sel := a.set.Page == i
	h := float32(38)
	col := pageColors[i]
	if sel {
		c.Round(x, y, w, h, 8, ColSel)
		c.Round(x, y+10, 3, h-20, 1.5, col)
	} else if a.hover == id {
		c.Round(x, y, w, h, 8, ColHover)
	}
	icol := uint32(ColDim)
	if sel {
		icol = col
	}
	c.Icon(x+8, y, 28, h, pageGlyphs[i], 15, icol)
	if !a.set.NavCollapsed {
		tcol := uint32(ColDim)
		if sel {
			tcol = ColText
		}
		f := ui.FontBody
		if sel {
			f = ui.FontSemi
		}
		c.Text(x+44, y, w-48, h, pageNames[i], f, tcol, ui.AlignLeft)
	}
	ii := i
	a.addZone(Zone{X: x, Y: y, W: w, H: h, ID: id, Click: func() { a.goPage(ii) }})
}

// logo draws the Pulse heartbeat mark.
func logo(c *ui.Canvas, x, y float32) {
	c.Round(x, y, 26, 26, 7, 0x1B1630)
	pts := []float32{3, 14, 8, 14, 10.5, 9, 13.5, 20, 16.5, 5, 19, 14, 23, 14}
	for i := 0; i+3 < len(pts); i += 2 {
		t := float32(i) / float32(len(pts))
		c.Line(x+pts[i], y+pts[i+1], x+pts[i+2], y+pts[i+3], ui.Mix(ColAccent2, ColAccent, t*1.4), 1.8)
	}
}

func (a *App) drawTitle(c *ui.Canvas, W float32) {
	c.Fill(0, 0, W, titleH, ColBG)
	a.addZone(Zone{X: 0, Y: 0, W: W, H: titleH, Caption: true})
	// hamburger
	a.btn(c, 10, 9, 36, 30, "hamb", "", GMenu, 3, true, func() {
		a.set.NavCollapsed = !a.set.NavCollapsed
		a.set.save()
	})
	logo(c, 54, 11)
	c.Text(88, 0, 80, titleH, "Pulse", ui.Font{Size: 15, Weight: 700}, ColText, ui.AlignLeft)
	if win.IsAdmin() {
		c.Text(140, 0, 90, titleH, "Administrator", ui.FontSmall, ColFaint, ui.AlignLeft)
	}

	// search box
	sw := W * 0.36
	if sw > 460 {
		sw = 460
	}
	if sw < 220 {
		sw = 220
	}
	sx := (W - sw) / 2
	if sx < 240 {
		sx = 240
	}
	bg := uint32(ColPanel)
	border := uint32(ColLine2)
	if a.searchFocus {
		border = ColAccent
		bg = ColPanel2
	} else if a.hover == "search" {
		bg = ColPanel2
	}
	c.Round(sx, 8, sw, 32, 8, bg)
	c.Border(sx, 8, sw, 32, 8, border, 1)
	c.Icon(sx+6, 8, 26, 32, GSearch, 12, ColDim)
	txt, col := a.search, uint32(ColText)
	if txt == "" && !a.searchFocus {
		txt, col = "Type a name, publisher, or PID to search", ColFaint
	}
	if a.searchFocus && time.Now().UnixMilli()/530%2 == 0 {
		txt += "|"
	}
	c.Text(sx+34, 8, sw-60, 32, txt, ui.FontBody, col, ui.AlignLeft)
	a.addZone(Zone{X: sx, Y: 8, W: sw, H: 32, ID: "search", Cursor: a.cursorIBeam, Click: func() { a.searchFocus = true }})
	if a.search != "" {
		a.btn(c, sx+sw-30, 12, 24, 24, "searchx", "", GClose, 3, true, func() { a.search = "" })
	}
	if a.searchFocus {
		win.SetTimer(a.hwnd, timerAnim, 530)
	}

	// window buttons
	bw := float32(46)
	x := W - bw
	a.capBtn(c, x, "close", GClose, true, func() { win.PostMessage(a.hwnd, win.WM_CLOSE, 0, 0) })
	x -= bw
	g := rune(GMax)
	if win.IsZoomed(a.hwnd) {
		g = GRestore
	}
	a.capBtn(c, x, "max", g, false, func() {
		if win.IsZoomed(a.hwnd) {
			win.ShowWindow(a.hwnd, win.SW_RESTORE)
		} else {
			win.ShowWindow(a.hwnd, win.SW_MAXIMIZE)
		}
	})
	x -= bw
	a.capBtn(c, x, "min", GMin, false, func() { win.ShowWindow(a.hwnd, win.SW_MINIMIZE) })
	x -= bw + 8
	pin := rune(GPin)
	if a.set.OnTop {
		pin = GPinned
	}
	a.btn(c, x, 9, 36, 30, "ontop", "", pin, 3, true, func() { a.setOnTop(!a.set.OnTop) })
	if a.set.OnTop {
		c.Round(x+14, 38, 8, 2, 1, ColAccent)
	}
	x -= 40
	a.btn(c, x, 9, 36, 30, "compact", "", GCompact, 3, true, a.toggleCompact)
	if a.hover == "compact" {
		a.tip(c, x-60, titleH, "Compact widget")
	} else if a.hover == "ontop" {
		a.tip(c, x-20, titleH, "Always on top")
	}
}

func (a *App) tip(c *ui.Canvas, x, y float32, s string) {
	w := c.TextW(s, ui.FontSmall) + 16
	c.Round(x, y, w, 24, 5, 0x24242E)
	c.Text(x, y, w, 24, s, ui.FontSmall, ColText, ui.AlignCenter)
}

func (a *App) capBtn(c *ui.Canvas, x float32, id string, g rune, danger bool, click func()) {
	if a.hover == id {
		bg := uint32(ColHover)
		if danger {
			bg = 0xC42B1C
		}
		c.Fill(x, 0, 46, 36, bg)
	}
	c.Icon(x, 0, 46, 36, g, 10, ColText)
	a.addZone(Zone{X: x, Y: 0, W: 46, H: 36, ID: id, Click: click})
}

func (a *App) drawStatus(c *ui.Canvas, W, H float32) {
	y := H - statusH
	s := &a.col.Snap
	items := []struct {
		label, val string
		col        uint32
	}{
		{"Processes", fmt.Sprint(len(s.Procs)), ColText},
		{"CPU", pct(s.CPU.Total), ColCPU},
		{"Memory", pct(s.Mem.Load), ColMem},
	}
	if s.GPU.Name != "" {
		items = append(items, struct {
			label, val string
			col        uint32
		}{"GPU", fmt.Sprintf("%d%%", s.GPU.Stat.Util), ColGPU})
		if s.GPU.Stat.MemTotal > 0 {
			items = append(items, struct {
				label, val string
				col        uint32
			}{"VRAM", fmt.Sprintf("%.1f / %.1f GB", float64(s.GPU.Stat.MemUsed)/(1<<30), float64(s.GPU.Stat.MemTotal)/(1<<30)), ColVRAM})
		}
	}
	x := float32(16)
	for _, it := range items {
		c.Circle(x+3, y+statusH/2, 3, it.col)
		x += 12
		lw := c.TextW(it.label, ui.FontSmall)
		c.Text(x, y, lw+4, statusH, it.label, ui.FontSmall, ColDim, ui.AlignLeft)
		x += lw + 6
		vw := c.TextW(it.val, ui.FontSmall) + 4
		c.Text(x, y, vw, statusH, it.val, ui.Font{Size: 11.5, Weight: 600}, ColText, ui.AlignLeft)
		x += vw + 18
	}
	selfMem := s.SelfPriv
	if selfMem == 0 {
		selfMem = s.SelfWS
	}
	self := fmt.Sprintf("Pulse uses %s RAM", mb(float64(selfMem)))
	sp := "Updating every " + speedName(a.set.SpeedMs)
	if a.set.SpeedMs <= 0 {
		sp = "Paused"
	}
	tw := c.TextW(self, ui.FontSmall)
	c.Icon(W-tw-40, y, 20, statusH, GHeart, 10, ColAccent)
	c.Text(W-tw-20, y, tw+4, statusH, self, ui.FontSmall, ColDim, ui.AlignLeft)
	sw := c.TextW(sp, ui.FontSmall)
	c.Text(W-tw-sw-64, y, sw+4, statusH, sp, ui.FontSmall, ColFaint, ui.AlignLeft)
}

func speedName(ms int) string {
	switch {
	case ms <= 0:
		return "paused"
	case ms < 1000:
		return fmt.Sprintf("%.1f s", float64(ms)/1000)
	}
	return fmt.Sprintf("%d s", ms/1000)
}

// pageHeader draws the title + returns y below it.
func (a *App) pageHeader(c *ui.Canvas, x, y, w float32, title, sub string) float32 {
	c.Text(x, y, w, 30, title, ui.FontTitle, ColText, ui.AlignLeft)
	if sub != "" {
		tw := c.TextW(title, ui.FontTitle)
		c.Text(x+tw+12, y+4, w-tw-12, 30, sub, ui.FontBody, ColFaint, ui.AlignLeft)
	}
	return y + 44
}
