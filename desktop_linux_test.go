//go:build linux

package main

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"pulse/linuxmon"
)

func TestDesktopResponsiveLayouts(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	a.Settings().SetTheme(pulseTheme{theme.DefaultTheme()})
	w := a.NewWindow("Pulse layout test")
	d := &desktop{a: a, w: w, selected: -1, body: container.NewStack(), search: widget.NewEntry(), status: widget.NewLabel(""), graphs: map[string][]float64{"CPU total": {10, 20}, "Memory": {40, 42}, "GPU": {80, 90}, "VRAM": {50, 60}}, gpu: map[string]string{"name": "GeForce RTX 5060 Ti", "utilization.gpu": "90", "memory.used": "10000", "memory.total": "16311"}, vram: map[int32]uint64{}, gpuProc: map[int32]float64{}, s: linuxmon.Snapshot{Total: 32 << 30, Memory: 14 << 30, CPU: []float64{20, 30}, Procs: []linuxmon.Proc{{PID: 42, Name: "long process name that must not overlap resource columns", CPU: 20, Memory: 1 << 30}}, History: map[string]linuxmon.History{}}}
	d.table = d.createTable()
	shell := d.shell()
	w.SetContent(shell)
	for _, size := range []fyne.Size{fyne.NewSize(1080, 720), fyne.NewSize(1280, 820), fyne.NewSize(1920, 1080)} {
		w.Resize(size)
		d.showPage("Processes")
		_ = w.Canvas().Capture()
		if shell.MinSize().Width > size.Width || shell.MinSize().Height > size.Height {
			t.Fatalf("shell does not fit %v: minimum %v", size, shell.MinSize())
		}
		if d.table.Size().Width < 620 {
			t.Fatalf("table collapsed at %v", size)
		}
		d.showPage("Details")
		if d.headers[d.sortCol] != "CPU" {
			t.Fatal("Details should initially sort by CPU")
		}
		d.showPage("Performance")
		for _, resource := range []string{"CPU", "Memory", "GPU"} {
			d.perfMode = resource
			d.refreshPerformance()
			_ = w.Canvas().Capture()
			if d.performanceDetail.MinSize().Width > d.perfScroll.Size().Width {
				t.Fatalf("%s card overflows %v: %v > %v", resource, size, d.performanceDetail.MinSize(), d.perfScroll.Size())
			}
		}
	}
}
func TestGraphHandlesPartialAndOutOfRangeHistory(t *testing.T) {
	for _, v := range [][]float64{nil, {0}, {-10, 0, 100, 150}} {
		img := graphGenerator(v, color.NRGBA{76, 194, 255, 255})(320, 120)
		if img.Bounds().Dx() != 320 || img.Bounds().Dy() != 120 {
			t.Fatal("incorrect graph bounds")
		}
		_, _, _, alpha := img.At(100, 50).RGBA()
		if alpha != 65535 {
			t.Fatal("graph background must remain opaque")
		}
	}
}

func TestSortTransferRates(t *testing.T) {
	if sortValue("1.1 MiB/s") <= sortValue("900 KiB/s") {
		t.Fatal("rate sorting must compare byte rates, not numeric prefixes")
	}
}
