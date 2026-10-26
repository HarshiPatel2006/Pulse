//go:build windows

package app

import (
	"fmt"
	"math"
	"os"
	"strings"

	"pulse/sys"
	"pulse/ui"
)

type perfPage struct {
	sel    string // "cpu", "mem", "disk:<key>", "net:<key>", "gpu", "temp"
}

type device struct {
	id, title, sub, value string
	col                   uint32
	series                ui.Series
	max                   float64
	series2               ui.Series
	col2                  uint32
}

// niceMax rounds up to 1/2/5 x 10^n for auto-scaled charts.
func niceMax(v float64) float64 {
	if v <= 0 {
		return 1
	}
	e := math.Pow(10, math.Floor(math.Log10(v)))
	for _, m := range []float64{1, 2, 5, 10} {
		if v <= m*e {
			return m * e
		}
	}
	return 10 * e
}

func maxOf(rs ...*sys.Ring) float64 {
	m := 0.0
	for _, r := range rs {
		if v := r.Max(); v > m {
			m = v
		}
	}
	return m
}

func (pp *perfPage) devices(a *App) []device {
	s := &a.col.Snap
	col := a.col
	var out []device
	out = append(out, device{id: "cpu", title: "CPU", sub: fmt.Sprintf("%s  %.2f GHz", pct(s.CPU.Total), s.CPU.MHz/1000), col: ColCPU, series: &col.CPUHist, max: 100})
	out = append(out, device{id: "mem", title: "Memory", sub: fmt.Sprintf("%.1f/%.1f GB (%.0f%%)", float64(s.Mem.Used)/(1<<30), float64(s.Mem.Total)/(1<<30), s.Mem.Load), col: ColMem, series: &col.MemHist, max: float64(s.Mem.Total)})
	for _, d := range s.Disks {
		out = append(out, device{id: "disk:" + d.Key, title: d.Label, sub: "Active " + pct(d.Active), col: ColDisk, series: d.Hist, max: 100})
	}
	for _, n := range s.Nets {
		m := niceMax(maxOf(n.SendHist, n.RecvHist))
		out = append(out, device{id: "net:" + n.Key, title: n.Label, sub: fmt.Sprintf("S: %s  R: %s", bits(n.SendBps), bits(n.RecvBps)), col: ColNet, series: n.RecvHist, series2: n.SendHist, col2: 0xFFD166, max: m})
	}
	if s.GPU.Name != "" {
		out = append(out, device{id: "gpu", title: "GPU 0", sub: fmt.Sprintf("%s  %d%%", shortGPU(s.GPU.Name), s.GPU.Stat.Util), col: ColGPU, series: &col.GPUHist, max: 100})
	}
	tsub := ""
	if s.GPU.Stat.HasTemp {
		tsub = fmt.Sprintf("GPU %d°C", s.GPU.Stat.Temp)
	}
	if s.CPU.TempOK {
		tsub += fmt.Sprintf("  CPU %.0f°C", s.CPU.Temp)
	}
	if tsub != "" {
		out = append(out, device{id: "temp", title: "Temperatures", sub: strings.TrimSpace(tsub), col: ColTemp, series: &col.GTempHist, series2: &col.CTempHist, col2: ColCPU, max: 100})
	}
	return out
}

func shortGPU(n string) string {
	n = strings.TrimPrefix(n, "NVIDIA ")
	n = strings.TrimPrefix(n, "GeForce ")
	return n
}

