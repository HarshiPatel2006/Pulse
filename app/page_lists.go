//go:build windows

package app

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"pulse/sys"
	"pulse/ui"
	"pulse/win"
)

func toggleSort(t *Table, k string) {
	if t.Sort == k {
		t.Desc = !t.Desc
	} else {
		t.Sort, t.Desc = k, k != "name"
	}
}

func contains(q string, fields ...string) bool {
	if q == "" {
		return true
	}
	q = strings.ToLower(q)
	for _, f := range fields {
		if strings.Contains(strings.ToLower(f), q) {
			return true
		}
	}
	return false
}

// ======================= App history =======================

type histPage struct{ t Table }

func (hp *histPage) moveSel(d int) { hp.t.MoveSel(d) }

func (hp *histPage) draw(a *App, c *ui.Canvas, x, y, w, h float32) {
	if hp.t.Sort == "" {
		hp.t.Sort, hp.t.Desc = "cpu", true
	}
	since := a.col.App.Since
	y2 := a.pageHeader(c, x, y, w, "App history", "Resource usage since "+since.Format("02 Jan 2006, 15:04"))
	a.btn(c, x+w-150, y, 150, 32, "histreset", "Delete history", GEnd, 0, true, func() {
		if a.confirm("Clear the app history collected so far?") {
			a.col.App.Reset()
		}
	})
	rows := a.col.App.Rows()
	var maxCPU, maxDisk, maxGPU float64
	var out []sys.AppRow
	for _, r := range rows {
		if !contains(a.search, r.Exe, r.Name) {
			continue
		}
		out = append(out, r)
		maxCPU = max(maxCPU, float64(r.CPU))
		maxDisk = max(maxDisk, float64(r.Disk))
		maxGPU = max(maxGPU, r.GPUSec)
	}
	sort.Slice(out, func(i, j int) bool {
		var l bool
		switch hp.t.Sort {
		case "cpu":
			l = out[i].CPU < out[j].CPU
		case "disk":
			l = out[i].Disk < out[j].Disk
		case "gpu":
			l = out[i].GPUSec < out[j].GPUSec
		default:
			l = strings.ToLower(out[i].Name+out[i].Exe) > strings.ToLower(out[j].Name+out[j].Exe)
		}
		if hp.t.Desc {
			return !l
		}
		return l
	})
	if len(out) > 300 {
		out = out[:300]
	}
	cols := []Col{
		{Key: "name", Title: "Name"},
		{Key: "cpu", Title: "CPU time", W: 130, Align: ui.AlignRight, Heat: ColCPU},
		{Key: "disk", Title: "Disk read + write", W: 150, Align: ui.AlignRight, Heat: ColDisk},
		{Key: "gpu", Title: "GPU time", W: 120, Align: ui.AlignRight, Heat: ColGPU},
	}
	var tr []Row
	for _, r := range out {
		name := r.Name
		if name == "" {
			name = strings.TrimSuffix(r.Exe, ".exe")
		}
		tr = append(tr, Row{Key: r.Exe, Name: name, Glyph: GHistory, GlyphCol: ColNet, Badge: r.Exe, BadgeCol: ColFaint, Cells: []Cell{
			{Text: cpuTime(r.CPU), Heat: float64(r.CPU) / (maxCPU + 1)},
			{Text: mb(float64(r.Disk)), Heat: float64(r.Disk) / (maxDisk + 1)},
			{Text: cpuTime(uint64(r.GPUSec * 1e7)), Heat: r.GPUSec / (maxGPU + 0.001)},
		}})
	}
	hp.t.Draw(a, c, "hist", x, y2, w, h-(y2-y), cols, tr, func(k string) { toggleSort(&hp.t, k) }, nil, nil)
}

// ======================= Startup apps =======================

type startupPage struct {
	t      Table
	items  []sys.StartupApp
	loaded time.Time
}

func (sp *startupPage) moveSel(d int) { sp.t.MoveSel(d) }

