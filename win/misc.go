//go:build windows

package win

import (
	"syscall"
	"unsafe"
)

var (
	pWTSEnumerateSessions       = wtsapi32.NewProc("WTSEnumerateSessionsW")
	pWTSQuerySessionInformation = wtsapi32.NewProc("WTSQuerySessionInformationW")
	pWTSFreeMemory              = wtsapi32.NewProc("WTSFreeMemory")
	pWTSDisconnectSession       = wtsapi32.NewProc("WTSDisconnectSession")
	pWTSLogoffSession           = wtsapi32.NewProc("WTSLogoffSession")
	pShellExecute               = shell32.NewProc("ShellExecuteW")
	pIsUserAnAdmin              = shell32.NewProc("IsUserAnAdmin")
	pDwmSetWindowAttribute      = dwmapi.NewProc("DwmSetWindowAttribute")
	pDwmExtendFrameIntoClientArea = dwmapi.NewProc("DwmExtendFrameIntoClientArea")
	pNtQuerySystemInformation   = ntdll.NewProc("NtQuerySystemInformation")
	pCallNtPowerInformation     = powrprof.NewProc("CallNtPowerInformation")
	pGetAdaptersInfo            = iphlpapi.NewProc("GetAdaptersInfo")
	pGetFileVersionInfoSize     = version.NewProc("GetFileVersionInfoSizeW")
	pGetFileVersionInfo         = version.NewProc("GetFileVersionInfoW")
	pVerQueryValue              = version.NewProc("VerQueryValueW")
	pPrivateExtractIcons        = user32.NewProc("PrivateExtractIconsW")
	pDrawIconEx                 = user32.NewProc("DrawIconEx")
	pDestroyIcon                = user32.NewProc("DestroyIcon")
	pCreateDXGIFactory1         = dxgi.NewProc("CreateDXGIFactory1")
)

// ---------------- sessions (Users tab) ----------------

type Session struct {
	ID              uint32
	Station, User   string
	State           uint32 // 0 Active, 1 Connected, 4 Disconnected ...
}

func Sessions() []Session {
	var p uintptr
	var n uint32
	if call(pWTSEnumerateSessions, 0, 0, 1, uintptr(unsafe.Pointer(&p)), uintptr(unsafe.Pointer(&n))) == 0 {
		return nil
	}
	defer call(pWTSFreeMemory, p)
	type info struct {
		ID      uint32
		Station *uint16
		State   uint32
	}
	items := unsafe.Slice((*info)(unsafe.Pointer(p)), n)
	var out []Session
	for _, it := range items {
		s := Session{ID: it.ID, Station: GoStr(uintptr(unsafe.Pointer(it.Station))), State: it.State}
		var up uintptr
		var bytes uint32
		if call(pWTSQuerySessionInformation, 0, uintptr(it.ID), 5 /*WTSUserName*/, uintptr(unsafe.Pointer(&up)), uintptr(unsafe.Pointer(&bytes))) != 0 {
			s.User = GoStr(up)
			call(pWTSFreeMemory, up)
		}
		out = append(out, s)
	}
	return out
}

func DisconnectSession(id uint32) bool { return call(pWTSDisconnectSession, 0, uintptr(id), 0) != 0 }
func LogoffSession(id uint32) bool     { return call(pWTSLogoffSession, 0, uintptr(id), 0) != 0 }

// ---------------- shell ----------------

func ShellOpen(verb, file, args, dir string) bool {
	var v uintptr
	if verb != "" {
		v = Str(verb)
	}
	var a, d uintptr
	if args != "" {
		a = Str(args)
	}
	if dir != "" {
		d = Str(dir)
	}
	return call(pShellExecute, 0, v, Str(file), a, d, SW_SHOW) > 32
}

func IsAdmin() bool { return call(pIsUserAnAdmin) != 0 }

// ---------------- dark theme helpers ----------------

func DarkTitleBar(h uintptr) {
	on := int32(1)
	call(pDwmSetWindowAttribute, h, 20, uintptr(unsafe.Pointer(&on)), 4) // DWMWA_USE_IMMERSIVE_DARK_MODE
	round := int32(2)
	call(pDwmSetWindowAttribute, h, 33, uintptr(unsafe.Pointer(&round)), 4) // DWMWA_WINDOW_CORNER_PREFERENCE = ROUND
	cap := uint32(0x000E0B0B)
	call(pDwmSetWindowAttribute, h, 35, uintptr(unsafe.Pointer(&cap)), 4) // caption color
}

