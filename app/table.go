//go:build windows

package app

import (
	"fmt"
	"math"

	"pulse/ui"
)

type Col struct {
	Key     string
	Title   string
	W       float32 // fixed width; first column is flexible
	Align   int
	Heat    uint32 // color for heat-map cells (0 = none)
	Summary string // big value shown above the title (e.g. "14%")
}

type Cell struct {
	Text  string
	Heat  float64 // 0..1 intensity
	Color uint32  // text color override
}

type Row struct {
	Key        string
	Name       string
	Sub        string
	Depth      int
	Group      bool
	Expandable bool
	Expanded   bool
	Icon       uintptr
	Glyph      rune
	GlyphCol   uint32
	Cells      []Cell
	Toggle     func()
	Dim        bool
	Badge      string
	BadgeCol   uint32
}

type Table struct {
	Sort   string
	Desc   bool
	Scroll float32
	Sel    string
	keys   []string
	view   float32
	total  float32
}

const rowH = 30

func (t *Table) MoveSel(d int) {
	if len(t.keys) == 0 {
		return
	}
	idx := -1
	for i, k := range t.keys {
		if k == t.Sel {
			idx = i
		}
	}
	idx += d
	if idx < 0 {
		idx = 0
	}
	if idx >= len(t.keys) {
		idx = len(t.keys) - 1
	}
	t.Sel = t.keys[idx]
	// keep it visible
	top := float32(idx) * rowH
	if top < t.Scroll {
		t.Scroll = top
	}
	if top+rowH > t.Scroll+t.view {
		t.Scroll = top + rowH - t.view
	}
}

