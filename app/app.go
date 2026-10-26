//go:build windows

// Package app is the Pulse window: a Task-Manager-style monitor drawn with GDI/GDI+.
package app

import (
	"runtime"
	"runtime/debug"
	"syscall"
	"time"
	"unsafe"

	"pulse/sys"
	"pulse/ui"
	"pulse/win"
)

const (
	wmData    = win.WM_APP + 1
	timerAnim = 1
	timerTrim = 2
	className = "PulseMonitorWindow"
	Title     = "Pulse"
)

// Page indices
const (
	PProcesses = iota
	PPerformance
	PAppHistory
	PStartup
	PUsers
	PDetails
	PServices
	PSysInfo
	PSettings
)

var pageNames = []string{"Processes", "Performance", "App history", "Startup apps", "Users", "Details", "Services", "System info", "Settings"}
var pageGlyphs = []rune{GProcesses, GPerf, GHistory, GStartup, GUsers, GDetails, GServices, GSysInfo, GSettings}

type Zone struct {
	X, Y, W, H float32
	ID         string
	Cursor     uintptr
	Click      func()
	Right      func()
	Dbl        func()
	Wheel      func(d float32)
	Drag       func(x, y float32)
	Caption    bool // part of title bar drag area (no click handler)
}

type App struct {
	hwnd   uintptr
	cv     *ui.Canvas
	col    *sys.Collector
	set    Settings
	static *sys.StaticInfo

	memDC   uintptr
	memBmp  uintptr
	bmpW    int32
	bmpH    int32
	scale   float32

	zones    []Zone
	hover    string
	pressed  string
	dragZone *Zone
	mx, my   float32

	// text input
	search      string
	searchFocus bool

	toast   string
	toastAt time.Time

	icons map[string]uintptr

	pages  map[int]page
	cursorArrow, cursorHand, cursorIBeam, cursorSizeWE uintptr
}

type page interface {
	draw(a *App, c *ui.Canvas, x, y, w, h float32)
}

var theApp *App

func Run(demo, restarted bool) {
	runtime.LockOSThread()
	if !demo && !win.SingleInstance(`Local\PulseMonitorSingleInstance`, className, restarted) {
		return
	}
	debug.SetGCPercent(40)
	debug.SetMemoryLimit(40 << 20)

	win.SetDpiAware()
	win.GdiplusStartup()
	win.DarkMenus()
	if f := win.RegString(win.HKEY_LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, "CurrentBuildNumber"); f < "22000" && len(f) == 5 {
		ui.IconFace = "Segoe MDL2 Assets"
		ui.UIFace = "Segoe UI"
	}
	if demo {
		ui.UIFace = "Tahoma"
		ui.MonoFace = "Courier New"
	}

	a := &App{cv: ui.NewCanvas(), set: loadSettings(), icons: map[string]uintptr{}, scale: 1}
	theApp = a
	a.pages = map[int]page{
		PProcesses: &procPage{}, PPerformance: &perfPage{}, PAppHistory: &histPage{}, PStartup: &startupPage{},
		PUsers: &usersPage{}, PDetails: &detailsPage{}, PServices: &servicesPage{}, PSysInfo: &sysinfoPage{}, PSettings: &settingsPage{},
	}
	a.cursorArrow = win.LoadCursor(win.IDC_ARROW)
	a.cursorHand = win.LoadCursor(win.IDC_HAND)
	a.cursorIBeam = win.LoadCursor(win.IDC_IBEAM)
	a.cursorSizeWE = win.LoadCursor(win.IDC_SIZEWE)

	a.col = sys.NewCollector(func() { win.PostMessage(a.hwnd, wmData, 0, 0) })
	a.col.Demo = demo
	a.applySpeed()

	inst := win.ModuleHandle()
	wc := win.WNDCLASSEX{
		Style:     win.CS_DBLCLKS,
		WndProc:   syscall.NewCallback(wndProc),
		Instance:  inst,
		Cursor:    a.cursorArrow,
		Icon:      win.LoadIcon(inst, 1),
		IconSm:    win.LoadIcon(inst, 1),
		ClassName: win.StrPtr(className),
	}
	wc.Size = uint32(unsafe.Sizeof(wc))
	win.RegisterClassEx(&wc)

	x, y, w, h := a.set.WX, a.set.WY, a.set.WW, a.set.WH
	if a.set.Compact {
		x, y, w, h = a.set.CX, a.set.CY, compactW, compactH
	}
	if x < 0 || y < 0 {
		x, y = 120, 80
	}
	ex := uint32(win.WS_EX_APPWINDOW)
	if a.set.OnTop || a.set.Compact {
		ex |= win.WS_EX_TOPMOST
	}
	a.hwnd = win.CreateWindowEx(ex, className, Title, win.WS_OVERLAPPEDWINDOW, x, y, 100, 100, 0)
	a.scale = float32(win.GetDpiForWindow(a.hwnd)) / 96
	// apply logical size at the window's DPI
	sw, sh := int32(float32(w)*a.scale), int32(float32(h)*a.scale)
	win.SetWindowPos(a.hwnd, 0, 0, 0, sw, sh, win.SWP_NOMOVE|win.SWP_NOZORDER|win.SWP_FRAMECHANGED)
	win.DarkTitleBar(a.hwnd)
	win.FrameShadow(a.hwnd)
	if a.set.Maximized && !a.set.Compact {
		win.ShowWindow(a.hwnd, win.SW_MAXIMIZE)
	} else {
		win.ShowWindow(a.hwnd, win.SW_SHOW)
	}
	win.UpdateWindow(a.hwnd)
	win.EcoMode() // ask Windows to run Pulse efficiently
	win.SetTimer(a.hwnd, timerTrim, 20000)

	debugLog()
	go a.col.Run()
	go func() {
		st := sys.CollectStatic(a.col.Snap.GPU.Name, a.col.Snap.GPU.Driver)
		a.static = st
		win.PostMessage(a.hwnd, wmData, 0, 0)
	}()

	var m win.MSG
	for win.GetMessage(&m) > 0 {
		win.TranslateMessage(&m)
		win.DispatchMessage(&m)
	}
	a.col.App.Save()
}

