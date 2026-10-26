//go:build windows

package app

import (
	"fmt"
	"strings"

	"pulse/sys"
	"pulse/ui"
	"pulse/win"
)

type sysinfoPage struct{ scroll float32 }

func (sp *sysinfoPage) draw(a *App, c *ui.Canvas, x, y, w, h float32) {
	st := a.static
	y2 := a.pageHeader(c, x, y, w, "System info", "")
	if st == nil {
		c.Text(x, y2, w, 30, "Reading system information…", ui.FontBody, ColDim, ui.AlignLeft)
		return
	}
	s := &a.col.Snap
	a.btn(c, x+w-130, y, 130, 32, "sicopy", "Copy all", GDetails, 0, true, func() {
		var b strings.Builder
		for _, sec := range st.Sections {
			b.WriteString("== " + sec.Title + " ==\r\n")
			for _, r := range sec.Rows {
				b.WriteString(strings.TrimSpace(r.K) + ": " + r.V + "\r\n")
			}
			b.WriteString("\r\n")
		}
		win.CopyToClipboard(a.hwnd, b.String())
		a.showToast("System info copied to clipboard")
	})

	view := h - (y2 - y)
	c.Clip(x-4, y2, w+8, view)
	a.addZone(Zone{X: x, Y: y2, W: w, H: view, Wheel: func(d float32) { sp.scroll -= d * 60 }})
	cy := y2 - sp.scroll

	// hero tiles
	tiles := []struct {
		label, value, sub string
		col               uint32
		glyph             rune
	}{
		{"Windows", st.OSName, st.OSVersion + " · build " + st.OSBuild, ColAccent, 0xE7F8},
		{"Processor", short(st.CPUName, 34), fmt.Sprintf("%d cores · %d threads", st.Cores, st.Logical), ColCPU, 0xE950},
		{"Memory", fmt.Sprintf("%.0f GB %s", float64(s.Mem.Total)/(1<<30)+0.4, st.MemType), speedText(st.MemSpeed), ColMem, 0xE964},
		{"Graphics", short(s.GPU.Name, 34), fmt.Sprintf("%.0f GB VRAM", float64(s.GPU.Stat.MemTotal)/(1<<30)+0.4), ColGPU, 0xE7F4},
	}
	tw := (w - 3*12) / 4
	for i, t := range tiles {
		tx := x + float32(i)*(tw+12)
		c.Round(tx, cy, tw, 96, 12, ui.Mix(ColPanel, t.col, 0.08))
		c.Border(tx, cy, tw, 96, 12, ui.Mix(ColLine, t.col, 0.35), 1)
		c.Round(tx+14, cy+14, 32, 32, 8, ui.Mix(ColBG, t.col, 0.22))
		c.Icon(tx+14, cy+14, 32, 32, t.glyph, 15, t.col)
		c.Text(tx+56, cy+12, tw-64, 18, t.label, ui.FontSmall, ColDim, ui.AlignLeft)
		c.Text(tx+56, cy+28, tw-64, 22, t.value, ui.FontSemi, ColText, ui.AlignLeft)
		c.Text(tx+14, cy+60, tw-28, 22, t.sub, ui.FontSmall, t.col, ui.AlignLeft)
	}
	cy += 112

	// live strip
	up := dur(s.CPU.Uptime)
	c.Text(x, cy, w, 20, "Up time "+up+"   ·   "+fmt.Sprintf("%d processes, %d threads, %d handles", s.CPU.Procs, s.CPU.Threads, s.CPU.Handles), ui.FontSmall, ColDim, ui.AlignLeft)
	cy += 30

	// section cards in two columns
	colW := (w - 14) / 2
	ys := [2]float32{cy, cy}
	for _, sec := range st.Sections {
		col := 0
		if ys[1] < ys[0] {
			col = 1
		}
		sx := x + float32(col)*(colW+14)
		sh := float32(48 + 26*len(sec.Rows))
		if len(sec.Rows) == 0 {
			sh = 78
		}
		card(c, sx, ys[col], colW, sh)
		c.Icon(sx+14, ys[col]+12, 24, 24, sec.Icon, 14, ColAccent2)
		c.Text(sx+44, ys[col]+12, colW-60, 24, sec.Title, ui.FontH2, ColText, ui.AlignLeft)
		ry := ys[col] + 44
		for _, r := range sec.Rows {
			k := strings.TrimLeft(r.K, " ")
			ind := float32(len(r.K)-len(k)) * 6
			c.Text(sx+16+ind, ry, colW*0.38-ind, 24, k, ui.FontBody, ColDim, ui.AlignLeft)
			c.Text(sx+colW*0.38+8, ry, colW*0.62-24, 24, r.V, ui.FontBody, ColText, ui.AlignLeft)
			ry += 26
		}
		if len(sec.Rows) == 0 {
			c.Text(sx+16, ry, colW, 24, "Not available", ui.FontBody, ColFaint, ui.AlignLeft)
		}
		ys[col] += sh + 14
	}
	c.Unclip()
	total := max(ys[0], ys[1]) + sp.scroll - y2
	if sp.scroll > total-view {
		sp.scroll = total - view
	}
	if sp.scroll < 0 {
		sp.scroll = 0
	}
}

func speedText(mts int) string {
	if mts == 0 {
		return ""
	}
	return fmt.Sprintf("%d MT/s", mts)
}

func short(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len([]rune(s)) > n {
		return string([]rune(s)[:n-1]) + "…"
	}
	return s
}

var _ = sys.FmtBytes
