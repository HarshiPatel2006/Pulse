//go:build windows

package app

import (
	"fmt"
	"sort"
	"strings"

	"pulse/sys"
	"pulse/ui"
)

type procPage struct {
	t        Table
	expanded map[string]bool
	groups   map[string][]*sys.Proc
	byPid    map[uint32]*sys.Proc
}

type procGroup struct {
	key     string
	name    string
	procs   []*sys.Proc
	isApp   bool
	isWin   bool
	cpu     float64
	mem     uint64
	disk    float64
	gpu     float64
	vram    uint64
	path    string
}

func matches(q string, p *sys.Proc) bool {
	if q == "" {
		return true
	}
	q = strings.ToLower(q)
	return strings.Contains(strings.ToLower(p.Name), q) || strings.Contains(strings.ToLower(p.Desc), q) ||
		strings.Contains(strings.ToLower(p.Company), q) || fmt.Sprint(p.Pid) == q || strings.Contains(strings.ToLower(p.Title), q)
}

func isWindowsProc(p *sys.Proc) bool {
	lp := strings.ToLower(p.Path)
	return p.Pid <= 4 || (p.Path == "" && !p.Access) || strings.HasPrefix(lp, `c:\windows\`)
}

func displayName(p *sys.Proc) string {
	if p.Desc != "" {
		return p.Desc
	}
	return strings.TrimSuffix(p.Name, ".exe")
}

func (pp *procPage) endSelected(a *App) {
	if g, ok := pp.groups[pp.t.Sel]; ok {
		a.endProcs(g, pp.t.Sel)
		return
	}
	var pid uint32
	if _, err := fmt.Sscan(pp.t.Sel, &pid); err == nil {
		if p := pp.byPid[pid]; p != nil {
			a.endProcs([]*sys.Proc{p}, p.Name)
		}
	}
}

func (pp *procPage) moveSel(d int) { pp.t.MoveSel(d) }

func (pp *procPage) draw(a *App, c *ui.Canvas, x, y, w, h float32) {
	if pp.expanded == nil {
		pp.expanded = map[string]bool{}
		pp.t.Sort, pp.t.Desc = a.set.ProcSort, a.set.ProcDesc
	}
	s := &a.col.Snap
	y = a.pageHeader(c, x, y, w, "Processes", "")

	// actions (right side of header)
	bx := x + w - 120
	a.btn(c, bx, y-42, 120, 32, "endtask", "End task", GEnd, 2, pp.t.Sel != "", func() { pp.endSelected(a) })
	bx -= 148
	a.btn(c, bx, y-42, 140, 32, "runtask", "Run new task", GAdd, 0, true, runNewTask)
	bx -= 8
	gw := a.segmented(c, bx-150, y-42, 32, "grp", []string{"Grouped", "All"}, map[bool]int{true: 0, false: 1}[a.set.GroupApps], func(i int) {
		a.set.GroupApps = i == 0
		a.set.save()
	})
	_ = gw

	// build groups
	groups := map[string]*procGroup{}
	pp.byPid = make(map[uint32]*sys.Proc, len(s.Procs))
	var totalVRAM uint64
	var totalDisk float64
	for _, p := range s.Procs {
		pp.byPid[p.Pid] = p
		totalVRAM += p.VRAM
		totalDisk += p.DiskBps
		if p.Pid == 0 || !matches(a.search, p) {
			continue
		}
		key := strings.ToLower(p.Name)
		if !a.set.GroupApps {
			key = fmt.Sprintf("%s#%d", key, p.Pid)
		}
		g := groups[key]
		if g == nil {
			g = &procGroup{key: key, name: displayName(p), path: p.Path}
			groups[key] = g
		}
		g.procs = append(g.procs, p)
		g.cpu += p.CPU
		g.mem += p.Private
		g.disk += p.DiskBps
		g.gpu += p.GPU
		g.vram += p.VRAM
		if p.IsApp {
			g.isApp = true
		}
		if isWindowsProc(p) {
			g.isWin = true
		}
	}
	list := make([]*procGroup, 0, len(groups))
	pp.groups = map[string][]*sys.Proc{}
	for _, g := range groups {
		if g.gpu > 100 {
			g.gpu = 100
		}
		list = append(list, g)
		pp.groups[g.key] = g.procs
	}
	less := func(i, j *procGroup) bool {
		switch pp.t.Sort {
		case "cpu":
			return i.cpu < j.cpu
		case "mem":
			return i.mem < j.mem
		case "disk":
			return i.disk < j.disk
		case "gpu":
			return i.gpu < j.gpu
		case "vram":
			return i.vram < j.vram
		}
		return strings.ToLower(i.name) > strings.ToLower(j.name)
	}
	sort.SliceStable(list, func(i, j int) bool {
		if pp.t.Desc {
			return less(list[j], list[i])
		}
		return less(list[i], list[j])
	})

	memTotal := float64(s.Mem.Total)
	vramTotal := float64(s.GPU.Stat.MemTotal)
	cols := []Col{
		{Key: "name", Title: "Name"},
		{Key: "cpu", Title: "CPU", W: 86, Align: ui.AlignRight, Heat: ColCPU, Summary: pct(s.CPU.Total)},
		{Key: "mem", Title: "Memory", W: 104, Align: ui.AlignRight, Heat: ColMem, Summary: pct(s.Mem.Load)},
		{Key: "disk", Title: "Disk", W: 100, Align: ui.AlignRight, Heat: ColDisk, Summary: rate(totalDisk)},
		{Key: "gpu", Title: "GPU", W: 80, Align: ui.AlignRight, Heat: ColGPU, Summary: fmt.Sprintf("%d%%", s.GPU.Stat.Util)},
		{Key: "vram", Title: "GPU memory", W: 112, Align: ui.AlignRight, Heat: ColVRAM, Summary: mb(float64(s.GPU.Stat.MemUsed))},
	}
	cellsOf := func(cpu float64, mem uint64, disk, gpu float64, vram uint64) []Cell {
		cells := []Cell{
			{Text: pct(cpu), Heat: cpu / 100 * 1.5},
			{Text: mb(float64(mem)), Heat: float64(mem) / memTotal * 6},
			{Text: rate(disk), Heat: disk / 50e6},
			{Text: pct(gpu), Heat: gpu / 100},
			{Text: mb(float64(vram)), Heat: float64(vram) / vramTotal * 1.5},
		}
		if vram == 0 {
			cells[4].Text, cells[4].Color = "—", ColFaint
		}
		return cells
	}

	var rows []Row
	addSection := func(title string, filter func(g *procGroup) bool) {
		var sec []*procGroup
		for _, g := range list {
			if filter(g) {
				sec = append(sec, g)
			}
		}
		if len(sec) == 0 {
			return
		}
		rows = append(rows, Row{Key: "§" + title, Group: true, Name: fmt.Sprintf("%s (%d)", title, len(sec))})
		for _, g := range sec {
			gg := g
			multi := len(g.procs) > 1
			p0 := g.procs[0]
			name := g.name
			if !a.set.GroupApps {
				name = displayName(p0)
			}
			r := Row{Key: g.key, Name: name, Icon: a.icon(g.path), Cells: cellsOf(g.cpu, g.mem, g.disk, g.gpu, g.vram),
				Expandable: multi, Expanded: pp.expanded[g.key]}
			if multi {
				r.Badge = fmt.Sprint(len(g.procs))
				r.BadgeCol = ColAccent2
				r.Toggle = func() { pp.expanded[gg.key] = !pp.expanded[gg.key] }
			}
			if !a.set.GroupApps {
				r.Badge = fmt.Sprint(p0.Pid)
				r.BadgeCol = ColFaint
			}
			rows = append(rows, r)
			if multi && pp.expanded[g.key] {
				kids := append([]*sys.Proc{}, g.procs...)
				sort.Slice(kids, func(i, j int) bool { return kids[i].CPU+float64(kids[i].Private)/1e10 > kids[j].CPU+float64(kids[j].Private)/1e10 })
				for _, p := range kids {
					n := displayName(p)
					if p.Title != "" {
						n = p.Title
					}
					rows = append(rows, Row{Key: fmt.Sprint(p.Pid), Name: n, Depth: 1, Icon: a.icon(p.Path), Badge: fmt.Sprint(p.Pid), BadgeCol: ColFaint,
						Cells: cellsOf(p.CPU, p.Private, p.DiskBps, p.GPU, p.VRAM)})
				}
			}
		}
	}
	addSection("Apps", func(g *procGroup) bool { return g.isApp })
	addSection("Background processes", func(g *procGroup) bool { return !g.isApp && !g.isWin })
	addSection("Windows processes", func(g *procGroup) bool { return !g.isApp && g.isWin })

	pp.t.Draw(a, c, "proc", x, y, w, h-(y-(y-0))-44, cols, rows,
		func(k string) {
			if pp.t.Sort == k {
				pp.t.Desc = !pp.t.Desc
			} else {
				pp.t.Sort, pp.t.Desc = k, k != "name"
			}
			a.set.ProcSort, a.set.ProcDesc = pp.t.Sort, pp.t.Desc
			a.set.save()
		},
		func(r *Row) {
			if g, ok := pp.groups[r.Key]; ok {
				a.procMenu(g, s.Procs, r.Name)
				return
			}
			var pid uint32
			fmt.Sscan(r.Key, &pid)
			if p := pp.byPid[pid]; p != nil {
				a.procMenu([]*sys.Proc{p}, s.Procs, p.Name)
			}
		}, nil)
}