func (a *App) applySpeed() {
	if a.set.SpeedMs <= 0 {
		a.col.Paused = true
		return
	}
	a.col.Paused = false
	a.col.Interval = time.Duration(a.set.SpeedMs) * time.Millisecond
	a.col.Wake()
}

func (a *App) showToast(s string) {
	a.toast, a.toastAt = s, time.Now()
	win.SetTimer(a.hwnd, timerAnim, 3200)
	win.Invalidate(a.hwnd)
}

func loword(l uintptr) int32 { return int32(int16(l & 0xFFFF)) }
func hiword(l uintptr) int32 { return int32(int16((l >> 16) & 0xFFFF)) }

func wndProc(hwnd uintptr, msg uint32, wp, lp uintptr) uintptr {
	a := theApp
	if a == nil || (a.hwnd != 0 && hwnd != a.hwnd) {
		return win.DefWindowProc(hwnd, msg, wp, lp)
	}
	switch msg {
	case win.WM_NCCALCSIZE:
		if wp != 0 {
			if win.IsZoomed(hwnd) {
				r := (*win.RECT)(unsafe.Pointer(lp))
				*r = win.MonitorWorkArea(hwnd)
			}
			return 0
		}
	case win.WM_NCHITTEST:
		return a.hitTest(loword(lp), hiword(lp))
	case win.WM_ERASEBKGND:
		return 1
	case win.WM_PAINT:
		a.paint()
		return 0
	case wmData:
		if !win.IsIconic(hwnd) {
			win.Invalidate(hwnd)
		}
		return 0
	case win.WM_TIMER:
		switch wp {
		case timerAnim:
			win.KillTimer(hwnd, timerAnim)
			win.Invalidate(hwnd)
		case timerTrim:
			if a.set.TrimMemory {
				runtime.GC()
				debug.FreeOSMemory()
				if win.IsIconic(hwnd) || a.set.Compact {
					win.TrimSelf()
				}
			}
		}
		return 0
	case win.WM_SIZE:
		if wp == 1 { // minimized: drop cached bitmap + trim memory
			a.freeBitmap()
			if a.set.TrimMemory {
				debug.FreeOSMemory()
				win.TrimSelf()
			}
		}
		win.Invalidate(hwnd)
		return 0
	case win.WM_DPICHANGED:
		a.scale = float32(hiword(wp)) / 96
		r := (*win.RECT)(unsafe.Pointer(lp))
		win.SetWindowPos(hwnd, 0, r.Left, r.Top, r.W(), r.H(), win.SWP_NOZORDER|win.SWP_NOACTIVATE)
		a.clearIcons()
		win.Invalidate(hwnd)
		return 0
	case win.WM_GETMINMAXINFO:
		mm := (*win.MINMAXINFO)(unsafe.Pointer(lp))
		s := a.scale
		if s == 0 {
			s = 1
		}
		if a.set.Compact {
			mm.MinTrackSize = win.POINT{X: int32(300 * s), Y: int32(160 * s)}
		} else {
			mm.MinTrackSize = win.POINT{X: int32(720 * s), Y: int32(460 * s)}
		}
		return 0
	case win.WM_MOUSEMOVE:
		a.mouseMove(float32(loword(lp))/a.scale, float32(hiword(lp))/a.scale)
		return 0
	case win.WM_MOUSELEAVE:
		if a.hover != "" {
			a.hover = ""
			win.Invalidate(hwnd)
		}
		return 0
	case win.WM_LBUTTONDOWN:
		a.mouseDown(float32(loword(lp))/a.scale, float32(hiword(lp))/a.scale)
		return 0
	case win.WM_LBUTTONUP:
		a.mouseUp(float32(loword(lp))/a.scale, float32(hiword(lp))/a.scale)
		return 0
	case win.WM_LBUTTONDBLCLK:
		if z := a.zoneAt(float32(loword(lp))/a.scale, float32(hiword(lp))/a.scale); z != nil {
			if z.Dbl != nil {
				z.Dbl()
			} else if z.Click != nil {
				z.Click()
			}
			win.Invalidate(hwnd)
		}
		return 0
	case win.WM_RBUTTONUP:
		if z := a.zoneAt(float32(loword(lp))/a.scale, float32(hiword(lp))/a.scale); z != nil && z.Right != nil {
			z.Right()
			win.Invalidate(hwnd)
		}
		return 0
	case win.WM_MOUSEWHEEL:
		p := win.ScreenToClient(hwnd, win.POINT{X: loword(lp), Y: hiword(lp)})
		d := float32(hiword(wp)) / 120
		for i := len(a.zones) - 1; i >= 0; i-- {
			z := &a.zones[i]
			if z.Wheel != nil && a.inZone(z, float32(p.X)/a.scale, float32(p.Y)/a.scale) {
				z.Wheel(d)
				win.Invalidate(hwnd)
				break
			}
		}
		return 0
	case win.WM_CHAR:
		a.char(rune(wp))
		return 0
	case win.WM_KEYDOWN:
		a.key(int(wp))
		return 0
	case win.WM_CLOSE:
		a.saveWindowPos()
		win.DestroyWindow(hwnd)
		return 0
	case win.WM_DESTROY:
		win.PostQuitMessage(0)
		return 0
	}
	return win.DefWindowProc(hwnd, msg, wp, lp)
}

