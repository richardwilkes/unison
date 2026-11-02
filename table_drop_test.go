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
	"net/url"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/tid"
	"github.com/richardwilkes/toolbox/v2/uti"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/drag"
)

// fakeDragInfo is a minimal drag.Info carrying a single data type.
type fakeDragInfo struct {
	dataType string
}

func (f *fakeDragInfo) SourceDragOpMask() drag.Op        { return drag.Copy | drag.Move }
func (f *fakeDragInfo) DataTypes() []string              { return []string{f.dataType} }
func (f *fakeDragInfo) HasString() bool                  { return false }
func (f *fakeDragInfo) HasFilePaths() bool               { return false }
func (f *fakeDragInfo) HasURLs() bool                    { return false }
func (f *fakeDragInfo) HasDataType(dataType string) bool { return dataType == f.dataType }
func (f *fakeDragInfo) Text() string                     { return "" }
func (f *fakeDragInfo) FilePaths() []string              { return nil }
func (f *fakeDragInfo) URLs() []*url.URL                 { return nil }
func (f *fakeDragInfo) Data(_ string) []byte             { return nil }

var tableDropTestDataType *uti.DataType

func tableDropDataType() *uti.DataType {
	if tableDropTestDataType == nil {
		tableDropTestDataType = unison.CreatePrivateDataType("unison.test.table-drop")
	}
	return tableDropTestDataType
}

// newDropTestTable builds a table containing an open container row "p" with children "c0" and "c1", followed by a
// plain sibling row "s", installs drop support configured to move rows for same-table drags, and registers a drag of
// the container row itself.
func newDropTestTable(t *testing.T) (table *unison.Table[*tableTestRow], parent *tableTestRow) {
	t.Helper()
	parent = newTableTestRow("p")
	parent.SetChildren([]*tableTestRow{newTableTestRow("c0"), newTableTestRow("c1")})
	parent.SetOpen(true)
	table = newTestTable(parent, newTableTestRow("s"))
	table.SetFrameRect(geom.NewRect(0, 0, 300, 300))
	table.InstallDropSupport[any](tableDropDataType(),
		func(from, to *unison.Table[*tableTestRow]) bool { return from == to }, nil, nil)
	unison.SetDragTableDataForTest(&unison.TableDragData[*tableTestRow]{
		Table: table,
		Rows:  []*tableTestRow{parent},
	})
	t.Cleanup(func() { unison.SetDragTableDataForTest(nil) })
	return table, parent
}

// lowerHalfOf returns a point in the lower half of the given disclosed row, which targets an insertion inside the row
// if it is a container, or after it otherwise.
func lowerHalfOf(table *unison.Table[*tableTestRow], rowIndex int) geom.Point {
	frame := table.RowFrame(rowIndex)
	return geom.NewPoint(frame.CenterX(), frame.CenterY()+1)
}

// upperHalfOf returns a point in the upper half of the given disclosed row, which targets an insertion into the row's
// parent at the row's own index.
func upperHalfOf(table *unison.Table[*tableTestRow], rowIndex int) geom.Point {
	frame := table.RowFrame(rowIndex)
	return geom.NewPoint(frame.CenterX(), frame.Y+1)
}

// installMoveDropSupport installs drop support on the table that always treats drops as moves and registers a drag of
// the given rows originating from the source table.
func installMoveDropSupport(t *testing.T, table, source *unison.Table[*tableTestRow], rows ...*tableTestRow) {
	t.Helper()
	table.SetFrameRect(geom.NewRect(0, 0, 300, 300))
	table.InstallDropSupport[any](tableDropDataType(),
		func(_, _ *unison.Table[*tableTestRow]) bool { return true }, nil, nil)
	unison.SetDragTableDataForTest(&unison.TableDragData[*tableTestRow]{Table: source, Rows: rows})
	t.Cleanup(func() { unison.SetDragTableDataForTest(nil) })
}

