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
	"slices"

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/tid"
	"github.com/richardwilkes/toolbox/v2/uti"
	"github.com/richardwilkes/unison/drag"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/enums/paintstyle"
)

// TableDrop provides default support for dropping data into a table. This should only be instantiated by a call to
// Table.InstallDropSupport().
type TableDrop[T TableRowConstraint[T], U any] struct {
	// Table is the view the drop support was installed on. Each view of a table that should accept drops needs its own
	// call to InstallDropSupport. It is also the target handed to the callbacks and to TableRowData.CloneForTarget; use
	// SameTable to tell whether a drag came from the same table.
	Table                  *Table[T]
	DataType               *uti.DataType
	originalDrawOver       func(*Canvas, geom.Rect)
	shouldMoveDataCallback func(from, to *Table[T]) bool
	willDropCallback       func(from, to *Table[T], move bool) *UndoEdit[U]
	didDropCallback        func(undo *UndoEdit[U], from, to *Table[T], move bool)
	TargetParent           T
	TargetIndex            int
	top                    float32
	left                   float32
	inDragOver             bool
}

// DrawOverCallback handles drawing the drop zone feedback.
func (d *TableDrop[T, U]) DrawOverCallback(gc *Canvas, rect geom.Rect) {
	if d.originalDrawOver != nil {
		d.originalDrawOver(gc, rect)
	}
	if d.inDragOver {
		r := d.Table.ContentRect(false).Inset(geom.NewUniformInsets(1))
		paint := ThemeWarning.Paint(gc, r, paintstyle.Stroke)
		paint.SetStrokeWidth(2)
		paint.SetColorFilter(Alpha30Filter())
		gc.DrawRect(r, paint)
		paint.SetColorFilter(nil)
		paint.SetPathEffect(DashEffect())
		gc.DrawLine(geom.NewPoint(d.left, d.top), geom.NewPoint(r.Right(), d.top), paint)
	}
}

// CanAcceptDropCallback reports whether this table is a candidate for the given drag, independent of pointer position.
func (d *TableDrop[T, U]) CanAcceptDropCallback(di drag.Info) bool {
	if dragTableData == nil || d.Table.filteredRows != nil || !d.Table.Enabled() || !di.HasDataType(d.DataType.UTI) {
		return false
	}
	_, ok := dragTableData.(*TableDragData[T])
	return ok
}

// DragEnterCallback provides the drag enter handling.
func (d *TableDrop[T, U]) DragEnterCallback(di drag.Info, where geom.Point, mods mod.Modifiers) drag.Op {
	var op drag.Op
	SafeCall(func() { op = d.DragUpdatedCallback(di, where, mods) })
	return op
}

// DragUpdatedCallback provides the drag updated handling.
func (d *TableDrop[T, U]) DragUpdatedCallback(di drag.Info, where geom.Point, _ mod.Modifiers) drag.Op {
	d.inDragOver = false
	accept := false
	SafeCall(func() { accept = d.CanAcceptDropCallback(di) })
	if !accept {
		return drag.None
	}
	data, ok := dragTableData.(*TableDragData[T])
	if !ok {
		return drag.None
	}
	// Only the view showing the last row appends below its rows; one showing earlier rows inserts after its last row
	// instead, and one whose range is empty, before the row where that range starts.
	start, end := d.Table.CurrentDrawRowRange()
	atEnd := end == d.Table.LastRowIndex()+1
	last := end - 1
	d.inDragOver = true
	var op drag.Op
	SafeCall(func() {
		if d.shouldMoveDataCallback(data.Table, d.Table) {
			op = drag.Move
		} else {
			op = drag.Copy
		}
	})
	if start == end && !atEnd {
		return d.targetRow(data, op, start, false, false)
	}
	if where.Y >= d.Table.ContentRect(false).Bottom()-2 {
		// Over bottom edge
		if atEnd {
			return d.targetEnd(op, last)
		}
		return d.targetAfter(data, op, last)
	}
	if rowIndex := d.Table.OverRow(where.Y); rowIndex != -1 {
		return d.targetRow(data, op, rowIndex, where.Y >= d.Table.RowFrame(rowIndex).CenterY(), true)
	}
	// Not over any row
	switch {
	case start < end && where.Y < d.Table.RowFrame(start).Y:
		return d.targetRow(data, op, start, false, false)
	case atEnd:
		return d.targetEnd(op, last)
	default:
		return d.targetAfter(data, op, last)
	}
}