func (a *App) saveWindowPos() {
	if win.IsIconic(a.hwnd) {
		a.set.save()
		return
	}
	r := win.GetWindowRect(a.hwnd)
	if a.set.Compact {
		a.set.CX, a.set.CY = r.Left, r.Top
	} else if win.IsZoomed(a.hwnd) {
		a.set.Maximized = true
	} else {
		a.set.Maximized = false
		a.set.WX, a.set.WY = r.Left, r.Top
		a.set.WW, a.set.WH = int32(float32(r.W())/a.scale), int32(float32(r.H())/a.scale)
	}
	a.set.save()
}

// ---------------- hit testing ----------------

func (a *App) inZone(z *Zone, x, y float32) bool {
	return x >= z.X && x < z.X+z.W && y >= z.Y && y < z.Y+z.H
}

func (a *App) zoneAt(x, y float32) *Zone {
	for i := len(a.zones) - 1; i >= 0; i-- {
		z := &a.zones[i]
		if (z.Click != nil || z.Right != nil || z.Dbl != nil || z.Drag != nil) && a.inZone(z, x, y) {
			return z
		}
	}
	return nil
}

func (a *App) hitTest(sx, sy int32) uintptr {
	p := win.ScreenToClient(a.hwnd, win.POINT{X: sx, Y: sy})
	r := win.GetClientRect(a.hwnd)
	b := int32(6 * a.scale)
	if !win.IsZoomed(a.hwnd) {
		l, t, rr, bb := p.X < b, p.Y < b, p.X >= r.Right-b, p.Y >= r.Bottom-b
		switch {
		case t && l:
			return win.HTTOPLEFT
		case t && rr:
			return win.HTTOPRIGHT
		case bb && l:
			return win.HTBOTTOMLEFT
		case bb && rr:
			return win.HTBOTTOMRIGHT
		case l:
			return win.HTLEFT
		case rr:
			return win.HTRIGHT
		case t:
			return win.HTTOP
		case bb:
			return win.HTBOTTOM
		}
	}
	x, y := float32(p.X)/a.scale, float32(p.Y)/a.scale
	if a.zoneAt(x, y) != nil {
		return win.HTCLIENT
	}
	for i := range a.zones {
		if a.zones[i].Caption && a.inZone(&a.zones[i], x, y) {
			return win.HTCAPTION
		}
	}
	return win.HTCLIENT
}