func TestTableDragUpdatedRejectsDropIntoDraggedRows(t *testing.T) {
	c := check.New(t)
	table, _ := newDropTestTable(t)
	di := &fakeDragInfo{dataType: tableDropDataType().UTI}

	// The lower half of the dragged container row targets the container itself, which must be rejected.
	c.Equal(drag.None, table.DragUpdatedCallback(di, lowerHalfOf(table, 0), 0))

	// The lower half of a child of the dragged container targets an insertion inside the container, likewise rejected.
	c.Equal(drag.None, table.DragUpdatedCallback(di, lowerHalfOf(table, 1), 0))

	// The lower half of the top-level sibling targets a top-level insertion, which remains a valid move.
	c.Equal(drag.Move, table.DragUpdatedCallback(di, lowerHalfOf(table, 3), 0))
}

func TestTableDropIntoDraggedRowsIsRefusedAndLeavesModelUntouched(t *testing.T) {
	c := check.New(t)
	table, parent := newDropTestTable(t)
	di := &fakeDragInfo{dataType: tableDropDataType().UTI}

	c.False(table.DropCallback(di, lowerHalfOf(table, 0), 0))

	// The model must be unchanged: same roots, the container still holds its children, and nothing was reparented.
	roots := table.RootRows()
	c.Equal(2, len(roots))
	c.Equal(tid.TID("p"), roots[0].ID())
	c.Equal(tid.TID("s"), roots[1].ID())
	c.Equal(2, len(parent.Children()))
	c.Equal(tid.TID("c0"), parent.Children()[0].ID())
	c.Equal(tid.TID("c1"), parent.Children()[1].ID())
	c.True(parent.Parent() == nil)
	c.Equal(4, table.LastRowIndex()+1)
}

// TestTableDropAboveEarlierRowStillAdjustsIndex verifies where a moved root row lands when dropped in the upper half of
// a row: within one table, whichever of its views the row is dragged from and dropped on, the insertion index accounts
// for the moved row's removal, so a row dropped above a later row lands just before it and one dropped directly above
// itself stays put, while a removal from another table must not shift the insertion point and is the only one the
// source table's DragRemovedRowsCallback hears of.
func TestTableDropAboveEarlierRowStillAdjustsIndex(t *testing.T) {
	for _, test := range []struct {
		name         string
		want         []tid.TID
		wantSource   []tid.TID
		dragged      int
		dropAbove    int
		crossTable   bool
		betweenViews bool
	}{
		{
			name:      "above a later row lands between it and the one before",
			dragged:   0,
			dropAbove: 2,
			want:      []tid.TID{"r1", "r0", "r2"},
		},
		{
			name:      "directly above itself is a no-op",
			dragged:   1,
			dropAbove: 1,
			want:      []tid.TID{"r0", "r1", "r2"},
		},
		{
			name:         "from another view of the table lands between it and the one before",
			betweenViews: true,
			dragged:      0,
			dropAbove:    2,
			want:         []tid.TID{"r1", "r0", "r2"},
		},
		{
			name:       "from another table keeps the target index",
			crossTable: true,
			dragged:    0,
			dropAbove:  1,
			want:       []tid.TID{"d0", "r0", "d1", "d2"},
			wantSource: []tid.TID{"r1"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := check.New(t)
			var src, dst *unison.Table[*tableTestRow]
			switch {
			case test.crossTable:
				src = newTestTable(flatRows(2)...)
				dst = newTestTable(newTableTestRow("d0"), newTableTestRow("d1"), newTableTestRow("d2"))
			case test.betweenViews:
				src = newTestTable(flatRows(3)...)
				src.SetDrawRowRange(0, 1)
				dst = src.NewView(1, 3)
			default:
				src = newTestTable(flatRows(3)...)
				dst = src
			}
			removed := 0
			src.DragRemovedRowsCallback = func() { removed++ }
			installMoveDropSupport(t, dst, src, src.RootRows()[test.dragged])
			di := &fakeDragInfo{dataType: tableDropDataType().UTI}
			rootIDs := func(table *unison.Table[*tableTestRow]) []tid.TID {
				rows := table.RootRows()
				ids := make([]tid.TID, len(rows))
				for i, row := range rows {
					ids[i] = row.ID()
				}
				return ids
			}

			c.True(dst.DropCallback(di, upperHalfOf(dst, test.dropAbove), 0))
			c.Equal(test.want, rootIDs(dst))
			wantRemoved := 0
			if test.crossTable {
				c.Equal(test.wantSource, rootIDs(src), "the source table lost the moved row")
				wantRemoved = 1
			}
			c.Equal(wantRemoved, removed, "only a drag from another table removes rows from the source")
		})
	}
}

