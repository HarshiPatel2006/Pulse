//go:build windows

package win

import (
	"syscall"
	"time"
	"unsafe"
)

var (
	pGlobalAlloc              = kernel32.NewProc("GlobalAlloc")
	pGlobalLock               = kernel32.NewProc("GlobalLock")
	pGlobalUnlock             = kernel32.NewProc("GlobalUnlock")
	pGetCurrentProcess        = kernel32.NewProc("GetCurrentProcess")
	pGetCurrentProcessId      = kernel32.NewProc("GetCurrentProcessId")
	pOpenProcess              = kernel32.NewProc("OpenProcess")
	pCloseHandle              = kernel32.NewProc("CloseHandle")
	pCreateToolhelp32Snapshot = kernel32.NewProc("CreateToolhelp32Snapshot")
	pProcess32First           = kernel32.NewProc("Process32FirstW")
	pProcess32Next            = kernel32.NewProc("Process32NextW")
	pGetProcessTimes          = kernel32.NewProc("GetProcessTimes")
	pGetSystemTimes           = kernel32.NewProc("GetSystemTimes")
	pGlobalMemoryStatusEx     = kernel32.NewProc("GlobalMemoryStatusEx")
	pK32GetPerformanceInfo    = kernel32.NewProc("K32GetPerformanceInfo")
	pK32GetProcessMemoryInfo  = kernel32.NewProc("K32GetProcessMemoryInfo")
	pK32EmptyWorkingSet       = kernel32.NewProc("K32EmptyWorkingSet")
	pSetProcessWorkingSetSizeEx = kernel32.NewProc("SetProcessWorkingSetSizeEx")
	pQueryFullProcessImageName = kernel32.NewProc("QueryFullProcessImageNameW")
	pGetProcessIoCounters     = kernel32.NewProc("GetProcessIoCounters")
	pGetProcessHandleCount    = kernel32.NewProc("GetProcessHandleCount")
	pTerminateProcess         = kernel32.NewProc("TerminateProcess")
	pProcessIdToSessionId     = kernel32.NewProc("ProcessIdToSessionId")
	pGetPriorityClass         = kernel32.NewProc("GetPriorityClass")
	pSetPriorityClass         = kernel32.NewProc("SetPriorityClass")
	pIsWow64Process           = kernel32.NewProc("IsWow64Process")
	pGetTickCount64           = kernel32.NewProc("GetTickCount64")
	pGetLogicalProcessorInformationEx = kernel32.NewProc("GetLogicalProcessorInformationEx")
	pGetComputerNameEx        = kernel32.NewProc("GetComputerNameExW")
	pGetLogicalDriveStrings   = kernel32.NewProc("GetLogicalDriveStringsW")
	pGetDiskFreeSpaceEx       = kernel32.NewProc("GetDiskFreeSpaceExW")
	pGetDriveType             = kernel32.NewProc("GetDriveTypeW")
	pGetVolumeInformation     = kernel32.NewProc("GetVolumeInformationW")
	pGetSystemFirmwareTable   = kernel32.NewProc("GetSystemFirmwareTable")
	pGetSystemPowerStatus     = kernel32.NewProc("GetSystemPowerStatus")
	pGetModuleHandle          = kernel32.NewProc("GetModuleHandleW")
	pGetModuleFileName        = kernel32.NewProc("GetModuleFileNameW")
	pSetProcessPriorityBoost  = kernel32.NewProc("SetProcessPriorityBoost")
	pGetNativeSystemInfo      = kernel32.NewProc("GetNativeSystemInfo")
	pSetProcessInformation    = kernel32.NewProc("SetProcessInformation")
)

const (
	PROCESS_TERMINATE                 = 0x0001
	PROCESS_VM_READ                   = 0x0010
	PROCESS_QUERY_INFORMATION         = 0x0400
	PROCESS_SET_INFORMATION           = 0x0200
	PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
	PROCESS_SET_QUOTA                 = 0x0100
)

