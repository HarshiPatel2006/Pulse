//go:build windows

package win

import (
	"syscall"
	"unsafe"
)

const (
	WS_OVERLAPPEDWINDOW = 0x00CF0000
	WS_POPUP            = 0x80000000
	WS_VISIBLE          = 0x10000000
	WS_THICKFRAME       = 0x00040000
	WS_MINIMIZEBOX      = 0x00020000
	WS_MAXIMIZEBOX      = 0x00010000
	WS_SYSMENU          = 0x00080000
	WS_CAPTION          = 0x00C00000
	WS_EX_TOPMOST       = 0x00000008
	WS_EX_TOOLWINDOW    = 0x00000080
	WS_EX_APPWINDOW     = 0x00040000

	CW_USEDEFAULT = 0x80000000

	WM_CREATE         = 0x0001
	WM_DESTROY        = 0x0002
	WM_SIZE           = 0x0005
	WM_ACTIVATE       = 0x0006
	WM_SETFOCUS       = 0x0007
	WM_KILLFOCUS      = 0x0008
	WM_PAINT          = 0x000F
	WM_CLOSE          = 0x0010
	WM_ERASEBKGND     = 0x0014
	WM_SETCURSOR      = 0x0020
	WM_GETMINMAXINFO  = 0x0024
	WM_NCCALCSIZE     = 0x0083
	WM_NCHITTEST      = 0x0084
	WM_NCLBUTTONDOWN  = 0x00A1
	WM_KEYDOWN        = 0x0100
	WM_CHAR           = 0x0102
	WM_SYSKEYDOWN     = 0x0104
	WM_COMMAND        = 0x0111
	WM_SYSCOMMAND     = 0x0112
	WM_TIMER          = 0x0113
	WM_MOUSEMOVE      = 0x0200
	WM_LBUTTONDOWN    = 0x0201
	WM_LBUTTONUP      = 0x0202
	WM_LBUTTONDBLCLK  = 0x0203
	WM_RBUTTONDOWN    = 0x0204
	WM_RBUTTONUP      = 0x0205
	WM_MOUSEWHEEL     = 0x020A
	WM_MOUSELEAVE     = 0x02A3
	WM_DPICHANGED     = 0x02E0
	WM_USER           = 0x0400
	WM_APP            = 0x8000
	WM_NCMOUSELEAVE   = 0x02A2

	SW_SHOW     = 5
	SW_HIDE     = 0
	SW_MINIMIZE = 6
	SW_MAXIMIZE = 3
	SW_RESTORE  = 9

	HTCLIENT      = 1
	HTCAPTION     = 2
	HTLEFT        = 10
	HTRIGHT       = 11
	HTTOP         = 12
	HTTOPLEFT     = 13
	HTTOPRIGHT    = 14
	HTBOTTOM      = 15
	HTBOTTOMLEFT  = 16
	HTBOTTOMRIGHT = 17

	SWP_NOSIZE       = 0x0001
	SWP_NOMOVE       = 0x0002
	SWP_NOZORDER     = 0x0004
	SWP_NOACTIVATE   = 0x0010
	SWP_FRAMECHANGED = 0x0020

	IDC_ARROW  = 32512
	IDC_IBEAM  = 32513
	IDC_HAND   = 32649
	IDC_SIZEWE = 32644

	MK_LBUTTON = 1
	CS_DBLCLKS = 0x0008
	CS_HREDRAW = 2
	CS_VREDRAW = 1

	VK_BACK   = 0x08
	VK_TAB    = 0x09
	VK_RETURN = 0x0D
	VK_ESCAPE = 0x1B
	VK_DELETE = 0x2E
	VK_UP     = 0x26
	VK_DOWN   = 0x28
	VK_PRIOR  = 0x21
	VK_NEXT   = 0x22
	VK_HOME   = 0x24
	VK_END    = 0x23
	VK_F5     = 0x74
	VK_CONTROL = 0x11
	VK_SHIFT   = 0x10

	MF_STRING    = 0x0000
	MF_GRAYED    = 0x0001
	MF_CHECKED   = 0x0008
	MF_POPUP     = 0x0010
	MF_SEPARATOR = 0x0800
	TPM_RETURNCMD = 0x0100
	TPM_RIGHTBUTTON = 0x0002

	MB_OK        = 0
	MB_YESNO     = 4
	MB_ICONWARNING = 0x30
	MB_ICONINFO  = 0x40
	IDYES        = 6

	SC_MOVE = 0xF010
	MONITOR_DEFAULTTONEAREST = 2
	GWL_STYLE   = -16
	GWL_EXSTYLE = -20
	SM_CXSCREEN = 0
	SM_CYSCREEN = 1
)

