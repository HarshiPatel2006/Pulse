//go:build windows

package sys

import (
	"math"
	"math/rand"
	"time"

	"pulse/win"
)

// runDemo feeds realistic fake data (used for screenshots / testing without hardware).
func (c *Collector) runDemo() {
	names := []struct {
		exe, desc, co string
		app           bool
		cpu, mem, vram float64
		gpu            float64
		n              int
	}{
		{"python.exe", "Python", "Python Software Foundation", true, 6, 3.6e9, 13.3e9, 96, 2},
		{"chrome.exe", "Google Chrome", "Google LLC", true, 2, 1.5e9, 0.31e9, 2, 23},
		{"Code.exe", "Visual Studio Code", "Microsoft Corporation", true, 1.2, 1.1e9, 0.12e9, 0, 9},
		{"explorer.exe", "Windows Explorer", "Microsoft Corporation", true, 0.3, 0.44e9, 1.0e9, 0, 1},
		{"claude.exe", "Claude", "Anthropic", true, 0.5, 1.46e9, 0.19e9, 0, 6},
		{"Discord.exe", "Discord", "Discord Inc.", true, 0.4, 0.6e9, 0.07e9, 1, 6},
		{"dwm.exe", "Desktop Window Manager", "Microsoft Corporation", false, 1.5, 0.18e9, 0.42e9, 4, 1},
		{"NVIDIA Overlay.exe", "NVIDIA Overlay", "NVIDIA Corporation", false, 0.1, 0.28e9, 0.19e9, 0, 2},
		{"svchost.exe", "Host Process for Windows Services", "Microsoft Corporation", false, 0.6, 0.9e9, 0, 0, 74},
		{"MsMpEng.exe", "Antimalware Service Executable", "Microsoft Corporation", false, 0.8, 0.31e9, 0, 0, 1},
		{"Pulse.exe", "Pulse", "Pulse", true, 0.2, 0.019e9, 0, 0, 1},
		{"csrss.exe", "Client Server Runtime Process", "Microsoft Corporation", false, 0.1, 0.01e9, 0.1e9, 0, 2},
	}
	c.mu.Lock()
	c.Snap.GPU = GPUInfo{Name: "GeForce RTX 5060 Ti", Driver: "581.42", OK: true, EngHist: map[string]*Ring{}, SharedTotal: 15.5e9}
	c.mu.Unlock()
	disks := []*DiskInfo{
		{Key: "0 C:", Label: "Disk 0 (C:)", Hist: &Ring{}, ReadHist: &Ring{}, WriteHist: &Ring{}},
		{Key: "1 D:", Label: "Disk 1 (D:)", Hist: &Ring{}, ReadHist: &Ring{}, WriteHist: &Ring{}},
	}
	nets := []*NetInfo{{Key: "eth", Label: "Ethernet", Desc: "Realtek PCIe 2.5GbE Family Controller", IP: "192.168.1.24", MAC: "D8-BB-C1-0A-11-22", LinkBps: 1e9, SendHist: &Ring{}, RecvHist: &Ring{}}}
	t := 0.0
	pid := uint32(1000)
	var procs []*Proc
	for _, n := range names {
		for i := 0; i < n.n; i++ {
			pid += 4
			f := 1.0 / float64(n.n)
			procs = append(procs, &Proc{Pid: pid, Name: n.exe, Desc: n.desc, Company: n.co, User: "Welcome", Session: 1,
				Path: `C:\Program Files\` + n.desc + `\` + n.exe, IsApp: n.app && i == 0, Title: n.desc,
				CPU: n.cpu * f, WS: uint64(n.mem * f), Private: uint64(n.mem * f * 0.8), VRAM: uint64(n.vram * f), GPU: n.gpu * f,
				Threads: uint32(20 + rand.Intn(80)), Handles: uint32(200 + rand.Intn(2000)), Priority: 0x20, Access: true})
		}
	}
	for {
		t += 1
		w := math.Sin(t / 7)
		for _, p := range procs {
			p.CPU = math.Max(0, p.CPU*(0.85+rand.Float64()*0.3))
			p.CPUTime += uint64(p.CPU * 1e5)
			p.DiskBps = rand.Float64() * 2e5 * rand.Float64()
		}
		procs[0].CPU = 6 + 4*w
		procs[0].GPU = 90 + 9*w
		cores := make([]float64, 16)
		for i := range cores {
			cores[i] = clamp(12 + 20*math.Sin(t/5+float64(i)) + rand.Float64()*10)
		}
		total := 0.0
		for _, v := range cores {
			total += v
		}
		total /= 16
		for _, d := range disks {
			d.Active = clamp(3 + 8*math.Abs(math.Sin(t/3+float64(len(d.Key)))) + rand.Float64()*4)
			d.ReadBps, d.WriteBps = rand.Float64()*30e6, rand.Float64()*8e6
			d.Hist.Push(d.Active)
			d.ReadHist.Push(d.ReadBps)
			d.WriteHist.Push(d.WriteBps)
		}
		for _, n := range nets {
			n.RecvBps = 2e6 + 1.5e6*math.Sin(t/4) + rand.Float64()*1e6
			n.SendBps = 3e5 + rand.Float64()*2e5
			n.SendHist.Push(n.SendBps)
			n.RecvHist.Push(n.RecvBps)
		}
		gs := win.GPUStat{MemUsed: uint64(13.1e9 + 1e9*w), MemTotal: 17095983104, Util: uint32(90 + 9*w), Temp: uint32(57 + 3*w),
			PowerW: 129 + 25*w, PowerLimitW: 180, ClockMHz: 2602, MemClockMHz: 13801, MaxClockMHz: 3090, Fan: 48, PcieGen: 5, PcieWidth: 8,
			HasTemp: true, HasPower: true, HasFan: true}
		eng := map[string]float64{"3D": 4 + 2*w, "Copy": 2, "VideoEncode": 0, "VideoDecode": 1, "Compute": 92 + 7*w}
		ctemp := 62 + 6*w
		c.mu.Lock()
		s := &c.Snap
		s.Seq++
		s.CPU = CPUInfo{Total: total, PerCore: cores, MHz: 4720, BaseMHz: 3800, Procs: 312, Threads: 4412, Handles: 158321, Uptime: uint64(t*1000) + 3*86400e3 + 4*3600e3, Temp: ctemp, TempOK: true}
		s.Mem = MemInfo{Total: 33406775296, Used: uint64(28.5e9 + 0.5e9*w), Avail: uint64(33406775296 - 28.5e9), Commit: 41e9, CommitLimit: 52e9, Cached: 3.1e9, Paged: 1.2e9, NonPaged: 0.6e9, Modified: 0.2e9, Standby: 2.9e9, Free: 1.6e9}
		s.Mem.Load = 100 * float64(s.Mem.Used) / float64(s.Mem.Total)
		s.Disks, s.Nets = disks, nets
		s.GPU.Stat = gs
		s.GPU.Engines = eng
		for k, v := range eng {
			if s.GPU.EngHist[k] == nil {
				s.GPU.EngHist[k] = &Ring{}
			}
			s.GPU.EngHist[k].Push(v)
		}
		s.GPU.SharedUsed = 0.4e9
		s.Procs = procs
		s.SelfWS, s.SelfPriv = 19e6, 11e6
		c.CPUHist.Push(total)
		for len(c.CoreHist) < 16 {
			c.CoreHist = append(c.CoreHist, &Ring{})
		}
		for i, v := range cores {
			c.CoreHist[i].Push(v)
		}
		c.MemHist.Push(float64(s.Mem.Used))
		c.GPUHist.Push(float64(gs.Util))
		c.VRAMHist.Push(float64(gs.MemUsed))
		c.GTempHist.Push(float64(gs.Temp))
		c.CTempHist.Push(ctemp)
		c.PowerHist.Push(gs.PowerW)
		c.SharedHist.Push(0.4e9)
		for _, p := range procs {
			c.App.AddCPU(p.Name, p.Desc, uint64(p.CPU*1e6))
			c.App.AddDisk(p.Name, p.Desc, uint64(p.DiskBps))
			c.App.AddGPU(p.Name, p.Desc, p.GPU)
		}
		c.mu.Unlock()
		if c.notify != nil {
			c.notify()
		}
		time.Sleep(60 * time.Millisecond)
	}
}
