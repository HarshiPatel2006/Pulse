//go:build windows

package app

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"pulse/sys"
	"pulse/win"
)

type menuItem struct {
	label   string
	enabled bool
	checked bool
	sub     []menuItem
	act     func()
	sep     bool
}

// popup shows a native (dark) context menu at the cursor.
func (a *App) popup(items []menuItem) {
	var acts []func()
	var build func(items []menuItem) uintptr
	build = func(items []menuItem) uintptr {
		m := win.CreatePopupMenu()
		for _, it := range items {
			if it.sep {
				win.AppendMenu(m, win.MF_SEPARATOR, 0, "")
				continue
			}
			flags := uint32(win.MF_STRING)
			if !it.enabled {
				flags |= win.MF_GRAYED
			}
			if it.checked {
				flags |= win.MF_CHECKED
			}
			if len(it.sub) > 0 {
				sub := build(it.sub)
				win.AppendMenu(m, flags|win.MF_POPUP, sub, it.label)
				continue
			}
			acts = append(acts, it.act)
			win.AppendMenu(m, flags, uintptr(len(acts)), it.label)
		}
		return m
	}
	m := build(items)
	p := win.GetCursorPos()
	r := win.TrackPopupMenu(m, a.hwnd, p.X, p.Y)
	win.DestroyMenu(m)
	if r > 0 && r <= len(acts) && acts[r-1] != nil {
		acts[r-1]()
	}
}

func (a *App) confirm(text string) bool {
	return win.MessageBox(a.hwnd, text, "Pulse", win.MB_YESNO|win.MB_ICONWARNING) == win.IDYES
}

var protectedNames = map[string]bool{
	"system": true, "system idle process": true, "registry": true, "smss.exe": true, "csrss.exe": true,
	"wininit.exe": true, "winlogon.exe": true, "services.exe": true, "lsass.exe": true, "memory compression": true,
	"secure system": true, "lsaiso.exe": true, "fontdrvhost.exe": true, "dwm.exe": true,
}

func isCritical(p *sys.Proc) bool {
	return p.Pid <= 4 || protectedNames[strings.ToLower(p.Name)]
}

// endProcs terminates processes (after confirming risky ones).
func (a *App) endProcs(procs []*sys.Proc, label string) {
	if len(procs) == 0 {
		return
	}
	for _, p := range procs {
		if isCritical(p) {
			if !a.confirm(fmt.Sprintf("%s is a critical Windows process.\nEnding it may crash or restart Windows.\n\nEnd it anyway?", p.Name)) {
				return
			}
			break
		}
	}
	ended, failed := 0, 0
	self := win.CurrentPid()
	for _, p := range procs {
		if p.Pid == self || p.Pid == 0 {
			continue
		}
		if err := win.Terminate(p.Pid); err != nil {
			failed++
		} else {
			ended++
		}
	}
	switch {
	case failed > 0 && ended == 0:
		a.showToast("Couldn't end " + label + " — access denied (try running Pulse as administrator)")
	case failed > 0:
		a.showToast(fmt.Sprintf("Ended %d, %d could not be ended", ended, failed))
	default:
		a.showToast("Ended " + label)
	}
	a.col.Wake()
}

// tree returns p and all its descendants.
func tree(all []*sys.Proc, root *sys.Proc) []*sys.Proc {
	out := []*sys.Proc{root}
	kids := map[uint32][]*sys.Proc{}
	for _, p := range all {
		if p.Pid != p.Ppid && p.Created >= root.Created {
			kids[p.Ppid] = append(kids[p.Ppid], p)
		}
	}
	for i := 0; i < len(out); i++ {
		out = append(out, kids[out[i].Pid]...)
		if len(out) > 2000 {
			break
		}
	}
	return out
}

func priorityItems(a *App, procs []*sys.Proc) []menuItem {
	cur := uint32(0)
	if len(procs) == 1 {
		cur = procs[0].Priority
	}
	levels := []struct {
		name string
		v    uint32
	}{{"Realtime", 0x100}, {"High", 0x80}, {"Above normal", 0x8000}, {"Normal", 0x20}, {"Below normal", 0x4000}, {"Low", 0x40}}
	var out []menuItem
	for _, l := range levels {
		lv := l
		out = append(out, menuItem{label: l.name, enabled: true, checked: cur == l.v, act: func() {
			bad := 0
			for _, p := range procs {
				if win.SetPriority(p.Pid, lv.v) != nil {
					bad++
				}
			}
			if bad > 0 {
				a.showToast("Couldn't change priority (access denied)")
			} else {
				a.showToast("Priority set to " + lv.name)
			}
		}})
	}
	return out
}

func (a *App) openLocation(path string) {
	if path == "" {
		a.showToast("File location unavailable")
		return
	}
	win.ShellOpen("open", "explorer.exe", `/select,"`+path+`"`, "")
}

func searchOnline(name string) {
	win.ShellOpen("open", "https://www.bing.com/search?q="+url.QueryEscape(name), "", "")
}

func (a *App) procMenu(procs []*sys.Proc, all []*sys.Proc, label string) {
	if len(procs) == 0 {
		return
	}
	p := procs[0]
	single := len(procs) == 1
	items := []menuItem{
		{label: "End task", enabled: true, act: func() { a.endProcs(procs, label) }},
		{label: "End process tree", enabled: single, act: func() {
			t := tree(all, p)
			if a.confirm(fmt.Sprintf("End %s and %d child process(es)?", label, len(t)-1)) {
				a.endProcs(t, label+" tree")
			}
		}},
		{sep: true},
		{label: "Set priority", enabled: true, sub: priorityItems(a, procs)},
		{label: "Go to details", enabled: true, act: func() {
			a.goPage(PDetails)
			if dp, ok := a.pages[PDetails].(*detailsPage); ok {
				dp.t.Sel = fmt.Sprint(p.Pid)
				dp.reveal = true
			}
		}},
		{sep: true},
		{label: "Open file location", enabled: p.Path != "", act: func() { a.openLocation(p.Path) }},
		{label: "Search online", enabled: true, act: func() { searchOnline(p.Name) }},
		{label: "Properties", enabled: p.Path != "", act: func() { win.ShellOpen("properties", p.Path, "", "") }},
		{sep: true},
		{label: "Copy", enabled: true, sub: []menuItem{
			{label: "Name", enabled: true, act: func() { win.CopyToClipboard(a.hwnd, p.Name) }},
			{label: "PID", enabled: true, act: func() { win.CopyToClipboard(a.hwnd, fmt.Sprint(p.Pid)) }},
			{label: "Path", enabled: p.Path != "", act: func() { win.CopyToClipboard(a.hwnd, p.Path) }},
		}},
	}
	a.popup(items)
}

func runNewTask() {
	win.ShellOpen("open", "explorer.exe", "shell:::{2559a1f3-21d7-11d4-bdaf-00c04f60b9f0}", "")
}

func baseName(p string) string { return filepath.Base(p) }
