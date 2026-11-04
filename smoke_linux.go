//go:build linux

package main

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
)

// Optional environment-driven captures exercise the same native window and data
// path as normal launches. No sample data or alternate renderer is involved.
func (d *desktop) startCapture() {
	target, proofDir := os.Getenv("PULSE_SCREENSHOT"), os.Getenv("PULSE_PROOF_DIR")
	if target == "" && proofDir == "" {
		return
	}
	go func() {
		delay := 7 * time.Second
		if v, e := time.ParseDuration(os.Getenv("PULSE_SCREENSHOT_DELAY")); e == nil && v >= 0 {
			delay = v
		}
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-d.stop:
			return
		case <-timer.C:
		}
		if proofDir != "" {
			if e := os.MkdirAll(proofDir, 0700); e != nil {
				fyne.Do(func() { d.err(e) })
				return
			}
			for _, page := range []string{"Processes", "Performance:GPU", "Performance:Memory", "Performance:CPU", "App history", "Startup apps", "Users", "Details", "Services", "System info", "Compact widget"} {
				parts := strings.SplitN(page, ":", 2)
				resource := ""
				if len(parts) > 1 {
					resource = parts[1]
				}
				name := strings.ToLower(strings.ReplaceAll(page, " ", "-"))
				name = strings.ReplaceAll(name, ":", "-")
				d.capturePage(parts[0], resource, filepath.Join(proofDir, name+".png"))
			}
			return
		}
		d.capturePage(os.Getenv("PULSE_SCREENSHOT_PAGE"), os.Getenv("PULSE_SCREENSHOT_RESOURCE"), target)
	}()
}
func (d *desktop) capturePage(page, resource, path string) {
	fyne.DoAndWait(func() {
		if page == "Compact widget" {
			d.openCompact()
		} else if page != "" {
			d.showPage(page)
			if resource != "" {
				d.perfMode = resource
				d.refreshPerformance()
			}
		}
	})
	// Let async service reads and the next layout/render pass complete.
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-d.stop:
		return
	case <-timer.C:
	}
	fyne.DoAndWait(func() {
		canvas := d.w.Canvas()
		if page == "Compact widget" {
			canvas = d.widget.Canvas()
		}
		f, e := os.Create(path)
		if e == nil {
			e = png.Encode(f, canvas.Capture())
			_ = f.Close()
		}
		d.err(e)
	})
}
