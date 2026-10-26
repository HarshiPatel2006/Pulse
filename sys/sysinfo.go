//go:build windows

package sys

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"pulse/win"
)

// Static system information (collected once, in the background).
type KV struct{ K, V string }
type Section struct {
	Title string
	Icon  rune
	Rows  []KV
}

type MemSlot struct {
	Locator, Bank, Type, Maker, Part string
	SizeMB, Speed, ConfSpeed           int
}

type StaticInfo struct {
	OSName, OSVersion, OSBuild string
	CPUName                    string
	Sockets, Cores, Logical    int
	L1, L2, L3                 uint64
	BaseMHz                    int
	Virtualization             bool
	MemSlots                   []MemSlot
	MemType                    string
	MemSpeed                   int
	SlotsUsed, SlotsTotal      int
	Sections                   []Section
	Boot                       time.Time
}

func fmtBytes(b uint64) string { return FmtBytes(float64(b)) }

func FmtBytes(b float64) string {
	switch {
	case b >= 1<<40:
		return fmt.Sprintf("%.1f TB", b/(1<<40))
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", b/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.0f MB", b/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.0f KB", b/(1<<10))
	}
	return fmt.Sprintf("%.0f B", b)
}

func CollectStatic(gpuName, gpuDriver string) *StaticInfo {
	s := &StaticInfo{}
	const nt = `SOFTWARE\Microsoft\Windows NT\CurrentVersion`
	prod := win.RegString(win.HKEY_LOCAL_MACHINE, nt, "ProductName")
	build := win.RegString(win.HKEY_LOCAL_MACHINE, nt, "CurrentBuildNumber")
	if b, _ := strconv.Atoi(build); b >= 22000 {
		prod = strings.Replace(prod, "Windows 10", "Windows 11", 1)
	}
	ubr, _ := win.RegDWORD(win.HKEY_LOCAL_MACHINE, nt, "UBR")
	s.OSName = prod
	s.OSVersion = win.RegString(win.HKEY_LOCAL_MACHINE, nt, "DisplayVersion")
	s.OSBuild = fmt.Sprintf("%s.%d", build, ubr)
	installed, _ := win.RegDWORD(win.HKEY_LOCAL_MACHINE, nt, "InstallDate")

	const cpuKey = `HARDWARE\DESCRIPTION\System\CentralProcessor\0`
	s.CPUName = strings.TrimSpace(win.RegString(win.HKEY_LOCAL_MACHINE, cpuKey, "ProcessorNameString"))
	mhz, _ := win.RegDWORD(win.HKEY_LOCAL_MACHINE, cpuKey, "~MHz")
	s.BaseMHz = int(mhz)
	s.Sockets, s.Cores, s.Logical, s.L1, s.L2, s.L3 = win.CPUTopology()
	s.Boot = time.Now().Add(-time.Duration(win.Uptime()) * time.Millisecond)

	bios, board, system := parseSMBIOS(s)

	// ----- sections -----
	osRows := []KV{
		{"Edition", prod},
		{"Version", s.OSVersion},
		{"OS build", s.OSBuild},
		{"Computer name", win.ComputerName()},
		{"User", win.UserName()},
		{"Pulse running as admin", yesNo(win.IsAdmin())},
		{"Last boot", s.Boot.Format("02 Jan 2006, 15:04")},
	}
	if installed > 0 {
		osRows = append(osRows, KV{"Installed on", time.Unix(int64(installed), 0).Format("02 Jan 2006")})
	}
	s.Sections = append(s.Sections, Section{"Windows", 0xE7F8, osRows})

	cpuRows := []KV{
		{"Processor", s.CPUName},
		{"Base speed", fmt.Sprintf("%.2f GHz", float64(s.BaseMHz)/1000)},
		{"Sockets", strconv.Itoa(s.Sockets)},
		{"Cores", strconv.Itoa(s.Cores)},
		{"Logical processors", strconv.Itoa(s.Logical)},
		{"L1 cache", fmtBytes(s.L1)},
		{"L2 cache", fmtBytes(s.L2)},
		{"L3 cache", fmtBytes(s.L3)},
	}
	s.Sections = append(s.Sections, Section{"Processor", 0xE950, cpuRows})

	ms := win.MemoryStatus()
	memRows := []KV{{"Installed", fmtBytes(roundGB(ms.TotalPhys))}, {"Usable", fmtBytes(ms.TotalPhys)}}
	if s.MemType != "" {
		memRows = append(memRows, KV{"Type", s.MemType})
	}
	if s.MemSpeed > 0 {
		memRows = append(memRows, KV{"Speed", fmt.Sprintf("%d MT/s", s.MemSpeed)})
	}
	if s.SlotsTotal > 0 {
		memRows = append(memRows, KV{"Slots used", fmt.Sprintf("%d of %d", s.SlotsUsed, s.SlotsTotal)})
	}
	for _, m := range s.MemSlots {
		if m.SizeMB == 0 {
			continue
		}
		v := fmt.Sprintf("%d GB %s", m.SizeMB/1024, m.Type)
		if m.ConfSpeed > 0 {
			v += fmt.Sprintf(" @ %d", m.ConfSpeed)
		}
		if m.Maker != "" {
			v += " · " + m.Maker
		}
		if m.Part != "" {
			v += " " + m.Part
		}
		memRows = append(memRows, KV{m.Locator, v})
	}
	s.Sections = append(s.Sections, Section{"Memory", 0xE964, memRows})

	var gpuRows []KV
	for i, a := range win.DXGIAdapters() {
		if a.Desc == "Microsoft Basic Render Driver" {
			continue
		}
		label := fmt.Sprintf("GPU %d", i)
		gpuRows = append(gpuRows, KV{label, a.Desc}, KV{"  Dedicated memory", fmtBytes(a.Dedicated)}, KV{"  Shared memory", fmtBytes(a.Shared)})
	}
	if gpuDriver != "" {
		gpuRows = append(gpuRows, KV{"NVIDIA driver", gpuDriver})
	}
	for i, d := range win.Displays() {
		v := fmt.Sprintf("%d × %d @ %d Hz", d.W, d.H, d.Hz)
		if d.Primary {
			v += " (primary)"
		}
		name := d.Name
		if name == "" {
			name = fmt.Sprintf("Display %d", i+1)
		}
		gpuRows = append(gpuRows, KV{name, v})
	}
	s.Sections = append(s.Sections, Section{"Graphics & displays", 0xE7F4, gpuRows})

	boardRows := []KV{}
	add := func(k, v string) {
		if strings.TrimSpace(v) != "" && !strings.EqualFold(v, "To be filled by O.E.M.") && !strings.EqualFold(v, "Default string") {
			boardRows = append(boardRows, KV{k, v})
		}
	}
	add("System maker", system[0])
	add("System model", system[1])
	add("Motherboard", strings.TrimSpace(board[0]+" "+board[1]))
	add("BIOS", strings.TrimSpace(bios[0]+" "+bios[1]))
	add("BIOS date", bios[2])
	s.Sections = append(s.Sections, Section{"Motherboard & BIOS", 0xE8EA, boardRows})

	var diskRows []KV
	for _, d := range win.Drives() {
		if d.Total == 0 {
			continue
		}
		label := d.Label
		if label == "" {
			label = "Local Disk"
		}
		used := d.Total - d.Free
		diskRows = append(diskRows, KV{d.Root + " " + label, fmt.Sprintf("%s free of %s (%s, %.0f%% used)", fmtBytes(d.Free), fmtBytes(d.Total), d.FS, 100*float64(used)/float64(d.Total))})
	}
	s.Sections = append(s.Sections, Section{"Storage", 0xEDA2, diskRows})

	var netRows []KV
	for _, a := range win.Adapters() {
		if a.IP == "" || a.IP == "0.0.0.0" {
			continue
		}
		v := a.IP
		if a.Gateway != "" && a.Gateway != "0.0.0.0" {
			v += " · gateway " + a.Gateway
		}
		netRows = append(netRows, KV{a.Desc, v}, KV{"  MAC", a.MAC})
	}
	s.Sections = append(s.Sections, Section{"Network", 0xE968, netRows})

	if p := win.Power(); p.Flag != 128 && p.Flag != 255 {
		src := "Battery"
		if p.ACLine == 1 {
			src = "Plugged in"
		}
		s.Sections = append(s.Sections, Section{"Battery", 0xE83F, []KV{{"Charge", fmt.Sprintf("%d%%", p.Percent)}, {"Power", src}}})
	}
	return s
}

