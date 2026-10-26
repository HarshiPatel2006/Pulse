//go:build windows

package app

// Black + colorful palette.
const (
	ColBG        = 0x0B0B0E // window
	ColPanel     = 0x121217 // cards / nav
	ColPanel2    = 0x17171D
	ColRowAlt    = 0x0F0F13
	ColHover     = 0x1C1C24
	ColSel       = 0x232336
	ColLine      = 0x24242C
	ColLine2     = 0x2E2E38
	ColText      = 0xECECF1
	ColDim       = 0x9A9AA6
	ColFaint     = 0x5E5E6A
	ColAccent    = 0x8B7CFF // violet
	ColAccent2   = 0x4CC2FF // cyan

	ColCPU   = 0x4CC2FF // cyan-blue
	ColMem   = 0xB47CFF // purple
	ColDisk  = 0x3DDC84 // green
	ColNet   = 0xFF9F43 // orange
	ColGPU   = 0xFF5C7A // pink-red
	ColVRAM  = 0xF46A48 // orange-red
	ColTemp  = 0x3ECF6E
	ColPower = 0xFFD166 // yellow
	ColWarn  = 0xE8A63C
	ColHot   = 0xE05353
	ColOK    = 0x3ECF6E
)

func tempColor(t float64) uint32 {
	switch {
	case t < 70:
		return ColOK
	case t < 85:
		return ColWarn
	}
	return ColHot
}

// Glyphs (Segoe Fluent Icons / Segoe MDL2 Assets).
const (
	GProcesses  = 0xE71D
	GPerf       = 0xE9D2
	GHistory    = 0xE81C
	GStartup    = 0xEC4A
	GUsers      = 0xE716
	GDetails    = 0xE8FD
	GServices   = 0xE90F
	GSysInfo    = 0xE946
	GSettings   = 0xE713
	GSearch     = 0xE721
	GMin        = 0xE921
	GMax        = 0xE922
	GRestore    = 0xE923
	GClose      = 0xE8BB
	GMenu       = 0xE700
	GRight      = 0xE76C
	GDown       = 0xE70D
	GEnd        = 0xE711
	GAdd        = 0xE710
	GCompact    = 0xE73F
	GExpand     = 0xE740
	GPin        = 0xE718
	GPinned     = 0xE841
	GFolder     = 0xE838
	GRefresh    = 0xE72C
	GPlay       = 0xE768
	GStop       = 0xE71A
	GCheck      = 0xE73E
	GHeart      = 0xEB52
)
