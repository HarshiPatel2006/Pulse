//go:build windows

package win

import (
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

// NVML - NVIDIA driver management library (ships with the NVIDIA driver).
type NVML struct {
	dll *syscall.LazyDLL
	dev uintptr
	ok  bool
}

func (n *NVML) p(name string) *syscall.LazyProc { return n.dll.NewProc(name) }

func OpenNVML(index int) *NVML {
	paths := []string{"nvml.dll", filepath.Join(os.Getenv("ProgramFiles"), `NVIDIA Corporation\NVSMI\nvml.dll`)}
	for _, path := range paths {
		d := syscall.NewLazyDLL(path)
		if d.Load() != nil {
			continue
		}
		n := &NVML{dll: d}
		if call(n.p("nvmlInit_v2")) != 0 {
			continue
		}
		if call(n.p("nvmlDeviceGetHandleByIndex_v2"), uintptr(index), uintptr(unsafe.Pointer(&n.dev))) != 0 {
			continue
		}
		n.ok = true
		return n
	}
	return &NVML{}
}

func (n *NVML) OK() bool { return n != nil && n.ok }

func (n *NVML) Name() string {
	if !n.OK() {
		return ""
	}
	b := make([]byte, 96)
	call(n.p("nvmlDeviceGetName"), n.dev, uintptr(unsafe.Pointer(&b[0])), 96)
	return cstring(b)
}

func (n *NVML) Driver() string {
	if !n.OK() {
		return ""
	}
	b := make([]byte, 80)
	call(n.p("nvmlSystemGetDriverVersion"), uintptr(unsafe.Pointer(&b[0])), 80)
	return cstring(b)
}

func cstring(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

type GPUStat struct {
	MemUsed, MemTotal       uint64
	Util, MemUtil           uint32
	Temp                    uint32
	PowerW, PowerLimitW     float64
	ClockMHz, MemClockMHz   uint32
	MaxClockMHz             uint32
	Fan                     uint32
	PcieGen, PcieWidth      uint32
	HasTemp, HasPower, HasFan bool
}

func (n *NVML) Read() (s GPUStat, ok bool) {
	if !n.OK() {
		return s, false
	}
	var mem struct{ Total, Free, Used uint64 }
	if call(n.p("nvmlDeviceGetMemoryInfo"), n.dev, uintptr(unsafe.Pointer(&mem))) == 0 {
		s.MemUsed, s.MemTotal = mem.Used, mem.Total
	}
	var u struct{ GPU, Mem uint32 }
	if call(n.p("nvmlDeviceGetUtilizationRates"), n.dev, uintptr(unsafe.Pointer(&u))) == 0 {
		s.Util, s.MemUtil = u.GPU, u.Mem
	}
	var t uint32
	if call(n.p("nvmlDeviceGetTemperature"), n.dev, 0, uintptr(unsafe.Pointer(&t))) == 0 {
		s.Temp, s.HasTemp = t, true
	}
	var mw uint32
	if call(n.p("nvmlDeviceGetPowerUsage"), n.dev, uintptr(unsafe.Pointer(&mw))) == 0 {
		s.PowerW, s.HasPower = float64(mw)/1000, true
	}
	if call(n.p("nvmlDeviceGetEnforcedPowerLimit"), n.dev, uintptr(unsafe.Pointer(&mw))) == 0 {
		s.PowerLimitW = float64(mw) / 1000
	}
	var c uint32
	if call(n.p("nvmlDeviceGetClockInfo"), n.dev, 0, uintptr(unsafe.Pointer(&c))) == 0 {
		s.ClockMHz = c
	}
	if call(n.p("nvmlDeviceGetClockInfo"), n.dev, 2, uintptr(unsafe.Pointer(&c))) == 0 {
		s.MemClockMHz = c
	}
	if call(n.p("nvmlDeviceGetMaxClockInfo"), n.dev, 0, uintptr(unsafe.Pointer(&c))) == 0 {
		s.MaxClockMHz = c
	}
	if call(n.p("nvmlDeviceGetFanSpeed"), n.dev, uintptr(unsafe.Pointer(&c))) == 0 {
		s.Fan, s.HasFan = c, true
	}
	if call(n.p("nvmlDeviceGetCurrPcieLinkGeneration"), n.dev, uintptr(unsafe.Pointer(&c))) == 0 {
		s.PcieGen = c
	}
	if call(n.p("nvmlDeviceGetCurrPcieLinkWidth"), n.dev, uintptr(unsafe.Pointer(&c))) == 0 {
		s.PcieWidth = c
	}
	return s, true
}