// targetAfter targets the place just after the row at index last, ahead of the rows after it: inside that row ahead of
// its children when they follow it, and after it otherwise, and returns op, or drag.None for a drop into the rows being
// dragged.
func (d *TableDrop[T, U]) targetAfter(data *TableDragData[T], op drag.Op, last int) drag.Op {
	var zero T
	next := d.Table.RowFromIndex(last + 1)
	return d.targetRow(data, op, last, true, next != zero && next.Parent() == d.Table.RowFromIndex(last))
}

// targetEnd targets the end of the top-level rows, drawing the drop line below the row at index last, and returns op.
func (d *TableDrop[T, U]) targetEnd(op drag.Op, last int) drag.Op {
	var zero T
	d.TargetParent = zero
	d.TargetIndex = d.Table.RootRowCount()
	rect := d.Table.RowFrame(last)
	d.top = min(rect.Bottom()+1+d.Table.Padding.Bottom, d.Table.ContentRect(false).Bottom()-1)
	d.left, _ = d.Table.ColumnEdges(max(d.Table.ColumnIndexForID(d.Table.HierarchyColumnID), 0))
	d.Table.MarkForRedraw()
	d.Table.FlushDrawing()
	return op
}

// targetRow targets the lower half of the row at rowIndex when lower is true, which is inside the row when it is a
// container and into is true and after it otherwise, or its upper half when lower is false, and returns op, or
// drag.None for a drop into the rows being dragged.
func (d *TableDrop[T, U]) targetRow(data *TableDragData[T], op drag.Op, rowIndex int, lower, into bool) drag.Op {
	var zero T
	contentRect := d.Table.ContentRect(false)
	hierarchyColumnIndex := d.Table.ColumnIndexForID(d.Table.HierarchyColumnID)
	d.TargetIndex = -1
	row := d.Table.RowFromIndex(rowIndex)
	rect := d.Table.CellFrame(rowIndex, max(hierarchyColumnIndex, 0))
	if lower {
		d.top = min(rect.Bottom()+1+d.Table.Padding.Bottom, contentRect.Bottom()-1)
		d.left = rect.X
		if into && row.CanHaveChildren() {
			// Row is a container; add to container at index 0
			d.TargetParent = row
			d.TargetIndex = 0
			if hierarchyColumnIndex != -1 {
				if hierarchyIndent := d.Table.CurrentHierarchyIndent(); hierarchyIndent > 0 {
					d.left += hierarchyIndent
				}
			}
		} else {
			// Add as sibling below this row
			d.TargetParent = row.Parent()
			if row = d.Table.RowFromIndex(rowIndex + 1); row == zero {
				if d.TargetParent == zero {
					d.TargetIndex = len(d.Table.RootRows())
				} else {
					d.TargetIndex = len(d.TargetParent.Children())
				}
			}
		}
	} else {
		// Add to parent of this row at this row's index
		d.TargetParent = row.Parent()
		d.top = max(rect.Y-d.Table.Padding.Bottom, 1)
		d.left = rect.X
	}
	if d.TargetIndex == -1 && row != zero {
		var children []T
		if d.TargetParent == zero {
			children = d.Table.RootRows()
		} else {
			children = d.TargetParent.Children()
		}
		for i, child := range children {
			if child.ID() == row.ID() {
				d.TargetIndex = i
				break
			}
		}
		if d.TargetIndex == -1 {
			d.TargetIndex = len(children)
		}
	}
	// Check to make sure we aren't trying to drop into the items being moved
	if d.TargetParent != zero && SameTable(data.Table, d.Table) {
		for _, r := range data.Rows {
			if !RowContainsRow(r, d.TargetParent) {
				continue
			}
			// Can't drop into itself, so reject the drop
			d.inDragOver = false
			d.TargetParent = zero
			d.Table.MarkForRedraw()
			d.Table.FlushDrawing()
			return drag.None
		}
	}
	d.Table.MarkForRedraw()
	d.Table.FlushDrawing()
	return op
}

