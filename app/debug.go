//go:build windows

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"pulse/sys"
	"pulse/win"
)

// debugLog writes memory stats periodically when PULSE_DEBUG is set.
func debugLog() {
	if os.Getenv("PULSE_DEBUG") == "" {
		return
	}
	go func() {
		for {
			time.Sleep(5 * time.Second)
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			pm, _ := win.ProcessMemory(win.CurrentProcess())
			line := fmt.Sprintf("%s heapAlloc=%.1fMB heapSys=%.1fMB sys=%.1fMB ws=%.1fMB private=%.1fMB gc=%d goroutines=%d\n",
				time.Now().Format("15:04:05"), mbf(m.HeapAlloc), mbf(m.HeapSys), mbf(m.Sys), mbf(uint64(pm.WorkingSetSize)), mbf(uint64(pm.PrivateUsage)), m.NumGC, runtime.NumGoroutine())
			f, err := os.OpenFile(filepath.Join(sys.DataDir(), "debug.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
			if err == nil {
				f.WriteString(line)
				f.Close()
			}
		}
	}()
}

func mbf(v uint64) float64 { return float64(v) / (1 << 20) }
