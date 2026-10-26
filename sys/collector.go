//go:build windows

package sys

import (
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"pulse/win"
)

// Collector samples the system on a background goroutine.
type Collector struct {
	mu   sync.RWMutex
	Snap Snapshot

	// histories for graphs
	CPUHist     Ring
	CoreHist    []*Ring
	MemHist     Ring
	GPUHist     Ring
	VRAMHist    Ring
	GTempHist   Ring
	CTempHist   Ring
	PowerHist   Ring
	SharedHist  Ring

	App *AppHistory

	q       *win.Query // light counters every tick
	gq      *win.Query // per-process GPU counters (heavier)
	nv      *win.NVML
	luid    string
	logical int

	prevIdle, prevKernel, prevUser uint64
	prevCores                      []win.CoreTimes
	coreBuf                        []win.CoreTimes
	entries                        []win.ProcEntry
	cache                          map[uint32]*procCache
	prevTick                       time.Time
	self                           uint32
	disks                          map[string]*DiskInfo
	nets                           map[string]*NetInfo
	adapters                       []win.Adapter
	adaptersAt                     time.Time
	appPids                        map[uint32]string
	appPidsAt                      time.Time

	Interval time.Duration
	Paused   bool
	notify   func()
	wake     chan struct{}
	Demo     bool
}

type procCache struct {
	h        uintptr
	created  uint64
	cpu      uint64
	io       uint64
	path     string
	desc     string
	company  string
	user     string
	session  uint32
	is32     bool
	seen     uint64
}

func NewCollector(notify func()) *Collector {
	c := &Collector{
		cache:    map[uint32]*procCache{},
		disks:    map[string]*DiskInfo{},
		nets:     map[string]*NetInfo{},
		Interval: time.Second,
		notify:   notify,
		wake:     make(chan struct{}, 1),
		self:     win.CurrentPid(),
		App:      LoadAppHistory(),
	}
	_, _, c.logical, _, _, _ = win.CPUTopology()
	if c.logical == 0 {
		c.logical = 1
	}
	return c
}

func (c *Collector) RLock()   { c.mu.RLock() }
func (c *Collector) RUnlock() { c.mu.RUnlock() }