type POINT struct{ X, Y int32 }
type RECT struct{ Left, Top, Right, Bottom int32 }

func (r RECT) W() int32 { return r.Right - r.Left }
func (r RECT) H() int32 { return r.Bottom - r.Top }

type MSG struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      POINT
	_       uint32
}

type WNDCLASSEX struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}

type PAINTSTRUCT struct {
	Hdc         uintptr
	Erase       int32
	RcPaint     RECT
	Restore     int32
	IncUpdate   int32
	RgbReserved [32]byte
}

type MINMAXINFO struct {
	Reserved     POINT
	MaxSize      POINT
	MaxPosition  POINT
	MinTrackSize POINT
	MaxTrackSize POINT
}

type MONITORINFO struct {
	Size    uint32
	Monitor RECT
	Work    RECT
	Flags   uint32
}

type TRACKMOUSEEVENT struct {
	Size      uint32
	Flags     uint32
	Hwnd      uintptr
	HoverTime uint32
}

var (
	pRegisterClassEx     = user32.NewProc("RegisterClassExW")
	pCreateWindowEx      = user32.NewProc("CreateWindowExW")
	pDefWindowProc       = user32.NewProc("DefWindowProcW")
	pGetMessage          = user32.NewProc("GetMessageW")
	pTranslateMessage    = user32.NewProc("TranslateMessage")
	pDispatchMessage     = user32.NewProc("DispatchMessageW")
	pPostQuitMessage     = user32.NewProc("PostQuitMessage")
	pPostMessage         = user32.NewProc("PostMessageW")
	pShowWindow          = user32.NewProc("ShowWindow")
	pUpdateWindow        = user32.NewProc("UpdateWindow")
	pInvalidateRect      = user32.NewProc("InvalidateRect")
	pBeginPaint          = user32.NewProc("BeginPaint")
	pEndPaint            = user32.NewProc("EndPaint")
	pGetClientRect       = user32.NewProc("GetClientRect")
	pGetWindowRect       = user32.NewProc("GetWindowRect")
	pSetTimer            = user32.NewProc("SetTimer")
	pKillTimer           = user32.NewProc("KillTimer")
	pLoadCursor          = user32.NewProc("LoadCursorW")
	pSetCursor           = user32.NewProc("SetCursor")
	pSetWindowPos        = user32.NewProc("SetWindowPos")
	pGetDpiForWindow     = user32.NewProc("GetDpiForWindow")
	pSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	pSetProcessDPIAware  = user32.NewProc("SetProcessDPIAware")
	pSetCapture          = user32.NewProc("SetCapture")
	pReleaseCapture      = user32.NewProc("ReleaseCapture")
	pTrackMouseEvent     = user32.NewProc("TrackMouseEvent")
	pCreatePopupMenu     = user32.NewProc("CreatePopupMenu")
	pAppendMenu          = user32.NewProc("AppendMenuW")
	pTrackPopupMenu      = user32.NewProc("TrackPopupMenu")
	pDestroyMenu         = user32.NewProc("DestroyMenu")
	pGetCursorPos        = user32.NewProc("GetCursorPos")
	pScreenToClient      = user32.NewProc("ScreenToClient")
	pClientToScreen      = user32.NewProc("ClientToScreen")
	pMessageBox          = user32.NewProc("MessageBoxW")
	pSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	pGetKeyState         = user32.NewProc("GetKeyState")
	pIsZoomed            = user32.NewProc("IsZoomed")
	pIsIconic            = user32.NewProc("IsIconic")
	pMonitorFromWindow   = user32.NewProc("MonitorFromWindow")
	pGetMonitorInfo      = user32.NewProc("GetMonitorInfoW")
	pEnumWindows         = user32.NewProc("EnumWindows")
	pIsWindowVisible     = user32.NewProc("IsWindowVisible")
	pGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	pGetWindow           = user32.NewProc("GetWindow")
	pGetWindowTextLength = user32.NewProc("GetWindowTextLengthW")
	pGetWindowText       = user32.NewProc("GetWindowTextW")
	pGetWindowLongPtr    = user32.NewProc("GetWindowLongPtrW")
	pSetWindowLongPtr    = user32.NewProc("SetWindowLongPtrW")
	pDestroyWindow       = user32.NewProc("DestroyWindow")
	pLoadIcon            = user32.NewProc("LoadIconW")
	pGetSystemMetrics    = user32.NewProc("GetSystemMetrics")
	pEnumDisplayDevices  = user32.NewProc("EnumDisplayDevicesW")
	pEnumDisplaySettings = user32.NewProc("EnumDisplaySettingsW")
	pOpenClipboard       = user32.NewProc("OpenClipboard")
	pCloseClipboard      = user32.NewProc("CloseClipboard")
	pEmptyClipboard      = user32.NewProc("EmptyClipboard")
	pSetClipboardData    = user32.NewProc("SetClipboardData")
	pSendMessage         = user32.NewProc("SendMessageW")
	pAdjustWindowRectExForDpi = user32.NewProc("AdjustWindowRectExForDpi")
	pGetGuiResources     = user32.NewProc("GetGuiResources")
)

