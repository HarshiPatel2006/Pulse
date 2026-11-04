//go:build linux

package main

import (
	"errors"
	"fmt"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	desktopDriver "fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/shirou/gopsutil/v4/process"
	"pulse/linuxmon"
)

type pulseTheme struct{ fyne.Theme }

func (t pulseTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	switch n {
	case theme.ColorNameBackground:
		return color.NRGBA{13, 16, 24, 255}
	case theme.ColorNamePrimary:
		return color.NRGBA{78, 183, 255, 255}
	case theme.ColorNameInputBackground:
		return color.NRGBA{23, 28, 40, 255}
	}
	return t.Theme.Color(n, theme.VariantDark)
}

type row struct {
	cells []string
	pid   int32
	key   string
	heat  []float64
}
type desktop struct {
	perfMode        string
	perfScroll      *container.Scroll
	a               fyne.App
	w               fyne.Window
	c               *linuxmon.Collector
	s               linuxmon.Snapshot
	page            string
	rows            []row
	headers         []string
	selected        int
	table           *widget.Table
	search          *widget.Entry
	summary, status *widget.Label
	body            *fyne.Container
	graphs          map[string][]float64
	gpu             map[string]string
	vram            map[int32]uint64
	gpuProc         map[int32]float64
	gpuMu           sync.Mutex
	info            string
	sortCol         int
	descending      bool
	widget          fyne.Window
	stop            chan struct{}
}

func bytes(v float64) string {
	for _, u := range []string{"B", "KiB", "MiB", "GiB", "TiB"} {
		if v < 1024 || u == "TiB" {
			return fmt.Sprintf("%.1f %s", v, u)
		}
		v /= 1024
	}
	return ""
}
func pct(v float64) string {
	if v < 0 {
		return "—"
	}
	return fmt.Sprintf("%.1f%%", v)
}
func (d *desktop) err(e error) {
	if e != nil {
		dialog.ShowError(e, d.w)
	}
}
func (d *desktop) chosen() *row {
	if d.selected < 0 || d.selected >= len(d.rows) {
		return nil
	}
	return &d.rows[d.selected]
}
func (d *desktop) confirm(title, message string, fn func() error) {
	dialog.ShowConfirm(title, message, func(ok bool) {
		if ok {
			d.err(fn())
		}
	}, d.w)
}
func (d *desktop) processAction(action string) {
	r := d.chosen()
	if r == nil || r.pid <= 0 {
		return
	}
	pid := r.pid
	var target linuxmon.Proc
	for _, p := range d.s.Procs {
		if p.PID == pid {
			target = p
		}
	}
	snapshot := append([]linuxmon.Proc(nil), d.s.Procs...)
	d.confirm(action, fmt.Sprintf("%s PID %d?", action, pid), func() error {
		if e := checkIdentity(target); e != nil {
			return e
		}
		switch action {
		case "End task":
			return syscall.Kill(int(pid), syscall.SIGTERM)
		case "Kill tree":
			return killTree(pid, snapshot)
		default:
			entry := widget.NewEntry()
			entry.SetText("0")
			dialog.ShowForm("Set nice value", "Apply", "Cancel", []*widget.FormItem{widget.NewFormItem("Nice (-20 to 19)", entry)}, func(ok bool) {
				if ok {
					v, e := strconv.Atoi(entry.Text)
					if e != nil || v < -20 || v > 19 {
						d.err(fmt.Errorf("enter an integer from -20 to 19"))
						return
					}
					if e := checkIdentity(target); e != nil {
						d.err(e)
						return
					}
					d.err(syscall.Setpriority(syscall.PRIO_PROCESS, int(pid), v))
				}
			}, d.w)
			return nil
		}
	})
}

