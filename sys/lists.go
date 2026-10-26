//go:build windows

package sys

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"pulse/win"
)

// ---------------- Startup apps ----------------

type StartupApp struct {
	Name, Command, Path string
	Publisher           string
	Location            string // "Registry (user)", "Registry (machine)", "Startup folder"
	Enabled             bool
	approvedRoot        uintptr
	approvedKey         string
	valueName           string
}

const approvedBase = `Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\`

func StartupApps() []StartupApp {
	var out []StartupApp
	regs := []struct {
		root     uintptr
		path     string
		access   uint32
		loc      string
		approved string
	}{
		{win.HKEY_CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, win.KEY_READ, "Registry (user)", "Run"},
		{win.HKEY_LOCAL_MACHINE, `Software\Microsoft\Windows\CurrentVersion\Run`, win.KEY_READ | win.KEY_WOW64_64KEY, "Registry (machine)", "Run"},
		{win.HKEY_LOCAL_MACHINE, `Software\Microsoft\Windows\CurrentVersion\Run`, win.KEY_READ | win.KEY_WOW64_32KEY, "Registry (machine, 32-bit)", "Run32"},
	}
	for _, r := range regs {
		k := win.RegOpen(r.root, r.path, r.access)
		if k == 0 {
			continue
		}
		for _, name := range win.RegValues(k) {
			cmd, _ := win.RegValue(k, name)
			if cmd == "" {
				continue
			}
			a := StartupApp{Name: name, Command: cmd, Path: exeFromCommand(cmd), Location: r.loc,
				approvedRoot: r.root, approvedKey: approvedBase + r.approved, valueName: name}
			out = append(out, a)
		}
		win.RegClose(k)
	}
	folders := []struct {
		dir  string
		root uintptr
		loc  string
	}{
		{filepath.Join(os.Getenv("APPDATA"), `Microsoft\Windows\Start Menu\Programs\Startup`), win.HKEY_CURRENT_USER, "Startup folder (user)"},
		{filepath.Join(os.Getenv("ProgramData"), `Microsoft\Windows\Start Menu\Programs\Startup`), win.HKEY_LOCAL_MACHINE, "Startup folder (all users)"},
	}
	for _, f := range folders {
		ents, _ := os.ReadDir(f.dir)
		for _, e := range ents {
			if e.IsDir() || strings.EqualFold(e.Name(), "desktop.ini") {
				continue
			}
			full := filepath.Join(f.dir, e.Name())
			out = append(out, StartupApp{Name: strings.TrimSuffix(e.Name(), filepath.Ext(e.Name())), Command: full, Path: full,
				Location: f.loc, approvedRoot: f.root, approvedKey: approvedBase + "StartupFolder", valueName: e.Name()})
		}
	}
	for i := range out {
		a := &out[i]
		a.Enabled = true
		if k := win.RegOpen(a.approvedRoot, a.approvedKey, win.KEY_READ|win.KEY_WOW64_64KEY); k != 0 {
			if _, raw := win.RegValue(k, a.valueName); len(raw) > 0 {
				a.Enabled = raw[0]&1 == 0 // 02/06 enabled, 03/07 disabled
			}
			win.RegClose(k)
		}
		if strings.HasSuffix(strings.ToLower(a.Path), ".exe") {
			desc, co := fileInfo(a.Path)
			a.Publisher = co
			if desc != "" && len(desc) < 60 {
				a.Name = desc
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}

func exeFromCommand(cmd string) string {
	cmd = strings.TrimSpace(ExpandEnv(cmd))
	if strings.HasPrefix(cmd, `"`) {
		if i := strings.Index(cmd[1:], `"`); i >= 0 {
			return cmd[1 : i+1]
		}
	}
	lower := strings.ToLower(cmd)
	if i := strings.Index(lower, ".exe"); i >= 0 {
		return cmd[:i+4]
	}
	if f := strings.Fields(cmd); len(f) > 0 {
		return f[0]
	}
	return cmd
}