func RegisterClassEx(wc *WNDCLASSEX) uintptr { return call(pRegisterClassEx, uintptr(unsafe.Pointer(wc))) }
func CreateWindowEx(ex uint32, cls, title string, style uint32, x, y, w, h int32, parent uintptr) uintptr {
	return call(pCreateWindowEx, uintptr(ex), Str(cls), Str(title), uintptr(style),
		uintptr(x), uintptr(y), uintptr(w), uintptr(h), parent, 0, 0, 0)
}
func DefWindowProc(h uintptr, m uint32, w, l uintptr) uintptr {
	r, _, _ := pDefWindowProc.Call(h, uintptr(m), w, l)
	return r
}
func GetMessage(m *MSG) int32 {
	r, _, _ := pGetMessage.Call(uintptr(unsafe.Pointer(m)), 0, 0, 0)
	return int32(r)
}
func TranslateMessage(m *MSG)              { pTranslateMessage.Call(uintptr(unsafe.Pointer(m))) }
func DispatchMessage(m *MSG)               { pDispatchMessage.Call(uintptr(unsafe.Pointer(m))) }
func PostQuitMessage(code int)             { call(pPostQuitMessage, uintptr(code)) }
func PostMessage(h uintptr, m uint32, w, l uintptr) { call(pPostMessage, h, uintptr(m), w, l) }
func SendMessage(h uintptr, m uint32, w, l uintptr) uintptr { return call(pSendMessage, h, uintptr(m), w, l) }
func ShowWindow(h uintptr, cmd int)        { call(pShowWindow, h, uintptr(cmd)) }
func UpdateWindow(h uintptr)               { call(pUpdateWindow, h) }
func Invalidate(h uintptr)                 { call(pInvalidateRect, h, 0, 0) }
func BeginPaint(h uintptr, ps *PAINTSTRUCT) uintptr { return call(pBeginPaint, h, uintptr(unsafe.Pointer(ps))) }
func EndPaint(h uintptr, ps *PAINTSTRUCT)  { call(pEndPaint, h, uintptr(unsafe.Pointer(ps))) }
func GetClientRect(h uintptr) (r RECT)     { call(pGetClientRect, h, uintptr(unsafe.Pointer(&r))); return }
func GetWindowRect(h uintptr) (r RECT)     { call(pGetWindowRect, h, uintptr(unsafe.Pointer(&r))); return }
func SetTimer(h uintptr, id, ms uintptr)   { call(pSetTimer, h, id, ms, 0) }
func KillTimer(h uintptr, id uintptr)      { call(pKillTimer, h, id) }
func LoadCursor(id uintptr) uintptr        { return call(pLoadCursor, 0, id) }
func SetCursor(c uintptr)                  { call(pSetCursor, c) }
func SetWindowPos(h, after uintptr, x, y, w, ht int32, flags uint32) {
	call(pSetWindowPos, h, after, uintptr(x), uintptr(y), uintptr(w), uintptr(ht), uintptr(flags))
}
func GetDpiForWindow(h uintptr) int {
	if r := call(pGetDpiForWindow, h); r != 0 {
		return int(r)
	}
	return 96
}
func SetDpiAware() {
	// PER_MONITOR_AWARE_V2 = -4
	if call(pSetProcessDpiAwarenessContext, ^uintptr(3)) == 0 {
		call(pSetProcessDPIAware)
	}
}
func SetCapture(h uintptr)  { call(pSetCapture, h) }
func ReleaseCapture()       { call(pReleaseCapture) }
func TrackMouseLeave(h uintptr) {
	t := TRACKMOUSEEVENT{Flags: 2, Hwnd: h}
	t.Size = uint32(unsafe.Sizeof(t))
	call(pTrackMouseEvent, uintptr(unsafe.Pointer(&t)))
}
func CreatePopupMenu() uintptr { return call(pCreatePopupMenu) }
func AppendMenu(m uintptr, flags uint32, id uintptr, text string) {
	if flags&MF_SEPARATOR != 0 {
		call(pAppendMenu, m, uintptr(flags), 0, 0)
		return
	}
	call(pAppendMenu, m, uintptr(flags), id, Str(text))
}
func TrackPopupMenu(m, h uintptr, x, y int32) int {
	SetForegroundWindow(h)
	return int(call(pTrackPopupMenu, m, TPM_RETURNCMD|TPM_RIGHTBUTTON, uintptr(x), uintptr(y), 0, h, 0))
}
func DestroyMenu(m uintptr) { call(pDestroyMenu, m) }
func GetCursorPos() (p POINT) { call(pGetCursorPos, uintptr(unsafe.Pointer(&p))); return }
func ClientToScreen(h uintptr, p POINT) POINT {
	call(pClientToScreen, h, uintptr(unsafe.Pointer(&p)))
	return p
}
func ScreenToClient(h uintptr, p POINT) POINT {
	call(pScreenToClient, h, uintptr(unsafe.Pointer(&p)))
	return p
}
func MessageBox(h uintptr, text, caption string, flags uint32) int {
	return int(call(pMessageBox, h, Str(text), Str(caption), uintptr(flags)))
}
func SetForegroundWindow(h uintptr) { call(pSetForegroundWindow, h) }
func KeyDown(vk int) bool          { return int16(call(pGetKeyState, uintptr(vk))) < 0 }
func IsZoomed(h uintptr) bool      { return call(pIsZoomed, h) != 0 }
func IsIconic(h uintptr) bool      { return call(pIsIconic, h) != 0 }
func MonitorWorkArea(h uintptr) RECT {
	m := call(pMonitorFromWindow, h, MONITOR_DEFAULTTONEAREST)
	mi := MONITORINFO{}
	mi.Size = uint32(unsafe.Sizeof(mi))
	call(pGetMonitorInfo, m, uintptr(unsafe.Pointer(&mi)))
	return mi.Work
}
func GetWindowLong(h uintptr, idx int) uintptr {
	return call(pGetWindowLongPtr, h, uintptr(idx))
}
func SetWindowLong(h uintptr, idx int, v uintptr) { call(pSetWindowLongPtr, h, uintptr(idx), v) }
func DestroyWindow(h uintptr)                     { call(pDestroyWindow, h) }
func LoadIcon(inst uintptr, id uintptr) uintptr   { return call(pLoadIcon, inst, id) }
func ScreenSize() (int32, int32) {
	return int32(call(pGetSystemMetrics, SM_CXSCREEN)), int32(call(pGetSystemMetrics, SM_CYSCREEN))
}
func GuiResources(proc uintptr, flag uint32) uint32 { return uint32(call(pGetGuiResources, proc, uintptr(flag))) }

