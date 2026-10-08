// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/i18n"
	"github.com/richardwilkes/toolbox/v2/tid"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/paintstyle"
	"github.com/richardwilkes/unison/enums/role"
)

// TableRowHider is implemented by a row that can be hidden. A hidden row and its descendants are not shown, but stay
// in the model, so rearranging the rows around them leaves them where they are.
type TableRowHider interface {
	// HiddenInTable returns true if the row should not be shown.
	HiddenInTable() bool
}

// tableHiddenRun is a run of consecutive hidden siblings.
type tableHiddenRun[T TableRowConstraint[T]] struct {
	rows  []T
	index int // the row cache index of the first row shown after the run
}

func rowHidden[T TableRowConstraint[T]](row T) bool {
	h, ok := any(row).(TableRowHider)
	return ok && h.HiddenInTable()
}

// buildRowCacheEntries adds the shown rows of the list to the row cache, starting at index, and records each run of
// hidden rows among them. Returns the index following the last entry added.
func (t *Table[T]) buildRowCacheEntries(rows []T, parentIndex, index, depth int) int {
	run := -1
	for _, row := range rows {
		if !rowHidden(row) {
			run = -1
			index = t.buildRowCacheEntry(row, parentIndex, index, depth)
			continue
		}
		if run == -1 {
			run = len(t.hiddenRuns)
			t.hiddenRuns = append(t.hiddenRuns, tableHiddenRun[T]{index: index})
		}
		t.hiddenRuns[run].rows = append(t.hiddenRuns[run].rows, row)
	}
	return index
}

// hiddenRunMarkerRects returns the area of each hidden run's marker.
func (t *Table[T]) hiddenRunMarkerRects() []geom.Rect {
	if !t.ShowHiddenRowsMarker || len(t.hiddenRuns) == 0 {
		return nil
	}
	y := t.ContentRect(false).Y
	tops := make([]float32, len(t.rowCache)+1)
	tops[0] = y
	for i := range t.rowCache {
		y += t.rowCache[i].height
		if t.ShowRowDivider {
			y++
		}
		tops[i+1] = y
	}
	rects := make([]geom.Rect, len(t.hiddenRuns))
	for i, run := range t.hiddenRuns {
		rects[i] = t.hiddenRunMarkerRect(tops[run.index])
	}
	return rects
}

// hiddenRunMarkerRect returns the area of the marker for a run of hidden rows at the boundary y, which the marker
// straddles against the table's left edge.
func (t *Table[T]) hiddenRunMarkerRect(y float32) geom.Rect {
	content := t.ContentRect(false)
	size := t.MinimumRowHeight / 2
	return geom.NewRect(content.X, min(max(y-size/2, content.Y), content.Bottom()-size), size, size)
}

// drawHiddenRunMarkers draws a small triangle in the left half of each hidden run's marker area.
func (t *Table[T]) drawHiddenRunMarkers(canvas *Canvas, dirty geom.Rect) {
	for i, rect := range t.hiddenRunMarkerRects() {
		if !rect.Intersects(dirty) {
			continue
		}
		path := NewPath()
		path.MoveTo(rect.Point)
		path.LineTo(geom.NewPoint(rect.X+rect.Width/2, rect.CenterY()))
		path.LineTo(geom.NewPoint(rect.X, rect.Bottom()))
		path.Close()
		canvas.DrawPath(path, t.HiddenRowsMarkerInk.Paint(canvas, rect, paintstyle.Fill))
		if t.HiddenClickCallback != nil {
			rows := t.hiddenRuns[i].rows
			t.hitRects = append(t.hitRects, tableHitRect{
				Rect:    rect,
				handler: func() { t.HiddenClickCallback(rows) },
			})
		}
	}
}

// hiddenRunTooltip sets up the tooltip for the marker at the point, if there is one, and returns the marker's area in
// root coordinates, or an empty rect.
func (t *Table[T]) hiddenRunTooltip(where geom.Point) geom.Rect {
	if t.HiddenTooltipCallback != nil {
		for i, rect := range t.hiddenRunMarkerRects() {
			if where.In(rect) {
				if text := t.HiddenTooltipCallback(t.hiddenRuns[i].rows); text != "" {
					t.borrowedTooltip = NewTooltipWithText(text)
					t.TooltipImmediate = false
					return t.RectToRoot(rect).Align()
				}
			}
		}
	}
	return geom.Rect{}
}

// axHiddenRowsKey is the key under which the marker of a run of hidden rows is described, by the run's first row.
type axHiddenRowsKey struct {
	Row tid.TID
}

// axAddHiddenRunMarkers describes the markers of the runs of hidden rows that sit against the row, whose frame is rect:
// those just below it, and for the first row, those above it.
func (t *Table[T]) axAddHiddenRunMarkers(b *AccessibilityBuilder, rowID accessibility.NodeID, row int, rect geom.Rect) {
	if !t.ShowHiddenRowsMarker {
		return
	}
	for _, run := range t.hiddenRuns {
		y := rect.Bottom()
		switch {
		case run.index == row+1:
			if t.ShowRowDivider {
				y++
			}
		case run.index == 0 && row == 0:
			y = rect.Y
		default:
			continue
		}
		name := i18n.Text("Hidden Rows")
		if t.HiddenTooltipCallback != nil {
			if text := t.HiddenTooltipCallback(run.rows); text != "" {
				name = text
			}
		}
		b.AddVirtualChildOf(rowID, axHiddenRowsKey{Row: run.rows[0].ID()}, func(n *accessibility.Node) {
			n.Role = role.Image
			n.Name = name
			n.Bounds = t.hiddenRunMarkerRect(y)
			if t.HiddenClickCallback != nil {
				n.Role = role.Button
				n.Actions = n.Actions.With(accessibility.Press)
			}
		})
	}
}

// axActOnHiddenRunMarker presses the marker of the run of hidden rows the key names.
func (t *Table[T]) axActOnHiddenRunMarker(key axHiddenRowsKey, req accessibility.ActionRequest) bool {
	if req.Action != accessibility.Press || t.HiddenClickCallback == nil {
		return false
	}
	for _, run := range t.hiddenRuns {
		if run.rows[0].ID() == key.Row {
			t.HiddenClickCallback(run.rows)
			return true
		}
	}
	return false
}