func (a *App) mouseMove(x, y float32) {
	a.mx, a.my = x, y
	win.TrackMouseLeave(a.hwnd)
	if a.dragZone != nil && a.dragZone.Drag != nil {
		a.dragZone.Drag(x, y)
		win.Invalidate(a.hwnd)
		return
	}
	id := ""
	var cur uintptr = a.cursorArrow
	if z := a.zoneAt(x, y); z != nil {
		id = z.ID
		if z.Cursor != 0 {
			cur = z.Cursor
		} else if z.Click != nil {
			cur = a.cursorArrow
		}
	}
	win.SetCursor(cur)
	if id != a.hover {
		a.hover = id
		win.Invalidate(a.hwnd)
	}
}

func (a *App) mouseDown(x, y float32) {
	z := a.zoneAt(x, y)
	if z == nil {
		if a.searchFocus {
			a.searchFocus = false
			win.Invalidate(a.hwnd)
		}
		return
	}
	a.pressed = z.ID
	if z.Drag != nil {
		zz := *z
		a.dragZone = &zz
		win.SetCapture(a.hwnd)
		zz.Drag(x, y)
	}
	win.Invalidate(a.hwnd)
}

func (a *App) mouseUp(x, y float32) {
	if a.dragZone != nil {
		a.dragZone = nil
		win.ReleaseCapture()
		a.pressed = ""
		win.Invalidate(a.hwnd)
		return
	}
	z := a.zoneAt(x, y)
	if z != nil && z.ID == a.pressed && z.Click != nil {
		if z.ID != "search" {
			a.searchFocus = false
		}
		z.Click()
	}
	a.pressed = ""
	win.Invalidate(a.hwnd)
}

func (a *App) addZone(z Zone) { a.zones = append(a.zones, z) }

// ---------------- keyboard ----------------

func (a *App) char(r rune) {
	if !a.searchFocus {
		if r >= 32 && r != 127 && !a.set.Compact && (a.set.Page == PProcesses || a.set.Page == PDetails || a.set.Page == PServices || a.set.Page == PStartup || a.set.Page == PAppHistory) {
			a.searchFocus = true // start typing = search, like Task Manager
		} else {
			return
		}
	}
	switch {
	case r == 8:
		if n := len([]rune(a.search)); n > 0 {
			a.search = string([]rune(a.search)[:n-1])
		}
	case r == 127: // ctrl+backspace
		a.search = ""
	case r == 27:
		a.search, a.searchFocus = "", false
	case r >= 32:
		if len(a.search) < 64 {
			a.search += string(r)
		}
	}
	win.Invalidate(a.hwnd)
}

func (a *App) key(vk int) {
	ctrl := win.KeyDown(win.VK_CONTROL)
	switch {
	case ctrl && vk == 'F':
		a.searchFocus = true
	case vk == win.VK_ESCAPE:
		a.search, a.searchFocus = "", false
	case vk == win.VK_F5:
		a.col.Wake()
	case ctrl && vk >= '1' && vk <= '9':
		a.goPage(vk - '1')
	case vk == win.VK_DELETE && !a.searchFocus:
		if p, ok := a.pages[a.set.Page].(interface{ endSelected(*App) }); ok {
			p.endSelected(a)
		}
	case vk == win.VK_UP || vk == win.VK_DOWN || vk == win.VK_PRIOR || vk == win.VK_NEXT:
		if p, ok := a.pages[a.set.Page].(interface{ moveSel(int) }); ok {
			d := map[int]int{win.VK_UP: -1, win.VK_DOWN: 1, win.VK_PRIOR: -10, win.VK_NEXT: 10}[vk]
			p.moveSel(d)
		}
	}
	win.Invalidate(a.hwnd)
}