var envRe = regexp.MustCompile(`%([^%]+)%`)

// ExpandEnv expands Windows-style %VAR% references.
func ExpandEnv(s string) string {
	return envRe.ReplaceAllStringFunc(s, func(m string) string {
		if v := os.Getenv(m[1 : len(m)-1]); v != "" {
			return v
		}
		return m
	})
}

// SetStartupEnabled flips the Task-Manager-compatible StartupApproved flag.
func SetStartupEnabled(a StartupApp, on bool) bool {
	k := win.RegOpen(a.approvedRoot, a.approvedKey, win.KEY_READ|win.KEY_WRITE|win.KEY_WOW64_64KEY)
	if k == 0 {
		return false
	}
	defer win.RegClose(k)
	data := make([]byte, 12)
	if on {
		data[0] = 2
	} else {
		data[0] = 3
		ft := uint64(time.Now().UnixNano()/100) + 116444736000000000
		binary.LittleEndian.PutUint64(data[4:], ft)
	}
	return win.RegSetBinary(k, a.valueName, data)
}

// ---------------- Services ----------------

type ServiceRow struct {
	win.Service
	Description string
	StartType   uint32
	Group       string
}

func StateName(s uint32) string {
	switch s {
	case 1:
		return "Stopped"
	case 2:
		return "Starting"
	case 3:
		return "Stopping"
	case 4:
		return "Running"
	case 5:
		return "Continuing"
	case 6:
		return "Pausing"
	case 7:
		return "Paused"
	}
	return ""
}

func StartTypeName(t uint32) string {
	switch t {
	case 0, 1:
		return "Boot"
	case 2:
		return "Automatic"
	case 3:
		return "Manual"
	case 4:
		return "Disabled"
	}
	return ""
}

type svcInfo struct {
	desc, group string
	start       uint32
}

var (
	svcDetail   = map[string]svcInfo{}
	svcDetailMu sync.Mutex
)

// Services lists services; details (description/start type) are cached after the first call.
func Services(withDetails bool) []ServiceRow {
	list := win.Services()
	out := make([]ServiceRow, 0, len(list))
	for _, s := range list {
		r := ServiceRow{Service: s}
		svcDetailMu.Lock()
		d, ok := svcDetail[s.Name]
		svcDetailMu.Unlock()
		if ok {
			r.Description, r.Group, r.StartType = d.desc, d.group, d.start
		} else if withDetails {
			r.StartType, r.Group = win.ServiceStartType(s.Name)
			r.Description = win.ServiceDescription(s.Name)
			svcDetailMu.Lock()
			svcDetail[s.Name] = svcInfo{r.Description, r.Group, r.StartType}
			svcDetailMu.Unlock()
		}
		out = append(out, r)
	}
	return out
}

// ---------------- Users ----------------

type UserRow struct {
	win.Session
	CPU     float64
	Mem     uint64
	Disk    float64
	GPU     float64
	VRAM    uint64
	Procs   int
}

func Users(procs []*Proc) []UserRow {
	var out []UserRow
	for _, s := range win.Sessions() {
		if s.User == "" {
			continue
		}
		r := UserRow{Session: s}
		for _, p := range procs {
			if p.Session == s.ID {
				r.CPU += p.CPU
				r.Mem += p.Private
				r.Disk += p.DiskBps
				r.GPU += p.GPU
				r.VRAM += p.VRAM
				r.Procs++
			}
		}
		if r.GPU > 100 {
			r.GPU = 100
		}
		out = append(out, r)
	}
	return out
}

func SessionState(s uint32) string {
	switch s {
	case 0:
		return "Active"
	case 1:
		return "Connected"
	case 4:
		return "Disconnected"
	case 5:
		return "Idle"
	case 6:
		return "Listening"
	}
	return "Other"
}