// FrameShadow keeps the DWM shadow on a borderless window.
func FrameShadow(h uintptr) {
	m := [4]int32{0, 0, 1, 0}
	call(pDwmExtendFrameIntoClientArea, h, uintptr(unsafe.Pointer(&m)))
}

// DarkMenus makes native popup menus dark (undocumented uxtheme ordinals, Win10 1903+).
func DarkMenus() {
	h, err := syscall.LoadLibrary("uxtheme.dll")
	if err != nil {
		return
	}
	gpa := kernel32.NewProc("GetProcAddress")
	if p := call(gpa, uintptr(h), 135); p != 0 {
		syscall.SyscallN(p, 2) // SetPreferredAppMode(ForceDark)
	}
	if p := call(gpa, uintptr(h), 136); p != 0 {
		syscall.SyscallN(p) // FlushMenuThemes
	}
}

// ---------------- per-core CPU times ----------------

type CoreTimes struct{ Idle, Kernel, User uint64 }

func PerCoreTimes(n int, out []CoreTimes) []CoreTimes {
	type spi struct {
		Idle, Kernel, User, Dpc, Interrupt int64
		InterruptCount                     uint32
		_                                  uint32
	}
	if n <= 0 {
		n = 64
	}
	buf := make([]spi, n)
	var ret uint32
	if call(pNtQuerySystemInformation, 8, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf))*unsafe.Sizeof(buf[0]), uintptr(unsafe.Pointer(&ret))) != 0 {
		return out[:0]
	}
	cnt := int(uintptr(ret) / unsafe.Sizeof(buf[0]))
	out = out[:0]
	for i := 0; i < cnt; i++ {
		out = append(out, CoreTimes{uint64(buf[i].Idle), uint64(buf[i].Kernel), uint64(buf[i].User)})
	}
	return out
}

// MaxMHz returns the base/max clock of CPU 0.
func MaxMHz(logical int) (max, cur uint32) {
	if logical <= 0 {
		logical = 1
	}
	type ppi struct{ Number, MaxMhz, CurrentMhz, MhzLimit, MaxIdle, CurIdle uint32 }
	buf := make([]ppi, logical)
	call(pCallNtPowerInformation, 11, 0, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf))*24)
	for _, p := range buf {
		if p.MaxMhz > max {
			max = p.MaxMhz
		}
		if p.CurrentMhz > cur {
			cur = p.CurrentMhz
		}
	}
	return
}

// ---------------- network adapters ----------------

type Adapter struct {
	Name, Desc, IP, MAC, Gateway string
	Type                         uint32
}

func Adapters() []Adapter {
	var size uint32
	call(pGetAdaptersInfo, 0, uintptr(unsafe.Pointer(&size)))
	if size == 0 {
		return nil
	}
	buf := make([]byte, size)
	if call(pGetAdaptersInfo, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size))) != 0 {
		return nil
	}
	cstr := func(b []byte) string {
		for i, c := range b {
			if c == 0 {
				return string(b[:i])
			}
		}
		return string(b)
	}
	var out []Adapter
	// IP_ADAPTER_INFO (x64): Next*8, ComboIndex4, AdapterName[260], Description[132], AddressLength4, Address[8],
	// Index4, Type4, DhcpEnabled4, CurrentIpAddress*8, IpAddressList(IP_ADDR_STRING: Next8, Ip16, Mask16, Context4 +pad)...
	for p := uintptr(unsafe.Pointer(&buf[0])); p != 0; {
		b := unsafe.Slice((*byte)(unsafe.Pointer(p)), 704)
		a := Adapter{Name: cstr(b[12:272]), Desc: cstr(b[272:404])}
		alen := *(*uint32)(unsafe.Pointer(&b[404]))
		mac := b[408 : 408+alen]
		for i, c := range mac {
			if i > 0 {
				a.MAC += "-"
			}
			a.MAC += string("0123456789ABCDEF"[c>>4]) + string("0123456789ABCDEF"[c&15])
		}
		a.Type = *(*uint32)(unsafe.Pointer(&b[420]))
		a.IP = cstr(b[448:464])      // IpAddressList.IpAddress
		a.Gateway = cstr(b[496:512]) // GatewayList.IpAddress
		out = append(out, a)
		p = *(*uintptr)(unsafe.Pointer(p))
	}
	return out
}

