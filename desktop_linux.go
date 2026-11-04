//go:build linux

package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"pulse/linuxmon"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	desktopDriver "fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var (
	windowBG   = hexColor(0x0b0b0e)
	panelBG    = hexColor(0x0e0e12)
	cardBG     = hexColor(0x121217)
	lineColor  = hexColor(0x24242c)
	textColor  = hexColor(0xececf1)
	dimColor   = hexColor(0x9a9aa6)
	faintColor = hexColor(0x5e5e6a)
)

func hexColor(v uint32) color.NRGBA { return color.NRGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255} }
func blend(a, b color.NRGBA, t float64) color.NRGBA {
	t = max(0, min(1, t))
	return color.NRGBA{uint8(float64(a.R)*(1-t) + float64(b.R)*t), uint8(float64(a.G)*(1-t) + float64(b.G)*t), uint8(float64(a.B)*(1-t) + float64(b.B)*t), 255}
}
func (t pulseTheme) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case theme.SizeNameText:
		return 13
	case theme.SizeNameHeadingText:
		return 21
	case theme.SizeNameSubHeadingText:
		return 16
	case theme.SizeNameCaptionText:
		return 11
	case theme.SizeNamePadding:
		return 4
	case theme.SizeNameInnerPadding:
		return 7
	case theme.SizeNameInputRadius, theme.SizeNameButtonRadius:
		return 7
	case theme.SizeNameScrollBar:
		return 7
	case theme.SizeNameSeparatorThickness:
		return 0
	}
	return t.Theme.Size(n)
}
func text(s string, size float32, c color.Color, bold bool) *canvas.Text {
	t := canvas.NewText(s, c)
	t.TextSize = size
	t.TextStyle.Bold = bold
	return t
}
func rounded(c color.Color, r float32) *canvas.Rectangle {
	x := canvas.NewRectangle(c)
	x.CornerRadius = r
	return x
}
func inset(top, bottom, left, right float32, o fyne.CanvasObject) *fyne.Container {
	return container.New(layout.NewCustomPaddedLayout(top, bottom, left, right), o)
}
func card(o fyne.CanvasObject) *fyne.Container {
	bg := rounded(cardBG, 10)
	bg.StrokeWidth = 1
	bg.StrokeColor = lineColor
	return container.NewStack(bg, inset(18, 18, 18, 18, o))
}
func fixedSize(w, h float32, o fyne.CanvasObject) *fyne.Container {
	return container.New(&fixedLayout{w: w, h: h}, o)
}

type fixedLayout struct{ w, h float32 }

func (l *fixedLayout) MinSize(o []fyne.CanvasObject) fyne.Size {
	s := o[0].MinSize()
	if l.w > 0 {
		s.Width = l.w
	}
	if l.h > 0 {
		s.Height = l.h
	}
	return s
}
func (l *fixedLayout) Layout(o []fyne.CanvasObject, s fyne.Size) {
	h, w := s.Height, s.Width
	if l.h > 0 {
		h = min(h, l.h)
	}
	if l.w > 0 {
		w = min(w, l.w)
	}
	o[0].Move(fyne.NewPos(0, (s.Height-h)/2))
	o[0].Resize(fyne.NewSize(w, h))
}

var iconPaths = map[string]string{
	"Processes":      `<rect x="4" y="5" width="16" height="14" rx="2"/><path d="M4 9h16M9 9v10"/>`,
	"Performance":    `<path d="M3 17l4-5 4 3 4-10 6 4M3 21h18"/>`,
	"App history":    `<path d="M4 10a8 8 0 1 1 0 6M4 4v6h6M12 7v5l3 2"/>`,
	"Startup apps":   `<path d="M12 3l7 8h-4v9H9v-9H5z"/>`,
	"Users":          `<circle cx="9" cy="8" r="3"/><path d="M3 20v-2a6 6 0 0 1 12 0v2M17 5a3 3 0 0 1 0 6M18 14a5 5 0 0 1 3 5"/>`,
	"Details":        `<path d="M8 5h13M8 12h13M8 19h13M3 5h1M3 12h1M3 19h1"/>`,
	"Services":       `<path d="M9 3h6l1 4 4 1v7l-4 1-1 5H9l-1-5-5-1V8l5-1z"/><circle cx="12" cy="12" r="3"/>`,
	"System info":    `<rect x="3" y="4" width="18" height="13" rx="2"/><path d="M8 21h8M12 17v4M12 8v2M12 12v2"/>`,
	"Compact widget": `<rect x="3" y="4" width="18" height="16" rx="2"/><path d="M3 9h18M14 9v11"/>`,
}