// Snapshot ancestry is used only for ordering; PID start times are checked before destructive actions.
func checkIdentity(target linuxmon.Proc) error {
	p, e := process.NewProcess(target.PID)
	if e != nil {
		return e
	}
	start, e := p.CreateTime()
	if e != nil {
		return e
	}
	if target.Start == 0 || start != target.Start {
		return fmt.Errorf("process changed; select it again")
	}
	return nil
}
func killTree(pid int32, ps []linuxmon.Proc) error {
	seen := map[int32]bool{}
	var visit func(int32) error
	visit = func(p int32) error {
		if seen[p] {
			return nil
		}
		seen[p] = true
		for _, c := range ps {
			if c.PPID == p {
				if e := visit(c.PID); e != nil && !errors.Is(e, syscall.ESRCH) && !errors.Is(e, os.ErrNotExist) && !errors.Is(e, process.ErrorProcessNotRunning) {
					return e
				}
			}
		}
		for _, target := range ps {
			if target.PID == p {
				if e := checkIdentity(target); e != nil {
					return e
				}
				return syscall.Kill(int(p), syscall.SIGKILL)
			}
		}
		return fmt.Errorf("PID %d is no longer in snapshot", p)
	}
	return visit(pid)
}
func (d *desktop) service(action string) {
	r := d.chosen()
	if r == nil {
		return
	}
	name := r.key
	d.confirm(action+" service", action+" "+name+"?", func() error {
		go func() {
			_, e := linuxmon.CommandTimeout(90*time.Second, "systemctl", action, name)
			fyne.Do(func() { d.err(e); d.loadServices() })
		}()
		return nil
	})
}
func desktopField(s, key string) string {
	in := false
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			in = line == "[Desktop Entry]"
		}
		if in && strings.HasPrefix(line, key+"=") {
			return strings.TrimPrefix(line, key+"=")
		}
	}
	return ""
}
func toggleStartup(path string) error {
	b, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	text := string(b)
	hidden := desktopField(text, "Hidden") == "true" || desktopField(text, "X-GNOME-Autostart-enabled") == "false"
	lines := strings.Split(text, "\n")
	out := []string{}
	in := false
	inserted := false
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "[") {
			in = strings.TrimSpace(line) == "[Desktop Entry]"
			out = append(out, line)
			if in {
				out = append(out, fmt.Sprintf("Hidden=%t", !hidden))
				inserted = true
			}
			continue
		}
		if in && (strings.HasPrefix(strings.TrimSpace(line), "Hidden=") || strings.HasPrefix(strings.TrimSpace(line), "X-GNOME-Autostart-enabled=")) {
			continue
		}
		out = append(out, line)
	}
	if !inserted {
		return fmt.Errorf("missing [Desktop Entry] section")
	}
	return os.WriteFile(path, []byte(strings.Join(out, "\n")), 0600)
}
func (d *desktop) loadStartup() {
	d.rows = nil
	d.headers = []string{"Name", "Enabled", "Command", "File"}
	p, _ := os.UserConfigDir()
	paths, _ := filepath.Glob(filepath.Join(p, "autostart", "*.desktop"))
	for _, path := range paths {
		b, e := os.ReadFile(path)
		if e != nil {
			continue
		}
		s := string(b)
		enabled := "Yes"
		if desktopField(s, "Hidden") == "true" || desktopField(s, "X-GNOME-Autostart-enabled") == "false" {
			enabled = "No"
		}
		d.rows = append(d.rows, row{cells: []string{desktopField(s, "Name"), enabled, desktopField(s, "Exec"), filepath.Base(path)}, key: path})
	}
	d.refreshTable()
}
func (d *desktop) loadServices() {
	d.status.SetText("Reading systemd services…")
	go func() {
		s, e := linuxmon.Command("systemctl", "list-units", "--all", "--type=service", "--no-pager", "--no-legend", "--plain")
		fyne.Do(func() {
			if d.page != "Services" {
				return
			}
			d.headers = []string{"Unit", "Load", "Active", "State", "Description"}
			d.rows = nil
			for _, line := range strings.Split(s, "\n") {
				f := strings.Fields(line)
				if len(f) < 4 {
					continue
				}
				d.rows = append(d.rows, row{cells: []string{f[0], f[1], f[2], f[3], strings.Join(f[4:], " ")}, key: f[0]})
			}
			if e != nil {
				d.status.SetText(e.Error())
			} else {
				d.status.SetText("System services • actions may request system authentication")
			}
			d.refreshTable()
		})
	}()
}
func (d *desktop) refreshTable() {
	d.selected = -1
	d.table.UnselectAll()
	d.table.Refresh()
	for i := range d.headers {
		width := float32(115)
		if i == 0 {
			width = 230
		}
		if d.page == "Startup apps" && i == 2 {
			width = 420
		}
		if d.page == "Services" && i == 4 {
			width = 430
		}
		d.table.SetColumnWidth(i, width)
	}
}
func (d *desktop) updateRows() {
	if d.page == "Startup apps" || d.page == "Services" || d.page == "System info" || d.page == "Performance" {
		return
	}
	var oldPID int32
	var oldKey string
	if r := d.chosen(); r != nil {
		oldPID = r.pid
		oldKey = r.key
	}
	d.rows = nil
	switch d.page {
	case "App history":
		d.headers = []string{"Application", "CPU seconds", "Read", "Written", "GPU seconds"}
		for name, h := range d.s.History {
			d.rows = append(d.rows, row{cells: []string{name, fmt.Sprintf("%.1f", h.CPU), bytes(float64(h.Read)), bytes(float64(h.Write)), fmt.Sprintf("%.1f", h.GPU)}, heat: []float64{0, h.CPU / 100, 0, 0}})
		}
	case "Users":
		d.headers = []string{"User", "Processes", "CPU", "Memory", "Disk / s", "VRAM"}
		type agg struct {
			n         int
			cpu, disk float64
			mem, vram uint64
		}
		users := map[string]agg{}
		for _, p := range d.s.Procs {
			u := users[p.User]
			u.n++
			u.cpu += p.CPU
			u.mem += p.Memory
			u.disk += p.Disk
			u.vram += d.vram[p.PID]
			users[p.User] = u
		}
		for name, u := range users {
			d.rows = append(d.rows, row{cells: []string{name, strconv.Itoa(u.n), pct(u.cpu), bytes(float64(u.mem)), bytes(u.disk), bytes(float64(u.vram))}, heat: []float64{0, 0, u.cpu / 100, float64(u.mem) / float64(d.s.Total), u.disk / 1e8, float64(u.vram) / 8e9}})
		}
	default:
		d.headers = []string{"Name", "PID", "User", "CPU", "Memory", "Disk / s", "GPU", "VRAM"}
		if d.page == "Details" {
			d.headers = append(d.headers, "Nice", "Threads", "CPU seconds")
		}
		for _, p := range d.s.Procs {
			g := -1.0
			if v, ok := d.gpuProc[p.PID]; ok {
				g = v
			}
			vr := "—"
			if v, ok := d.vram[p.PID]; ok {
				vr = bytes(float64(v))
			}
			diskText := bytes(p.Disk)
			if !p.IOAvailable {
				diskText = "—"
			}
			r := row{pid: p.PID, cells: []string{p.Name, fmt.Sprint(p.PID), p.User, pct(p.CPU), bytes(float64(p.Memory)), diskText, pct(g), vr}, heat: []float64{0, 0, 0, p.CPU / 100, float64(p.Memory) / float64(d.s.Total), p.Disk / 1e8, g / 100, float64(d.vram[p.PID]) / 8e9}}
			if d.page == "Details" {
				r.cells = append(r.cells, fmt.Sprint(p.Nice), fmt.Sprint(p.Threads), fmt.Sprintf("%.1f", p.Seconds))
			}
			d.rows = append(d.rows, r)
		}
	}
	q := strings.ToLower(d.search.Text)
	filtered := d.rows[:0]
	for _, r := range d.rows {
		if strings.Contains(strings.ToLower(strings.Join(r.cells, " ")), q) {
			filtered = append(filtered, r)
		}
	}
	d.rows = filtered
	sort.SliceStable(d.rows, func(i, j int) bool {
		col := d.sortCol
		if col >= len(d.rows[i].cells) {
			col = 0
		}
		a, b := d.rows[i].cells[col], d.rows[j].cells[col]
		cmp := strings.Compare(strings.ToLower(a), strings.ToLower(b))
		if col > 0 {
			av, bv := sortValue(a), sortValue(b)
			if av < bv {
				cmp = -1
			} else if av > bv {
				cmp = 1
			} else {
				cmp = 0
			}
		}
		if d.descending {
			return cmp > 0
		}
		return cmp < 0
	})
	d.refreshTable()
	if oldPID > 0 || oldKey != "" {
		for i, r := range d.rows {
			if (oldPID > 0 && r.pid == oldPID) || (oldKey != "" && r.key == oldKey) {
				d.selected = i
				d.table.Select(widget.TableCellID{Row: i + 1, Col: 0})
				break
			}
		}
	}
}
func sortValue(s string) float64 {
	f := strings.Fields(strings.TrimSuffix(s, "%"))
	if len(f) == 0 {
		return 0
	}
	v, _ := strconv.ParseFloat(f[0], 64)
	if len(f) > 1 {
		switch f[1] {
		case "KiB":
			v *= 1024
		case "MiB":
			v *= 1048576
		case "GiB":
			v *= 1073741824
		}
	}
	return v
}
func (d *desktop) showPage(page string) {
	d.page = page
	d.sortCol = 0
	d.descending = false
	tools := container.NewHBox()
	switch page {
	case "Processes", "Details":
		tools.Add(widget.NewButton("End task", func() { d.processAction("End task") }))
		tools.Add(widget.NewButton("Kill tree", func() { d.processAction("Kill tree") }))
		tools.Add(widget.NewButton("Priority", func() { d.processAction("Priority") }))
	case "Services":
		for _, a := range []string{"start", "stop", "restart"} {
			action := a
			tools.Add(widget.NewButton(strings.Title(a), func() { d.service(action) }))
		}
		tools.Add(widget.NewButton("Refresh", d.loadServices))
	case "Startup apps":
		tools.Add(widget.NewButton("Enable / disable", func() {
			if r := d.chosen(); r != nil {
				path := r.key
				d.confirm("Toggle startup", path, func() error { e := toggleStartup(path); d.loadStartup(); return e })
			}
		}))
	}
	sorter := widget.NewSelect([]string{"Name ↑", "CPU ↓", "Memory ↓", "Disk ↓", "GPU ↓", "VRAM ↓"}, func(s string) {
		switch s {
		case "CPU ↓":
			d.sortCol = 3
		case "Memory ↓":
			d.sortCol = 4
		case "Disk ↓":
			d.sortCol = 5
		case "GPU ↓":
			d.sortCol = 6
		case "VRAM ↓":
			d.sortCol = 7
		default:
			d.sortCol = 0
		}
		d.descending = d.sortCol > 0
		d.updateRows()
	})
	sorter.SetSelected("Name ↑")
	if page == "Processes" || page == "Details" {
		tools.Add(sorter)
	}
	var center fyne.CanvasObject = d.table
	switch page {
	case "Performance":
		center = d.performance()
	case "System info":
		info := d.info + fmt.Sprintf("\nGPU: %s\nNVIDIA driver: %s\nVRAM: %s MiB\n", d.gpu["name"], d.gpu["driver_version"], d.gpu["memory.total"])
		label := widget.NewLabel(info)
		label.TextStyle.Monospace = true
		center = container.NewScroll(label)
		tools.Add(widget.NewButton("Copy", func() { d.w.Clipboard().SetContent(info) }))
	case "Startup apps":
		d.loadStartup()
	case "Services":
		d.loadServices()
	default:
		d.updateRows()
	}
	d.body.Objects = []fyne.CanvasObject{container.NewBorder(container.NewVBox(widget.NewLabelWithStyle(page, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), tools), nil, nil, nil, center)}
	d.body.Refresh()
}