func (pp *perfPage) draw(a *App, c *ui.Canvas, x, y, w, h float32) {
	devs := pp.devices(a)
	if pp.sel == "" {
		pp.sel = os.Getenv("PULSE_PERF")
		if pp.sel == "" {
			pp.sel = "cpu"
		}
	}
	found := false
	for _, d := range devs {
		if d.id == pp.sel {
			found = true
		}
	}
	cur := pp.sel
	if !found {
		cur = "cpu" // device not present (yet) - show CPU but remember the choice
	}
	y0 := a.pageHeader(c, x, y, w, "Performance", "")

	// left device list
	lw := float32(236)
	ly := y0
	for _, d := range devs {
		dd := d
		id := "dev-" + d.id
		sel := d.id == cur
		bg := uint32(ColBG)
		if sel {
			bg = ColSel
		} else if a.hover == id {
			bg = ColHover
		}
		c.Round(x-6, ly, lw, 62, 9, bg)
		if sel {
			c.Round(x-6, ly+14, 3, 34, 1.5, d.col)
		}
		// mini chart
		c.Round(x+4, ly+9, 72, 44, 5, 0x0A0A0D)
		c.Border(x+4, ly+9, 72, 44, 5, ui.Mix(d.col, ColBG, 0.6), 1)
		c.Clip(x+5, ly+10, 70, 42)
		if d.series2 != nil {
			c.Chart(x+5, ly+10, 70, 42, d.series2, sys.HistLen, d.max, d.col2, false, 0)
		}
		c.Chart(x+5, ly+10, 70, 42, d.series, sys.HistLen, d.max, d.col, false, 90)
		c.Unclip()
		c.Text(x+86, ly+10, lw-96, 22, d.title, ui.FontSemi, ColText, ui.AlignLeft)
		c.Text(x+86, ly+31, lw-96, 20, d.sub, ui.FontSmall, ColDim, ui.AlignLeft)
		a.addZone(Zone{X: x - 6, Y: ly, W: lw, H: 62, ID: id, Click: func() { pp.sel = dd.id }})
		ly += 66
	}

	// right detail panel
	rx := x + lw + 14
	rw := w - lw - 14
	card(c, rx, y0, rw, h-(y0-y))
	px, py, pw := rx+20, y0+16, rw-40
	ph := h - (y0 - y) - 32
	switch sel := cur; {
	case sel == "cpu":
		pp.drawCPU(a, c, px, py, pw, ph)
	case sel == "mem":
		pp.drawMem(a, c, px, py, pw, ph)
	case strings.HasPrefix(sel, "disk:"):
		pp.drawDisk(a, c, px, py, pw, ph, strings.TrimPrefix(sel, "disk:"))
	case strings.HasPrefix(sel, "net:"):
		pp.drawNet(a, c, px, py, pw, ph, strings.TrimPrefix(sel, "net:"))
	case sel == "gpu":
		pp.drawGPU(a, c, px, py, pw, ph)
	case sel == "temp":
		pp.drawTemp(a, c, px, py, pw, ph)
	}
}

func detailHead(c *ui.Canvas, x, y, w float32, title, sub string, col uint32) float32 {
	c.Text(x, y, w, 34, title, ui.FontBig, ColText, ui.AlignLeft)
	c.Text(x, y, w, 34, sub, ui.FontH2, col, ui.AlignRight)
	return y + 44
}

func chartBox(c *ui.Canvas, x, y, w, h float32, label, maxLabel string, col uint32) {
	c.Text(x, y, w, 18, label, ui.FontSmall, ColDim, ui.AlignLeft)
	c.Text(x, y, w, 18, maxLabel, ui.FontSmall, ColDim, ui.AlignRight)
	c.Fill(x, y+20, w, h-20, 0x0A0A0D)
	c.Border(x, y+20, w, h-20, 2, ui.Mix(col, ColBG, 0.55), 1)
}

// statsGrid draws label/value pairs in columns.
func statsGrid(c *ui.Canvas, x, y, w float32, cols int, items [][2]string) float32 {
	cw := w / float32(cols)
	for i, it := range items {
		cx := x + float32(i%cols)*cw
		cy := y + float32(i/cols)*48
		kv(c, cx, cy, cw-10, it[0], it[1])
	}
	return y + float32((len(items)+cols-1)/cols)*48
}

// infoList draws small "key   value" rows (Task Manager's right-hand static info).
func infoList(c *ui.Canvas, x, y, w float32, items [][2]string) {
	for i, it := range items {
		yy := y + float32(i)*22
		c.Text(x, yy, w*0.55, 22, it[0], ui.FontSmall, ColDim, ui.AlignLeft)
		c.Text(x+w*0.55, yy, w*0.45, 22, it[1], ui.FontSmall, ColText, ui.AlignLeft)
	}
}

