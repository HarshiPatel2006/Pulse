//go:build windows

package sys

import (
	"encoding/csv"
	"fmt"
	"os"
)

// ExportProcesses writes a portable snapshot that can be opened in a spreadsheet.
func ExportProcesses(path string, procs []*Proc) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err = w.Write([]string{"name", "pid", "user", "cpu_percent", "memory_bytes", "disk_bytes_per_second", "gpu_percent", "vram_bytes", "path"}); err != nil {
		return err
	}
	for _, p := range procs {
		if err = w.Write([]string{p.Name, fmt.Sprint(p.Pid), p.User, fmt.Sprintf("%.2f", p.CPU), fmt.Sprint(p.Private), fmt.Sprintf("%.0f", p.DiskBps), fmt.Sprintf("%.2f", p.GPU), fmt.Sprint(p.VRAM), p.Path}); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}
