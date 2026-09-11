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
	"github.com/richardwilkes/toolbox/v2/tid"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/enums/mod"
)

// testColumnHeader is a minimal TableColumnHeader built on a plain panel, avoiding the font work a label-based header
// would require.
type testColumnHeader struct {
	*unison.Panel
	state unison.SortState
}

func newTestColumnHeader() *testColumnHeader {
	return &testColumnHeader{Panel: unison.NewPanel()}
}

func (h *testColumnHeader) SortState() unison.SortState         { return h.state }
func (h *testColumnHeader) SetSortState(state unison.SortState) { h.state = state }
func (h *testColumnHeader) Less() func(a, b string) bool        { return nil }

// newHeaderTestTable builds a table with two 100-wide columns.
func newHeaderTestTable() *unison.Table[*tableTestRow] {
	table := newTestTable(flatRows(2)...)
	table.Columns = append(table.Columns,
		unison.ColumnInfo{ID: 0, Current: 100},
		unison.ColumnInfo{ID: 1, Current: 100},
	)
	table.SetFrameRect(geom.NewRect(0, 0, 300, 300))
	return table
}

func TestTableHeaderToleratesFewerHeadersThanColumns(t *testing.T) {
	c := check.New(t)
	table := newHeaderTestTable()
	// The sizing and drawing paths tolerate fewer column headers than columns, so the interaction paths must too. This
	// header has no column headers at all.
	header := unison.NewTableHeader[*tableTestRow](table)
	header.SetFrameRect(geom.NewRect(0, 0, 300, 20))

	pt := geom.NewPoint(50, 10) // Over column 0, away from any divider
	c.True(header.DefaultUpdateCursorCallback(pt) == nil)
	c.True(header.DefaultUpdateTooltipCallback(pt, geom.Rect{}).Empty())
	c.False(header.DefaultMouseMove(pt, 0))
	c.True(header.DefaultMouseDown(pt, unison.ButtonLeft, 1, 0))
	c.False(header.DefaultMouseUp(pt, unison.ButtonLeft, 0))
}

func TestTableHeaderStillDispatchesToPresentHeaders(t *testing.T) {
	c := check.New(t)
	table := newHeaderTestTable()
	colHeader := newTestColumnHeader()
	downCalls := 0
	upCalls := 0
	moveCalls := 0
	colHeader.MouseDownCallback = func(_ geom.Point, _, _ int, _ mod.Modifiers) bool { downCalls++; return true }
	colHeader.MouseUpCallback = func(_ geom.Point, _ int, _ mod.Modifiers) bool { upCalls++; return true }
	colHeader.MouseMoveCallback = func(_ geom.Point, _ mod.Modifiers) bool { moveCalls++; return true }
	// One header for two columns: column 0 dispatches to it, column 1 has no header and must be ignored.
	header := unison.NewTableHeader(table, unison.TableColumnHeader[*tableTestRow](colHeader))
	header.SetFrameRect(geom.NewRect(0, 0, 300, 20))

	over0 := geom.NewPoint(50, 10)
	c.True(header.DefaultMouseMove(over0, 0))
	c.True(header.DefaultMouseDown(over0, unison.ButtonLeft, 1, 0))
	c.True(header.DefaultMouseUp(over0, unison.ButtonLeft, 0))
	c.Equal(1, moveCalls)
	c.Equal(1, downCalls)
	c.Equal(1, upCalls)

	over1 := geom.NewPoint(150, 10)
	c.False(header.DefaultMouseMove(over1, 0))
	c.True(header.DefaultMouseDown(over1, unison.ButtonLeft, 1, 0))
	c.False(header.DefaultMouseUp(over1, unison.ButtonLeft, 0))
	c.Equal(1, moveCalls)
	c.Equal(1, downCalls)
	c.Equal(1, upCalls)
}

// TestTableHeaderColumnWithOnlyMinimumIsResizable verifies that the header's resize gate agrees with the table's: a
// column with a positive Minimum and no Maximum is resizable, while one pinned to a single width is not.
func TestTableHeaderColumnWithOnlyMinimumIsResizable(t *testing.T) {
	c := check.New(t)
	table := newHeaderTestTable()
	table.Columns[0].Minimum = 50
	header := unison.NewTableHeader[*tableTestRow](table)
	header.SetFrameRect(geom.NewRect(0, 0, 300, 20))

	divider := geom.NewPoint(100, 10)
	c.Equal(0, table.OverColumnDivider(divider.X), "expected a divider at the column boundary")
	c.True(header.DefaultMouseDown(divider, unison.ButtonLeft, 1, 0))
	c.True(header.DefaultMouseDrag(divider.Add(geom.NewPoint(20, 0)), unison.ButtonLeft, 0))
	c.Equal(float32(120), table.Columns[0].Current)
	header.DefaultMouseUp(divider.Add(geom.NewPoint(20, 0)), unison.ButtonLeft, 0)

	// A column pinned to a single width is not resizable, so the press lands on the header instead of the divider.
	pinned := newHeaderTestTable()
	pinned.Columns[0].Minimum = 100
	pinned.Columns[0].Maximum = 100
	pinnedHeader := unison.NewTableHeader[*tableTestRow](pinned)
	pinnedHeader.SetFrameRect(geom.NewRect(0, 0, 300, 20))
	pinnedHeader.DefaultMouseDown(divider, unison.ButtonLeft, 1, 0)
	pinnedHeader.DefaultMouseDrag(divider.Add(geom.NewPoint(20, 0)), unison.ButtonLeft, 0)
	c.Equal(float32(100), pinned.Columns[0].Current)
	pinnedHeader.DefaultMouseUp(divider.Add(geom.NewPoint(20, 0)), unison.ButtonLeft, 0)
}

// TestTableHeaderSortsHierarchicalFilterInPlace verifies that a sort applied while a hierarchical filter is in force
// orders the rows the filter shows, at every level, without reordering the model behind them.
func TestTableHeaderSortsHierarchicalFilterInPlace(t *testing.T) {
	c := check.New(t)
	c1 := newTableTestRow("c1")
	c0 := newTableTestRow("c0")
	parent := newTableTestRow("p")
	parent.SetChildren([]*tableTestRow{c1, c0})
	a := newTableTestRow("a")
	table := newTestTable(parent, a)
	table.Columns = append(table.Columns, unison.ColumnInfo{ID: 0, Current: 100})
	colHeader := newTestColumnHeader()
	colHeader.SetSortState(unison.SortState{Order: 0, Ascending: true, Sortable: true})
	header := unison.NewTableHeader(table, unison.TableColumnHeader[*tableTestRow](colHeader))
	c.True(header.HasSort())

	table.ApplyHierarchicalFilter(func(_ *tableTestRow) bool { return false })
	c.Equal(4, table.LastRowIndex()+1)
	c.Equal(tid.TID("a"), table.RowFromIndex(0).ID(), "the root rows must be sorted")
	c.Equal(tid.TID("p"), table.RowFromIndex(1).ID())
	c.Equal(tid.TID("c0"), table.RowFromIndex(2).ID(), "the children the filter kept must be sorted too")
	c.Equal(tid.TID("c1"), table.RowFromIndex(3).ID())

	c.Equal(tid.TID("p"), table.Model.RootRows()[0].ID(), "the model's order must be left alone")
	c.Equal(tid.TID("a"), table.Model.RootRows()[1].ID())
	c.Equal(tid.TID("c1"), parent.Children()[0].ID(), "the model's children must be left alone")
	c.Equal(tid.TID("c0"), parent.Children()[1].ID())
}