func (pp *perfPage) drawCPU(a *App, c *ui.Canvas, x, y, w, h float32) {
	s := &a.col.Snap
	name := ""
	if a.static != nil {
		name = a.static.CPUName
	}
	y = detailHead(c, x, y, w, "CPU", name, ColCPU)
	// mode switch
	mode := 0
	if a.set.CPULogical {
		mode = 1
	}
	a.segmented(c, x, y, 28, "cpumode", []string{"Overall utilization", "Logical processors"}, mode, func(i int) {
		a.set.CPULogical = i == 1
		a.set.save()
	})
	y += 40
	chartH := h - 44 - 40 - 220
	if chartH < 120 {
		chartH = 120
	}
	if a.set.CPULogical && len(a.col.CoreHist) > 0 {
		n := len(a.col.CoreHist)
		cols := int(math.Ceil(math.Sqrt(float64(n) * float64(w) / float64(chartH))))
		if cols < 1 {
			cols = 1
		}
		rows := (n + cols - 1) / cols
		gw := (w - float32(cols-1)*6) / float32(cols)
		gh := (chartH - float32(rows-1)*6) / float32(rows)
		for i, r := range a.col.CoreHist {
			gx := x + float32(i%cols)*(gw+6)
			gy := y + float32(i/cols)*(gh+6)
			c.Fill(gx, gy, gw, gh, 0x0A0A0D)
			c.Border(gx, gy, gw, gh, 2, ui.Mix(ColCPU, ColBG, 0.6), 1)
			c.Clip(gx+1, gy+1, gw-2, gh-2)
			c.Chart(gx+1, gy+1, gw-2, gh-2, r, sys.HistLen, 100, ColCPU, false, 80)
			c.Unclip()
		}
	} else {
		chartBox(c, x, y, w, chartH, "% Utilization over 60 seconds", "100%", ColCPU)
		c.Clip(x+1, y+21, w-2, chartH-22)
		c.Chart(x+1, y+21, w-2, chartH-22, &a.col.CPUHist, sys.HistLen, 100, ColCPU, true, 110)
		c.Unclip()
	}
	y += chartH + 16
	left := [][2]string{
		{"Utilization", pct(s.CPU.Total)},
		{"Speed", fmt.Sprintf("%.2f GHz", s.CPU.MHz/1000)},
		{"Processes", fmt.Sprint(s.CPU.Procs)},
		{"Threads", fmt.Sprint(s.CPU.Threads)},
		{"Handles", fmt.Sprint(s.CPU.Handles)},
		{"Up time", dur(s.CPU.Uptime)},
	}
	if s.CPU.TempOK {
		left = append(left, [2]string{"Temperature", fmt.Sprintf("%.0f °C", s.CPU.Temp)})
	}
	statsGrid(c, x, y, w*0.62, 3, left)
	if st := a.static; st != nil {
		infoList(c, x+w*0.64, y, w*0.36, [][2]string{
			{"Base speed:", fmt.Sprintf("%.2f GHz", float64(st.BaseMHz)/1000)},
			{"Sockets:", fmt.Sprint(st.Sockets)},
			{"Cores:", fmt.Sprint(st.Cores)},
			{"Logical processors:", fmt.Sprint(st.Logical)},
			{"L1 cache:", mb(float64(st.L1))},
			{"L2 cache:", mb(float64(st.L2))},
			{"L3 cache:", mb(float64(st.L3))},
		})
	}
}

