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
	var zero T
	d.inDragOver = true
	last := d.Table.LastRowIndex()
	contentRect := d.Table.ContentRect(false)
	hierarchyColumnIndex := d.Table.ColumnIndexForID(d.Table.HierarchyColumnID)
	var op drag.Op
	SafeCall(func() {
		if d.shouldMoveDataCallback(data.Table, d.Table) {
			op = drag.Move
		} else {
			op = drag.Copy
		}
	})
	if where.Y >= contentRect.Bottom()-2 {
		// Over bottom edge, adding to end of top-level rows
		d.TargetParent = zero
		d.TargetIndex = d.Table.RootRowCount()
		rect := d.Table.RowFrame(last)
		d.top = min(rect.Bottom()+1+d.Table.Padding.Bottom, contentRect.Bottom()-1)
		d.left, _ = d.Table.ColumnEdges(max(hierarchyColumnIndex, 0))
		d.Table.MarkForRedraw()
		d.Table.FlushDrawing()
		return op
	}
	if rowIndex := d.Table.OverRow(where.Y); rowIndex != -1 {
		// Over row
		d.TargetIndex = -1
		row := d.Table.RowFromIndex(rowIndex)
		rect := d.Table.CellFrame(rowIndex, max(hierarchyColumnIndex, 0))
		if where.Y >= d.Table.RowFrame(rowIndex).CenterY() {
			d.top = min(rect.Bottom()+1+d.Table.Padding.Bottom, contentRect.Bottom()-1)
			d.left = rect.X
			// Over lower half of row
			if row.CanHaveChildren() {
				// Row is a container; add to container at index 0
				d.TargetParent = row
				d.TargetIndex = 0
				if hierarchyColumnIndex != -1 {
					if hierarchyIndent := d.Table.CurrentHierarchyIndent(); hierarchyIndent > 0 {
						d.left += hierarchyIndent
					}
				}
			} else {
				// Row is not a container; add as sibling below this row
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
			// Over upper half of row; add to parent of this row at this row's index
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
		if d.TargetParent != zero && data.Table == d.Table {
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
	// Not over any row, adding to end of top-level rows
	d.TargetParent = zero
	d.TargetIndex = d.Table.RootRowCount()
	rect := d.Table.RowFrame(last)
	d.top = min(rect.Bottom()+1+d.Table.Padding.Bottom, contentRect.Bottom()-1)
	d.left, _ = d.Table.ColumnEdges(max(hierarchyColumnIndex, 0))
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
		for parent, list := range commonParents {
			var children []T
			if parent == zero {
				children = data.Table.RootRows()
			} else {
				children = parent.Children()
			}
			list = d.pruneRows(parent, children, makeRowSet(list))
			if parent == zero {
				data.Table.Model.SetRootRows(list)
			} else {
				parent.SetChildren(list)
			}
		}
		data.Table.ClearSelection()
		data.Table.SyncToModel()

		// Set the new parent
		for _, row := range rows {
			row.SetParent(d.TargetParent)
		}

		// Notify the source table if it is different from the destination
		if d.Table != data.Table {
			if d.Table != data.Table && data.Table.DragRemovedRowsCallback != nil {
				SafeCall(data.Table.DragRemovedRowsCallback)
			}
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

func (d *TableDrop[T, U]) pruneRows(parent T, rows []T, movingSet map[tid.TID]bool) []T {
	movingToThisParent := d.TargetParent == parent
	list := make([]T, 0, len(rows))
	for i, row := range rows {
		if movingSet[row.ID()] {
			if movingToThisParent && d.TargetIndex >= i {
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
