// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison_test

import (
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/enums/mod"
)

// TestTableDragAndUpAfterModelShrink verifies that a mouse drag or mouse up whose interaction row was captured at
// mouse down does not panic (and does not replay the event into a stale cell) when the model shrinks mid-gesture,
// such as when a cell callback mutates the model and calls SyncToModel before the drag or up event arrives.
func TestTableDragAndUpAfterModelShrink(t *testing.T) {
	c := check.New(t)
	var downCount, dragCount, upCount int
	makeRow := func(id string) *tableTestRow {
		row := newTableTestRow(id)
		panel := unison.NewPanel()
		panel.MouseDownCallback = func(_ geom.Point, _, _ int, _ mod.Modifiers) bool {
			downCount++
			return false
		}
		panel.MouseDragCallback = func(_ geom.Point, _ int, _ mod.Modifiers) bool {
			dragCount++
			return true
		}
		panel.MouseUpCallback = func(_ geom.Point, _ int, _ mod.Modifiers) bool {
			upCount++
			return true
		}
		row.cellFactory = func(_, _ int) unison.Paneler { return panel }
		return row
	}
	model := &unison.SimpleTableModel[*tableTestRow]{}
	model.SetRootRows([]*tableTestRow{makeRow("A"), makeRow("B"), makeRow("C")})
	table := unison.NewTable[*tableTestRow](model)
	table.Columns = []unison.ColumnInfo{{ID: 0, Current: 100}}
	table.SyncToModel()

	// Sanity check: a normal down/drag/up gesture over the last row reaches the cell's callbacks.
	pt := table.CellFrame(2, 0).Point.Add(geom.NewPoint(1, 1))
	table.DefaultMouseDown(pt, unison.ButtonLeft, 1, 0)
	c.Equal(1, downCount)
	table.DefaultMouseDrag(pt, unison.ButtonLeft, 0)
	c.Equal(1, dragCount)
	table.DefaultMouseUp(pt, unison.ButtonLeft, 0)
	c.Equal(1, upCount)

	// Start a gesture on the last row, then shrink the model out from under it before the drag and up arrive. The
	// stale interaction row must not be replayed into the now out-of-range row cache.
	table.DefaultMouseDown(pt, unison.ButtonLeft, 1, 0)
	c.Equal(2, downCount)
	model.SetRootRows([]*tableTestRow{makeRow("D")})
	table.SyncToModel()
	table.DefaultMouseDrag(pt, unison.ButtonLeft, 0)
	c.Equal(1, dragCount)
	table.DefaultMouseUp(pt, unison.ButtonLeft, 0)
	c.Equal(1, upCount)
}
