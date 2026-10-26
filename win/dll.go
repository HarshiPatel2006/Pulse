//go:build windows

// Package win holds the raw Win32 bindings Pulse needs (no third-party deps).
package win

import (
	"syscall"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	gdiplus  = syscall.NewLazyDLL("gdiplus.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	advapi32 = syscall.NewLazyDLL("advapi32.dll")
	psapi    = syscall.NewLazyDLL("psapi.dll")
	pdh      = syscall.NewLazyDLL("pdh.dll")
	wtsapi32 = syscall.NewLazyDLL("wtsapi32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	dwmapi   = syscall.NewLazyDLL("dwmapi.dll")
	uxtheme  = syscall.NewLazyDLL("uxtheme.dll")
	ntdll    = syscall.NewLazyDLL("ntdll.dll")
	powrprof = syscall.NewLazyDLL("powrprof.dll")
	iphlpapi = syscall.NewLazyDLL("iphlpapi.dll")
	version  = syscall.NewLazyDLL("version.dll")
	dxgi     = syscall.NewLazyDLL("dxgi.dll")
)

// Call runs a proc if it exists; missing procs return 0 instead of panicking
// (important under older Windows or Wine).
func call(p *syscall.LazyProc, a ...uintptr) uintptr {
	if p.Find() != nil {
		return 0
	}
	r, _, _ := p.Call(a...)
	return r
}

func call2(p *syscall.LazyProc, a ...uintptr) (uintptr, error) {
	if err := p.Find(); err != nil {
		return 0, err
	}
	r, _, e := p.Call(a...)
	return r, e
}

// Str converts a Go string to a *uint16 pointer (as uintptr).
func Str(s string) uintptr {
	p, _ := syscall.UTF16PtrFromString(s)
	return uintptr(unsafe.Pointer(p))
}

func StrPtr(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

// GoStr reads a NUL-terminated UTF-16 string from memory.
func GoStr(p uintptr) string {
	if p == 0 {
		return ""
	}
	var buf []uint16
	for i := uintptr(0); ; i += 2 {
		c := *(*uint16)(unsafe.Pointer(p + i))
		if c == 0 {
			break
		}
		buf = append(buf, c)
		if len(buf) > 32768 {
			break
		}
	}
	return syscall.UTF16ToString(buf)
}

func B(v bool) uintptr {
	if v {
		return 1
	}
	return 0
}
