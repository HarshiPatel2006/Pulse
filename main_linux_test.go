//go:build linux

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v4/process"
	"pulse/linuxmon"
)

func TestStartupTogglePreservesOtherSections(t *testing.T) {
	p := filepath.Join(t.TempDir(), "test.desktop")
	s := "[Desktop Entry]\nType=Application\nName=Test\nExec=sample --arg\nX-GNOME-Autostart-enabled=false\n[Other]\nHidden=untouched\n"
	if e := os.WriteFile(p, []byte(s), 0600); e != nil {
		t.Fatal(e)
	}
	if e := toggleStartup(p); e != nil {
		t.Fatal(e)
	}
	b, _ := os.ReadFile(p)
	if desktopField(string(b), "Hidden") != "false" || !strings.Contains(string(b), "[Other]\nHidden=untouched") || !strings.Contains(string(b), "Exec=sample --arg") {
		t.Fatalf("bad rewrite: %s", b)
	}
	if e := toggleStartup(p); e != nil {
		t.Fatal(e)
	}
	b, _ = os.ReadFile(p)
	if desktopField(string(b), "Hidden") != "true" {
		t.Fatal("second toggle failed")
	}
}
func TestRejectReusedPID(t *testing.T) {
	p, e := process.NewProcess(int32(os.Getpid()))
	if e != nil {
		t.Fatal(e)
	}
	start, e := p.CreateTime()
	if e != nil {
		t.Fatal(e)
	}
	r := linuxmon.Proc{PID: p.Pid, Start: start}
	if e := checkIdentity(r); e != nil {
		t.Fatal(e)
	}
	r.Start--
	if checkIdentity(r) == nil {
		t.Fatal("stale identity accepted")
	}
}

func TestKillTreeOwnChild(t *testing.T) {
	cmd := exec.Command("sh", "-c", "sleep 30 & wait")
	if e := cmd.Start(); e != nil {
		t.Fatal(e)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	parent, e := process.NewProcess(int32(cmd.Process.Pid))
	if e != nil {
		t.Fatal(e)
	}
	var children []*process.Process
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		children, _ = parent.Children()
		if len(children) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(children) == 0 {
		t.Fatal("test child did not start")
	}
	start, _ := parent.CreateTime()
	ps := []linuxmon.Proc{{PID: parent.Pid, Start: start}}
	for _, p := range children {
		start, _ := p.CreateTime()
		ps = append(ps, linuxmon.Proc{PID: p.Pid, PPID: parent.Pid, Start: start})
	}
	if e := killTree(parent.Pid, ps); e != nil {
		t.Fatal(e)
	}
	if e := cmd.Wait(); e == nil {
		t.Fatal("test parent was not terminated")
	}
	for _, p := range children {
		deadline := time.Now().Add(time.Second)
		stopped := false
		for time.Now().Before(deadline) {
			status, e := p.Status()
			if e != nil {
				stopped = true
				break
			}
			if len(status) == 0 || (len(status) == 1 && (status[0] == process.Zombie || status[0] == "dead")) {
				stopped = true
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if !stopped {
			t.Fatalf("child %d still running", p.Pid)
		}
	}
}