func GlobalAlloc(flags, n uintptr) uintptr { return call(pGlobalAlloc, flags, n) }
func CurrentProcess() uintptr            { return call(pGetCurrentProcess) }
func CurrentPid() uint32                 { return uint32(call(pGetCurrentProcessId)) }
func OpenProcess(access uint32, pid uint32) uintptr {
	return call(pOpenProcess, uintptr(access), 0, uintptr(pid))
}
func CloseHandle(h uintptr) { call(pCloseHandle, h) }
func ModuleHandle() uintptr { return call(pGetModuleHandle, 0) }
func ExePath() string {
	buf := make([]uint16, 1024)
	call(pGetModuleFileName, 0, uintptr(unsafe.Pointer(&buf[0])), 1024)
	return syscall.UTF16ToString(buf)
}

type PROCESSENTRY32 struct {
	Size            uint32
	Usage           uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	Threads         uint32
	ParentProcessID uint32
	PriClassBase    int32
	Flags           uint32
	ExeFile         [260]uint16
}

type ProcEntry struct {
	Pid, Ppid, Threads uint32
	Name               string
	BasePri            int32
}

// Snapshot lists all processes (cheap; no handles opened per process).
func Snapshot(out []ProcEntry) []ProcEntry {
	out = out[:0]
	h := call(pCreateToolhelp32Snapshot, 2, 0) // TH32CS_SNAPPROCESS
	if h == 0 || h == ^uintptr(0) {
		return out
	}
	defer CloseHandle(h)
	var e PROCESSENTRY32
	e.Size = uint32(unsafe.Sizeof(e))
	for ok := call(pProcess32First, h, uintptr(unsafe.Pointer(&e))); ok != 0; ok = call(pProcess32Next, h, uintptr(unsafe.Pointer(&e))) {
		out = append(out, ProcEntry{Pid: e.ProcessID, Ppid: e.ParentProcessID, Threads: e.Threads,
			Name: syscall.UTF16ToString(e.ExeFile[:]), BasePri: e.PriClassBase})
	}
	return out
}

func ft(v [2]uint32) uint64 { return uint64(v[1])<<32 | uint64(v[0]) }

// ProcessTimes returns creation time and kernel+user CPU time (100ns units).
func ProcessTimes(h uintptr) (created, cpu uint64, ok bool) {
	var c, e, k, u [2]uint32
	if call(pGetProcessTimes, h, uintptr(unsafe.Pointer(&c)), uintptr(unsafe.Pointer(&e)), uintptr(unsafe.Pointer(&k)), uintptr(unsafe.Pointer(&u))) == 0 {
		return 0, 0, false
	}
	return ft(c), ft(k) + ft(u), true
}

func SystemTimes() (idle, kernel, user uint64) {
	var i, k, u [2]uint32
	call(pGetSystemTimes, uintptr(unsafe.Pointer(&i)), uintptr(unsafe.Pointer(&k)), uintptr(unsafe.Pointer(&u)))
	return ft(i), ft(k), ft(u)
}

type MEMORYSTATUSEX struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

func MemoryStatus() (m MEMORYSTATUSEX) {
	m.Length = uint32(unsafe.Sizeof(m))
	call(pGlobalMemoryStatusEx, uintptr(unsafe.Pointer(&m)))
	return
}

type PERFORMANCE_INFORMATION struct {
	Size              uint32
	CommitTotal       uintptr
	CommitLimit       uintptr
	CommitPeak        uintptr
	PhysicalTotal     uintptr
	PhysicalAvailable uintptr
	SystemCache       uintptr
	KernelTotal       uintptr
	KernelPaged       uintptr
	KernelNonpaged    uintptr
	PageSize          uintptr
	HandleCount       uint32
	ProcessCount      uint32
	ThreadCount       uint32
}

func PerformanceInfo() (p PERFORMANCE_INFORMATION) {
	p.Size = uint32(unsafe.Sizeof(p))
	call(pK32GetPerformanceInfo, uintptr(unsafe.Pointer(&p)), uintptr(p.Size))
	return
}