// Draw renders the table and registers its interaction zones.
func (t *Table) Draw(a *App, c *ui.Canvas, id string, x, y, w, h float32, cols []Col, rows []Row,
	onSort func(key string), onRight func(r *Row), onDbl func(r *Row)) {

	hasSummary := false
	fixed := float32(0)
	for i, col := range cols {
		if col.Summary != "" {
			hasSummary = true
		}
		if i > 0 {
			fixed += col.W
		}
	}
	hh := float32(32)
	if hasSummary {
		hh = 48
	}
	first := w - fixed - 10
	if first < 160 {
		first = 160
	}

	// header
	cx := x
	for i, col := range cols {
		cw := col.W
		if i == 0 {
			cw = first
		}
		hid := fmt.Sprintf("%s-h-%s", id, col.Key)
		if a.hover == hid {
			c.Round(cx+2, y+2, cw-4, hh-4, 6, ColHover)
		}
		align := col.Align
		pad := float32(10)
		tcol := uint32(ColDim)
		if t.Sort == col.Key {
			tcol = ColText
		}
		if col.Summary != "" {
			sc := uint32(ColText)
			if col.Heat != 0 {
				sc = col.Heat
			}
			c.Text(cx+pad, y+4, cw-2*pad, 22, col.Summary, ui.Font{Size: 15, Weight: 600}, sc, align)
			c.Text(cx+pad, y+24, cw-2*pad, 20, col.Title, ui.FontSmall, tcol, align)
		} else {
			c.Text(cx+pad, y, cw-2*pad, hh, col.Title, ui.FontSmall, tcol, align)
		}
		if t.Sort == col.Key {
			arrow := rune(0xE70E) // up
			if t.Desc {
				arrow = 0xE70D
			}
			ax := cx + cw/2 - 5
			c.Icon(ax, y-2, 10, 10, arrow, 7, ColDim)
		}
		if i > 0 {
			c.LineA(cx, y+8, cx, y+hh-8, 0xFFFFFF, 18, 1)
		}
		key := col.Key
		if onSort != nil {
			a.addZone(Zone{X: cx, Y: y, W: cw, H: hh, ID: hid, Click: func() { onSort(key) }})
		}
		cx += cw
	}
	c.Line(x, y+hh, x+w, y+hh, ColLine, 1)

	// body
	by := y + hh + 1
	bh := h - hh - 1
	t.view = bh
	t.total = float32(len(rows)) * rowH
	maxScroll := t.total - bh
	if maxScroll < 0 {
		maxScroll = 0
	}
	if t.Scroll > maxScroll {
		t.Scroll = maxScroll
	}
	if t.Scroll < 0 {
		t.Scroll = 0
	}
	t.keys = t.keys[:0]
	for _, r := range rows {
		if !r.Group {
			t.keys = append(t.keys, r.Key)
		}
	}
	a.addZone(Zone{X: x, Y: by, W: w, H: bh, Wheel: func(d float32) { t.Scroll -= d * rowH * 3 }})

	c.Clip(x, by, w, bh)
	start := int(t.Scroll / rowH)
	for i := start; i < len(rows); i++ {
		ry := by + float32(i)*rowH - t.Scroll
		if ry > by+bh {
			break
		}
		r := &rows[i]
		rid := id + "-r-" + r.Key
		if r.Group {
			c.Text(x+10, ry+6, w, rowH-6, r.Name, ui.FontSemi, ColAccent2, ui.AlignLeft)
			continue
		}
		sel := t.Sel == r.Key
		if sel {
			c.Round(x+2, ry+1, w-4, rowH-2, 6, ColSel)
			c.Round(x+2, ry+8, 3, rowH-16, 1.5, ColAccent)
		} else if a.hover == rid {
			c.Round(x+2, ry+1, w-4, rowH-2, 6, ColHover)
		}
		// cells
		cx := x + first
		for ci, col := range cols[1:] {
			if ci >= len(r.Cells) {
				break
			}
			cell := r.Cells[ci]
			if col.Heat != 0 && cell.Heat > 0.004 && !sel {
				hv := math.Min(1, cell.Heat)
				alpha := uint8(18 + hv*150)
				c.FillA(cx+1, ry+1, col.W-2, rowH-2, col.Heat, alpha)
			}
			tc := uint32(ColText)
			if cell.Color != 0 {
				tc = cell.Color
			}
			if r.Dim {
				tc = ui.Mix(tc, ColBG, 0.45)
			}
			c.Text(cx+10, ry, col.W-20, rowH, cell.Text, ui.FontBody, tc, col.Align)
			cx += col.W
		}
		// name column
		nx := x + 10 + float32(r.Depth)*22
		if r.Expandable {
			g := rune(GRight)
			if r.Expanded {
				g = GDown
			}
			tid := rid + "-tog"
			gc := uint32(ColDim)
			if a.hover == tid {
				gc = ColText
			}
			c.Icon(nx-4, ry, 20, rowH, g, 9, gc)
			if r.Toggle != nil {
				a.addZone(Zone{X: nx - 6, Y: ry, W: 22, H: rowH, ID: tid, Click: r.Toggle})
			}
		}
		nx += 18
		if r.Icon != 0 {
			c.DrawHIcon(nx, ry+7, 16, r.Icon)
		} else if r.Glyph != 0 {
			gc := r.GlyphCol
			if gc == 0 {
				gc = ColDim
			}
			c.Icon(nx-2, ry, 20, rowH, r.Glyph, 13, gc)
		} else {
			c.Round(nx+2, ry+9, 12, 12, 3, 0x2A2A34)
		}
		nx += 26
		ncol := uint32(ColText)
		if r.Dim {
			ncol = ColDim
		}
		avail := x + first - nx - 8
		name := r.Name
		c.Text(nx, ry, avail, rowH, name, ui.FontBody, ncol, ui.AlignLeft)
		if r.Badge != "" {
			nw := c.TextW(name, ui.FontBody)
			bw := c.TextW(r.Badge, ui.FontSmall) + 12
			if nx+nw+8+bw < x+first-8 {
				bc := r.BadgeCol
				if bc == 0 {
					bc = ColDim
				}
				c.RoundA(nx+nw+8, ry+7, bw, 16, 8, bc, 40)
				c.Text(nx+nw+8, ry+7, bw, 16, r.Badge, ui.FontSmall, bc, ui.AlignCenter)
			}
		}
		rr := r
		z := Zone{X: x, Y: ry, W: w - 10, H: rowH, ID: rid, Click: func() { t.Sel = rr.Key }}
		if onRight != nil {
			z.Right = func() { t.Sel = rr.Key; onRight(rr) }
		}
		if onDbl != nil {
			z.Dbl = func() { t.Sel = rr.Key; onDbl(rr) }
		} else if rr.Toggle != nil {
			z.Dbl = rr.Toggle
		}
		if ry+rowH > by && ry < by+bh {
			a.addZone(z)
		}
	}
	c.Unclip()

	// scrollbar
	if t.total > bh {
		sbX := x + w - 7
		th := bh * bh / t.total
		if th < 30 {
			th = 30
		}
		ty := by + (bh-th)*(t.Scroll/maxScroll)
		col := uint32(0x3A3A46)
		if a.hover == id+"-sb" || a.dragZone != nil && a.dragZone.ID == id+"-sb" {
			col = 0x5A5A6A
		}
		c.Round(sbX, ty, 5, th, 2.5, col)
		grab := float32(-1)
		a.addZone(Zone{X: sbX - 6, Y: by, W: 13, H: bh, ID: id + "-sb", Drag: func(mx, my float32) {
			if grab < 0 {
				cur := by + (bh-th)*(t.Scroll/maxScroll)
				if my >= cur && my <= cur+th {
					grab = my - cur
				} else {
					grab = th / 2
				}
			}
			f := (my - grab - by) / (bh - th)
			t.Scroll = f * maxScroll
		}})
	}
	if len(rows) == 0 {
		c.Text(x, by+20, w, 30, "Nothing to show", ui.FontBody, ColFaint, ui.AlignCenter)
	}
}