func yesNo(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

func roundGB(b uint64) uint64 {
	gb := (b + (1 << 30) - 1) >> 30
	// round up to a common module total (e.g. 31.1 -> 32)
	for _, n := range []uint64{2, 4, 6, 8, 12, 16, 24, 32, 48, 64, 96, 128, 192, 256} {
		if gb <= n {
			return n << 30
		}
	}
	return gb << 30
}

// parseSMBIOS fills memory slots and returns BIOS {vendor,version,date}, board {maker,product}, system {maker,model}.
func parseSMBIOS(s *StaticInfo) (bios [3]string, board [2]string, system [2]string) {
	raw := win.SMBIOS()
	if len(raw) < 8 {
		return
	}
	data := raw[8:]
	memTypes := map[byte]string{0x18: "DDR3", 0x1A: "DDR4", 0x1B: "LPDDR", 0x1C: "LPDDR2", 0x1D: "LPDDR3", 0x1E: "LPDDR4", 0x22: "DDR5", 0x23: "LPDDR5"}
	for i := 0; i+4 <= len(data); {
		t, l := data[i], int(data[i+1])
		if l < 4 || i+l > len(data) {
			break
		}
		f := data[i : i+l]
		// strings section
		j := i + l
		var strs []string
		start := j
		for j+1 < len(data) {
			if data[j] == 0 {
				if j == start {
					break
				}
				strs = append(strs, string(data[start:j]))
				start = j + 1
				if data[j+1] == 0 {
					break
				}
			}
			j++
		}
		str := func(off int) string {
			if off >= len(f) {
				return ""
			}
			idx := int(f[off])
			if idx == 0 || idx > len(strs) {
				return ""
			}
			return strings.TrimSpace(strs[idx-1])
		}
		w := func(off int) int {
			if off+1 >= len(f) {
				return 0
			}
			return int(f[off]) | int(f[off+1])<<8
		}
		switch t {
		case 0:
			bios = [3]string{str(4), str(5), str(8)}
		case 1:
			system = [2]string{str(4), str(5)}
		case 2:
			board = [2]string{str(4), str(5)}
		case 17:
			s.SlotsTotal++
			size := w(0x0C)
			mb := 0
			switch {
			case size == 0 || size == 0xFFFF:
			case size == 0x7FFF && len(f) >= 0x20:
				mb = int(f[0x1C]) | int(f[0x1D])<<8 | int(f[0x1E])<<16 | int(f[0x1F])<<24
			case size&0x8000 != 0:
				mb = (size & 0x7FFF) / 1024
			default:
				mb = size
			}
			slot := MemSlot{Locator: str(0x10), Bank: str(0x11), SizeMB: mb, Speed: w(0x15), Maker: str(0x17), Part: str(0x1A)}
			if len(f) > 0x12 {
				slot.Type = memTypes[f[0x12]]
			}
			if len(f) >= 0x22 {
				slot.ConfSpeed = w(0x20)
			}
			if slot.ConfSpeed == 0 {
				slot.ConfSpeed = slot.Speed
			}
			if mb > 0 {
				s.SlotsUsed++
				if slot.Type != "" {
					s.MemType = slot.Type
				}
				if slot.ConfSpeed > s.MemSpeed {
					s.MemSpeed = slot.ConfSpeed
				}
			}
			s.MemSlots = append(s.MemSlots, slot)
		case 127:
			return
		}
		i = j + 2
	}
	return
}