type PROCESS_MEMORY_COUNTERS_EX struct {
	Size                       uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
	PrivateUsage               uintptr
}

func ProcessMemory(h uintptr) (m PROCESS_MEMORY_COUNTERS_EX, ok bool) {
	m.Size = uint32(unsafe.Sizeof(m))
	ok = call(pK32GetProcessMemoryInfo, h, uintptr(unsafe.Pointer(&m)), uintptr(m.Size)) != 0
	return
}

// TrimSelf releases Pulse's own unused working-set pages back to Windows.
func TrimSelf() {
	call(pSetProcessWorkingSetSizeEx, CurrentProcess(), ^uintptr(0), ^uintptr(0), 0)
}

// EcoMode asks Windows to run Pulse with power throttling (EcoQoS) - lower CPU/power.
func EcoMode() {
	// ProcessPowerThrottling = 4 ; {Version=1, ControlMask=1, StateMask=1}
	st := [3]uint32{1, 1, 1}
	call(pSetProcessInformation, CurrentProcess(), 4, uintptr(unsafe.Pointer(&st)), 12)
}

func ImagePath(h uintptr) string {
	buf := make([]uint16, 1024)
	n := uint32(1024)
	if call(pQueryFullProcessImageName, h, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n))) == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf[:n])
}

type IO_COUNTERS struct {
	ReadOps, WriteOps, OtherOps, ReadBytes, WriteBytes, OtherBytes uint64
}

func IoCounters(h uintptr) (io IO_COUNTERS, ok bool) {
	ok = call(pGetProcessIoCounters, h, uintptr(unsafe.Pointer(&io))) != 0
	return
}

func HandleCount(h uintptr) uint32 {
	var n uint32
	call(pGetProcessHandleCount, h, uintptr(unsafe.Pointer(&n)))
	return n
}

func Terminate(pid uint32) error {
	h := OpenProcess(PROCESS_TERMINATE, pid)
	if h == 0 {
		return syscall.GetLastError()
	}
	defer CloseHandle(h)
	if r, err := call2(pTerminateProcess, h, 1); r == 0 {
		return err
	}
	return nil
}

func SessionOf(pid uint32) uint32 {
	var s uint32 = 0xFFFFFFFF
	call(pProcessIdToSessionId, uintptr(pid), uintptr(unsafe.Pointer(&s)))
	return s
}

func PriorityClass(h uintptr) uint32 { return uint32(call(pGetPriorityClass, h)) }
func SetPriority(pid uint32, class uint32) error {
	h := OpenProcess(PROCESS_SET_INFORMATION, pid)
	if h == 0 {
		return syscall.GetLastError()
	}
	defer CloseHandle(h)
	if r, err := call2(pSetPriorityClass, h, uintptr(class)); r == 0 {
		return err
	}
	return nil
}

func IsWow64(h uintptr) bool {
	var b int32
	call(pIsWow64Process, h, uintptr(unsafe.Pointer(&b)))
	return b != 0
}

func Uptime() uint64 { return uint64(call(pGetTickCount64)) }

// CPUTopology returns sockets, physical cores, logical processors and L1/L2/L3 cache sizes.
func CPUTopology() (sockets, cores, logical int, l1, l2, l3 uint64) {
	var n uint32
	call(pGetLogicalProcessorInformationEx, 0xFFFF, 0, uintptr(unsafe.Pointer(&n)))
	if n == 0 {
		return
	}
	buf := make([]byte, n)
	if call(pGetLogicalProcessorInformationEx, 0xFFFF, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n))) == 0 {
		return
	}
	for off := uint32(0); off+8 <= n; {
		rel := *(*uint32)(unsafe.Pointer(&buf[off]))
		size := *(*uint32)(unsafe.Pointer(&buf[off+4]))
		if size == 0 {
			break
		}
		body := off + 8
		switch rel {
		case 0: // RelationProcessorCore
			cores++
			// PROCESSOR_RELATIONSHIP: Flags(1) EfficiencyClass(1) Reserved(20) GroupCount(2) GroupMask[]
			gc := *(*uint16)(unsafe.Pointer(&buf[body+22]))
			for g := uint32(0); g < uint32(gc); g++ {
				mask := *(*uint64)(unsafe.Pointer(&buf[body+24+g*16]))
				for ; mask != 0; mask &= mask - 1 {
					logical++
				}
			}
		case 3: // RelationProcessorPackage
			sockets++
		case 2: // RelationCache: Level(1) Assoc(1) LineSize(2) CacheSize(4) Type(4)
			lvl := buf[body]
			sz := uint64(*(*uint32)(unsafe.Pointer(&buf[body+4])))
			switch lvl {
			case 1:
				l1 += sz
			case 2:
				l2 += sz
			case 3:
				l3 += sz
			}
		}
		off += size
	}
	return
}