func svgIcon(name string, c color.NRGBA) fyne.Resource {
	body := iconPaths[name]
	if body == "" {
		body = `<rect x="4" y="4" width="16" height="16" rx="4"/><path d="M8 9l3 3-3 3M13 15h3"/>`
	}
	return fyne.NewStaticResource(name+".svg", []byte(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="#%02x%02x%02x" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">%s</svg>`, c.R, c.G, c.B, body)))
}
func logoResource() fyne.Resource {
	return fyne.NewStaticResource("pulse.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 28 28"><rect width="28" height="28" rx="8" fill="#1b1630"/><path d="M3 14h5l3-5 3 12 3-16 3 9h5" fill="none" stroke="#8b7cff" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/><path d="M3 14h5l3-5" fill="none" stroke="#4cc2ff" stroke-width="1.8"/></svg>`))
}

type railItem struct {
	widget.BaseWidget
	name, detail  string
	accent        color.NRGBA
	active, hover bool
	tap           func()
	values        []float64
	mini          bool
}

func newRail(name, detail string, c color.NRGBA, mini bool, tap func()) *railItem {
	r := &railItem{name: name, detail: detail, accent: c, mini: mini, tap: tap}
	r.ExtendBaseWidget(r)
	return r
}
func (r *railItem) Tapped(*fyne.PointEvent) {
	if r.tap != nil {
		r.tap()
	}
}
func (r *railItem) MouseIn(*desktopDriver.MouseEvent)    { r.hover = true; r.Refresh() }
func (r *railItem) MouseMoved(*desktopDriver.MouseEvent) {}
func (r *railItem) MouseOut()                            { r.hover = false; r.Refresh() }
func (r *railItem) CreateRenderer() fyne.WidgetRenderer {
	bg := rounded(color.Transparent, 8)
	mark := rounded(r.accent, 1.5)
	icon := canvas.NewImageFromResource(svgIcon(r.name, dimColor))
	icon.FillMode = canvas.ImageFillContain
	name := text(r.name, 12, dimColor, false)
	detail := text(r.detail, 11, dimColor, false)
	mini := graph(r.values, r.accent).(*canvas.Raster)
	return &railRenderer{r: r, bg: bg, mark: mark, icon: icon, name: name, detail: detail, mini: mini}
}

type railRenderer struct {
	r            *railItem
	bg, mark     *canvas.Rectangle
	icon         *canvas.Image
	name, detail *canvas.Text
	mini         *canvas.Raster
}

func (v *railRenderer) Layout(s fyne.Size) {
	v.bg.Resize(s)
	v.mark.Move(fyne.NewPos(0, 10))
	v.mark.Resize(fyne.NewSize(3, s.Height-20))
	if v.r.mini {
		v.icon.Hide()
		v.mini.Show()
		v.mini.Move(fyne.NewPos(11, 10))
		v.mini.Resize(fyne.NewSize(72, s.Height-20))
		v.name.Text = ellipsize(v.r.name, max(0, s.Width-102), 12, fyne.TextStyle{Bold: true})
		v.name.Move(fyne.NewPos(94, 11))
		v.detail.Move(fyne.NewPos(94, 32))
	} else {
		v.mini.Hide()
		v.detail.Hide()
		v.icon.Show()
		v.icon.Move(fyne.NewPos(14, (s.Height-17)/2))
		v.icon.Resize(fyne.NewSize(17, 17))
		v.name.Move(fyne.NewPos(44, (s.Height-v.name.MinSize().Height)/2))
	}
}
func (v *railRenderer) MinSize() fyne.Size {
	if v.r.mini {
		return fyne.NewSize(225, 64)
	}
	return fyne.NewSize(200, 38)
}
func (v *railRenderer) Refresh() {
	v.bg.FillColor = color.Transparent
	v.name.Color = dimColor
	if v.r.mini {
		v.name.Color = textColor
	}
	v.name.TextStyle.Bold = v.r.mini
	v.mark.Hide()
	if v.r.hover {
		v.bg.FillColor = hexColor(0x1c1c24)
	}
	c := dimColor
	if v.r.active {
		v.bg.FillColor = hexColor(0x232336)
		v.name.Color = textColor
		v.name.TextStyle.Bold = true
		v.mark.Show()
		c = v.r.accent
	}
	v.icon.Resource = svgIcon(v.r.name, c)
	v.name.Text = v.r.name
	v.detail.Text = ellipsize(v.r.detail, max(0, v.r.Size().Width-102), 11, fyne.TextStyle{})
	v.mini.Generator = graphGenerator(append([]float64(nil), v.r.values...), v.r.accent)
	canvas.Refresh(v.r)
	v.Layout(v.r.Size())
}
func (v *railRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{v.bg, v.mark, v.icon, v.name, v.detail, v.mini}
}
func (v *railRenderer) Destroy() {}

// Text uses the toolkit's measured glyphs and scales with the desktop DPI.
type pulseCell struct {
	widget.BaseWidget
	value, metric            string
	header, numeric, process bool
	ink, fill                color.NRGBA
}

func newCell() *pulseCell { c := &pulseCell{}; c.ExtendBaseWidget(c); return c }
func (c *pulseCell) CreateRenderer() fyne.WidgetRenderer {
	return &cellRenderer{c: c, bg: canvas.NewRectangle(panelBG), primary: text("", 13, textColor, false), metric: text("", 15, textColor, true), icon: canvas.NewImageFromResource(svgIcon("process", faintColor))}
}

type cellRenderer struct {
	c               *pulseCell
	bg              *canvas.Rectangle
	primary, metric *canvas.Text
	icon            *canvas.Image
}

func (v *cellRenderer) MinSize() fyne.Size { return fyne.NewSize(60, 28) }
func (v *cellRenderer) Layout(s fyne.Size) {
	v.bg.Resize(s)
	x := float32(10)
	if v.c.process && !v.c.header {
		x = 34
		v.icon.Move(fyne.NewPos(10, (s.Height-14)/2))
		v.icon.Resize(fyne.NewSize(14, 14))
		v.icon.Show()
	} else {
		v.icon.Hide()
	}
	v.primary.Text = ellipsize(v.c.value, s.Width-x-10, v.primary.TextSize, v.primary.TextStyle)
	v.primary.Resize(fyne.NewSize(max(0, s.Width-x-10), v.primary.MinSize().Height))
	v.primary.Move(fyne.NewPos(x, (s.Height-v.primary.MinSize().Height)/2))
	v.metric.Resize(fyne.NewSize(s.Width-20, v.metric.MinSize().Height))
	v.metric.Move(fyne.NewPos(10, 4))
	if v.c.header && v.c.metric != "" {
		v.metric.Show()
		v.primary.Move(fyne.NewPos(x, 27))
	} else {
		v.metric.Hide()
	}
}
func (v *cellRenderer) Refresh() {
	v.bg.FillColor = v.c.fill
	v.primary.Color = textColor
	v.primary.TextSize = 13
	v.primary.TextStyle.Bold = false
	v.primary.Alignment = fyne.TextAlignLeading
	v.metric.Alignment = fyne.TextAlignTrailing
	if v.c.numeric {
		v.primary.Alignment = fyne.TextAlignTrailing
	}
	if v.c.header {
		v.primary.Color = dimColor
		v.primary.TextSize = 11
	}
	v.metric.Text = v.c.metric
	v.metric.Color = v.c.ink
	v.Layout(v.c.Size())
	canvas.Refresh(v.c)
}
func (v *cellRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{v.bg, v.primary, v.metric, v.icon}
}
func (v *cellRenderer) Destroy() {}
func ellipsize(s string, width, size float32, style fyne.TextStyle) string {
	if fyne.MeasureText(s, size, style).Width <= width {
		return s
	}
	r := []rune(s)
	for len(r) > 0 {
		r = r[:len(r)-1]
		if fyne.MeasureText(string(r)+"…", size, style).Width <= width {
			return string(r) + "…"
		}
	}
	return ""
}

type tableLayout struct {
	d       *desktop
	width   float32
	columns int
	page    string
}

func (l *tableLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(620, 200) }
func (l *tableLayout) Layout(o []fyne.CanvasObject, s fyne.Size) {
	d := l.d
	if l.width != s.Width || l.columns != len(d.headers) || l.page != d.page {
		l.width = s.Width
		l.columns = len(d.headers)
		l.page = d.page
		columns := len(d.headers)
		if columns > 0 {
			widths := make([]float32, columns)
			for i := range widths {
				widths[i] = 110
			}
			switch d.page {
			case "Processes":
				widths = []float32{max(240, s.Width-542), 80, 110, 110, 80, 120}
			case "Users":
				widths = []float32{max(230, s.Width-542), 80, 90, 110, 110, 120}
			case "Details":
				widths = []float32{210, 72, 110, 80, 110, 110, 80, 115, 65, 75, 110}
			case "Services":
				widths = []float32{250, 80, 85, 85, max(240, s.Width-524)}
			case "Startup apps":
				widths = []float32{220, 90, max(270, s.Width-474), 140}
			case "App history":
				widths = []float32{max(250, s.Width-464), 110, 110, 110, 110}
			}
			for i, w := range widths {
				if i < columns {
					d.table.SetColumnWidth(i, w)
				}
			}
			d.table.SetRowHeight(0, 50)
		}
	}
	o[0].Move(fyne.NewPos(0, 0))
	o[0].Resize(s)
}
func (d *desktop) createTable() *widget.Table {
	table := widget.NewTable(func() (int, int) { return len(d.rows) + 1, len(d.headers) }, func() fyne.CanvasObject { return newCell() }, func(id widget.TableCellID, o fyne.CanvasObject) {
		c := o.(*pulseCell)
		c.value = ""
		c.metric = ""
		c.header = id.Row == 0
		c.numeric = id.Col > 0
		c.process = id.Col == 0 && (d.page == "Processes" || d.page == "Details")
		c.fill = panelBG
		c.ink = textColor
		if id.Col >= len(d.headers) {
			return
		}
		if c.header {
			c.value = d.headers[id.Col]
			c.ink = resourceColor(c.value)
			if d.page == "Processes" {
				c.metric = d.columnTotal(c.value)
			}
		} else if id.Row-1 < len(d.rows) {
			r := d.rows[id.Row-1]
			if id.Col < len(r.cells) {
				c.value = r.cells[id.Col]
			}
			if id.Col > 0 && id.Col < len(r.heat) && r.heat[id.Col] >= 0 && (d.page == "Processes" || r.heat[id.Col] > 0) {
				strength := min(1, r.heat[id.Col])
				c.fill = blend(panelBG, resourceColor(d.headers[id.Col]), .055+.56*math.Pow(strength, .55))
			}
		}
		if !c.header && id.Row-1 == d.selected {
			c.fill = blend(c.fill, hexColor(0x393955), .45)
		}
		c.Refresh()
	})
	table.ShowHeaderRow = false
	table.StickyRowCount = 1
	table.HideSeparators = true
	table.OnSelected = func(id widget.TableCellID) {
		if id.Row == 0 {
			d.sortCol = id.Col
			d.descending = !d.descending
			d.updateRows()
			return
		}
		d.selected = id.Row - 1
		for _, b := range d.actionButtons {
			b.Enable()
		}
		d.table.Refresh()
	}
	return table
}
func (d *desktop) columnTotal(header string) string {
	switch header {
	case "CPU":
		v := 0.0
		for _, c := range d.s.CPU {
			v += c
		}
		return pct(v / float64(max(1, len(d.s.CPU))))
	case "Memory":
		return pct(float64(d.s.Memory) / float64(max(d.s.Total, 1)) * 100)
	case "Disk":
		v := 0.0
		for _, io := range d.s.Disks {
			v += io[0] + io[1]
		}
		return bytes(v) + "/s"
	case "GPU":
		if v := d.gpu["utilization.gpu"]; v != "" {
			return v + "%"
		}
	case "GPU memory":
		if v := d.gpu["memory.used"]; v != "" {
			return bytes(sortValue(v) * 1048576)
		}
	}
	return ""
}

// Cardinal interpolation produces a thin, continuous trace over a translucent fill.
// Values are immutable copies; render workers never touch the live collector maps.
func graphGenerator(values []float64, c color.NRGBA) func(int, int) image.Image {
	return func(w, h int) image.Image {
		out := image.NewNRGBA(image.Rect(0, 0, w, h))
		if w < 2 || h < 2 {
			return out
		}
		heights := make([]float64, w)
		for x := 0; x < w; x++ {
			heights[x] = -1
			t := float64(x)/float64(w-1)*59 - float64(60-len(values))
			if len(values) == 0 || t < 0 {
				continue
			}
			i := int(t)
			f := t - float64(i)
			at := func(j int) float64 { return values[max(0, min(len(values)-1, j))] }
			p0, p1, p2, p3 := at(i-1), at(i), at(i+1), at(i+2)
			v := .5 * ((2 * p1) + (-p0+p2)*f + (2*p0-5*p1+4*p2-p3)*f*f + (-p0+3*p1-3*p2+p3)*f*f*f)
			heights[x] = float64(h-2)*(1-max(0, min(100, v))/100) + 1
		}
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				bg := hexColor(0x0b0b0e)
				grid := x%max(w/6, 1) == 0 || y%max(h/4, 1) == 0
				if grid {
					bg = hexColor(0x202027)
				}
				lineY := heights[x]
				if lineY >= 0 && float64(y) >= lineY {
					alpha := .08 + .29*(1-float64(y)/float64(h))
					bg = blend(bg, c, alpha)
				}
				if lineY >= 0 {
					dy := 0.0
					if x > 0 && heights[x-1] >= 0 {
						dy = lineY - heights[x-1]
					}
					distance := math.Abs(float64(y)-lineY) / math.Sqrt(1+dy*dy)
					if distance < 1.2 {
						bg = blend(bg, c, min(1, 1.4-distance))
					}
				}
				if x == 0 || y == 0 || x == w-1 || y == h-1 {
					bg = blend(hexColor(0x202027), c, .45)
				}
				out.SetNRGBA(x, y, bg)
			}
		}
		return out
	}
}

type shellHeaderLayout struct{}

func (*shellHeaderLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(900, 48) }
func (*shellHeaderLayout) Layout(o []fyne.CanvasObject, s fyne.Size) {
	o[0].Move(fyne.NewPos(18, 10))
	o[0].Resize(fyne.NewSize(180, 28))
	width := min(float32(460), s.Width*.38)
	o[1].Move(fyne.NewPos((s.Width-width)/2, 8))
	o[1].Resize(fyne.NewSize(width, 32))
	o[2].Move(fyne.NewPos(s.Width-42, 8))
	o[2].Resize(fyne.NewSize(32, 32))
}
func (d *desktop) shell() fyne.CanvasObject {
	d.navItems = map[string]*railItem{}
	nav := container.NewVBox()
	for i, p := range []string{"Processes", "Performance", "App history", "Startup apps", "Users", "Details", "Services", "System info"} {
		page := p
		c := []color.NRGBA{resourceColor("CPU"), resourceColor("GPU"), resourceColor("Network"), resourceColor("Disk"), resourceColor("Memory"), hexColor(0xffd166), hexColor(0x5c9dff), hexColor(0x8b7cff)}[i]
		item := newRail(p, "", c, false, func() { d.showPage(page) })
		d.navItems[p] = item
		nav.Add(item)
	}
	compact := newRail("Compact widget", "", dimColor, false, d.openCompact)
	side := fixedSize(212, 0, inset(4, 8, 6, 6, container.NewBorder(nil, compact, nil, nil, nav)))
	panel := rounded(panelBG, 12)
	panel.StrokeWidth = 1
	panel.StrokeColor = lineColor
	content := inset(0, 0, 0, 7, container.NewStack(panel, inset(12, 12, 18, 18, d.body)))
	logo := canvas.NewImageFromResource(logoResource())
	logo.FillMode = canvas.ImageFillContain
	brand := container.NewHBox(fixedSize(28, 28, logo), text("Pulse", 15, textColor, true), inset(4, 0, 8, 0, text("Linux", 11, faintColor, false)))
	compactButton := widget.NewButtonWithIcon("", svgIcon("Compact widget", dimColor), d.openCompact)
	compactButton.Importance = widget.LowImportance
	top := container.New(&shellHeaderLayout{}, brand, d.search, compactButton)
	d.footer = container.NewHBox(text("●", 8, dimColor, false), text("Collecting live data…", 11, dimColor, false))
	d.status.SetText("Updating every 1 s")
	d.status.TextStyle = fyne.TextStyle{}
	d.status.Truncation = fyne.TextTruncateEllipsis
	bottom := fixedSize(0, 28, inset(0, 0, 16, 12, container.NewBorder(nil, nil, d.footer, fixedSize(245, 0, d.status), nil)))
	return container.NewBorder(top, bottom, nil, nil, container.NewBorder(nil, nil, side, nil, content))
}
func (d *desktop) updateFooter() {
	items := []fyne.CanvasObject{}
	add := func(label, value string, c color.NRGBA) {
		items = append(items, inset(0, 0, 0, 12, container.NewHBox(text("•", 12, c, true), text(label, 11, dimColor, false), text(value, 11, textColor, true))))
	}
	add("Processes", fmt.Sprint(len(d.s.Procs)), dimColor)
	add("CPU", d.columnTotal("CPU"), resourceColor("CPU"))
	add("Memory", d.columnTotal("Memory"), resourceColor("Memory"))
	add("GPU", d.columnTotal("GPU"), resourceColor("GPU"))
	add("VRAM", d.columnTotal("GPU memory"), resourceColor("VRAM"))
	d.footer.Objects = items
	d.footer.Refresh()
}
func (d *desktop) refreshNav() {
	for p, item := range d.navItems {
		item.active = p == d.page
		item.Refresh()
	}
}

func (d *desktop) performance() fyne.CanvasObject {
	if d.perfMode == "" {
		d.perfMode = "CPU"
	}
	d.resourceItems = map[string]*railItem{}
	d.resourceRail = container.NewVBox()
	d.performanceDetail = container.NewStack()
	d.perfScroll = container.NewVScroll(d.performanceDetail)
	d.refreshPerformance()
	left := fixedSize(230, 0, container.NewVScroll(d.resourceRail))
	return container.NewBorder(nil, nil, inset(0, 0, 0, 20, left), nil, d.perfScroll)
}
func (d *desktop) refreshPerformance() {
	if d.performanceDetail == nil {
		return
	}
	type resource struct{ key, name, detail, graph string }
	resources := []resource{{"CPU", "CPU", d.columnTotal("CPU"), "CPU total"}, {"Memory", "Memory", bytes(float64(d.s.Memory)) + " / " + bytes(float64(d.s.Total)), "Memory"}}
	keys := []string{}
	for k := range d.s.Disks {
		keys = append(keys, k)
	}
	sortStrings(keys)
	for i, k := range keys {
		v := d.s.Disks[k]
		resources = append(resources, resource{"Disk:" + k, fmt.Sprintf("Disk %d", i), k + " • " + bytes(v[0]+v[1]) + "/s", "Disk " + k + " read"})
	}
	keys = nil
	for k := range d.s.Networks {
		if k != "lo" {
			keys = append(keys, k)
		}
	}
	sortStrings(keys)
	for _, k := range keys {
		v := d.s.Networks[k]
		resources = append(resources, resource{"Network:" + k, "Network", k + " • " + bytes(v[0]+v[1]) + "/s", "Network " + k + " receive"})
	}
	resources = append(resources, resource{"GPU", "GPU 0", d.gpu["name"], "GPU"})
	objects := []fyne.CanvasObject{}
	for _, r := range resources {
		item := d.resourceItems[r.key]
		if item == nil {
			key := r.key
			item = newRail(r.name, r.detail, resourceColor(r.graph), true, func() { d.perfMode = key; d.refreshPerformance() })
			d.resourceItems[r.key] = item
		}
		item.detail = r.detail
		if r.key == "GPU" {
			item.detail = strings.TrimPrefix(r.detail, "NVIDIA ")
		}
		item.values = append([]float64(nil), d.graphs[r.graph]...)
		if strings.HasPrefix(r.key, "Disk:") || strings.HasPrefix(r.key, "Network:") {
			normalize(item.values)
		}
		item.active = d.perfMode == r.key
		item.Refresh()
		objects = append(objects, item)
	}
	d.resourceRail.Objects = objects
	d.resourceRail.Refresh()
	d.performanceDetail.Objects = []fyne.CanvasObject{d.performanceCard()}
	d.performanceDetail.Refresh()
}
func sortStrings(s []string) { sort.Strings(s) }
func normalize(v []float64) float64 {
	peak := 1.0
	for _, x := range v {
		peak = max(peak, x)
	}
	for i := range v {
		v[i] = v[i] / peak * 100
	}
	return peak
}
func (d *desktop) plot(key, label string, height float32, rate bool) fyne.CanvasObject {
	v := append([]float64(nil), d.graphs[key]...)
	last := 0.0
	if len(v) > 0 {
		last = v[len(v)-1]
	}
	value := pct(last)
	if rate {
		peak := normalize(v)
		value = bytes(last) + "/s • max " + bytes(peak) + "/s"
	}
	header := container.NewBorder(nil, nil, text(label, 11, dimColor, false), text(value, 11, dimColor, false), nil)
	return container.NewVBox(header, fixedSize(0, height, graph(v, resourceColor(key))))
}
func stat(label, value string) fyne.CanvasObject {
	return container.NewVBox(text(label, 11, dimColor, false), text(value, 17, textColor, true))
}
func smallStat(label, value string) fyne.CanvasObject {
	return container.NewBorder(nil, nil, text(label, 11, dimColor, false), text(value, 11, textColor, false), nil)
}
func (d *desktop) performanceCard() fyne.CanvasObject {
	mode := d.perfMode
	title, subtitle := "CPU", ""
	content := container.NewVBox()
	if mode == "GPU" {
		title = "GPU 0"
		subtitle = strings.TrimPrefix(d.gpu["name"], "NVIDIA ")
	} else if mode == "Memory" {
		title = "Memory"
		subtitle = bytes(float64(d.s.Total))
	} else if strings.HasPrefix(mode, "Disk:") {
		title = "Disk"
		subtitle = strings.TrimPrefix(mode, "Disk:")
	} else if strings.HasPrefix(mode, "Network:") {
		title = "Network"
		subtitle = strings.TrimPrefix(mode, "Network:")
	} else {
		for _, line := range strings.Split(d.info, "\n") {
			if strings.HasPrefix(line, "CPU:") {
				subtitle = strings.TrimSpace(strings.TrimPrefix(line, "CPU:"))
				break
			}
		}
	}
	heading := container.NewBorder(nil, nil, text(title, 26, textColor, true), text(subtitle, 13, resourceColor(mode), true), nil)
	content.Add(heading)
	content.Add(fixedSize(0, 8, canvas.NewRectangle(color.Transparent)))
	switch {
	case mode == "GPU":
		charts := container.NewGridWithColumns(2, d.plot("GPU", "GPU utilization", 128, false), d.plot("GPU memory controller", "Memory controller", 128, false), d.plot("GPU encode", "Video encode", 100, false), d.plot("GPU decode", "Video decode", 100, false))
		content.Add(charts)
		content.Add(d.plot("VRAM", "Dedicated GPU memory", 48, false))
		g := d.gpu
		content.Add(inset(12, 0, 0, 0, container.NewGridWithColumns(3, stat("Utilization", d.columnTotal("GPU")), stat("Dedicated GPU memory", bytes(sortValue(g["memory.used"])*1048576)+" / "+bytes(sortValue(g["memory.total"])*1048576)), stat("Temperature", g["temperature.gpu"]+" °C"), stat("Power", g["power.draw"]+" / "+g["power.limit"]+" W"), stat("Graphics clock", g["clocks.gr"]+" MHz"), stat("Fan speed", g["fan.speed"]+"%"))))
		content.Add(inset(10, 0, 0, 0, container.NewVBox(smallStat("Driver version", g["driver_version"]), smallStat("Memory clock", g["clocks.mem"]+" MHz"))))
		if g["status"] != "" {
			content.Add(text(g["status"], 12, dimColor, false))
		}
	case mode == "Memory":
		content.Add(d.plot("Memory", "Memory usage", 260, false))
		content.Add(text("Memory composition", 11, dimColor, false))
		used := float64(d.s.Memory) / float64(max(d.s.Total, 1))
		cached := min(float64(d.s.Cached)/float64(max(d.s.Total, 1)), 1-used)
		bar := canvas.NewRasterWithPixels(func(x, y, w, h int) color.Color {
			v := float64(x) / float64(max(w, 1))
			if v < used {
				return resourceColor("Memory")
			}
			if v < used+cached {
				return hexColor(0x626ac0)
			}
			return hexColor(0x2a2a35)
		})
		content.Add(fixedSize(0, 20, bar))
		content.Add(container.NewHBox(text("● In use", 11, resourceColor("Memory"), false), inset(0, 0, 20, 0, text("● Cached", 11, hexColor(0x626ac0), false)), inset(0, 0, 20, 0, text("● Available", 11, faintColor, false))))
		content.Add(inset(14, 0, 0, 0, container.NewGridWithColumns(3, stat("In use", bytes(float64(d.s.Memory))), stat("Available", bytes(float64(d.s.Available))), stat("Total", bytes(float64(d.s.Total))), stat("Cached", bytes(float64(d.s.Cached))), stat("Swap used", bytes(float64(d.s.SwapUsed))), stat("Swap total", bytes(float64(d.s.SwapTotal))))))
	case strings.HasPrefix(mode, "Disk:"):
		k := strings.TrimPrefix(mode, "Disk:")
		v := d.s.Disks[k]
		content.Add(d.plot("Disk "+k+" read", "Read speed", 180, true))
		content.Add(d.plot("Disk "+k+" write", "Write speed", 180, true))
		content.Add(inset(12, 0, 0, 0, container.NewGridWithColumns(3, stat("Read speed", bytes(v[0])+"/s"), stat("Write speed", bytes(v[1])+"/s"), stat("Active time", pct(d.s.DiskActivity[k])))))
	case strings.HasPrefix(mode, "Network:"):
		k := strings.TrimPrefix(mode, "Network:")
		v := d.s.Networks[k]
		content.Add(d.plot("Network "+k+" receive", "Receive", 180, true))
		content.Add(d.plot("Network "+k+" send", "Send", 180, true))
		content.Add(inset(12, 0, 0, 0, container.NewGridWithColumns(2, stat("Receive", bytes(v[0])+"/s"), stat("Send", bytes(v[1])+"/s"))))
	default:
		content.Add(d.plot("CPU total", "Utilization", 210, false))
		threads := int32(0)
		for _, p := range d.s.Procs {
			threads += p.Threads
		}
		content.Add(inset(12, 10, 0, 0, container.NewGridWithColumns(3, stat("Utilization", d.columnTotal("CPU")), stat("Processes", fmt.Sprint(len(d.s.Procs))), stat("Threads", fmt.Sprint(threads)))))
		content.Add(text("Logical processors", 11, dimColor, false))
		cores := container.NewGridWithColumns(4)
		for i := range d.s.CPU {
			cores.Add(d.plot(fmt.Sprintf("CPU %02d", i), fmt.Sprintf("CPU %d", i), 70, false))
		}
		content.Add(cores)
	}
	content.Add(container.NewBorder(nil, nil, text("60 seconds", 10, faintColor, false), text("0", 10, faintColor, false), nil))
	return card(content)
}
func (d *desktop) systemInfoView(info string) fyne.CanvasObject {
	sections := append([]linuxmon.InfoSection(nil), d.sections...)
	graphics := linuxmon.InfoSection{Title: "Graphics", Rows: [][2]string{{"GPU", d.gpu["name"]}, {"Driver", d.gpu["driver_version"]}, {"Dedicated memory", bytes(sortValue(d.gpu["memory.total"]) * 1048576)}}}
	at := min(3, len(sections))
	sections = append(sections[:at], append([]linuxmon.InfoSection{graphics}, sections[at:]...)...)
	summaries := container.NewGridWithColumns(4)
	for _, r := range []struct{ label, value, meta string }{{"Linux", sectionValue(sections, "Linux", "Distribution"), sectionValue(sections, "Linux", "Kernel")}, {"Processor", sectionValue(sections, "Processor", "Processor"), sectionValue(sections, "Processor", "Logical processors") + " logical processors"}, {"Memory", sectionValue(sections, "Memory", "Usable RAM"), "Kernel-visible memory"}, {"Graphics", strings.TrimPrefix(d.gpu["name"], "NVIDIA "), bytes(sortValue(d.gpu["memory.total"])*1048576) + " VRAM"}} {
		c := resourceColor(r.label)
		if r.label == "Linux" {
			c = hexColor(0x8b7cff)
		}
		value := widget.NewLabelWithStyle(r.value, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		value.Truncation = fyne.TextTruncateEllipsis
		label := text(r.label, 11, dimColor, false)
		meta := text(r.meta, 11, c, false)
		bg := rounded(blend(cardBG, c, .07), 9)
		bg.StrokeWidth = 1
		bg.StrokeColor = blend(lineColor, c, .4)
		summaries.Add(container.NewStack(bg, inset(12, 12, 14, 14, container.NewVBox(label, value, meta))))
	}
	left, right := container.NewVBox(), container.NewVBox()
	grid := container.NewGridWithColumns(2, left, right)
	for i, section := range sections {
		rows := container.NewVBox(text(section.Title, 14, textColor, true), fixedSize(0, 5, canvas.NewRectangle(color.Transparent)))
		for _, pair := range section.Rows {
			label := text(pair[0], 12, dimColor, false)
			value := widget.NewLabel(pair[1])
			value.Wrapping = fyne.TextWrapWord
			rows.Add(container.NewBorder(nil, nil, fixedSize(140, 0, label), nil, value))
		}
		if i%2 == 0 {
			left.Add(card(rows))
		} else {
			right.Add(card(rows))
		}
	}
	return container.NewVScroll(container.NewVBox(summaries, fixedSize(0, 10, canvas.NewRectangle(color.Transparent)), grid))
}
func sectionValue(sections []linuxmon.InfoSection, title, key string) string {
	for _, s := range sections {
		if s.Title == title {
			for _, r := range s.Rows {
				if r[0] == key {
					return r[1]
				}
			}
		}
	}
	return "Unavailable"
}
