//go:build linux

// Package linuxmon collects Linux metrics without elevated privileges.
package linuxmon

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	netstat "github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
	"github.com/shirou/gopsutil/v4/sensors"
)

type Proc struct {
	IOAvailable         bool
	Start               int64
	PID, PPID           int32
	Name, User          string
	CPU                 float64
	Memory, Read, Write uint64
	Disk                float64
	Nice, Threads       int32
	Seconds             float64
}
type History struct {
	CPU         float64
	GPU         float64
	Read, Write uint64
}
type Snapshot struct {
	Available, Cached, SwapTotal, SwapUsed uint64
	DiskActivity                           map[string]float64
	Procs                                  []Proc
	CPU                                    []float64
	Memory, Total                          uint64
	Errors                                 []string
	Disks, Networks                        map[string][2]float64
	History                                map[string]History
}
type Collector struct {
	last        map[int32]Proc
	disks       map[string]disk.IOCountersStat
	nets        map[string]netstat.IOCountersStat
	at          time.Time
	History     map[string]History
	historyPath string
}

func New() *Collector {
	c := &Collector{last: map[int32]Proc{}, History: map[string]History{}}
	if p, e := os.UserConfigDir(); e == nil {
		c.historyPath = filepath.Join(p, "Pulse", "history-linux.json")
		b, _ := os.ReadFile(c.historyPath)
		_ = json.Unmarshal(b, &c.History)
		if c.History == nil {
			c.History = map[string]History{}
		}
	}
	return c
}
func (c *Collector) Save() error {
	if c.historyPath == "" {
		return nil
	}
	if e := os.MkdirAll(filepath.Dir(c.historyPath), 0700); e != nil {
		return e
	}
	b, e := json.Marshal(c.History)
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(c.historyPath), "history-*.tmp")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, e = f.Write(b); e != nil {
		_ = f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(tmp, c.historyPath)
}
func Command(name string, args ...string) (string, error) {
	return CommandTimeout(4*time.Second, name, args...)
}
func CommandTimeout(timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	b, e := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if e != nil {
		return string(b), fmt.Errorf("%s: %w: %s", name, e, strings.TrimSpace(string(b)))
	}
	return string(b), nil
}
func num(s string) float64 { v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64); return v }
func GPU() (map[string]string, map[int32]uint64, map[int32]float64) {
	g := map[string]string{}
	vram := map[int32]uint64{}
	usage := map[int32]float64{}
	keys := []string{"name", "driver_version", "utilization.gpu", "memory.used", "memory.total", "temperature.gpu", "power.draw", "power.limit", "clocks.gr", "clocks.mem", "fan.speed", "utilization.memory", "utilization.encoder", "utilization.decoder"}
	s, e := Command("nvidia-smi", "--query-gpu="+strings.Join(keys, ","), "--format=csv,noheader,nounits")
	if e != nil {
		g["status"] = "NVIDIA telemetry unavailable"
		return g, vram, usage
	}
	rows, _ := csv.NewReader(strings.NewReader(s)).ReadAll()
	if len(rows) > 0 {
		for i, k := range keys {
			if i < len(rows[0]) {
				g[k] = strings.TrimSpace(rows[0][i])
			}
		}
	}
	s, _ = Command("nvidia-smi", "--query-compute-apps=pid,used_gpu_memory", "--format=csv,noheader,nounits")
	rows, _ = csv.NewReader(strings.NewReader(s)).ReadAll()
	for _, r := range rows {
		if len(r) == 2 {
			pid, _ := strconv.ParseInt(strings.TrimSpace(r[0]), 10, 32)
			vram[int32(pid)] = uint64(num(r[1]) * 1048576)
		}
	}
	s, _ = Command("nvidia-smi", "pmon", "-c", "1", "-s", "um")
	parsePmon(s, vram, usage)
	return g, vram, usage
}
func (c *Collector) Collect() Snapshot {
	s := Snapshot{DiskActivity: map[string]float64{}, Disks: map[string][2]float64{}, Networks: map[string][2]float64{}}
	now := time.Now()
	dt := now.Sub(c.at).Seconds()
	s.CPU, _ = cpu.Percent(0, true)
	if m, e := mem.VirtualMemory(); e == nil {
		s.Memory = m.Used
		s.Total = m.Total
		s.Available = m.Available
		s.Cached = m.Cached
		s.SwapTotal = m.SwapTotal
		s.SwapUsed = m.SwapTotal - m.SwapFree
	} else {
		s.Errors = append(s.Errors, e.Error())
	}
	pids, e := process.Pids()
	if e != nil {
		s.Errors = append(s.Errors, e.Error())
	}
	last := map[int32]Proc{}
	for _, pid := range pids {
		p, _ := process.NewProcess(pid)
		if p == nil {
			continue
		}
		r := Proc{PID: pid}
		r.Name, _ = p.Name()
		if r.Name == "" {
			continue
		}
		r.PPID, _ = p.Ppid()
		r.Start, _ = p.CreateTime()
		r.User, _ = p.Username()

		if m, e := p.MemoryInfo(); e == nil {
			r.Memory = m.RSS
		}
		r.Nice, _ = p.Nice()
		r.Threads, _ = p.NumThreads()
		if t, e := p.Times(); e == nil {
			r.Seconds = t.User + t.System
		}
		if io, e := p.IOCounters(); e == nil {
			r.IOAvailable = true
			r.Read = io.ReadBytes
			r.Write = io.WriteBytes
		}
		old, ok := c.last[pid]
		h := c.History[r.Name]
		if ok && r.Start == old.Start && dt > 0 && r.Seconds >= old.Seconds {
			h.CPU += r.Seconds - old.Seconds
			r.CPU = (r.Seconds - old.Seconds) / dt / float64(max(len(s.CPU), 1)) * 100
			if r.IOAvailable && old.IOAvailable && r.Read >= old.Read {
				h.Read += r.Read - old.Read
			}
			if r.IOAvailable && old.IOAvailable && r.Write >= old.Write {
				h.Write += r.Write - old.Write
			}
			if r.IOAvailable && old.IOAvailable && r.Read >= old.Read && r.Write >= old.Write {
				r.Disk = float64(r.Read-old.Read+r.Write-old.Write) / dt
			}
		}
		c.History[r.Name] = h
		last[pid] = r
		s.Procs = append(s.Procs, r)
	}
	d, _ := disk.IOCounters()
	for k, v := range d {
		if strings.HasPrefix(k, "loop") || strings.HasPrefix(k, "ram") {
			continue
		}
		if _, e := os.Stat(filepath.Join("/sys/class/block", k, "partition")); e == nil {
			continue
		}
		if old, ok := c.disks[k]; ok && dt > 0 && v.ReadBytes >= old.ReadBytes && v.WriteBytes >= old.WriteBytes {
			if v.IoTime >= old.IoTime {
				s.DiskActivity[k] = min(100, float64(v.IoTime-old.IoTime)/dt/10)
			}
			s.Disks[k] = [2]float64{float64(v.ReadBytes-old.ReadBytes) / dt, float64(v.WriteBytes-old.WriteBytes) / dt}
		}
	}
	n, _ := netstat.IOCounters(true)
	for _, v := range n {
		if old, ok := c.nets[v.Name]; ok && dt > 0 && v.BytesRecv >= old.BytesRecv && v.BytesSent >= old.BytesSent {
			s.Networks[v.Name] = [2]float64{float64(v.BytesRecv-old.BytesRecv) / dt, float64(v.BytesSent-old.BytesSent) / dt}
		}
	}
	c.nets = map[string]netstat.IOCountersStat{}
	for _, v := range n {
		c.nets[v.Name] = v
	}
	c.disks = d
	c.last = last
	c.at = now
	s.History = make(map[string]History, len(c.History))
	for k, v := range c.History {
		s.History[k] = v
	}
	return s
}
func SystemInfo() string {
	var b strings.Builder
	if h, e := host.Info(); e == nil {
		fmt.Fprintf(&b, "Host: %s\nOS: %s %s\nKernel: %s (%s)\nUptime: %s\n", h.Hostname, h.Platform, h.PlatformVersion, h.KernelVersion, h.KernelArch, time.Duration(h.Uptime)*time.Second)
	}
	if cs, e := cpu.Info(); e == nil && len(cs) > 0 {
		fmt.Fprintf(&b, "CPU: %s\nLogical processors: %d\n", cs[0].ModelName, len(cs))
	}
	for _, p := range []string{"/sys/class/dmi/id/board_vendor", "/sys/class/dmi/id/board_name", "/sys/class/dmi/id/bios_version", "/sys/class/dmi/id/bios_vendor", "/sys/class/dmi/id/bios_date", "/sys/class/dmi/id/product_name", "/proc/meminfo"} {
		if x, e := os.ReadFile(p); e == nil {
			fmt.Fprintf(&b, "\n%s:\n%s\n", filepath.Base(p), strings.TrimSpace(string(x)))
		}
	}

	models, _ := filepath.Glob("/sys/class/block/*/device/model")
	for _, p := range models {
		if x, e := os.ReadFile(p); e == nil {
			fmt.Fprintf(&b, "Drive %s: %s\n", filepath.Base(filepath.Dir(filepath.Dir(p))), strings.TrimSpace(string(x)))
		}
	}
	displays, _ := filepath.Glob("/sys/class/drm/card*-*/status")
	for _, p := range displays {
		if x, e := os.ReadFile(p); e == nil && strings.TrimSpace(string(x)) == "connected" {
			m, _ := os.ReadFile(filepath.Join(filepath.Dir(p), "modes"))
			fmt.Fprintf(&b, "Display %s: %s\n", filepath.Base(filepath.Dir(p)), strings.TrimSpace(string(m)))
		}
	}
	temps, _ := sensors.SensorsTemperatures()
	for _, t := range temps {
		fmt.Fprintf(&b, "%s: %.1f °C\n", t.SensorKey, t.Temperature)
	}
	parts, _ := disk.Partitions(false)
	for _, p := range parts {
		fmt.Fprintf(&b, "%s → %s (%s)\n", p.Device, p.Mountpoint, p.Fstype)
	}
	ifs, _ := netstat.Interfaces()
	for _, i := range ifs {
		fmt.Fprintf(&b, "%s %s %v\n", i.Name, i.HardwareAddr, i.Addrs)
	}
	return b.String()
}

