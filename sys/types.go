//go:build windows

// Package sys collects everything Pulse shows: CPU, memory, disks, network,
// GPU, processes, users, services, startup apps and static system info.
package sys

import "pulse/win"

// History length for graphs (60 samples = 1 min at normal speed, like Task Manager).
const HistLen = 60

type Ring struct {
	V   [HistLen]float64
	N   int
	pos int
}

func (r *Ring) Push(v float64) {
	r.V[r.pos] = v
	r.pos = (r.pos + 1) % HistLen
	if r.N < HistLen {
		r.N++
	}
}

// At returns sample i (0 = oldest kept).
func (r *Ring) At(i int) float64 {
	start := (r.pos - r.N + HistLen) % HistLen
	return r.V[(start+i)%HistLen]
}

func (r *Ring) Last() float64 {
	if r.N == 0 {
		return 0
	}
	return r.At(r.N - 1)
}

type CPUInfo struct {
	Total           float64
	PerCore         []float64
	MHz, BaseMHz    float64
	Procs, Threads  uint32
	Handles         uint32
	Uptime          uint64 // ms
	Temp            float64
	TempOK          bool
}

type MemInfo struct {
	Total, Used, Avail       uint64
	Commit, CommitLimit      uint64
	Cached, Paged, NonPaged  uint64
	Modified, Standby, Free  uint64
	Load                     float64
}

type DiskInfo struct {
	Key       string // pdh instance "0 C:"
	Label     string // "Disk 0 (C:)"
	Active    float64
	ReadBps   float64
	WriteBps  float64
	RespMs    float64
	Hist      *Ring
	ReadHist  *Ring
	WriteHist *Ring
}

type NetInfo struct {
	Key           string
	Label         string // "Ethernet" / "Wi-Fi"
	Desc          string
	IP, MAC       string
	SendBps       float64
	RecvBps       float64
	LinkBps       float64
	SendHist      *Ring
	RecvHist      *Ring
}

type GPUInfo struct {
	Name, Driver       string
	OK                 bool
	Stat               win.GPUStat
	Engines            map[string]float64 // 3D, Copy, VideoDecode, VideoEncode ...
	SharedUsed         uint64
	SharedTotal        uint64
	EngHist            map[string]*Ring
}

type Proc struct {
	Pid, Ppid       uint32
	Name            string // exe name
	Path            string
	Desc, Company   string
	User            string
	Session         uint32
	CPU             float64 // % of whole machine
	CPUTime         uint64  // 100ns
	WS, Private     uint64
	DiskBps         float64
	GPU             float64
	VRAM            uint64
	Threads         uint32
	Handles         uint32
	Priority        uint32
	Is32            bool
	IsApp           bool
	Title           string
	Created         uint64
	Access          bool // could open the process
}

type Snapshot struct {
	CPU      CPUInfo
	Mem      MemInfo
	Disks    []*DiskInfo
	Nets     []*NetInfo
	GPU      GPUInfo
	Procs    []*Proc
	SelfWS   uint64
	SelfPriv uint64
	Seq      uint64
}

func (r *Ring) Len() int { return r.N }

// Max returns the largest kept sample.
func (r *Ring) Max() float64 {
	m := 0.0
	for i := 0; i < r.N; i++ {
		if v := r.At(i); v > m {
			m = v
		}
	}
	return m
}