func (sp *startupPage) draw(a *App, c *ui.Canvas, x, y, w, h float32) {
	if time.Since(sp.loaded) > 10*time.Second {
		sp.items, sp.loaded = sys.StartupApps(), time.Now()
	}
	if sp.t.Sort == "" {
		sp.t.Sort = "name"
	}
	y2 := a.pageHeader(c, x, y, w, "Startup apps", fmt.Sprintf("%d apps run when Windows starts", len(sp.items)))
	var sel *sys.StartupApp
	for i := range sp.items {
		if sp.items[i].Location+sp.items[i].Name == sp.t.Sel {
			sel = &sp.items[i]
		}
	}
	toggle := func(it *sys.StartupApp) {
		if sys.SetStartupEnabled(*it, !it.Enabled) {
			a.showToast(map[bool]string{true: "Enabled ", false: "Disabled "}[!it.Enabled] + it.Name)
		} else {
			a.showToast("Couldn't change it — machine-wide entries need Pulse to run as administrator")
		}
		sp.loaded = time.Time{}
	}
	if sel != nil {
		label := "Disable"
		if !sel.Enabled {
			label = "Enable"
		}
		s := sel
		a.btn(c, x+w-120, y, 120, 32, "suToggle", label, 0, 1, true, func() { toggle(s) })
	}
	items := []sys.StartupApp{}
	for _, it := range sp.items {
		if contains(a.search, it.Name, it.Publisher, it.Command) {
			items = append(items, it)
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		var l bool
		switch sp.t.Sort {
		case "pub":
			l = items[i].Publisher < items[j].Publisher
		case "status":
			l = items[i].Enabled && !items[j].Enabled
		case "loc":
			l = items[i].Location < items[j].Location
		default:
			l = strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
		}
		if sp.t.Desc {
			return !l
		}
		return l
	})
	cols := []Col{
		{Key: "name", Title: "Name"},
		{Key: "pub", Title: "Publisher", W: 200},
		{Key: "status", Title: "Status", W: 110},
		{Key: "loc", Title: "Location", W: 200},
	}
	var rows []Row
	for i := range items {
		it := items[i]
		st, sc := "Enabled", uint32(ColOK)
		if !it.Enabled {
			st, sc = "Disabled", ColFaint
		}
		rows = append(rows, Row{Key: it.Location + it.Name, Name: it.Name, Icon: a.icon(it.Path), Dim: !it.Enabled,
			Cells: []Cell{{Text: it.Publisher, Color: ColDim}, {Text: st, Color: sc}, {Text: it.Location, Color: ColDim}}})
	}
	sp.t.Draw(a, c, "su", x, y2, w, h-(y2-y), cols, rows, func(k string) { toggleSort(&sp.t, k) }, func(r *Row) {
		for i := range sp.items {
			it := &sp.items[i]
			if it.Location+it.Name == r.Key {
				label := "Disable"
				if !it.Enabled {
					label = "Enable"
				}
				a.popup([]menuItem{
					{label: label, enabled: true, act: func() { toggle(it) }},
					{label: "Open file location", enabled: it.Path != "", act: func() { a.openLocation(it.Path) }},
					{label: "Search online", enabled: true, act: func() { searchOnline(it.Name) }},
					{label: "Copy command", enabled: true, act: func() { win.CopyToClipboard(a.hwnd, it.Command) }},
				})
			}
		}
	}, nil)
}

// ======================= Users =======================

type usersPage struct {
	t        Table
	sessions []sys.UserRow
	at       time.Time
}

func (up *usersPage) draw(a *App, c *ui.Canvas, x, y, w, h float32) {
	s := &a.col.Snap
	y2 := a.pageHeader(c, x, y, w, "Users", "")
	if time.Since(up.at) > 2*time.Second {
		up.sessions, up.at = sys.Users(s.Procs), time.Now()
	}
	users := up.sessions
	cols := []Col{
		{Key: "name", Title: "User"},
		{Key: "status", Title: "Status", W: 120},
		{Key: "cpu", Title: "CPU", W: 90, Align: ui.AlignRight, Heat: ColCPU, Summary: pct(s.CPU.Total)},
		{Key: "mem", Title: "Memory", W: 110, Align: ui.AlignRight, Heat: ColMem, Summary: pct(s.Mem.Load)},
		{Key: "disk", Title: "Disk", W: 100, Align: ui.AlignRight, Heat: ColDisk},
		{Key: "gpu", Title: "GPU", W: 80, Align: ui.AlignRight, Heat: ColGPU},
		{Key: "vram", Title: "GPU memory", W: 110, Align: ui.AlignRight, Heat: ColVRAM},
	}
	var rows []Row
	for _, u := range users {
		rows = append(rows, Row{Key: fmt.Sprint(u.ID), Name: u.User, Glyph: GUsers, GlyphCol: ColMem,
			Badge: fmt.Sprintf("session %d · %d processes", u.ID, u.Procs), BadgeCol: ColFaint,
			Cells: []Cell{
				{Text: sys.SessionState(u.State), Color: ColDim},
				{Text: pct(u.CPU), Heat: u.CPU / 100},
				{Text: mb(float64(u.Mem)), Heat: float64(u.Mem) / float64(s.Mem.Total+1)},
				{Text: rate(u.Disk), Heat: u.Disk / 50e6},
				{Text: pct(u.GPU), Heat: u.GPU / 100},
				{Text: mb(float64(u.VRAM)), Heat: float64(u.VRAM) / float64(s.GPU.Stat.MemTotal+1)},
			}})
	}
	up.t.Draw(a, c, "users", x, y2, w, h-(y2-y), cols, rows, nil, func(r *Row) {
		var id uint32
		fmt.Sscan(r.Key, &id)
		a.popup([]menuItem{
			{label: "Disconnect", enabled: true, act: func() {
				if a.confirm("Disconnect this user session?") && !win.DisconnectSession(id) {
					a.showToast("Couldn't disconnect (needs administrator)")
				}
			}},
			{label: "Sign off", enabled: true, act: func() {
				if a.confirm("Sign off this user? Unsaved work will be lost.") && !win.LogoffSession(id) {
					a.showToast("Couldn't sign off (needs administrator)")
				}
			}},
		})
	}, nil)
}

