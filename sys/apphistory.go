//go:build windows

package sys

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AppHistory keeps per-app resource totals (CPU time, disk, GPU time) and
// saves them so they survive restarts, like Task Manager's App history.
type AppHistory struct {
	mu     sync.Mutex
	Since  time.Time           `json:"since"`
	Apps   map[string]*AppUse  `json:"apps"`
	dirty  int
}

type AppUse struct {
	Name    string  `json:"name"`
	CPU     uint64  `json:"cpu"`  // 100ns
	Disk    uint64  `json:"disk"` // bytes
	GPUSec  float64 `json:"gpu"`  // seconds of 100% GPU
}

func DataDir() string {
	d := os.Getenv("LOCALAPPDATA")
	if d == "" {
		d = os.TempDir()
	}
	d = filepath.Join(d, "Pulse")
	os.MkdirAll(d, 0o755)
	return d
}

func histPath() string { return filepath.Join(DataDir(), "apphistory.json") }

func LoadAppHistory() *AppHistory {
	h := &AppHistory{Since: time.Now(), Apps: map[string]*AppUse{}}
	if b, err := os.ReadFile(histPath()); err == nil {
		json.Unmarshal(b, h)
		if h.Apps == nil {
			h.Apps = map[string]*AppUse{}
		}
	}
	return h
}

func (h *AppHistory) get(exe, desc string) *AppUse {
	a := h.Apps[exe]
	if a == nil {
		a = &AppUse{Name: desc}
		h.Apps[exe] = a
	}
	if a.Name == "" && desc != "" {
		a.Name = desc
	}
	return a
}

func (h *AppHistory) AddCPU(exe, desc string, v uint64) {
	if v == 0 || exe == "System Idle Process" {
		return
	}
	h.mu.Lock()
	h.get(exe, desc).CPU += v
	h.mu.Unlock()
}
func (h *AppHistory) AddDisk(exe, desc string, v uint64) {
	if v == 0 {
		return
	}
	h.mu.Lock()
	h.get(exe, desc).Disk += v
	h.mu.Unlock()
}
func (h *AppHistory) AddGPU(exe, desc string, v float64) {
	if v <= 0 {
		return
	}
	h.mu.Lock()
	h.get(exe, desc).GPUSec += v / 100
	h.mu.Unlock()
}

// Tick saves to disk about once a minute.
func (h *AppHistory) Tick() {
	h.dirty++
	if h.dirty >= 60 {
		h.dirty = 0
		h.Save()
	}
}

func (h *AppHistory) Save() {
	h.mu.Lock()
	b, err := json.Marshal(h)
	h.mu.Unlock()
	if err == nil {
		os.WriteFile(histPath(), b, 0o644)
	}
}

func (h *AppHistory) Reset() {
	h.mu.Lock()
	h.Apps = map[string]*AppUse{}
	h.Since = time.Now()
	h.mu.Unlock()
	h.Save()
}

type AppRow struct {
	Exe string
	AppUse
}

func (h *AppHistory) Rows() []AppRow {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]AppRow, 0, len(h.Apps))
	for k, v := range h.Apps {
		out = append(out, AppRow{k, *v})
	}
	return out
}
