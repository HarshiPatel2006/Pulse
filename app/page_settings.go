//go:build windows

package app

import (
	"fmt"
	"os"

	"pulse/sys"
	"pulse/ui"
	"pulse/win"
)

type settingsPage struct{}

const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`

func startsWithWindows() bool {
	return win.RegString(win.HKEY_CURRENT_USER, runKey, "Pulse") != ""
}

func setStartWithWindows(on bool) bool {
	k := win.RegOpen(win.HKEY_CURRENT_USER, runKey, win.KEY_READ|win.KEY_WRITE)
	if k == 0 {
		return false
	}
	defer win.RegClose(k)
	if on {
		return win.RegSetString(k, "Pulse", `"`+win.ExePath()+`"`)
	}
	return win.RegDelete(k, "Pulse")
}

func (sp *settingsPage) draw(a *App, c *ui.Canvas, x, y, w, h float32) {
	y = a.pageHeader(c, x, y, w, "Settings", "")
	cw := w
	if cw > 760 {
		cw = 760
	}
	row := func(title, sub string, ctrl func(rx, ry float32)) {
		card(c, x, y, cw, 64)
		c.Text(x+18, y+12, cw-260, 22, title, ui.FontSemi, ColText, ui.AlignLeft)
		c.Text(x+18, y+33, cw-260, 20, sub, ui.FontSmall, ColDim, ui.AlignLeft)
		ctrl(x+cw-18, y+22)
		y += 72
	}
	speeds := []int{500, 1000, 4000, 0}
	sel := 1
	for i, v := range speeds {
		if v == a.set.SpeedMs {
			sel = i
		}
	}
	row("Real-time update speed", "How often numbers and graphs refresh. Slower = less CPU.", func(rx, ry float32) {
		names := []string{"High", "Normal", "Low", "Paused"}
		w := float32(0)
		for _, n := range names {
			w += c.TextW(n, ui.FontBody) + 24
		}
		a.segmented(c, rx-w-4, ry-4, 28, "speed", names, sel, func(i int) {
			a.set.SpeedMs = speeds[i]
			a.applySpeed()
			a.set.save()
		})
	})
	row("Always on top", "Keep Pulse above other windows.", func(rx, ry float32) {
		a.toggle(c, rx-40, ry, "set-top", a.set.OnTop, func() { a.setOnTop(!a.set.OnTop) })
	})
	sww := startsWithWindows()
	row("Start with Windows", "Open Pulse automatically when you sign in.", func(rx, ry float32) {
		a.toggle(c, rx-40, ry, "set-boot", sww, func() {
			if !setStartWithWindows(!sww) {
				a.showToast("Couldn't change startup setting")
			}
		})
	})
	row("Save memory", "Give unused memory back to Windows while minimized or in compact mode.", func(rx, ry float32) {
		a.toggle(c, rx-40, ry, "set-trim", a.set.TrimMemory, func() {
			a.set.TrimMemory = !a.set.TrimMemory
			a.set.save()
		})
	})
	row("Compact widget", "Small always-on-top meter: VRAM, RAM, GPU, CPU and temperatures.", func(rx, ry float32) {
		a.btn(c, rx-130, ry-6, 130, 32, "set-compact", "Switch to widget", GCompact, 1, true, a.toggleCompact)
	})
	admin := win.IsAdmin()
	adminSub := "Needed to end some processes, control services and change machine-wide startup apps."
	if admin {
		adminSub = "Pulse is running with administrator rights."
	}
	row("Administrator mode", adminSub, func(rx, ry float32) {
		a.btn(c, rx-170, ry-6, 170, 32, "set-admin", "Restart as admin", 0xE7EF, 0, !admin, func() {
			if win.ShellOpen("runas", win.ExePath(), "--restarted", "") {
				a.saveWindowPos()
				os.Exit(0)
			}
		})
	})
	y += 10
	s := &a.col.Snap
	c.Text(x, y, cw, 20, "About", ui.FontH2, ColText, ui.AlignLeft)
	y += 28
	infoList(c, x, y, cw, [][2]string{
		{"Version", Version},
		{"Pulse memory (working set)", mb(float64(s.SelfWS))},
		{"Pulse memory (private)", mb(float64(s.SelfPriv))},
		{"Settings & history folder", sys.DataDir()},
		{"Shortcuts", "Ctrl+F search · Del end task · Ctrl+1…9 pages · F5 refresh"},
	})
	_ = fmt.Sprint
}

var Version = "2.0"
