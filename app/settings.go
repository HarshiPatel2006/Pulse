//go:build windows

package app

import (
	"encoding/json"
	"os"
	"path/filepath"

	"pulse/sys"
)

type Settings struct {
	Page          int     `json:"page"`
	NavCollapsed  bool    `json:"nav_collapsed"`
	SpeedMs       int     `json:"speed_ms"` // 0 = paused
	OnTop         bool    `json:"on_top"`
	Compact       bool    `json:"compact"`
	WX            int32   `json:"wx"`
	WY            int32   `json:"wy"`
	WW            int32   `json:"ww"`
	WH            int32   `json:"wh"`
	Maximized     bool    `json:"maximized"`
	CX            int32   `json:"cx"`
	CY            int32   `json:"cy"`
	CPULogical    bool    `json:"cpu_logical"`
	GroupApps     bool    `json:"group_apps"`
	ProcSort      string  `json:"proc_sort"`
	ProcDesc      bool    `json:"proc_desc"`
	DetailSort    string  `json:"detail_sort"`
	DetailDesc    bool    `json:"detail_desc"`
	ShowGraphs    bool    `json:"compact_graphs"`
	TrimMemory    bool    `json:"trim_memory"`
}

func defaults() Settings {
	return Settings{SpeedMs: 1000, GroupApps: true, ProcSort: "cpu", ProcDesc: true, DetailSort: "name",
		WW: 1180, WH: 760, CX: -1, CY: -1, WX: -1, WY: -1, ShowGraphs: true, TrimMemory: true}
}

func settingsPath() string { return filepath.Join(sys.DataDir(), "settings.json") }

func loadSettings() Settings {
	s := defaults()
	if b, err := os.ReadFile(settingsPath()); err == nil {
		json.Unmarshal(b, &s)
	}
	if s.WW < 640 {
		s.WW = 1180
	}
	if s.WH < 420 {
		s.WH = 760
	}
	return s
}

func (s *Settings) save() {
	if b, err := json.MarshalIndent(s, "", "  "); err == nil {
		os.WriteFile(settingsPath(), b, 0o644)
	}
}
