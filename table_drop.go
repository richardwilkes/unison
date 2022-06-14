// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"github.com/google/uuid"
	"github.com/richardwilkes/toolbox/xmath"
	"golang.org/x/exp/slices"
)

// TableDrop provides default support for dropping data into a table. This should only be instantiated by a call to
// Table.InstallDropSupport().
type TableDrop[T TableRowConstraint[T], U any] struct {
	Table                  *Table[T]
	DragKey                string
	TargetParent           T
	TargetIndex            int
	AllDragData            map[string]any
	TableDragData          *TableDragData[T]
	originalDrawOver       func(*Canvas, Rect)
	shouldMoveDataCallback func(from, to *Table[T]) bool
	willDropCallback       func(from, to *Table[T], move bool) *UndoEdit[U]
	didDropCallback        func(undo *UndoEdit[U], from, to *Table[T], move bool)
	top                    float32
	left                   float32
	inDragOver             bool
}

// DrawOverCallback handles drawing the drop zone feedback.
func (d *TableDrop[T, U]) DrawOverCallback(gc *Canvas, rect Rect) {
	if d.originalDrawOver != nil {
		d.originalDrawOver(gc, rect)
	}
	if d.inDragOver {
		r := d.Table.ContentRect(false)
		r.Inset(NewUniformInsets(1))
		paint := DropAreaColor.Paint(gc, r, Stroke)
		paint.SetStrokeWidth(2)
		paint.SetColorFilter(Alpha30Filter())
		gc.DrawRect(r, paint)
		paint.SetColorFilter(nil)
		paint.SetPathEffect(DashEffect())
		gc.DrawLine(d.left, d.top, r.Right(), d.top, paint)
	}
}