func (a *App) goPage(p int) {
	if p < 0 || p >= len(pageNames) {
		return
	}
	a.set.Page = p
	a.search, a.searchFocus = "", false
	a.set.save()
	win.Invalidate(a.hwnd)
}

// ---------------- painting ----------------

func (a *App) freeBitmap() {
	if a.memDC != 0 {
		win.DeleteDC(a.memDC)
		a.memDC = 0
	}
	if a.memBmp != 0 {
		win.DeleteObject(a.memBmp)
		a.memBmp = 0
	}
	a.bmpW, a.bmpH = 0, 0
}

func (a *App) paint() {
	var ps win.PAINTSTRUCT
	hdc := win.BeginPaint(a.hwnd, &ps)
	defer win.EndPaint(a.hwnd, &ps)
	r := win.GetClientRect(a.hwnd)
	if r.W() <= 0 || r.H() <= 0 {
		return
	}
	if a.memDC == 0 || a.bmpW != r.W() || a.bmpH != r.H() {
		a.freeBitmap()
		a.memDC = win.CreateCompatibleDC(hdc)
		a.memBmp = win.CreateCompatibleBitmap(hdc, r.W(), r.H())
		win.SelectObject(a.memDC, a.memBmp)
		a.bmpW, a.bmpH = r.W(), r.H()
	}
	a.scale = float32(win.GetDpiForWindow(a.hwnd)) / 96
	c := a.cv
	c.Begin(a.memDC, r.W(), r.H(), a.scale)
	a.zones = a.zones[:0]
	a.col.RLock()
	if a.set.Compact {
		a.drawCompact(c)
	} else {
		a.drawFrame(c)
	}
	a.col.RUnlock()
	c.End()
	win.BitBlt(hdc, 0, 0, r.W(), r.H(), a.memDC, 0, 0)
}

// icon returns a cached small icon for an exe path.
func (a *App) icon(path string) uintptr {
	if path == "" {
		return 0
	}
	if h, ok := a.icons[path]; ok {
		return h
	}
	if len(a.icons) > 400 {
		a.clearIcons()
	}
	h := win.ExtractIcon(path, int32(16*a.scale+0.5))
	a.icons[path] = h
	return h
}

func (a *App) clearIcons() {
	for _, h := range a.icons {
		if h != 0 {
			win.DestroyIcon(h)
		}
	}
	a.icons = map[string]uintptr{}
}

func (a *App) toggleCompact() {
	a.saveWindowPos()
	a.set.Compact = !a.set.Compact
	s := a.scale
	if a.set.Compact {
		if win.IsZoomed(a.hwnd) {
			win.ShowWindow(a.hwnd, win.SW_RESTORE)
		}
		x, y := a.set.CX, a.set.CY
		r := win.GetWindowRect(a.hwnd)
		if x < 0 {
			x, y = r.Left, r.Top
		}
		win.SetWindowPos(a.hwnd, ^uintptr(0) /*TOPMOST*/, x, y, int32(compactW*s), int32(compactH*s), win.SWP_FRAMECHANGED)
	} else {
		after := ^uintptr(1) // NOTOPMOST
		if a.set.OnTop {
			after = ^uintptr(0)
		}
		x, y := a.set.WX, a.set.WY
		if x < 0 {
			r := win.GetWindowRect(a.hwnd)
			x, y = r.Left, r.Top
		}
		win.SetWindowPos(a.hwnd, after, x, y, int32(float32(a.set.WW)*s), int32(float32(a.set.WH)*s), win.SWP_FRAMECHANGED)
		if a.set.Maximized {
			win.ShowWindow(a.hwnd, win.SW_MAXIMIZE)
		}
	}
	a.set.save()
	win.Invalidate(a.hwnd)
}

func (a *App) setOnTop(on bool) {
	a.set.OnTop = on
	after := ^uintptr(1)
	if on {
		after = ^uintptr(0)
	}
	win.SetWindowPos(a.hwnd, after, 0, 0, 0, 0, win.SWP_NOMOVE|win.SWP_NOSIZE|win.SWP_NOACTIVATE)
	a.set.save()
}