// VisibleWindowPids returns pids that own a visible, titled top-level window (the "Apps" group).
// The callback is created once: Windows/Go only allow a limited number of callbacks per process.
var (
	enumOut map[uint32]string
	enumCB  = syscall.NewCallback(func(h, _ uintptr) uintptr {
		if call(pIsWindowVisible, h) == 0 || call(pGetWindow, h, 4) != 0 { // skip hidden / owned popups
			return 1
		}
		if GetWindowLong(h, GWL_EXSTYLE)&WS_EX_TOOLWINDOW != 0 {
			return 1
		}
		n := call(pGetWindowTextLength, h)
		if n == 0 {
			return 1
		}
		buf := make([]uint16, n+1)
		call(pGetWindowText, h, uintptr(unsafe.Pointer(&buf[0])), n+1)
		var pid uint32
		call(pGetWindowThreadProcessId, h, uintptr(unsafe.Pointer(&pid)))
		if _, ok := enumOut[pid]; !ok {
			enumOut[pid] = syscall.UTF16ToString(buf)
		}
		return 1
	})
)

func VisibleWindowPids() map[uint32]string {
	enumOut = map[uint32]string{}
	call(pEnumWindows, enumCB, 0)
	return enumOut
}

func CopyToClipboard(h uintptr, s string) {
	u, _ := syscall.UTF16FromString(s)
	n := uintptr(len(u) * 2)
	mem := call(pGlobalAlloc, 0x0002, n)
	if mem == 0 {
		return
	}
	p := call(pGlobalLock, mem)
	copy(unsafe.Slice((*uint16)(unsafe.Pointer(p)), len(u)), u)
	call(pGlobalUnlock, mem)
	if call(pOpenClipboard, h) != 0 {
		call(pEmptyClipboard)
		call(pSetClipboardData, 13, mem) // CF_UNICODETEXT
		call(pCloseClipboard)
	}
}