func (pp *perfPage) drawMem(a *App, c *ui.Canvas, x, y, w, h float32) {
	s := &a.col.Snap
	m := s.Mem
	y = detailHead(c, x, y, w, "Memory", fmt.Sprintf("%.1f GB", float64(m.Total)/(1<<30)), ColMem)
	chartH := h - 44 - 260
	if chartH < 120 {
		chartH = 120
	}
	chartBox(c, x, y, w, chartH+20, "Memory usage", fmt.Sprintf("%.1f GB", float64(m.Total)/(1<<30)), ColMem)
	c.Clip(x+1, y+21, w-2, chartH-2)
	c.Chart(x+1, y+21, w-2, chartH-2, &a.col.MemHist, sys.HistLen, float64(m.Total), ColMem, true, 110)
	c.Unclip()
	y += chartH + 30
	// composition bar
	c.Text(x, y, w, 18, "Memory composition", ui.FontSmall, ColDim, ui.AlignLeft)
	y += 20
	inUse := float64(m.Used - minU(m.Modified, m.Used))
	parts := []struct {
		v   float64
		col uint32
		n   string
	}{
		{inUse, ColMem, "In use"},
		{float64(m.Modified), 0xE87CC8, "Modified"},
		{float64(m.Standby), 0x5C6BC0, "Standby"},
		{float64(m.Free), 0x2A2A36, "Free"},
	}
	bx := x
	tot := float64(m.Total)
	for _, p := range parts {
		pw := float32(p.v/tot) * w
		if pw > 1 {
			c.Fill(bx, y, pw, 22, p.col)
			c.Fill(bx+pw-1, y, 1, 22, ColPanel)
		}
		bx += pw
	}
	c.Border(x, y, w, 22, 2, ColLine2, 1)
	y += 30
	lx := x
	for _, p := range parts {
		c.Round(lx, y+4, 10, 10, 2, p.col)
		t := fmt.Sprintf("%s %s", p.n, mb(p.v))
		c.Text(lx+14, y, 160, 18, t, ui.FontSmall, ColDim, ui.AlignLeft)
		lx += c.TextW(t, ui.FontSmall) + 34
	}
	y += 30
	statsGrid(c, x, y, w*0.62, 3, [][2]string{
		{"In use", mb(float64(m.Used))},
		{"Available", mb(float64(m.Avail))},
		{"Committed", fmt.Sprintf("%.1f/%.1f GB", float64(m.Commit)/(1<<30), float64(m.CommitLimit)/(1<<30))},
		{"Cached", mb(float64(m.Cached))},
		{"Paged pool", mb(float64(m.Paged))},
		{"Non-paged pool", mb(float64(m.NonPaged))},
	})
	if st := a.static; st != nil {
		items := [][2]string{}
		if st.MemSpeed > 0 {
			items = append(items, [2]string{"Speed:", fmt.Sprintf("%d MT/s", st.MemSpeed)})
		}
		if st.SlotsTotal > 0 {
			items = append(items, [2]string{"Slots used:", fmt.Sprintf("%d of %d", st.SlotsUsed, st.SlotsTotal)})
		}
		if st.MemType != "" {
			items = append(items, [2]string{"Type:", st.MemType})
		}
		infoList(c, x+w*0.64, y, w*0.36, items)
	}
}

func minU(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}

func (pp *perfPage) drawDisk(a *App, c *ui.Canvas, x, y, w, h float32, key string) {
	var d *sys.DiskInfo
	for _, dd := range a.col.Snap.Disks {
		if dd.Key == key {
			d = dd
		}
	}
	if d == nil {
		return
	}
	y = detailHead(c, x, y, w, d.Label, "", ColDisk)
	ch := (h - 44 - 150) / 2
	if ch < 90 {
		ch = 90
	}
	chartBox(c, x, y, w, ch, "Active time", "100%", ColDisk)
	c.Clip(x+1, y+21, w-2, ch-22)
	c.Chart(x+1, y+21, w-2, ch-22, d.Hist, sys.HistLen, 100, ColDisk, true, 110)
	c.Unclip()
	y += ch + 12
	mx := niceMax(maxOf(d.ReadHist, d.WriteHist))
	chartBox(c, x, y, w, ch, "Disk transfer rate  (read solid · write yellow)", rate(mx), ColDisk)
	c.Clip(x+1, y+21, w-2, ch-22)
	c.Chart(x+1, y+21, w-2, ch-22, d.WriteHist, sys.HistLen, mx, ColPower, true, 0)
	c.Chart(x+1, y+21, w-2, ch-22, d.ReadHist, sys.HistLen, mx, ColDisk, false, 90)
	c.Unclip()
	y += ch + 16
	statsGrid(c, x, y, w, 4, [][2]string{
		{"Active time", pct(d.Active)},
		{"Average response time", fmt.Sprintf("%.1f ms", d.RespMs)},
		{"Read speed", rate(d.ReadBps)},
		{"Write speed", rate(d.WriteBps)},
	})
}