// ======================= Details =======================

type detailsPage struct {
	t      Table
	reveal bool
	byPid  map[uint32]*sys.Proc
}

func (dp *detailsPage) moveSel(d int) { dp.t.MoveSel(d) }

func (dp *detailsPage) endSelected(a *App) {
	var pid uint32
	if _, err := fmt.Sscan(dp.t.Sel, &pid); err == nil {
		if p := dp.byPid[pid]; p != nil {
			a.endProcs([]*sys.Proc{p}, p.Name)
		}
	}
}

func prioName(p uint32) string {
	switch p {
	case 0x100:
		return "Realtime"
	case 0x80:
		return "High"
	case 0x8000:
		return "Above normal"
	case 0x20:
		return "Normal"
	case 0x4000:
		return "Below normal"
	case 0x40:
		return "Low"
	}
	return "—"
}

func (dp *detailsPage) draw(a *App, c *ui.Canvas, x, y, w, h float32) {
	s := &a.col.Snap
	if dp.t.Sort == "" {
		dp.t.Sort, dp.t.Desc = a.set.DetailSort, a.set.DetailDesc
	}
	y2 := a.pageHeader(c, x, y, w, "Details", fmt.Sprintf("%d processes", len(s.Procs)))
	a.btn(c, x+w-120, y, 120, 32, "dend", "End task", GEnd, 2, dp.t.Sel != "", func() { dp.endSelected(a) })
	list := make([]*sys.Proc, 0, len(s.Procs))
	dp.byPid = make(map[uint32]*sys.Proc, len(s.Procs))
	for _, p := range s.Procs {
		dp.byPid[p.Pid] = p
		if matches(a.search, p) {
			list = append(list, p)
		}
	}
	sort.SliceStable(list, func(i, j int) bool {
		pi, pj := list[i], list[j]
		var l bool
		switch dp.t.Sort {
		case "pid":
			l = pi.Pid < pj.Pid
		case "user":
			l = strings.ToLower(pi.User) < strings.ToLower(pj.User)
		case "cpu":
			l = pi.CPU < pj.CPU
		case "time":
			l = pi.CPUTime < pj.CPUTime
		case "mem":
			l = pi.Private < pj.Private
		case "ws":
			l = pi.WS < pj.WS
		case "threads":
			l = pi.Threads < pj.Threads
		case "handles":
			l = pi.Handles < pj.Handles
		case "vram":
			l = pi.VRAM < pj.VRAM
		case "prio":
			l = pi.Priority < pj.Priority
		default:
			l = strings.ToLower(pi.Name) < strings.ToLower(pj.Name)
		}
		if dp.t.Desc {
			return !l
		}
		return l
	})
	cols := []Col{
		{Key: "name", Title: "Name"},
		{Key: "pid", Title: "PID", W: 70, Align: ui.AlignRight},
		{Key: "user", Title: "User name", W: 120},
		{Key: "cpu", Title: "CPU", W: 64, Align: ui.AlignRight, Heat: ColCPU},
		{Key: "time", Title: "CPU time", W: 90, Align: ui.AlignRight},
		{Key: "mem", Title: "Memory (private)", W: 124, Align: ui.AlignRight, Heat: ColMem},
		{Key: "ws", Title: "Working set", W: 100, Align: ui.AlignRight},
		{Key: "vram", Title: "GPU memory", W: 98, Align: ui.AlignRight, Heat: ColVRAM},
		{Key: "threads", Title: "Threads", W: 70, Align: ui.AlignRight},
		{Key: "handles", Title: "Handles", W: 76, Align: ui.AlignRight},
		{Key: "prio", Title: "Priority", W: 100},
		{Key: "arch", Title: "Arch", W: 56},
	}
	// drop columns that don't fit (keep the most important)
	for total := func() float32 {
		t := float32(220)
		for _, c := range cols[1:] {
			t += c.W
		}
		return t
	}(); total > w && len(cols) > 5; total -= cols[len(cols)-1].W {
		cols = cols[:len(cols)-1]
	}
	rows := make([]Row, 0, len(list))
	for _, p := range list {
		arch := "x64"
		if p.Is32 {
			arch = "x86"
		}
		cells := []Cell{
			{Text: fmt.Sprint(p.Pid), Color: ColDim},
			{Text: p.User, Color: ColDim},
			{Text: fmt.Sprintf("%02.0f", p.CPU), Heat: p.CPU / 100 * 1.5},
			{Text: cpuTime(p.CPUTime), Color: ColDim},
			{Text: fmt.Sprintf("%s K", commas(p.Private/1024)), Heat: float64(p.Private) / float64(s.Mem.Total+1) * 6},
			{Text: fmt.Sprintf("%s K", commas(p.WS/1024)), Color: ColDim},
			{Text: mb(float64(p.VRAM)), Heat: float64(p.VRAM) / float64(s.GPU.Stat.MemTotal+1) * 1.5},
			{Text: fmt.Sprint(p.Threads), Color: ColDim},
			{Text: fmt.Sprint(p.Handles), Color: ColDim},
			{Text: prioName(p.Priority), Color: ColDim},
			{Text: arch, Color: ColDim},
		}
		if p.VRAM == 0 {
			cells[6] = Cell{Text: "—", Color: ColFaint}
		}
		rows = append(rows, Row{Key: fmt.Sprint(p.Pid), Name: p.Name, Icon: a.icon(p.Path), Cells: cells[:len(cols)-1], Dim: !p.Access})
	}
	if dp.reveal {
		dp.reveal = false
		for i, r := range rows {
			if r.Key == dp.t.Sel {
				dp.t.Scroll = float32(i)*rowH - 100
			}
		}
	}
	dp.t.Draw(a, c, "det", x, y2, w, h-(y2-y), cols, rows, func(k string) {
		toggleSort(&dp.t, k)
		a.set.DetailSort, a.set.DetailDesc = dp.t.Sort, dp.t.Desc
		a.set.save()
	}, func(r *Row) {
		var pid uint32
		fmt.Sscan(r.Key, &pid)
		if p := dp.byPid[pid]; p != nil {
			a.procMenu([]*sys.Proc{p}, s.Procs, p.Name)
		}
	}, nil)
}