func TestTableMoveOutOfFilteredSourcePreservesHierarchy(t *testing.T) {
	c := check.New(t)
	parent := newTableTestRow("p")
	parent.SetChildren([]*tableTestRow{newTableTestRow("c0"), newTableTestRow("c1")})
	parent.SetOpen(true)
	sibling := newTableTestRow("s")
	src := newTestTable(parent, sibling)
	src.ApplyFilter(func(_ *tableTestRow) bool { return false }) // Keep every row; the view is the flattened [p c0 c1 s]
	c.True(src.IsFiltered())

	dst := newTestTable(newTableTestRow("d0"))
	installMoveDropSupport(t, dst, src, sibling)
	di := &fakeDragInfo{dataType: tableDropDataType().UTI}

	// Drop at the bottom edge of the destination, appending "s" to its root rows.
	c.True(dst.DropCallback(di, geom.NewPoint(10, 299), 0))
	got := dst.RootRows()
	c.Equal(2, len(got))
	c.Equal(tid.TID("d0"), got[0].ID())
	c.Equal(tid.TID("s"), got[1].ID())

	// The source model must retain its real hierarchy rather than being replaced by the flattened filter results: root
	// "p" still holds its children and only "s" was removed.
	c.Equal(1, src.Model.RootRowCount())
	c.Equal(tid.TID("p"), src.Model.RootRows()[0].ID())
	c.Equal(2, len(parent.Children()))
	c.Equal(tid.TID("c0"), parent.Children()[0].ID())
	c.Equal(tid.TID("c1"), parent.Children()[1].ID())

	// The still-filtered source view no longer shows the moved row.
	c.True(src.IsFiltered())
	c.Equal(3, src.LastRowIndex()+1)
	c.Equal(tid.TID("p"), src.RowFromIndex(0).ID())
	c.Equal(tid.TID("c0"), src.RowFromIndex(1).ID())
	c.Equal(tid.TID("c1"), src.RowFromIndex(2).ID())
}

func TestTableDropOntoSiblingStillMoves(t *testing.T) {
	c := check.New(t)
	table, parent := newDropTestTable(t)
	di := &fakeDragInfo{dataType: tableDropDataType().UTI}

	// Dropping in the lower half of the top-level sibling moves the dragged container after it.
	c.True(table.DropCallback(di, lowerHalfOf(table, 3), 0))
	roots := table.RootRows()
	c.Equal(2, len(roots))
	c.Equal(tid.TID("s"), roots[0].ID())
	c.Equal(tid.TID("p"), roots[1].ID())
	c.Equal(2, len(parent.Children()))
}

// TestTableMoveOutOfHierarchicallyFilteredSource verifies that a row dragged out of a table showing a hierarchical
// filter leaves the source model's hierarchy alone and disappears from the filtered view, along with any row that was
// only shown for its sake.
func TestTableMoveOutOfHierarchicallyFilteredSource(t *testing.T) {
	c := check.New(t)
	child := newTableTestRow("c1")
	parent := newTableTestRow("p")
	parent.SetChildren([]*tableTestRow{newTableTestRow("c0"), child})
	sibling := newTableTestRow("s")
	src := newTestTable(parent, sibling)
	src.ApplyHierarchicalFilter(func(row *tableTestRow) bool { return row.ID() != "c1" && row.ID() != "s" })
	c.True(src.IsFiltered())
	c.Equal(3, src.LastRowIndex()+1, "the view is [p c1 s], with p shown for c1's sake")

	dst := newTestTable(newTableTestRow("d0"))
	installMoveDropSupport(t, dst, src, child)
	di := &fakeDragInfo{dataType: tableDropDataType().UTI}

	// Drop at the bottom edge of the destination, appending "c1" to its root rows.
	c.True(dst.DropCallback(di, geom.NewPoint(10, 299), 0))
	got := dst.RootRows()
	c.Equal(2, len(got))
	c.Equal(tid.TID("d0"), got[0].ID())
	c.Equal(tid.TID("c1"), got[1].ID())

	// The source model keeps its hierarchy, less the moved row.
	c.Equal(2, src.Model.RootRowCount())
	c.Equal(1, len(parent.Children()))
	c.Equal(tid.TID("c0"), parent.Children()[0].ID())

	// The still-filtered source view no longer shows the moved row, nor the container that was only shown for it.
	c.True(src.IsFiltered())
	c.Equal(1, src.LastRowIndex()+1)
	c.Equal(tid.TID("s"), src.RowFromIndex(0).ID())
	c.Equal(1, src.RootRowCount())
}