func (pp *perfPage) drawNet(a *App, c *ui.Canvas, x, y, w, h float32, key string) {
	var n *sys.NetInfo
	for _, nn := range a.col.Snap.Nets {
		if nn.Key == key {
			n = nn
		}
	}
	if n == nil {
		return
	}
	y = detailHead(c, x, y, w, n.Label, n.Desc, ColNet)
	ch := h - 44 - 170
	if ch < 120 {
		ch = 120
	}
	mx := niceMax(maxOf(n.SendHist, n.RecvHist))
	chartBox(c, x, y, w, ch, "Throughput  (receive orange · send yellow)", bits(mx), ColNet)
	c.Clip(x+1, y+21, w-2, ch-22)
	c.Chart(x+1, y+21, w-2, ch-22, n.SendHist, sys.HistLen, mx, ColPower, true, 0)
	c.Chart(x+1, y+21, w-2, ch-22, n.RecvHist, sys.HistLen, mx, ColNet, false, 100)
	c.Unclip()
	y += ch + 16
	statsGrid(c, x, y, w*0.5, 2, [][2]string{{"Send", bits(n.SendBps)}, {"Receive", bits(n.RecvBps)}})
	infoList(c, x+w*0.52, y, w*0.48, [][2]string{
		{"Adapter name:", n.Label},
		{"Link speed:", bits(n.LinkBps)},
		{"IPv4 address:", n.IP},
		{"MAC address:", n.MAC},
	})
}

func (pp *perfPage) drawGPU(a *App, c *ui.Canvas, x, y, w, h float32) {
	s := &a.col.Snap
	g := s.GPU
	y = detailHead(c, x, y, w, "GPU 0", g.Name, ColGPU)
	// engine charts 2x2
	engs := []struct{ key, label string }{{"3D", "3D"}, {"Compute", "Compute (CUDA / AI)"}, {"Copy", "Copy"}, {"VideoDecode", "Video Decode"}}
	gh := (h - 44 - 240) / 2
	if gh < 70 {
		gh = 70
	}
	gw := (w - 12) / 2
	for i, e := range engs {
		gx := x + float32(i%2)*(gw+12)
		gy := y + float32(i/2)*(gh+8)
		r := g.EngHist[e.key]
		chartBox(c, gx, gy, gw, gh, e.label, pct(g.Engines[e.key]), ColGPU)
		if r != nil {
			c.Clip(gx+1, gy+21, gw-2, gh-22)
			c.Chart(gx+1, gy+21, gw-2, gh-22, r, sys.HistLen, 100, ColGPU, true, 100)
			c.Unclip()
		}
	}
	y += 2*gh + 16
	mh := float32(64)
	chartBox(c, x, y, gw, mh, "Dedicated GPU memory", fmt.Sprintf("%.1f GB", float64(g.Stat.MemTotal)/(1<<30)), ColVRAM)
	c.Clip(x+1, y+21, gw-2, mh-22)
	c.Chart(x+1, y+21, gw-2, mh-22, &a.col.VRAMHist, sys.HistLen, float64(g.Stat.MemTotal), ColVRAM, false, 120)
	c.Unclip()
	chartBox(c, x+gw+12, y, gw, mh, "Shared GPU memory", fmt.Sprintf("%.1f GB", float64(g.SharedTotal)/(1<<30)), ColMem)
	c.Clip(x+gw+13, y+21, gw-2, mh-22)
	c.Chart(x+gw+13, y+21, gw-2, mh-22, &a.col.SharedHist, sys.HistLen, math.Max(1, float64(g.SharedTotal)), ColMem, false, 120)
	c.Unclip()
	y += mh + 14
	st := g.Stat
	items := [][2]string{
		{"Utilization", fmt.Sprintf("%d%%", st.Util)},
		{"Dedicated memory", fmt.Sprintf("%.1f/%.1f GB", float64(st.MemUsed)/(1<<30), float64(st.MemTotal)/(1<<30))},
		{"Shared memory", fmt.Sprintf("%.1f GB", float64(g.SharedUsed)/(1<<30))},
	}
	if st.HasTemp {
		items = append(items, [2]string{"Temperature", fmt.Sprintf("%d °C", st.Temp)})
	}
	if st.HasPower {
		items = append(items, [2]string{"Power", fmt.Sprintf("%.0f / %.0f W", st.PowerW, st.PowerLimitW)})
	}
	if st.ClockMHz > 0 {
		items = append(items, [2]string{"Clock", fmt.Sprintf("%d MHz", st.ClockMHz)})
	}
	statsGrid(c, x, y, w*0.62, 3, items)
	info := [][2]string{}
	if g.Driver != "" {
		info = append(info, [2]string{"Driver version:", g.Driver})
	}
	if st.MemClockMHz > 0 {
		info = append(info, [2]string{"Memory clock:", fmt.Sprintf("%d MHz", st.MemClockMHz)})
	}
	if st.MaxClockMHz > 0 {
		info = append(info, [2]string{"Max clock:", fmt.Sprintf("%d MHz", st.MaxClockMHz)})
	}
	if st.HasFan {
		info = append(info, [2]string{"Fan speed:", fmt.Sprintf("%d%%", st.Fan)})
	}
	if st.PcieGen > 0 {
		info = append(info, [2]string{"PCIe link:", fmt.Sprintf("Gen %d x%d", st.PcieGen, st.PcieWidth)})
	}
	infoList(c, x+w*0.64, y, w*0.36, info)
}

