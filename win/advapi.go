//go:build windows

package win

import (
	"syscall"
	"time"
	"unsafe"
)

var (
	pRegOpenKeyEx      = advapi32.NewProc("RegOpenKeyExW")
	pRegQueryValueEx   = advapi32.NewProc("RegQueryValueExW")
	pRegEnumValue      = advapi32.NewProc("RegEnumValueW")
	pRegEnumKeyEx      = advapi32.NewProc("RegEnumKeyExW")
	pRegSetValueEx     = advapi32.NewProc("RegSetValueExW")
	pRegCloseKey       = advapi32.NewProc("RegCloseKey")
	pOpenProcessToken  = advapi32.NewProc("OpenProcessToken")
	pGetTokenInformation = advapi32.NewProc("GetTokenInformation")
	pLookupAccountSid  = advapi32.NewProc("LookupAccountSidW")
	pOpenSCManager     = advapi32.NewProc("OpenSCManagerW")
	pEnumServicesStatusEx = advapi32.NewProc("EnumServicesStatusExW")
	pOpenService       = advapi32.NewProc("OpenServiceW")
	pStartService      = advapi32.NewProc("StartServiceW")
	pControlService    = advapi32.NewProc("ControlService")
	pCloseServiceHandle = advapi32.NewProc("CloseServiceHandle")
	pQueryServiceConfig = advapi32.NewProc("QueryServiceConfigW")
	pQueryServiceConfig2 = advapi32.NewProc("QueryServiceConfig2W")
	pGetUserName       = advapi32.NewProc("GetUserNameW")
)

const (
	HKEY_CLASSES_ROOT  = 0x80000000
	HKEY_CURRENT_USER  = 0x80000001
	HKEY_LOCAL_MACHINE = 0x80000002
	KEY_READ           = 0x20019
	KEY_WRITE          = 0x20006
	KEY_WOW64_64KEY    = 0x0100
	KEY_WOW64_32KEY    = 0x0200
)

func RegOpen(root uintptr, path string, access uint32) uintptr {
	var k uintptr
	if call(pRegOpenKeyEx, root, Str(path), 0, uintptr(access), uintptr(unsafe.Pointer(&k))) != 0 {
		return 0
	}
	return k
}
func RegClose(k uintptr) { call(pRegCloseKey, k) }

// RegString reads a REG_SZ / REG_EXPAND_SZ value ("" if missing).
func RegString(root uintptr, path, name string) string {
	k := RegOpen(root, path, KEY_READ|KEY_WOW64_64KEY)
	if k == 0 {
		return ""
	}
	defer RegClose(k)
	s, _ := RegValue(k, name)
	return s
}

func RegDWORD(root uintptr, path, name string) (uint32, bool) {
	k := RegOpen(root, path, KEY_READ|KEY_WOW64_64KEY)
	if k == 0 {
		return 0, false
	}
	defer RegClose(k)
	var t, v, n uint32 = 0, 0, 4
	if call(pRegQueryValueEx, k, Str(name), 0, uintptr(unsafe.Pointer(&t)), uintptr(unsafe.Pointer(&v)), uintptr(unsafe.Pointer(&n))) != 0 {
		return 0, false
	}
	return v, true
}

// RegValue returns a string value and its raw bytes.
func RegValue(k uintptr, name string) (string, []byte) {
	var t, n uint32
	call(pRegQueryValueEx, k, Str(name), 0, uintptr(unsafe.Pointer(&t)), 0, uintptr(unsafe.Pointer(&n)))
	if n == 0 {
		return "", nil
	}
	buf := make([]byte, n+2)
	if call(pRegQueryValueEx, k, Str(name), 0, uintptr(unsafe.Pointer(&t)), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n))) != 0 {
		return "", nil
	}
	buf = buf[:n]
	if t == 1 || t == 2 || t == 7 { // SZ, EXPAND_SZ, MULTI_SZ
		u := unsafe.Slice((*uint16)(unsafe.Pointer(&buf[0])), len(buf)/2)
		return syscall.UTF16ToString(u), buf
	}
	return "", buf
}