// DropCallback handles processing a drop.
func (d *TableDrop[T, U]) DropCallback(di drag.Info, where geom.Point, mods mod.Modifiers) bool {
	defer func() { SafeCall(d.DragExitCallback) }()
	var op drag.Op
	SafeCall(func() { op = d.DragUpdatedCallback(di, where, mods) })
	if op == drag.None {
		return false
	}
	data, ok := dragTableData.(*TableDragData[T])
	if !ok {
		return false
	}
	var savedScrollX, savedScrollY float32
	if scroller := d.Table.ScrollRoot(); scroller != nil {
		savedScrollX, savedScrollY = scroller.Position()
		defer func() {
			scroller.SetPosition(savedScrollX, savedScrollY)
		}()
	}
	var zero T
	d.inDragOver = false
	move := false
	SafeCall(func() { move = d.shouldMoveDataCallback(data.Table, d.Table) })
	var undo *UndoEdit[U]
	if d.willDropCallback != nil {
		SafeCall(func() { undo = d.willDropCallback(data.Table, d.Table, move) })
	}
	rows := slices.Clone(data.Rows)
	if move {
		// Remove the drag rows from their original places
		commonParents := collectCommonParents(rows)
		sameTable := SameTable(data.Table, d.Table)
		for parent, list := range commonParents {
			var children []T
			if parent == zero {
				// Use the model's root rows rather than the table's RootRows(), since the latter returns the flattened
				// filter results when the source table has a filter applied, which would replace the model's real
				// hierarchy with the filter results below.
				children = data.Table.Model.RootRows()
			} else {
				children = parent.Children()
			}
			list = d.pruneRows(parent, children, makeRowSet(list), sameTable)
			if parent == zero {
				data.Table.Model.SetRootRows(list)
			} else {
				parent.SetChildren(list)
			}
		}
		// Remove the moved rows and their descendants from the source table's filtered view, if it has one, since they
		// are no longer part of its model.
		data.Table.removeRowsFromFilter(rows)
		data.Table.ClearSelection()
		data.Table.SyncToModel()

		// Set the new parent
		for _, row := range rows {
			row.SetParent(d.TargetParent)
		}

		// Notify the source table if it is different from the destination
		if !SameTable(d.Table, data.Table) && data.Table.DragRemovedRowsCallback != nil {
			SafeCall(data.Table.DragRemovedRowsCallback)
		}
	} else {
		// Make a copy of the data
		for i, row := range rows {
			rows[i] = row.CloneForTarget(d.Table, d.TargetParent)
		}
	}

	// Insert the rows into their new location
	var targetRows []T
	if d.TargetParent == zero {
		targetRows = d.Table.RootRows()
	} else {
		targetRows = d.TargetParent.Children()
	}
	targetRows = slices.Insert(slices.Clone(targetRows), max(min(d.TargetIndex, len(targetRows)), 0), rows...)
	if d.TargetParent == zero {
		d.Table.SetRootRows(targetRows)
	} else {
		d.TargetParent.SetChildren(targetRows)
		d.Table.SyncToModel()
	}

	// Restore selection
	selMap := make(map[tid.TID]bool, len(rows))
	for _, row := range rows {
		selMap[row.ID()] = true
	}
	d.Table.SetSelectionMap(selMap)

	// Notify the destination table
	SafeCall(d.Table.DropOccurredCallback)

	if d.didDropCallback != nil {
		SafeCall(func() { d.didDropCallback(undo, data.Table, d.Table, move) })
	}

	d.Table.MarkForRedraw()
	return true
}

// DragExitCallback handles resetting the state when a drag is no longer of interest.
func (d *TableDrop[T, U]) DragExitCallback() {
	d.inDragOver = false
	var zero T
	d.TargetParent = zero
	d.Table.MarkForRedraw()
	d.Table.FlushDrawing()
}

func (d *TableDrop[T, U]) pruneRows(parent T, rows []T, movingSet map[tid.TID]bool, sameTable bool) []T {
	// Only adjust TargetIndex when the removals come from the destination table's target parent. Without the sameTable
	// check, a cross-table move of root rows would match here (both parents are the zero value) and shift the insertion
	// point in the destination based on removals from the source.
	movingToThisParent := sameTable && d.TargetParent == parent
	list := make([]T, 0, len(rows))
	for i, row := range rows {
		if movingSet[row.ID()] {
			// Only removals strictly before the insertion point shift it; a row being removed at the insertion point
			// itself sits after the insertion, so dropping rows directly above themselves remains a no-op.
			if movingToThisParent && d.TargetIndex > i {
				d.TargetIndex--
			}
		} else {
			list = append(list, row)
		}
	}
	return list
}

func makeRowSet[T TableRowConstraint[T]](rows []T) map[tid.TID]bool {
	set := make(map[tid.TID]bool, len(rows))
	for _, row := range rows {
		set[row.ID()] = true
	}
	return set
}

func collectCommonParents[T TableRowConstraint[T]](rows []T) map[T][]T {
	m := make(map[T][]T)
	for _, row := range rows {
		parent := row.Parent()
		m[parent] = append(m[parent], row)
	}
	return m
}