// ---------------- file version info (description / company) ----------------

func FileInfo(path string) (desc, company string) {
	n := call(pGetFileVersionInfoSize, Str(path), 0)
	if n == 0 {
		return
	}
	buf := make([]byte, n)
	if call(pGetFileVersionInfo, Str(path), 0, n, uintptr(unsafe.Pointer(&buf[0]))) == 0 {
		return
	}
	var tp uintptr
	var tl uint32
	lang := "040904B0"
	if call(pVerQueryValue, uintptr(unsafe.Pointer(&buf[0])), Str(`\VarFileInfo\Translation`), uintptr(unsafe.Pointer(&tp)), uintptr(unsafe.Pointer(&tl))) != 0 && tl >= 4 {
		w := *(*[2]uint16)(unsafe.Pointer(tp))
		lang = hex4(w[0]) + hex4(w[1])
	}
	q := func(key string) string {
		var p uintptr
		var l uint32
		if call(pVerQueryValue, uintptr(unsafe.Pointer(&buf[0])), Str(`\StringFileInfo\`+lang+`\`+key), uintptr(unsafe.Pointer(&p)), uintptr(unsafe.Pointer(&l))) == 0 || l == 0 {
			return ""
		}
		return GoStr(p)
	}
	return q("FileDescription"), q("CompanyName")
}

func hex4(v uint16) string {
	const h = "0123456789ABCDEF"
	return string([]byte{h[v>>12&15], h[v>>8&15], h[v>>4&15], h[v&15]})
}

// ---------------- icons ----------------

func ExtractIcon(path string, size int32) uintptr {
	var ico uintptr
	var id uint32
	call(pPrivateExtractIcons, Str(path), 0, uintptr(size), uintptr(size), uintptr(unsafe.Pointer(&ico)), uintptr(unsafe.Pointer(&id)), 1, 0)
	return ico
}
func DrawIcon(hdc uintptr, ico uintptr, x, y, size int32) {
	call(pDrawIconEx, hdc, uintptr(x), uintptr(y), ico, uintptr(size), uintptr(size), 0, 0, 3)
}
func DestroyIcon(ico uintptr) { call(pDestroyIcon, ico) }

// ---------------- DXGI: adapter list + LUIDs ----------------

type DxAdapter struct {
	Desc      string
	Vendor    uint32
	Dedicated uint64
	Shared    uint64
	LUID      string // "luid_0xHHHHHHHH_0xLLLLLLLL"
}

func vcall(obj uintptr, idx int, args ...uintptr) uintptr {
	vtbl := *(*uintptr)(unsafe.Pointer(obj))
	fn := *(*uintptr)(unsafe.Pointer(vtbl + uintptr(idx)*8))
	r, _, _ := syscall.SyscallN(fn, append([]uintptr{obj}, args...)...)
	return r
}

func DXGIAdapters() []DxAdapter {
	iid := [16]byte{0x78, 0xae, 0x0a, 0x77, 0x6f, 0xf2, 0xba, 0x4d, 0xa8, 0x29, 0x25, 0x3c, 0x83, 0xd1, 0xb3, 0x87}
	var fac uintptr
	if r, err := call2(pCreateDXGIFactory1, uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&fac))); err != nil && fac == 0 || r != 0 {
		return nil
	}
	defer vcall(fac, 2)
	var out []DxAdapter
	for i := 0; i < 16; i++ {
		var ad uintptr
		if vcall(fac, 7, uintptr(i), uintptr(unsafe.Pointer(&ad))) != 0 || ad == 0 {
			break
		}
		var d struct {
			Desc                             [128]uint16
			Vendor, Device, SubSys, Revision uint32
			DedicatedVideo, DedicatedSystem, SharedSystem uintptr
			LuidLow  uint32
			LuidHigh int32
		}
		if vcall(ad, 8, uintptr(unsafe.Pointer(&d))) == 0 {
			out = append(out, DxAdapter{
				Desc: syscall.UTF16ToString(d.Desc[:]), Vendor: d.Vendor,
				Dedicated: uint64(d.DedicatedVideo), Shared: uint64(d.SharedSystem),
				LUID: "luid_0x" + hex8(uint32(d.LuidHigh)) + "_0x" + hex8(d.LuidLow),
			})
		}
		vcall(ad, 2)
	}
	return out
}

func hex8(v uint32) string { return hex4(uint16(v>>16)) + hex4(uint16(v)) }