// RegValues enumerates (name, string, raw) of every value under a key.
func RegValues(k uintptr) (names []string) {
	for i := uint32(0); ; i++ {
		nb := make([]uint16, 512)
		n := uint32(512)
		if call(pRegEnumValue, k, uintptr(i), uintptr(unsafe.Pointer(&nb[0])), uintptr(unsafe.Pointer(&n)), 0, 0, 0, 0) != 0 {
			break
		}
		names = append(names, syscall.UTF16ToString(nb[:n]))
	}
	return
}

func RegSubkeys(k uintptr) (names []string) {
	for i := uint32(0); ; i++ {
		nb := make([]uint16, 256)
		n := uint32(256)
		if call(pRegEnumKeyEx, k, uintptr(i), uintptr(unsafe.Pointer(&nb[0])), uintptr(unsafe.Pointer(&n)), 0, 0, 0, 0) != 0 {
			break
		}
		names = append(names, syscall.UTF16ToString(nb[:n]))
	}
	return
}

func RegSetBinary(k uintptr, name string, data []byte) bool {
	return call(pRegSetValueEx, k, Str(name), 0, 3, uintptr(unsafe.Pointer(&data[0])), uintptr(len(data))) == 0
}

// ProcessUser returns DOMAIN-less user name owning a process handle.
func ProcessUser(h uintptr) string {
	var tok uintptr
	if call(pOpenProcessToken, h, 0x0008 /*TOKEN_QUERY*/, uintptr(unsafe.Pointer(&tok))) == 0 {
		return ""
	}
	defer CloseHandle(tok)
	var n uint32
	call(pGetTokenInformation, tok, 1, 0, 0, uintptr(unsafe.Pointer(&n)))
	if n == 0 {
		return ""
	}
	buf := make([]byte, n)
	if call(pGetTokenInformation, tok, 1, uintptr(unsafe.Pointer(&buf[0])), uintptr(n), uintptr(unsafe.Pointer(&n))) == 0 {
		return ""
	}
	sid := *(*uintptr)(unsafe.Pointer(&buf[0]))
	return SidName(sid)
}

var sidCache = map[string]string{}

func SidName(sid uintptr) string {
	name := make([]uint16, 256)
	dom := make([]uint16, 256)
	nn, dn := uint32(256), uint32(256)
	var use uint32
	if call(pLookupAccountSid, 0, sid, uintptr(unsafe.Pointer(&name[0])), uintptr(unsafe.Pointer(&nn)),
		uintptr(unsafe.Pointer(&dom[0])), uintptr(unsafe.Pointer(&dn)), uintptr(unsafe.Pointer(&use))) == 0 {
		return ""
	}
	return syscall.UTF16ToString(name)
}

func UserName() string {
	buf := make([]uint16, 256)
	n := uint32(256)
	call(pGetUserName, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n)))
	return syscall.UTF16ToString(buf)
}

// ---------------- services ----------------

type Service struct {
	Name, Display string
	State, Pid    uint32
	Type          uint32
}

type enumServiceProcess struct {
	ServiceName *uint16
	DisplayName *uint16
	Status      struct {
		ServiceType, CurrentState, ControlsAccepted, Win32ExitCode,
		ServiceSpecificExitCode, CheckPoint, WaitHint, ProcessId, ServiceFlags uint32
	}
}

func Services() []Service {
	scm := call(pOpenSCManager, 0, 0, 0x0004 /*SC_MANAGER_ENUMERATE_SERVICE*/)
	if scm == 0 {
		return nil
	}
	defer call(pCloseServiceHandle, scm)
	var need, count, resume uint32
	call(pEnumServicesStatusEx, scm, 0, 0x30 /*WIN32*/, 3 /*ALL*/, 0, 0, uintptr(unsafe.Pointer(&need)), uintptr(unsafe.Pointer(&count)), uintptr(unsafe.Pointer(&resume)), 0)
	if need == 0 {
		return nil
	}
	buf := make([]byte, need+4096)
	resume = 0
	if call(pEnumServicesStatusEx, scm, 0, 0x30, 3, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), uintptr(unsafe.Pointer(&need)), uintptr(unsafe.Pointer(&count)), uintptr(unsafe.Pointer(&resume)), 0) == 0 {
		return nil
	}
	items := unsafe.Slice((*enumServiceProcess)(unsafe.Pointer(&buf[0])), count)
	out := make([]Service, 0, count)
	for _, it := range items {
		out = append(out, Service{
			Name:    GoStr(uintptr(unsafe.Pointer(it.ServiceName))),
			Display: GoStr(uintptr(unsafe.Pointer(it.DisplayName))),
			State:   it.Status.CurrentState, Pid: it.Status.ProcessId, Type: it.Status.ServiceType,
		})
	}
	return out
}