// TestTableDropOutsideAViewsRows verifies where a drop outside the rows a view shows lands: above them, before its first
// row; below them, after its last row when a later view shows more rows, and inside that row ahead of its children when
// they follow it; before the row its range starts at when it shows none; and at the end of the top-level rows only
// below the view showing the last row.
func TestTableDropOutsideAViewsRows(t *testing.T) {
	for _, test := range []struct {
		name         string
		wantRoots    []tid.TID
		wantChildren []tid.TID // The container's children afterward, c0 and c1 when nil
		view         int
		dragged      int
		edge         bool
		empty        bool // The view's range is emptied where it starts
		above        bool // The drop is in a top inset above the view's first row
		early        bool // The container is the second root row, so the first view's last row
		closed       bool // The container is closed
	}{
		{name: "bottom edge of a middle view", view: 1, edge: true, wantRoots: []tid.TID{"r1", "r2", "r3", "r0", "p"}},
		{name: "beneath a middle view's last row", view: 1, wantRoots: []tid.TID{"r1", "r2", "r3", "r0", "p"}},
		{name: "bottom edge of the last view", view: 2, edge: true, wantRoots: []tid.TID{"r1", "r2", "r3", "p", "r0"}},
		{name: "above the last view's first row", view: 2, above: true, wantRoots: []tid.TID{"r1", "r2", "r3", "r0", "p"}},
		{
			name:      "bottom edge of an empty view before the first row",
			view:      0,
			dragged:   3,
			edge:      true,
			empty:     true,
			wantRoots: []tid.TID{"r3", "r0", "r1", "r2", "p"},
		},
		{
			name:      "bottom edge of a view ending with a closed container",
			view:      0,
			dragged:   3,
			edge:      true,
			early:     true,
			closed:    true,
			wantRoots: []tid.TID{"r0", "p", "r3", "r1", "r2"},
		},
		{
			name:         "beneath a view ending with an open container",
			view:         0,
			dragged:      3,
			early:        true,
			wantRoots:    []tid.TID{"r0", "p", "r1", "r2"},
			wantChildren: []tid.TID{"r3", "c0", "c1"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := check.New(t)
			rows := flatRows(4)
			parent := newTableTestRow("p")
			parent.SetChildren([]*tableTestRow{newTableTestRow("c0"), newTableTestRow("c1")})
			parent.SetOpen(!test.closed)
			roots := []*tableTestRow{rows[0], rows[1], rows[2], rows[3], parent}
			if test.early {
				roots = []*tableTestRow{rows[0], parent, rows[1], rows[2], rows[3]}
			}
			table := newTestTable(roots...)
			table.SetDrawRowRange(0, 2)
			views := []*unison.Table[*tableTestRow]{table, table.NewView(2, 4), table.NewView(4, 7)}
			dst := views[test.view]
			if test.empty {
				start, _ := dst.CurrentDrawRowRange()
				dst.SetDrawRowRange(start, start)
			}
			installMoveDropSupport(t, dst, table, rows[test.dragged])
			where := geom.NewPoint(10, 299)
			switch {
			case test.above:
				dst.SetBorder(unison.NewEmptyBorder(geom.Insets{Top: 10}))
				where.Y = 5
			case !test.edge:
				_, end := dst.CurrentDrawRowRange()
				where.Y = dst.RowFrame(end-1).Bottom() + 5
			}
			c.True(dst.DropCallback(&fakeDragInfo{dataType: tableDropDataType().UTI}, where, 0))
			ids := func(rows []*tableTestRow) []tid.TID {
				list := make([]tid.TID, len(rows))
				for i, row := range rows {
					list[i] = row.ID()
				}
				return list
			}
			c.Equal(test.wantRoots, ids(table.RootRows()))
			wantChildren := test.wantChildren
			if wantChildren == nil {
				wantChildren = []tid.TID{"c0", "c1"}
			}
			c.Equal(wantChildren, ids(parent.Children()))
		})
	}
}