func commas(v uint64) string {
	s := fmt.Sprint(v)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

// ======================= Services =======================

type servicesPage struct {
	t        Table
	list     []sys.ServiceRow
	loaded   time.Time
	detailed bool
	loading  bool
}

func (sv *servicesPage) moveSel(d int) { sv.t.MoveSel(d) }

func (sv *servicesPage) draw(a *App, c *ui.Canvas, x, y, w, h float32) {
	if time.Since(sv.loaded) > 3*time.Second {
		sv.list, sv.loaded = sys.Services(false), time.Now()
		if !sv.detailed && !sv.loading {
			sv.loading = true
			go func() { // descriptions/start types are slow: fetch once in background
				sys.Services(true)
				sv.detailed, sv.loaded = true, time.Time{}
				win.PostMessage(a.hwnd, wmData, 0, 0)
			}()
		}
	}
	if sv.t.Sort == "" {
		sv.t.Sort = "name"
	}
	running := 0
	for _, s := range sv.list {
		if s.State == 4 {
			running++
		}
	}
	y2 := a.pageHeader(c, x, y, w, "Services", fmt.Sprintf("%d running of %d", running, len(sv.list)))
	a.btn(c, x+w-150, y, 150, 32, "svcmsc", "Open Services", GServices, 0, true, func() { win.ShellOpen("open", "services.msc", "", "") })
	var sel *sys.ServiceRow
	for i := range sv.list {
		if sv.list[i].Name == sv.t.Sel {
			sel = &sv.list[i]
		}
	}
	act := func(name, action string) {
		go func() {
			if err := win.ServiceControl(name, action); err != nil {
				a.showToast("Couldn't " + action + " " + name + " — " + errText(err))
			} else {
				a.showToast(strings.Title(action) + "ed " + name)
			}
			sv.loaded = time.Time{}
			win.PostMessage(a.hwnd, wmData, 0, 0)
		}()
	}
	if sel != nil {
		bx := x + w - 150 - 8
		n := sel.Name
		if sel.State == 4 {
			a.btn(c, bx-100, y, 100, 32, "svcstop", "Stop", GStop, 2, true, func() { act(n, "stop") })
			a.btn(c, bx-208, y, 100, 32, "svcrestart", "Restart", GRefresh, 0, true, func() { act(n, "restart") })
		} else {
			a.btn(c, bx-100, y, 100, 32, "svcstart", "Start", GPlay, 1, true, func() { act(n, "start") })
		}
	}
	items := []sys.ServiceRow{}
	for _, s := range sv.list {
		if contains(a.search, s.Name, s.Display, s.Description, fmt.Sprint(s.Pid)) {
			items = append(items, s)
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		var l bool
		switch sv.t.Sort {
		case "pid":
			l = items[i].Pid < items[j].Pid
		case "desc":
			l = items[i].Display < items[j].Display
		case "status":
			l = items[i].State > items[j].State
		case "start":
			l = items[i].StartType < items[j].StartType
		case "group":
			l = items[i].Group < items[j].Group
		default:
			l = strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
		}
		if sv.t.Desc {
			return !l
		}
		return l
	})
	cols := []Col{
		{Key: "name", Title: "Name"},
		{Key: "pid", Title: "PID", W: 70, Align: ui.AlignRight},
		{Key: "desc", Title: "Description", W: 320},
		{Key: "status", Title: "Status", W: 100},
		{Key: "start", Title: "Startup type", W: 110},
		{Key: "group", Title: "Group", W: 130},
	}
	rows := make([]Row, 0, len(items))
	for _, s := range items {
		st := sys.StateName(s.State)
		sc := uint32(ColFaint)
		if s.State == 4 {
			sc = ColOK
		} else if s.State == 2 || s.State == 3 {
			sc = ColWarn
		}
		pid := ""
		if s.Pid != 0 {
			pid = fmt.Sprint(s.Pid)
		}
		rows = append(rows, Row{Key: s.Name, Name: s.Name, Glyph: GServices, GlyphCol: ui.Mix(sc, ColBG, 0.2), Cells: []Cell{
			{Text: pid, Color: ColDim}, {Text: s.Display, Color: ColDim}, {Text: st, Color: sc},
			{Text: sys.StartTypeName(s.StartType), Color: ColDim}, {Text: s.Group, Color: ColFaint},
		}})
	}
	sv.t.Draw(a, c, "svc", x, y2, w, h-(y2-y), cols, rows, func(k string) { toggleSort(&sv.t, k) }, func(r *Row) {
		for _, s := range sv.list {
			if s.Name == r.Key {
				ss := s
				a.popup([]menuItem{
					{label: "Start", enabled: s.State == 1, act: func() { act(ss.Name, "start") }},
					{label: "Stop", enabled: s.State == 4, act: func() { act(ss.Name, "stop") }},
					{label: "Restart", enabled: s.State == 4, act: func() { act(ss.Name, "restart") }},
					{sep: true},
					{label: "Open Services", enabled: true, act: func() { win.ShellOpen("open", "services.msc", "", "") }},
					{label: "Search online", enabled: true, act: func() { searchOnline(ss.Name + " service") }},
					{label: "Go to details", enabled: s.Pid != 0, act: func() {
						a.goPage(PDetails)
						if dp, ok := a.pages[PDetails].(*detailsPage); ok {
							dp.t.Sel, dp.reveal = fmt.Sprint(ss.Pid), true
						}
					}},
				})
			}
		}
	}, nil)
}

func errText(err error) string {
	s := err.Error()
	if strings.Contains(strings.ToLower(s), "access is denied") {
		return "run Pulse as administrator"
	}
	return s
}