func (pp *perfPage) drawTemp(a *App, c *ui.Canvas, x, y, w, h float32) {
	s := &a.col.Snap
	y = detailHead(c, x, y, w, "Temperatures", "°C", ColTemp)
	ch := h - 44 - 190
	if ch < 120 {
		ch = 120
	}
	chartBox(c, x, y, w, ch, "GPU (green) and CPU (blue) over 60 seconds", "100 °C", ColTemp)
	c.Clip(x+1, y+21, w-2, ch-22)
	// warning zones
	c.FillA(x+1, y+21, w-2, (ch-22)*0.15, ColHot, 22)
	c.FillA(x+1, y+21+(ch-22)*0.15, w-2, (ch-22)*0.15, ColWarn, 14)
	if s.CPU.TempOK {
		c.Chart(x+1, y+21, w-2, ch-22, &a.col.CTempHist, sys.HistLen, 100, ColCPU, true, 0)
	}
	c.Chart(x+1, y+21, w-2, ch-22, &a.col.GTempHist, sys.HistLen, 100, ColTemp, false, 70)
	c.Unclip()
	y += ch + 16
	gauge := func(gx float32, label string, v float64, ok bool, note string) {
		c.Text(gx, y, 200, 18, label, ui.FontSmall, ColDim, ui.AlignLeft)
		if !ok {
			c.Text(gx, y+20, 300, 30, "not available", ui.FontBody, ColFaint, ui.AlignLeft)
			return
		}
		col := tempColor(v)
		c.Text(gx, y+18, 120, 40, fmt.Sprintf("%.0f°C", v), ui.Font{Size: 28, Weight: 600}, col, ui.AlignLeft)
		bar(c, gx, y+64, 220, 8, v/100, col)
		c.Text(gx, y+76, 300, 18, note, ui.FontSmall, ColFaint, ui.AlignLeft)
	}
	gauge(x, "GPU", float64(s.GPU.Stat.Temp), s.GPU.Stat.HasTemp, "Normal under load: up to ~83°C")
	gauge(x+w/2, "CPU (thermal zone)", s.CPU.Temp, s.CPU.TempOK, "Approximate — Windows ACPI sensor")
}