// pmon column sets differ by driver and device. Read names from its header.
func parsePmon(s string, vram map[int32]uint64, usage map[int32]float64) {
	columns := map[string]int{}
	for _, line := range strings.Split(s, "\n") {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		if f[0] == "#" {
			if len(f) > 2 && f[1] == "gpu" && f[2] == "pid" {
				for i, k := range f[1:] {
					columns[k] = i
				}
			}
			continue
		}
		pi, ok := columns["pid"]
		if !ok || pi >= len(f) {
			continue
		}
		pid, e := strconv.ParseInt(f[pi], 10, 32)
		if e != nil {
			continue
		}
		if i, ok := columns["sm"]; ok && i < len(f) && f[i] != "-" {
			usage[int32(pid)] = num(f[i])
		}
		if i, ok := columns["fb"]; ok && i < len(f) && f[i] != "-" {
			vram[int32(pid)] = uint64(num(f[i]) * 1048576)
		}
	}
}

// AddGPUHistory integrates only driver-reported utilization during observed time.
func (c *Collector) AddGPUHistory(s *Snapshot, usage map[int32]float64, dt float64) {
	for _, p := range s.Procs {
		if u, ok := usage[p.PID]; ok && dt > 0 {
			h := c.History[p.Name]
			h.GPU += u / 100 * dt
			c.History[p.Name] = h
			s.History[p.Name] = h
		}
	}
}