type DISPLAY_DEVICE struct {
	Cb           uint32
	DeviceName   [32]uint16
	DeviceString [128]uint16
	StateFlags   uint32
	DeviceID     [128]uint16
	DeviceKey    [128]uint16
}

type DEVMODE struct {
	DeviceName       [32]uint16
	SpecVersion      uint16
	DriverVersion    uint16
	Size             uint16
	DriverExtra      uint16
	Fields           uint32
	PositionX        int32
	PositionY        int32
	DisplayOrientation uint32
	DisplayFixedOutput uint32
	Color            int16
	Duplex           int16
	YResolution      int16
	TTOption         int16
	Collate          int16
	FormName         [32]uint16
	LogPixels        uint16
	BitsPerPel       uint32
	PelsWidth        uint32
	PelsHeight       uint32
	DisplayFlags     uint32
	DisplayFrequency uint32
	_                [8]uint32
}

type Display struct {
	Name, Adapter       string
	W, H, Hz, Bits      int
	Primary             bool
}

func Displays() []Display {
	var out []Display
	for i := 0; i < 16; i++ {
		var dd DISPLAY_DEVICE
		dd.Cb = uint32(unsafe.Sizeof(dd))
		if call(pEnumDisplayDevices, 0, uintptr(i), uintptr(unsafe.Pointer(&dd)), 0) == 0 {
			break
		}
		if dd.StateFlags&1 == 0 { // DISPLAY_DEVICE_ATTACHED_TO_DESKTOP
			continue
		}
		var dm DEVMODE
		dm.Size = uint16(unsafe.Sizeof(dm))
		call(pEnumDisplaySettings, uintptr(unsafe.Pointer(&dd.DeviceName[0])), ^uintptr(0), uintptr(unsafe.Pointer(&dm))) // ENUM_CURRENT_SETTINGS
		var mon DISPLAY_DEVICE
		mon.Cb = uint32(unsafe.Sizeof(mon))
		name := ""
		if call(pEnumDisplayDevices, uintptr(unsafe.Pointer(&dd.DeviceName[0])), 0, uintptr(unsafe.Pointer(&mon)), 0) != 0 {
			name = syscall.UTF16ToString(mon.DeviceString[:])
		}
		out = append(out, Display{Name: name, Adapter: syscall.UTF16ToString(dd.DeviceString[:]),
			W: int(dm.PelsWidth), H: int(dm.PelsHeight), Hz: int(dm.DisplayFrequency), Bits: int(dm.BitsPerPel),
			Primary: dd.StateFlags&4 != 0})
	}
	return out
}