func ComputerName() string {
	buf := make([]uint16, 256)
	n := uint32(256)
	call(pGetComputerNameEx, 4 /*ComputerNamePhysicalDnsHostname*/, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n)))
	return syscall.UTF16ToString(buf)
}

type Drive struct {
	Root, Label, FS string
	Type            uint32
	Total, Free     uint64
}

func Drives() []Drive {
	buf := make([]uint16, 512)
	n := call(pGetLogicalDriveStrings, 512, uintptr(unsafe.Pointer(&buf[0])))
	var out []Drive
	start := 0
	for i := 0; i < int(n); i++ {
		if buf[i] == 0 {
			root := syscall.UTF16ToString(buf[start:i])
			start = i + 1
			if root == "" {
				continue
			}
			t := uint32(call(pGetDriveType, Str(root)))
			d := Drive{Root: root, Type: t}
			if t == 3 || t == 2 { // fixed / removable
				var free, total, tfree uint64
				call(pGetDiskFreeSpaceEx, Str(root), uintptr(unsafe.Pointer(&free)), uintptr(unsafe.Pointer(&total)), uintptr(unsafe.Pointer(&tfree)))
				d.Total, d.Free = total, tfree
				lbl := make([]uint16, 128)
				fs := make([]uint16, 32)
				call(pGetVolumeInformation, Str(root), uintptr(unsafe.Pointer(&lbl[0])), 128, 0, 0, 0, uintptr(unsafe.Pointer(&fs[0])), 32)
				d.Label, d.FS = syscall.UTF16ToString(lbl), syscall.UTF16ToString(fs)
			}
			out = append(out, d)
		}
	}
	return out
}

// SMBIOS returns the raw SMBIOS table (for BIOS / board / RAM details).
func SMBIOS() []byte {
	const RSMB = 0x52534D42
	n := call(pGetSystemFirmwareTable, RSMB, 0, 0, 0)
	if n == 0 {
		return nil
	}
	buf := make([]byte, n)
	call(pGetSystemFirmwareTable, RSMB, 0, uintptr(unsafe.Pointer(&buf[0])), n)
	return buf
}

type PowerStatus struct {
	ACLine, Flag, Percent, Saver uint8
	LifeTime, FullLife          uint32
}

func Power() (p PowerStatus) {
	call(pGetSystemPowerStatus, uintptr(unsafe.Pointer(&p)))
	return
}

var (
	pCreateMutex = kernel32.NewProc("CreateMutexW")
	pFindWindow  = user32.NewProc("FindWindowW")
)

// SingleInstance returns false if another Pulse is running (and brings it to front).
func SingleInstance(name, class string, wait bool) bool {
	for i := 0; i < 50; i++ {
		_, err := call2(pCreateMutex, 0, 0, Str(name))
		if errno, ok := err.(syscall.Errno); !ok || errno != 183 { // ERROR_ALREADY_EXISTS
			return true
		}
		if !wait {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if h := call(pFindWindow, Str(class), 0); h != 0 {
		if IsIconic(h) {
			ShowWindow(h, SW_RESTORE)
		}
		SetForegroundWindow(h)
	}
	return false
}
