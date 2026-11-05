//go:build linux

package linuxmon

import (
	"encoding/csv"
	"fmt"
	"os"
)

// ExportProcesses writes a portable snapshot that can be opened in a spreadsheet.
func ExportProcesses(path string, procs []Proc) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err = w.Write([]string{"name", "pid", "user", "cpu_percent", "memory_bytes", "disk_bytes_per_second", "nice", "threads", "cpu_seconds"}); err != nil {
		return err
	}
	for _, p := range procs {
		if err = w.Write([]string{p.Name, fmt.Sprint(p.PID), p.User, fmt.Sprintf("%.2f", p.CPU), fmt.Sprint(p.Memory), fmt.Sprintf("%.0f", p.Disk), fmt.Sprint(p.Nice), fmt.Sprint(p.Threads), fmt.Sprintf("%.2f", p.Seconds)}); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}