// DataDragOverCallback handles determining if a given drag is one that we are interested in.
func (d *TableDrop[T, U]) DataDragOverCallback(where Point, data map[string]any) bool {
	var zero T
	d.inDragOver = false
	if dd, ok := data[d.DragKey]; ok {
		if d.TableDragData, ok = dd.(*TableDragData[T]); ok {
			d.inDragOver = true
			last := d.Table.LastRowIndex()
			contentRect := d.Table.ContentRect(false)
			if where.Y >= contentRect.Bottom()-2 {
				// Over bottom edge, adding to end of top-level rows
				d.TargetParent = zero
				d.TargetIndex = d.Table.RootRowCount()
				rect := d.Table.RowFrame(last)
				d.top = xmath.Min(rect.Bottom()+1+d.Table.Padding.Bottom, contentRect.Bottom()-1)
				d.left, _ = d.Table.ColumnEdges(xmath.Max(d.Table.HierarchyColumnIndex, 0))
				d.Table.MarkForRedraw()
				return true
			}
			if rowIndex := d.Table.OverRow(where.Y); rowIndex != -1 {
				// Over row
				d.TargetIndex = -1
				row := d.Table.RowFromIndex(rowIndex)
				rect := d.Table.CellFrame(rowIndex, xmath.Max(d.Table.HierarchyColumnIndex, 0))
				if where.Y >= d.Table.RowFrame(rowIndex).CenterY() {
					d.top = xmath.Min(rect.Bottom()+1+d.Table.Padding.Bottom, contentRect.Bottom()-1)
					d.left = rect.X
					// Over lower half of row
					if row.CanHaveChildren() {
						// Row is a container; add to container at index 0
						d.TargetParent = row
						d.TargetIndex = 0
						if d.Table.HierarchyColumnIndex != -1 {
							d.left += d.Table.HierarchyIndent
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
					d.top = xmath.Max(rect.Y-d.Table.Padding.Bottom, 1)
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
						if child.UUID() == row.UUID() {
							d.TargetIndex = i
							break
						}
					}
					if d.TargetIndex == -1 {
						d.TargetIndex = len(children)
					}
				}
				// Check to make sure we aren't trying to drop into the items being moved
				if d.TargetParent != zero && d.TableDragData.Table == d.Table {
					for _, r := range d.TableDragData.Rows {
						if RowContainsRow(r, d.TargetParent) {
							// Can't drop into itself, so abort
							d.inDragOver = false
							d.TargetParent = zero
							break
						}
					}
				}
				d.Table.MarkForRedraw()
				return true
			}
			// Not over any row, adding to end of top-level rows
			d.TargetParent = zero
			d.TargetIndex = d.Table.RootRowCount()
			rect := d.Table.RowFrame(last)
			d.top = xmath.Min(rect.Bottom()+1+d.Table.Padding.Bottom, contentRect.Bottom()-1)
			d.left, _ = d.Table.ColumnEdges(xmath.Max(d.Table.HierarchyColumnIndex, 0))
			d.Table.MarkForRedraw()
			return true
		}
	}
	return false
}

// DataDragExitCallback handles resetting the state when a drag is no longer of interest.
func (d *TableDrop[T, U]) DataDragExitCallback() {
	d.inDragOver = false
	var zero T
	d.TargetParent = zero
	d.Table.MarkForRedraw()
}

// DataDragDropCallback handles processing a drop.
func (d *TableDrop[T, U]) DataDragDropCallback(where Point, data map[string]any) {
	var zero T
	d.inDragOver = false
	var ok bool
	if d.TableDragData, ok = data[d.DragKey].(*TableDragData[T]); ok {
		d.AllDragData = data

		move := d.shouldMoveDataCallback(d.TableDragData.Table, d.Table)
		var undo *UndoEdit[U]
		if d.willDropCallback != nil {
			undo = d.willDropCallback(d.TableDragData.Table, d.Table, move)
		}
		rows := slices.Clone(d.TableDragData.Rows)
		if move {
			// Remove the drag rows from their original places
			commonParents := collectCommonParents(rows)
			for parent, list := range commonParents {
				var children []T
				if parent == zero {
					children = d.TableDragData.Table.RootRows()
				} else {
					children = parent.Children()
				}
				list = d.pruneRows(parent, children, makeRowSet(list))
				if parent == zero {
					d.TableDragData.Table.Model.SetRootRows(list)
				} else {
					parent.SetChildren(list)
				}
			}
			d.TableDragData.Table.ClearSelection()
			d.TableDragData.Table.SyncToModel()

			// Set the new parent
			for _, row := range rows {
				row.SetParent(d.TargetParent)
			}

			// Notify the source table if it is different from the destination
			if d.Table != d.TableDragData.Table {
				if d.Table != d.TableDragData.Table && d.TableDragData.Table.DragRemovedRowsCallback != nil {
					d.TableDragData.Table.DragRemovedRowsCallback()
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
		targetRows = slices.Insert(slices.Clone(targetRows), xmath.Max(xmath.Min(d.TargetIndex, len(targetRows)-1), 0), rows...)
		if d.TargetParent == zero {
			d.Table.SetRootRows(targetRows)
		} else {
			d.TargetParent.SetChildren(targetRows)
			d.Table.SyncToModel()
		}

		// Restore selection
		selMap := make(map[uuid.UUID]bool, len(rows))
		for _, row := range rows {
			selMap[row.UUID()] = true
		}
		d.Table.SetSelectionMap(selMap)

		// Notify the destination table
		if d.Table.DropOccurredCallback != nil {
			d.Table.DropOccurredCallback()
		}

		if d.didDropCallback != nil {
			d.didDropCallback(undo, d.TableDragData.Table, d.Table, move)
		}
	}
	d.Table.MarkForRedraw()
	d.TargetParent = zero
	d.AllDragData = nil
	d.TableDragData = nil
}

func (d *TableDrop[T, U]) pruneRows(parent T, rows []T, movingSet map[uuid.UUID]bool) []T {
	movingToThisParent := d.TargetParent == parent
	list := make([]T, 0, len(rows))
	for i, row := range rows {
		if movingSet[row.UUID()] {
			if movingToThisParent && d.TargetIndex >= i {
				d.TargetIndex--
			}
		} else {
			list = append(list, row)
		}
	}
	return list
}

func makeRowSet[T TableRowConstraint[T]](rows []T) map[uuid.UUID]bool {
	set := make(map[uuid.UUID]bool, len(rows))
	for _, row := range rows {
		set[row.UUID()] = true
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