// Wake forces an immediate sample (e.g. after changing speed).
func (c *Collector) Wake() {
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

func (c *Collector) Run() {
	if c.Demo {
		c.runDemo()
		return
	}
	c.initCounters()
	for {
		if !c.Paused {
			c.sample()
			if c.notify != nil {
				c.notify()
			}
		}
		select {
		case <-time.After(c.Interval):
		case <-c.wake:
		}
	}
}

func (c *Collector) initCounters() {
	c.q = win.NewQuery()
	q := c.q
	q.Add("util", `\Processor Information(_Total)\% Processor Utility`)
	q.Add("perf", `\Processor Information(_Total)\% Processor Performance`)
	q.Add("cache", `\Memory\Cache Bytes`)
	q.Add("modified", `\Memory\Modified Page List Bytes`)
	q.Add("standby", `\Memory\Standby Cache Normal Priority Bytes`)
	q.Add("standby2", `\Memory\Standby Cache Reserve Bytes`)
	q.Add("standby3", `\Memory\Standby Cache Core Bytes`)
	q.Add("free", `\Memory\Free & Zero Page List Bytes`)
	q.Add("diskidle", `\PhysicalDisk(*)\% Idle Time`)
	q.Add("diskr", `\PhysicalDisk(*)\Disk Read Bytes/sec`)
	q.Add("diskw", `\PhysicalDisk(*)\Disk Write Bytes/sec`)
	q.Add("diskresp", `\PhysicalDisk(*)\Avg. Disk sec/Transfer`)
	q.Add("netsend", `\Network Interface(*)\Bytes Sent/sec`)
	q.Add("netrecv", `\Network Interface(*)\Bytes Received/sec`)
	q.Add("netbw", `\Network Interface(*)\Current Bandwidth`)
	q.Add("zone", `\Thermal Zone Information(*)\Temperature`)
	q.Add("gshared", `\GPU Adapter Memory(*)\Shared Usage`)
	q.Collect()

	c.gq = win.NewQuery()
	c.gq.Add("pmem", `\GPU Process Memory(*)\Dedicated Usage`)
	c.gq.Add("eng", `\GPU Engine(*)\Utilization Percentage`)
	c.gq.Collect()

	c.nv = win.OpenNVML(0)
	c.mu.Lock()
	c.Snap.GPU.OK = c.nv.OK()
	c.Snap.GPU.Name = strings.TrimPrefix(c.nv.Name(), "NVIDIA ")
	c.Snap.GPU.Driver = c.nv.Driver()
	c.Snap.GPU.EngHist = map[string]*Ring{}
	c.mu.Unlock()
	// Pick the NVIDIA adapter (or the one with most VRAM) for per-process GPU counters.
	var best win.DxAdapter
	for _, a := range win.DXGIAdapters() {
		if a.Desc == "Microsoft Basic Render Driver" {
			continue
		}
		if (a.Vendor == 0x10DE && best.Vendor != 0x10DE) || (a.Vendor == best.Vendor && a.Dedicated > best.Dedicated) || best.LUID == "" {
			best = a
		}
	}
	c.luid = strings.ToLower(best.LUID)
	c.mu.Lock()
	if c.Snap.GPU.Name == "" {
		c.Snap.GPU.Name = best.Desc
		c.Snap.GPU.Stat.MemTotal = best.Dedicated
	}
	c.Snap.GPU.SharedTotal = best.Shared
	c.mu.Unlock()
	win.PerCoreTimes(c.logical, nil)
	c.prevIdle, c.prevKernel, c.prevUser = win.SystemTimes()
	c.prevCores = win.PerCoreTimes(c.logical, nil)
}

var pidRe = regexp.MustCompile(`pid_(\d+)_`)
var engRe = regexp.MustCompile(`engtype_([A-Za-z0-9]+)`)

func (c *Collector) sample() {
	now := time.Now()
	dt := now.Sub(c.prevTick).Seconds()
	if c.prevTick.IsZero() || dt <= 0 {
		dt = c.Interval.Seconds()
	}
	c.prevTick = now
	c.q.Collect()
	c.gq.Collect()

	// ---------- CPU ----------
	idle, kern, user := win.SystemTimes()
	di, dk, du := idle-c.prevIdle, kern-c.prevKernel, user-c.prevUser
	c.prevIdle, c.prevKernel, c.prevUser = idle, kern, user
	total := 0.0
	if dk+du > 0 {
		total = 100 * float64(dk+du-di) / float64(dk+du)
	}
	if v, ok := c.q.Value("util"); ok && v > 0 {
		total = math.Min(100, v) // what Task Manager shows on Win10+
	}
	c.coreBuf = win.PerCoreTimes(c.logical, c.coreBuf)
	cores := make([]float64, len(c.coreBuf))
	for i, ct := range c.coreBuf {
		if i < len(c.prevCores) {
			p := c.prevCores[i]
			d := (ct.Kernel - p.Kernel) + (ct.User - p.User)
			if d > 0 {
				cores[i] = clamp(100 * float64(d-(ct.Idle-p.Idle)) / float64(d))
			}
		}
	}
	c.prevCores = append(c.prevCores[:0], c.coreBuf...)
	maxMHz, _ := win.MaxMHz(c.logical)
	mhz := float64(maxMHz)
	if v, ok := c.q.Value("perf"); ok && v > 0 {
		mhz = float64(maxMHz) * v / 100
	}
	pi := win.PerformanceInfo()
	temp, tempOK := 0.0, false
	c.q.Array("zone", func(_ string, v float64) {
		if v > 200 && v-273.15 > temp {
			temp, tempOK = v-273.15, true
		}
	})

	// ---------- memory ----------
	ms := win.MemoryStatus()
	page := uint64(pi.PageSize)
	mem := MemInfo{
		Total: ms.TotalPhys, Avail: ms.AvailPhys, Used: ms.TotalPhys - ms.AvailPhys,
		Commit: uint64(pi.CommitTotal) * page, CommitLimit: uint64(pi.CommitLimit) * page,
		Paged: uint64(pi.KernelPaged) * page, NonPaged: uint64(pi.KernelNonpaged) * page,
		Load: float64(ms.MemoryLoad),
	}
	if ms.TotalPhys > 0 {
		mem.Load = 100 * float64(mem.Used) / float64(ms.TotalPhys)
	}
	if v, ok := c.q.Value("cache"); ok {
		mem.Cached = uint64(v)
	}
	sb := 0.0
	for _, k := range []string{"standby", "standby2", "standby3"} {
		if v, ok := c.q.Value(k); ok {
			sb += v
		}
	}
	mem.Standby = uint64(sb)
	if v, ok := c.q.Value("modified"); ok {
		mem.Modified = uint64(v)
	}
	if v, ok := c.q.Value("free"); ok {
		mem.Free = uint64(v)
	}
	if mem.Standby > 0 {
		mem.Cached = mem.Standby + mem.Modified
	}

	// ---------- disks ----------
	seenDisk := map[string]bool{}
	c.q.Array("diskidle", func(n string, v float64) {
		if n == "_Total" {
			return
		}
		d := c.disk(n)
		d.Active = clamp(100 - v)
		seenDisk[n] = true
	})
	c.q.Array("diskr", func(n string, v float64) {
		if d, ok := c.disks[n]; ok {
			d.ReadBps = v
		}
	})
	c.q.Array("diskw", func(n string, v float64) {
		if d, ok := c.disks[n]; ok {
			d.WriteBps = v
		}
	})
	c.q.Array("diskresp", func(n string, v float64) {
		if d, ok := c.disks[n]; ok {
			d.RespMs = v * 1000
		}
	})
	var disks []*DiskInfo
	for k, d := range c.disks {
		if !seenDisk[k] {
			continue
		}
		d.Hist.Push(d.Active)
		d.ReadHist.Push(d.ReadBps)
		d.WriteHist.Push(d.WriteBps)
		disks = append(disks, d)
	}
	sort.Slice(disks, func(i, j int) bool { return disks[i].Key < disks[j].Key })

	// ---------- network ----------
	if time.Since(c.adaptersAt) > 30*time.Second {
		c.adapters, c.adaptersAt = win.Adapters(), time.Now()
	}
	seenNet := map[string]bool{}
	c.q.Array("netbw", func(n string, v float64) {
		lower := strings.ToLower(n)
		if v <= 0 || strings.Contains(lower, "isatap") || strings.Contains(lower, "loopback") ||
			strings.Contains(lower, "teredo") || strings.Contains(lower, "pseudo") || strings.Contains(lower, "wan miniport") ||
			strings.Contains(lower, "virtual") || strings.Contains(lower, "hyper-v") || strings.Contains(lower, "filter") {
			return
		}
		x := c.net(n)
		x.LinkBps = v
		seenNet[n] = true
	})
	c.q.Array("netsend", func(n string, v float64) {
		if x, ok := c.nets[n]; ok {
			x.SendBps = v * 8
		}
	})
	c.q.Array("netrecv", func(n string, v float64) {
		if x, ok := c.nets[n]; ok {
			x.RecvBps = v * 8
		}
	})
	var nets []*NetInfo
	for k, x := range c.nets {
		if !seenNet[k] {
			continue
		}
		x.SendHist.Push(x.SendBps)
		x.RecvHist.Push(x.RecvBps)
		nets = append(nets, x)
	}
	sort.Slice(nets, func(i, j int) bool { return nets[i].Key < nets[j].Key })

	// ---------- GPU ----------
	gs, gok := c.nv.Read()
	engines := map[string]float64{}
	pidVRAM := map[uint32]uint64{}
	pidGPU := map[uint32]map[string]float64{}
	c.gq.Array("pmem", func(n string, v float64) {
		if c.luid != "" && !strings.Contains(strings.ToLower(n), c.luid) {
			return
		}
		if m := pidRe.FindStringSubmatch(n); m != nil {
			pid, _ := strconv.Atoi(m[1])
			pidVRAM[uint32(pid)] += uint64(v)
		}
	})
	c.gq.Array("eng", func(n string, v float64) {
		if c.luid != "" && !strings.Contains(strings.ToLower(n), c.luid) {
			return
		}
		m := pidRe.FindStringSubmatch(n)
		e := engRe.FindStringSubmatch(n)
		if m == nil {
			return
		}
		et := "Other"
		if e != nil {
			et = e[1]
		}
		pid, _ := strconv.Atoi(m[1])
		if pidGPU[uint32(pid)] == nil {
			pidGPU[uint32(pid)] = map[string]float64{}
		}
		pidGPU[uint32(pid)][et] += v
		engines[et] += v
	})
	var shared uint64
	c.q.Array("gshared", func(n string, v float64) {
		if c.luid == "" || strings.Contains(strings.ToLower(n), c.luid) {
			shared += uint64(v)
		}
	})

	// ---------- processes ----------
	procs := c.sampleProcs(dt, pidVRAM, pidGPU)

	// ---------- publish ----------
	var self win.PROCESS_MEMORY_COUNTERS_EX
	self, _ = win.ProcessMemory(win.CurrentProcess())

	c.mu.Lock()
	s := &c.Snap
	s.Seq++
	s.CPU = CPUInfo{Total: total, PerCore: cores, MHz: mhz, BaseMHz: float64(maxMHz),
		Procs: pi.ProcessCount, Threads: pi.ThreadCount, Handles: pi.HandleCount, Uptime: win.Uptime(),
		Temp: temp, TempOK: tempOK}
	s.Mem = mem
	s.Disks, s.Nets = disks, nets
	if gok {
		if gs.MemTotal == 0 {
			gs.MemTotal = s.GPU.Stat.MemTotal
		}
		s.GPU.Stat = gs
	} else {
		// no NVML: derive from Windows counters
		var used uint64
		for _, v := range pidVRAM {
			used += v
		}
		s.GPU.Stat.MemUsed = used
		s.GPU.Stat.Util = uint32(clamp(engines["3D"]))
	}
	for k, v := range engines {
		engines[k] = clamp(v)
	}
	s.GPU.Engines = engines
	for _, k := range []string{"3D", "Copy", "VideoEncode", "VideoDecode", "Compute"} {
		if s.GPU.EngHist[k] == nil {
			s.GPU.EngHist[k] = &Ring{}
		}
		s.GPU.EngHist[k].Push(engines[k])
	}
	s.GPU.SharedUsed = shared
	s.Procs = procs
	s.SelfWS, s.SelfPriv = uint64(self.WorkingSetSize), uint64(self.PrivateUsage)

	c.CPUHist.Push(total)
	for len(c.CoreHist) < len(cores) {
		c.CoreHist = append(c.CoreHist, &Ring{})
	}
	for i, v := range cores {
		c.CoreHist[i].Push(v)
	}
	c.MemHist.Push(float64(mem.Used))
	c.GPUHist.Push(float64(s.GPU.Stat.Util))
	c.VRAMHist.Push(float64(s.GPU.Stat.MemUsed))
	c.GTempHist.Push(float64(s.GPU.Stat.Temp))
	c.CTempHist.Push(temp)
	c.PowerHist.Push(s.GPU.Stat.PowerW)
	c.SharedHist.Push(float64(shared))
	c.mu.Unlock()
}

func (c *Collector) disk(key string) *DiskInfo {
	d := c.disks[key]
	if d == nil {
		parts := strings.SplitN(key, " ", 2)
		label := "Disk " + parts[0]
		if len(parts) > 1 && parts[1] != "" {
			label += " (" + strings.Join(strings.Fields(parts[1]), " ") + ")"
		}
		d = &DiskInfo{Key: key, Label: label, Hist: &Ring{}, ReadHist: &Ring{}, WriteHist: &Ring{}}
		c.disks[key] = d
	}
	return d
}

func normNet(s string) string {
	r := strings.NewReplacer("(", "[", ")", "]", "#", "_", "/", "_", "\\", "_")
	return strings.ToLower(r.Replace(s))
}

func (c *Collector) net(key string) *NetInfo {
	x := c.nets[key]
	if x == nil {
		x = &NetInfo{Key: key, Desc: key, Label: "Ethernet", SendHist: &Ring{}, RecvHist: &Ring{}}
		lower := strings.ToLower(key)
		if strings.Contains(lower, "wi-fi") || strings.Contains(lower, "wireless") || strings.Contains(lower, "wlan") || strings.Contains(lower, "802.11") {
			x.Label = "Wi-Fi"
		}
		c.nets[key] = x
	}
	for _, a := range c.adapters {
		if normNet(a.Desc) == lower(key) {
			x.IP, x.MAC, x.Desc = a.IP, a.MAC, a.Desc
			if a.Type == 71 {
				x.Label = "Wi-Fi"
			}
		}
	}
	return x
}

func lower(s string) string { return strings.ToLower(s) }

func clamp(v float64) float64 {
	if v < 0 || math.IsNaN(v) {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

// sampleProcs reads every process. Handles are kept open between ticks (cheaper).
func (c *Collector) sampleProcs(dt float64, pidVRAM map[uint32]uint64, pidGPU map[uint32]map[string]float64) []*Proc {
	c.entries = win.Snapshot(c.entries)
	if time.Since(c.appPidsAt) > 2*time.Second || c.appPids == nil {
		c.appPids, c.appPidsAt = win.VisibleWindowPids(), time.Now()
	}
	tick := c.Snap.Seq + 1
	ncpu := float64(c.logical)
	out := make([]*Proc, 0, len(c.entries))
	for _, e := range c.entries {
		pc := c.cache[e.Pid]
		if pc != nil && pc.h != 0 {
			// pid reuse check
			if created, _, ok := win.ProcessTimes(pc.h); ok && created != pc.created {
				win.CloseHandle(pc.h)
				pc = nil
			}
		}
		if pc == nil {
			pc = &procCache{}
			pc.h = win.OpenProcess(win.PROCESS_QUERY_LIMITED_INFORMATION|win.PROCESS_VM_READ, e.Pid)
			if pc.h == 0 {
				pc.h = win.OpenProcess(win.PROCESS_QUERY_LIMITED_INFORMATION, e.Pid)
			}
			if pc.h != 0 {
				pc.created, pc.cpu, _ = win.ProcessTimes(pc.h)
				pc.path = win.ImagePath(pc.h)
				pc.is32 = win.IsWow64(pc.h)
				pc.user = win.ProcessUser(pc.h)
				if io, ok := win.IoCounters(pc.h); ok {
					pc.io = io.ReadBytes + io.WriteBytes
				}
				if pc.path != "" {
					pc.desc, pc.company = fileInfo(pc.path)
				}
			}
			pc.session = win.SessionOf(e.Pid)
			c.cache[e.Pid] = pc
		}
		pc.seen = tick
		// a fresh Proc every tick: the UI may still be reading last tick's objects
		p := &Proc{Pid: e.Pid, Ppid: e.Ppid, Name: e.Name, Threads: e.Threads, Path: pc.path, Desc: pc.desc,
			Company: pc.company, User: pc.user, Session: pc.session, Is32: pc.is32, Created: pc.created, Access: pc.h != 0}
		if e.Pid == 0 {
			p.Name = "System Idle Process"
		}
		if pc.h != 0 {
			if _, cpu, ok := win.ProcessTimes(pc.h); ok {
				if cpu >= pc.cpu {
					p.CPU = clamp(float64(cpu-pc.cpu) / 1e7 / dt / ncpu * 100)
					c.App.AddCPU(p.Name, pc.desc, cpu-pc.cpu)
				}
				pc.cpu = cpu
				p.CPUTime = cpu
			}
			if m, ok := win.ProcessMemory(pc.h); ok {
				p.WS, p.Private = uint64(m.WorkingSetSize), uint64(m.PrivateUsage)
			}
			if io, ok := win.IoCounters(pc.h); ok {
				t := io.ReadBytes + io.WriteBytes
				if t >= pc.io {
					p.DiskBps = float64(t-pc.io) / dt
					c.App.AddDisk(p.Name, pc.desc, t-pc.io)
				}
				pc.io = t
			}
			p.Handles = win.HandleCount(pc.h)
			p.Priority = win.PriorityClass(pc.h)
		}
		p.VRAM = pidVRAM[e.Pid]
		if eng := pidGPU[e.Pid]; eng != nil {
			mx := 0.0
			for _, v := range eng {
				mx = math.Max(mx, v)
			}
			p.GPU = clamp(mx)
			c.App.AddGPU(p.Name, pc.desc, p.GPU*dt)
		}
		if t, ok := c.appPids[e.Pid]; ok {
			p.IsApp, p.Title = true, t
		}
		out = append(out, p)
	}
	for pid, pc := range c.cache {
		if pc.seen != tick {
			if pc.h != 0 {
				win.CloseHandle(pc.h)
			}
			delete(c.cache, pid)
		}
	}
	c.App.Tick()
	return out
}

var (
	fileInfoCache = map[string][2]string{}
	fileInfoMu    sync.Mutex
)

func fileInfo(path string) (string, string) {
	fileInfoMu.Lock()
	v, ok := fileInfoCache[path]
	fileInfoMu.Unlock()
	if ok {
		return v[0], v[1]
	}
	d, co := win.FileInfo(path)
	fileInfoMu.Lock()
	if len(fileInfoCache) > 2000 {
		fileInfoCache = map[string][2]string{}
	}
	fileInfoCache[path] = [2]string{d, co}
	fileInfoMu.Unlock()
	return d, co
}