func resourceColor(s string) color.NRGBA {
	switch {
	case strings.HasPrefix(s, "CPU"):
		return palette[0]
	case strings.HasPrefix(s, "Mem"):
		return palette[1]
	case strings.HasPrefix(s, "Disk"):
		return palette[2]
	case strings.HasPrefix(s, "Net"):
		return palette[3]
	default:
		return palette[4]
	}
}

var palette = []color.NRGBA{{76, 173, 255, 255}, {181, 126, 255, 255}, {255, 187, 80, 255}, {75, 221, 166, 255}, {247, 109, 159, 255}}

func graph(values []float64, c color.Color) fyne.CanvasObject {
	r := canvas.NewRasterWithPixels(func(x, y, w, h int) color.Color {
		if len(values) < 2 {
			return color.NRGBA{19, 24, 35, 255}
		}
		i := x * (len(values) - 1) / max(w-1, 1)
		height := int(values[i] / 100 * float64(h))
		if y >= h-height {
			return c
		}
		if y%(max(h/4, 1)) == 0 {
			return color.NRGBA{36, 43, 59, 255}
		}
		return color.NRGBA{19, 24, 35, 255}
	})
	r.SetMinSize(fyne.NewSize(250, 100))
	return r
}
func (d *desktop) performance() fyne.CanvasObject {
	if d.perfMode == "" {
		d.perfMode = "Overview"
	}
	d.perfScroll = container.NewVScroll(d.performanceCharts())
	selector := widget.NewSelect([]string{"Overview", "CPU", "Memory", "Disks", "Network", "GPU"}, func(mode string) { d.perfMode = mode; d.refreshPerformance() })
	selector.SetSelected(d.perfMode)
	return container.NewBorder(container.NewHBox(widget.NewLabel("Resource"), selector), nil, nil, nil, d.perfScroll)
}
func (d *desktop) refreshPerformance() {
	if d.perfScroll != nil {
		d.perfScroll.Content = d.performanceCharts()
		d.perfScroll.Refresh()
	}
}
func (d *desktop) performanceCharts() fyne.CanvasObject {
	boxes := container.NewVBox()
	keys := make([]string, 0, len(d.graphs))
	for k := range d.graphs {
		show := false
		switch d.perfMode {
		case "CPU":
			show = strings.HasPrefix(k, "CPU")
		case "Memory":
			show = k == "Memory"
		case "Disks":
			show = strings.HasPrefix(k, "Disk ")
		case "Network":
			show = strings.HasPrefix(k, "Network ")
		case "GPU":
			show = k == "GPU" || k == "VRAM"
		default:
			show = k == "CPU total" || k == "Memory" || k == "GPU" || k == "VRAM"
		}
		if show {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	grid := container.NewGridWithColumns(2)
	for _, k := range keys {
		v := append([]float64(nil), d.graphs[k]...)
		last := 0.0
		if len(v) > 0 {
			last = v[len(v)-1]
		}
		label := fmt.Sprintf("%s  %.1f%%", k, last)
		if strings.HasPrefix(k, "Disk ") || strings.HasPrefix(k, "Network ") {
			label = k + "  " + bytes(last) + "/s"
			peak := 1.0
			for _, x := range v {
				peak = max(peak, x)
			}
			for j := range v {
				v[j] = v[j] / peak * 100
			}
		}
		grid.Add(container.NewVBox(widget.NewLabel(label), graph(v, resourceColor(k))))
	}
	boxes.Add(grid)
	g := d.gpu
	boxes.Add(widget.NewLabel(fmt.Sprintf("GPU: %s • driver %s\nTemperature %s °C • power %s / %s W • fan %s %%\nGraphics %s MHz • memory %s MHz • VRAM %s / %s MiB\n%s", g["name"], g["driver_version"], g["temperature.gpu"], g["power.draw"], g["power.limit"], g["fan.speed"], g["clocks.gr"], g["clocks.mem"], g["memory.used"], g["memory.total"], g["status"])))
	keys = nil
	for k := range d.s.Disks {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := d.s.Disks[k]
		boxes.Add(widget.NewLabel(fmt.Sprintf("Disk %s    read %s/s    write %s/s", k, bytes(v[0]), bytes(v[1]))))
	}
	keys = nil
	for k := range d.s.Networks {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := d.s.Networks[k]
		boxes.Add(widget.NewLabel(fmt.Sprintf("Network %s    receive %s/s    send %s/s", k, bytes(v[0]), bytes(v[1]))))
	}

	return boxes
}
func (d *desktop) tick(s linuxmon.Snapshot) {
	d.s = s
	d.gpuMu.Lock()
	d.gpu = d.gpuCopy()
	d.gpuMu.Unlock()
	avg := 0.0
	for i, v := range s.CPU {
		avg += v
		d.push(fmt.Sprintf("CPU %02d", i), v)
	}
	if len(s.CPU) > 0 {
		avg /= float64(len(s.CPU))
	}
	d.push("CPU total", avg)
	memory := float64(s.Memory) / float64(s.Total) * 100
	d.push("Memory", memory)
	for k, v := range s.Disks {
		d.push("Disk "+k+" read", v[0])
		d.push("Disk "+k+" write", v[1])
	}
	for k, v := range s.Networks {
		d.push("Network "+k+" receive", v[0])
		d.push("Network "+k+" send", v[1])
	}
	if used, e := strconv.ParseFloat(d.gpu["memory.used"], 64); e == nil {
		if total, e := strconv.ParseFloat(d.gpu["memory.total"], 64); e == nil && total > 0 {
			d.push("VRAM", used/total*100)
		}
	}
	if v, e := strconv.ParseFloat(d.gpu["utilization.gpu"], 64); e == nil {
		d.push("GPU", v)
	}
	text := fmt.Sprintf("CPU %.1f%%    RAM %s / %s    GPU %s%%    %d processes", avg, bytes(float64(s.Memory)), bytes(float64(s.Total)), d.gpu["utilization.gpu"], len(s.Procs))
	d.summary.SetText(text)
	if d.widget != nil {
		d.updateWidget(avg, memory)
	}
	if len(s.Errors) > 0 {
		d.status.SetText(strings.Join(s.Errors, " • "))
	}
	if d.page == "Performance" {
		d.refreshPerformance()
	} else {
		d.updateRows()
	}
}

// GPU worker owns staging values; the UI owns the displayed maps.
var stagingGPU map[string]string
var stagingVRAM map[int32]uint64
var stagingUsage map[int32]float64

func (d *desktop) gpuCopy() map[string]string {
	g := map[string]string{}
	for k, v := range stagingGPU {
		g[k] = v
	}
	d.vram = map[int32]uint64{}
	for k, v := range stagingVRAM {
		d.vram[k] = v
	}
	d.gpuProc = map[int32]float64{}
	for k, v := range stagingUsage {
		d.gpuProc[k] = v
	}
	return g
}
func (d *desktop) push(k string, v float64) {
	d.graphs[k] = append(d.graphs[k], v)
	if len(d.graphs[k]) > 60 {
		d.graphs[k] = d.graphs[k][len(d.graphs[k])-60:]
	}
}
func (d *desktop) updateWidget(cpu, memory float64) {
	gpu, _ := strconv.ParseFloat(d.gpu["utilization.gpu"], 64)
	used, _ := strconv.ParseFloat(d.gpu["memory.used"], 64)
	total, _ := strconv.ParseFloat(d.gpu["memory.total"], 64)
	box := container.NewVBox()
	for _, r := range []struct {
		name, text string
		value      float64
	}{{"CPU", pct(cpu), cpu}, {"Memory", pct(memory), memory}, {"GPU", pct(gpu) + " • " + d.gpu["temperature.gpu"] + " °C", gpu}, {"VRAM", d.gpu["memory.used"] + " MiB", used / max(total, 1) * 100}} {
		c := resourceColor(r.name)
		text := canvas.NewText(r.name+"  "+r.text, c)
		text.TextSize = 14
		v := r.value
		bar := canvas.NewRasterWithPixels(func(x, y, w, h int) color.Color {
			if float64(x) < float64(w)*v/100 {
				return c
			}
			return color.NRGBA{35, 41, 55, 255}
		})
		bar.SetMinSize(fyne.NewSize(240, 8))
		box.Add(container.NewVBox(text, bar))
	}
	d.widget.SetContent(box)
}
func (d *desktop) openCompact() {

	if d.widget != nil {
		d.widget.Show()
		return
	}
	d.widget = d.a.NewWindow("Pulse widget")
	avg := 0.0
	for _, v := range d.s.CPU {
		avg += v
	}
	avg /= float64(max(len(d.s.CPU), 1))
	d.updateWidget(avg, float64(d.s.Memory)/float64(max(d.s.Total, 1))*100)
	d.widget.Resize(fyne.NewSize(280, 235))
	d.widget.SetCloseIntercept(func() { d.widget.Hide() })
	if native, ok := d.widget.(desktopDriver.Window); ok {
		native.RequestAlwaysOnTop()
	}
	d.widget.Show()
	d.status.SetText("Compact widget open • use your window manager's Always on Top / Keep Above action (Wayland policy)")

}

func main() {
	a := app.NewWithID("io.pulse.linux")
	a.Settings().SetTheme(pulseTheme{theme.DefaultTheme()})
	w := a.NewWindow("Pulse — Linux")
	w.SetMaster()
	d := &desktop{a: a, w: w, c: linuxmon.New(), selected: -1, graphs: map[string][]float64{}, gpu: map[string]string{}, vram: map[int32]uint64{}, gpuProc: map[int32]float64{}, body: container.NewStack(), summary: widget.NewLabel("Collecting live data…"), status: widget.NewLabel("Linux • CPU % is normalized across all cores • — means unavailable"), stop: make(chan struct{}), info: linuxmon.SystemInfo()}
	d.search = widget.NewEntry()
	d.search.SetPlaceHolder("Search processes, users or app history")
	d.search.OnChanged = func(string) { d.updateRows() }
	d.table = widget.NewTable(func() (int, int) { return len(d.rows) + 1, len(d.headers) }, func() fyne.CanvasObject {
		return container.NewStack(canvas.NewRectangle(color.Transparent), widget.NewLabel(""))
	}, func(id widget.TableCellID, o fyne.CanvasObject) {
		box := o.(*fyne.Container)
		bg := box.Objects[0].(*canvas.Rectangle)
		label := box.Objects[1].(*widget.Label)
		bg.FillColor = color.Transparent
		label.TextStyle.Bold = id.Row == 0
		if id.Row == 0 {
			if id.Col < len(d.headers) {
				label.SetText(d.headers[id.Col])
			}
		} else if id.Row-1 < len(d.rows) {
			r := d.rows[id.Row-1]
			if id.Col < len(r.cells) {
				label.SetText(r.cells[id.Col])
			}
			if id.Col < len(r.heat) && r.heat[id.Col] > 0 {
				p := resourceColor(d.headers[id.Col])
				p.A = uint8(20 + min(r.heat[id.Col], 1)*145)
				bg.FillColor = p
			}
		}
		bg.Refresh()
	})
	d.table.OnSelected = func(id widget.TableCellID) {
		if id.Row == 0 {
			d.sortCol = id.Col
			d.descending = !d.descending
			d.updateRows()
			return
		}
		d.selected = id.Row - 1
	}
	nav := container.NewVBox(widget.NewLabelWithStyle("PULSE", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	for _, p := range []string{"Processes", "Performance", "App history", "Startup apps", "Users", "Details", "Services", "System info"} {
		page := p
		nav.Add(widget.NewButton(page, func() { d.showPage(page) }))
	}
	nav.Add(widget.NewSeparator())
	nav.Add(widget.NewButton("Compact widget", d.openCompact))
	w.SetContent(container.NewBorder(container.NewVBox(d.summary, d.search), d.status, nav, nil, d.body))
	w.Resize(fyne.NewSize(1280, 820))
	d.showPage("Processes")
	collectorDone := make(chan struct{})
	closing := false
	w.SetCloseIntercept(func() {
		if closing {
			return
		}
		closing = true
		close(d.stop)
		go func() { <-collectorDone; fyne.Do(func() { w.SetCloseIntercept(nil); w.Close() }) }()
	})
	go func() {
		for {
			g, v, u := linuxmon.GPU()
			d.gpuMu.Lock()
			stagingGPU = g
			stagingVRAM = v
			stagingUsage = u
			d.gpuMu.Unlock()
			select {
			case <-d.stop:
				return
			case <-time.After(3 * time.Second):
			}
		}
	}()
	go func() {
		defer close(collectorDone)
		t := time.NewTicker(2 * time.Second)
		defer t.Stop()
		n := 0
		observed := time.Now()
		for {
			s := d.c.Collect()
			now := time.Now()
			d.gpuMu.Lock()
			usage := map[int32]float64{}
			for k, v := range stagingUsage {
				usage[k] = v
			}
			d.gpuMu.Unlock()
			d.c.AddGPUHistory(&s, usage, now.Sub(observed).Seconds())
			observed = now
			n++
			if n%15 == 0 {
				if e := d.c.Save(); e != nil {
					fyne.Do(func() { d.err(e) })
				}
			}
			fyne.DoAndWait(func() { d.tick(s) })
			select {
			case <-d.stop:
				_ = d.c.Save()
				return
			case <-t.C:
			}
		}
	}()
	if p := os.Getenv("PULSE_SCREENSHOT"); p != "" {
		go func() {
			time.Sleep(7 * time.Second)
			if page := os.Getenv("PULSE_SCREENSHOT_PAGE"); page != "" {
				fyne.DoAndWait(func() {
					if page == "Compact widget" {
						d.openCompact()
					} else {
						d.showPage(page)
					}
				})
				time.Sleep(time.Second)
			}
			fyne.Do(func() {
				f, e := os.Create(p)
				if e == nil {
					capture := w.Canvas()
					if os.Getenv("PULSE_SCREENSHOT_PAGE") == "Compact widget" && d.widget != nil {
						capture = d.widget.Canvas()
					}
					e = png.Encode(f, capture.Capture())
					_ = f.Close()
				}
				d.err(e)
			})
		}()
	}
	if duration, e := time.ParseDuration(os.Getenv("PULSE_SMOKE_DURATION")); e == nil && duration > 0 {
		go func() { time.Sleep(duration); fyne.Do(func() { w.Close() }) }()
	}
	w.ShowAndRun()
}
