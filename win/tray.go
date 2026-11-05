//go:build windows

package win

import (
	"syscall"
	"unsafe"
)

const (
	NIM_ADD     = 0
	NIM_MODIFY  = 1
	NIM_DELETE  = 2
	NIF_MESSAGE = 1
	NIF_ICON    = 2
	NIF_TIP     = 4
	WMTray      = WM_APP + 2
)

type notifyIconData struct {
	cbSize           uint32
	hWnd             uintptr
	uID              uint32
	uFlags           uint32
	uCallbackMessage uint32
	hIcon            uintptr
	szTip            [128]uint16
}

var shellNotifyIcon = shell32.NewProc("Shell_NotifyIconW")

func TrayAdd(hwnd, icon uintptr, tip string) bool    { return tray(NIM_ADD, hwnd, icon, tip) }
func TrayModify(hwnd, icon uintptr, tip string) bool { return tray(NIM_MODIFY, hwnd, icon, tip) }
func TrayDelete(hwnd uintptr) bool                   { return tray(NIM_DELETE, hwnd, 0, "") }

func tray(action int, hwnd, icon uintptr, tip string) bool {
	n := notifyIconData{cbSize: uint32(unsafe.Sizeof(notifyIconData{})), hWnd: hwnd, uID: 1, uFlags: NIF_MESSAGE | NIF_ICON | NIF_TIP, uCallbackMessage: WMTray, hIcon: icon}
	if text, err := syscall.UTF16FromString(tip); err == nil {
		copy(n.szTip[:], text)
	}
	return call(shellNotifyIcon, uintptr(action), uintptr(unsafe.Pointer(&n))) != 0
}
