//go:build windows

package app

import (
	"fmt"

	"pulse/sys"
	"pulse/ui"
	"pulse/win"
)

const (
	compactW = 380
	compactH = 262
)

var vramPeak uint64

// drawCompact is the small widget (the original Pulse look).
func (a *App) drawCompact(c *ui.Canvas) {
	W, H := c.W, c.H
	s := &a.col.Snap
	c.Fill(0, 0, W, H, 0x0C0C0E)
	c.Border(0, 0, W, H, 0, ColLine, 1)
	// whole widget drags the window
	a.addZone(Zone{X: 0, Y: 0, W: W, H: H, Caption: true})

	// header
	dot := uint32(ColOK)
	if s.GPU.Name == "" {
		dot = 0x666666
	}
	c.Circle(20, 22, 4.5, dot)
	c.Text(32, 8, 60, 28, "PULSE", ui.Font{Size: 12.5, Weight: 700}, ColText, ui.AlignLeft)
	c.Text(80, 8, W-180, 28, "· "+shortGPU(s.GPU.Name), ui.Font{Size: 12.5, Mono: true}, ColDim, ui.AlignLeft)
	a.btn(c, W-102, 7, 30, 28, "c-graph", "", GPerf, 3, true, func() { a.set.ShowGraphs = !a.set.ShowGraphs; a.set.save() })
	a.btn(c, W-70, 7, 30, 28, "c-expand", "", GExpand, 3, true, a.toggleCompact)
	a.btn(c, W-38, 7, 30, 28, "c-close", "", GClose, 3, true, func() { win.PostMessage(a.hwnd, win.WM_CLOSE, 0, 0) })

	if s.GPU.Stat.MemUsed > vramPeak {
		vramPeak = s.GPU.Stat.MemUsed
	}
	gb := func(v uint64) float64 { return float64(v) / (1 << 30) }
	type meter struct {
		label string
		frac  float64
		val   string
		unit  string
		col   uint32
		hist  *sys.Ring
		max   float64
	}
	ms := []meter{
		{"VRAM", frac(s.GPU.Stat.MemUsed, s.GPU.Stat.MemTotal), fmt.Sprintf("%.1f", gb(s.GPU.Stat.MemUsed)), fmt.Sprintf("/%.1f GB", gb(s.GPU.Stat.MemTotal)), ColVRAM, &a.col.VRAMHist, float64(s.GPU.Stat.MemTotal)},
		{"RAM", s.Mem.Load / 100, fmt.Sprintf("%.1f", gb(s.Mem.Used)), fmt.Sprintf("/%.1f GB", gb(s.Mem.Total)), ColWarn, &a.col.MemHist, float64(s.Mem.Total)},
		{"GPU", float64(s.GPU.Stat.Util) / 100, fmt.Sprint(s.GPU.Stat.Util), "%", ColGPU, &a.col.GPUHist, 100},
		{"CPU", s.CPU.Total / 100, fmt.Sprintf("%.0f", s.CPU.Total), "%", ColCPU, &a.col.CPUHist, 100},
	}
	y := float32(44)
	rh := float32(36)
	for _, m := range ms {
		c.Text(16, y, 60, rh, m.label, ui.Font{Size: 13, Mono: true}, 0xA5A5AA, ui.AlignLeft)
		bx, bw := float32(70), W-70-118
		if a.set.ShowGraphs {
			c.Round(bx, y+5, bw, rh-10, 5, 0x15151A)
			c.Clip(bx, y+5, bw, rh-10)
			c.Chart(bx, y+5, bw, rh-10, m.hist, sys.HistLen, m.max, m.col, false, 120)
			c.Unclip()
		} else {
			bar(c, bx, y+rh/2-6, bw, 12, m.frac, m.col)
		}
		if m.label == "VRAM" && s.GPU.Stat.MemTotal > 0 && !a.set.ShowGraphs {
			px := bx + bw*float32(frac(vramPeak, s.GPU.Stat.MemTotal))
			c.Fill(px-1, y+rh/2-9, 2.5, 18, 0xFFD9CC)
		}
		vw := c.TextW(m.unit, ui.Font{Size: 11, Mono: true})
		c.Text(W-16-vw, y, vw+2, rh, m.unit, ui.Font{Size: 11, Mono: true}, ColDim, ui.AlignLeft)
		c.Text(W-120, y, 104-vw, rh, m.val, ui.Font{Size: 15, Weight: 700, Mono: true}, ColText, ui.AlignRight)
		y += rh
	}
	// temperatures
	c.Text(16, y, 60, rh, "TEMP", ui.Font{Size: 13, Mono: true}, 0xA5A5AA, ui.AlignLeft)
	half := (W - 70 - 16) / 2
	tempCell := func(x float32, label string, v float64, ok bool) {
		c.Text(x, y, 34, rh, label, ui.Font{Size: 10.5, Mono: true}, ColFaint, ui.AlignLeft)
		if !ok {
			c.Text(x+34, y, half-40, rh, "n/a", ui.Font{Size: 11, Mono: true}, ColFaint, ui.AlignLeft)
			return
		}
		bar(c, x+34, y+rh/2-5, half-96, 10, v/100, tempColor(v))
		c.Text(x+half-60, y, 54, rh, fmt.Sprintf("%.0f°C", v), ui.Font{Size: 14, Weight: 700, Mono: true}, ColText, ui.AlignRight)
	}
	tempCell(70, "GPU", float64(s.GPU.Stat.Temp), s.GPU.Stat.HasTemp)
	tempCell(70+half+8, "CPU", s.CPU.Temp, s.CPU.TempOK)
	y += rh + 4
	// footer
	foot := fmt.Sprintf("PWR %.0f W   PEAK %.1f GB", s.GPU.Stat.PowerW, gb(vramPeak))
	c.Text(16, y, W-32, 24, foot, ui.Font{Size: 12, Mono: true}, ColDim, ui.AlignLeft)
	a.addZone(Zone{X: 16, Y: y, W: 230, H: 24, ID: "c-peak", Click: func() { vramPeak = 0 }})
	c.Text(16, y, W-32, 24, fmt.Sprintf("%s", mb(float64(s.SelfWS))), ui.Font{Size: 11, Mono: true}, ColFaint, ui.AlignRight)
}

func frac(a, b uint64) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}