type InfoSection struct {
	Title string
	Rows  [][2]string
}

func SystemSections() []InfoSection {
	var sections []InfoSection
	if h, e := host.Info(); e == nil {
		sections = append(sections, InfoSection{"Linux", [][2]string{{"Distribution", h.Platform + " " + h.PlatformVersion}, {"Kernel", h.KernelVersion}, {"Architecture", h.KernelArch}, {"Computer name", h.Hostname}, {"Up time", (time.Duration(h.Uptime) * time.Second).String()}}})
	}
	if cs, e := cpu.Info(); e == nil && len(cs) > 0 {
		c := cs[0]
		sections = append(sections, InfoSection{"Processor", [][2]string{{"Processor", c.ModelName}, {"Logical processors", fmt.Sprint(len(cs))}, {"Clock", fmt.Sprintf("%.0f MHz", c.Mhz)}, {"Cache", fmt.Sprintf("%d KiB", c.CacheSize)}}})
	}
	if m, e := mem.VirtualMemory(); e == nil {
		sections = append(sections, InfoSection{"Memory", [][2]string{{"Usable RAM", fmt.Sprintf("%.1f GiB", float64(m.Total)/1073741824)}, {"Available", fmt.Sprintf("%.1f GiB", float64(m.Available)/1073741824)}, {"Cached", fmt.Sprintf("%.1f GiB", float64(m.Cached)/1073741824)}}})
	}
	board := InfoSection{Title: "Motherboard & BIOS"}
	for _, r := range [][2]string{{"Manufacturer", "board_vendor"}, {"Motherboard", "board_name"}, {"BIOS vendor", "bios_vendor"}, {"BIOS version", "bios_version"}, {"BIOS date", "bios_date"}} {
		if x, e := os.ReadFile(filepath.Join("/sys/class/dmi/id", r[1])); e == nil {
			board.Rows = append(board.Rows, [2]string{r[0], strings.TrimSpace(string(x))})
		}
	}
	if len(board.Rows) > 0 {
		sections = append(sections, board)
	}
	storage := InfoSection{Title: "Storage"}
	if parts, e := disk.Partitions(false); e == nil {
		for _, p := range parts {
			if strings.HasPrefix(p.Device, "/dev/loop") || strings.HasPrefix(p.Mountpoint, "/snap/") {
				continue
			}
			storage.Rows = append(storage.Rows, [2]string{p.Device, p.Mountpoint + " (" + p.Fstype + ")"})
		}
	}
	sections = append(sections, storage)
	network := InfoSection{Title: "Network"}
	ifs, _ := netstat.Interfaces()
	for _, i := range ifs {
		addresses := []string{}
		for _, a := range i.Addrs {
			addresses = append(addresses, a.Addr)
		}
		network.Rows = append(network.Rows, [2]string{i.Name, strings.Join(addresses, ", ")})
	}
	sections = append(sections, network)
	displays := InfoSection{Title: "Displays"}
	paths, _ := filepath.Glob("/sys/class/drm/card*-*/status")
	for _, p := range paths {
		if x, e := os.ReadFile(p); e == nil && strings.TrimSpace(string(x)) == "connected" {
			m, _ := os.ReadFile(filepath.Join(filepath.Dir(p), "modes"))
			modes := strings.Fields(string(m))
			mode := "Connected"
			if len(modes) > 0 {
				mode = modes[0]
			}
			displays.Rows = append(displays.Rows, [2]string{filepath.Base(filepath.Dir(p)), mode})
		}
	}
	if len(displays.Rows) > 0 {
		sections = append(sections, displays)
	}
	return sections
}