// ServiceStartType returns 2=Automatic 3=Manual 4=Disabled (0/1 = boot/system).
func ServiceStartType(name string) (uint32, string) {
	scm := call(pOpenSCManager, 0, 0, 0x0001)
	if scm == 0 {
		return 0, ""
	}
	defer call(pCloseServiceHandle, scm)
	s := call(pOpenService, scm, Str(name), 0x0001 /*QUERY_CONFIG*/)
	if s == 0 {
		return 0, ""
	}
	defer call(pCloseServiceHandle, s)
	var need uint32
	call(pQueryServiceConfig, s, 0, 0, uintptr(unsafe.Pointer(&need)))
	if need == 0 {
		return 0, ""
	}
	buf := make([]byte, need)
	if call(pQueryServiceConfig, s, uintptr(unsafe.Pointer(&buf[0])), uintptr(need), uintptr(unsafe.Pointer(&need))) == 0 {
		return 0, ""
	}
	start := *(*uint32)(unsafe.Pointer(&buf[4]))
	// lpLoadOrderGroup is the 5th field (after 3 DWORDs + 1 pointer, aligned)
	group := GoStr(*(*uintptr)(unsafe.Pointer(&buf[24])))
	return start, group
}

func ServiceDescription(name string) string {
	scm := call(pOpenSCManager, 0, 0, 0x0001)
	if scm == 0 {
		return ""
	}
	defer call(pCloseServiceHandle, scm)
	s := call(pOpenService, scm, Str(name), 0x0001)
	if s == 0 {
		return ""
	}
	defer call(pCloseServiceHandle, s)
	var need uint32
	call(pQueryServiceConfig2, s, 1, 0, 0, uintptr(unsafe.Pointer(&need)))
	if need == 0 {
		return ""
	}
	buf := make([]byte, need)
	if call(pQueryServiceConfig2, s, 1, uintptr(unsafe.Pointer(&buf[0])), uintptr(need), uintptr(unsafe.Pointer(&need))) == 0 {
		return ""
	}
	return GoStr(*(*uintptr)(unsafe.Pointer(&buf[0])))
}

// ServiceControl: action "start", "stop", "restart".
func ServiceControl(name, action string) error {
	scm := call(pOpenSCManager, 0, 0, 0x0001)
	if scm == 0 {
		return syscall.GetLastError()
	}
	defer call(pCloseServiceHandle, scm)
	s, err := call2(pOpenService, scm, Str(name), 0x0010|0x0020|0x0004) // START|STOP|QUERY_STATUS
	if s == 0 {
		return err
	}
	defer call(pCloseServiceHandle, s)
	var st [7]uint32
	if action == "stop" || action == "restart" {
		if r, err := call2(pControlService, s, 1, uintptr(unsafe.Pointer(&st))); r == 0 && action == "stop" {
			return err
		}
	}
	if action == "start" || action == "restart" {
		if action == "restart" {
			for i := 0; i < 50; i++ { // wait up to ~5s for it to stop
				var q [9]uint32
				var need uint32
				call(pQueryServiceStatusEx, s, 0, uintptr(unsafe.Pointer(&q)), 36, uintptr(unsafe.Pointer(&need)))
				if q[1] == 1 {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
		if r, err := call2(pStartService, s, 0, 0); r == 0 {
			return err
		}
	}
	return nil
}

var pQueryServiceStatusEx = advapi32.NewProc("QueryServiceStatusEx")

var pRegDeleteValue = advapi32.NewProc("RegDeleteValueW")

func RegSetString(k uintptr, name, val string) bool {
	u, _ := syscall.UTF16FromString(val)
	return call(pRegSetValueEx, k, Str(name), 0, 1, uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)*2)) == 0
}

func RegDelete(k uintptr, name string) bool { return call(pRegDeleteValue, k, Str(name)) == 0 }
