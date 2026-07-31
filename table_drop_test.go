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
	unison.InstallDropSupport[*tableTestRow, any](table, tableDropDataType(),
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
	unison.InstallDropSupport[*tableTestRow, any](table, tableDropDataType(),
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

func TestTableDropDirectlyAboveItselfIsANoOp(t *testing.T) {
	c := check.New(t)
	rows := flatRows(3)
	table := newTestTable(rows...)
	installMoveDropSupport(t, table, table, rows[1])
	di := &fakeDragInfo{dataType: tableDropDataType().UTI}

	// Dropping r1 in the upper half of its own row targets its current position and must leave the order unchanged.
	c.True(table.DropCallback(di, upperHalfOf(table, 1), 0))
	got := table.RootRows()
	c.Equal(3, len(got))
	c.Equal(tid.TID("r0"), got[0].ID())
	c.Equal(tid.TID("r1"), got[1].ID())
	c.Equal(tid.TID("r2"), got[2].ID())
}

func TestTableDropAboveEarlierRowStillAdjustsIndex(t *testing.T) {
	c := check.New(t)
	rows := flatRows(3)
	table := newTestTable(rows...)
	installMoveDropSupport(t, table, table, rows[0])
	di := &fakeDragInfo{dataType: tableDropDataType().UTI}

	// Dropping r0 in the upper half of r2 lands it between r1 and r2, exercising the index adjustment for removals
	// before the insertion point.
	c.True(table.DropCallback(di, upperHalfOf(table, 2), 0))
	got := table.RootRows()
	c.Equal(3, len(got))
	c.Equal(tid.TID("r1"), got[0].ID())
	c.Equal(tid.TID("r0"), got[1].ID())
	c.Equal(tid.TID("r2"), got[2].ID())
}

func TestTableCrossTableDropOfRootRowKeepsTargetIndex(t *testing.T) {
	c := check.New(t)
	src := newTestTable(flatRows(2)...)
	dst := newTestTable(newTableTestRow("d0"), newTableTestRow("d1"), newTableTestRow("d2"))
	installMoveDropSupport(t, dst, src, src.RootRows()[0])
	di := &fakeDragInfo{dataType: tableDropDataType().UTI}

	// Dropping r0 in the upper half of d1 must insert it at index 1; removals from the source table must not shift the
	// insertion point in the destination.
	c.True(dst.DropCallback(di, upperHalfOf(dst, 1), 0))
	got := dst.RootRows()
	c.Equal(4, len(got))
	c.Equal(tid.TID("d0"), got[0].ID())
	c.Equal(tid.TID("r0"), got[1].ID())
	c.Equal(tid.TID("d1"), got[2].ID())
	c.Equal(tid.TID("d2"), got[3].ID())

	// The source table lost the moved row.
	c.Equal(1, len(src.RootRows()))
	c.Equal(tid.TID("r1"), src.RootRows()[0].ID())
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
