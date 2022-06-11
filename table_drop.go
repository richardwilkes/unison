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
	"github.com/richardwilkes/toolbox/xmath"
	"golang.org/x/exp/slices"
)

// TableDrop provides default support for dropping data into a table. This should only be instantiated by a call to
// Table.InstallDropSupport().
type TableDrop[T TableRowConstraint[T]] struct {
	Table                  *Table[T]
	DragKey                string
	TargetParent           T
	TargetIndex            int
	AllDragData            map[string]any
	TableDragData          *TableDragData[T]
	originalDrawOver       func(*Canvas, Rect)
	shouldMoveDataCallback func(drop *TableDrop[T]) bool
	copyCallback           func(drop *TableDrop[T])
	setRowParentCallback   func(drop *TableDrop[T], row, newParent T)
	setChildRowsCallback   func(drop *TableDrop[T], row T, children []T)
	droppedCallback        func(drop *TableDrop[T], moved bool)
	top                    float32
	left                   float32
	inDragOver             bool
}

// DrawOverCallback handles drawing the drop zone feedback.
func (d *TableDrop[T]) DrawOverCallback(gc *Canvas, rect Rect) {
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
func (d *TableDrop[T]) DataDragOverCallback(where Point, data map[string]any) bool {
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
				d.TargetIndex = d.Table.TopLevelRowCount()
				rect := d.Table.RowFrame(last)
				d.top = rect.Bottom() - 1
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
					d.top = xmath.Min(rect.Bottom()+1, contentRect.Bottom()-1)
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
						row = d.Table.RowFromIndex(rowIndex + 1)
					}
				} else {
					// Over upper half of row; add to parent of this row at this row's index
					d.TargetParent = row.Parent()
					d.top = xmath.Max(rect.Y, 1)
					d.left = rect.X
				}
				if d.TargetIndex == -1 {
					var children []T
					if d.TargetParent == zero {
						children = d.Table.TopLevelRows()
					} else {
						children = d.TargetParent.Children()
					}
					for i, child := range children {
						if child == row {
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
			d.TargetIndex = d.Table.TopLevelRowCount()
			rect := d.Table.RowFrame(last)
			d.top = xmath.Min(rect.Bottom()+1, contentRect.Bottom()-1)
			d.left, _ = d.Table.ColumnEdges(xmath.Max(d.Table.HierarchyColumnIndex, 0))
			d.Table.MarkForRedraw()
			return true
		}
	}
	return false
}

// DataDragExitCallback handles resetting the state when a drag is no longer of interest.
func (d *TableDrop[T]) DataDragExitCallback() {
	d.inDragOver = false
	var zero T
	d.TargetParent = zero
	d.Table.MarkForRedraw()
}

// DataDragDropCallback handles processing a drop.
func (d *TableDrop[T]) DataDragDropCallback(where Point, data map[string]any) {
	var zero T
	d.inDragOver = false
	var ok bool
	if d.TableDragData, ok = data[d.DragKey].(*TableDragData[T]); ok {
		d.AllDragData = data
		top := d.Table.TopLevelRows()

		move := d.shouldMoveDataCallback(d)
		if move {
			// Remove the drag rows from their original places
			commonParents := collectCommonParents(d.TableDragData.Rows)
			for parent, rows := range commonParents {
				var children []T
				if parent == zero {
					children = top
				} else {
					children = parent.Children()
				}
				rows = d.pruneRows(parent, children, makeRowSet(rows))
				if parent == zero {
					top = rows
				}
				d.setChildRowsCallback(d, parent, rows)
			}
		} else {
			// Make a copy of the data
			d.copyCallback(d)
		}

		// Set the new parent
		for _, row := range d.TableDragData.Rows {
			d.setRowParentCallback(d, row, d.TargetParent)
		}

		// Insert the rows into their new location
		var rows []T
		if d.TargetParent == zero {
			rows = top
		} else {
			rows = d.TargetParent.Children()
		}
		rows = slices.Insert(rows, xmath.Max(d.TargetIndex, 0), d.TableDragData.Rows...)
		if d.TargetParent == zero {
			top = rows
		}
		d.setChildRowsCallback(d, d.TargetParent, rows)

		// Sync the data
		d.Table.SetTopLevelRows(top)
		d.Table.SyncToModel()
		d.droppedCallback(d, move)
	}
	d.Table.MarkForRedraw()
	d.TargetParent = zero
	d.AllDragData = nil
	d.TableDragData = nil
}

func (d *TableDrop[T]) pruneRows(parent T, rows []T, movingSet map[T]bool) []T {
	movingToThisParent := d.TargetParent == parent
	j := 0
	for i, row := range rows {
		if movingSet[row] {
			if movingToThisParent && d.TargetIndex >= i {
				d.TargetIndex--
			}
		} else {
			rows[j] = row
			j++
		}
	}
	var zero T
	i := j
	for ; i < len(rows); i++ {
		rows[i] = zero
	}
	return rows[:j]
}

func makeRowSet[T TableRowConstraint[T]](rows []T) map[T]bool {
	set := make(map[T]bool, len(rows))
	for _, row := range rows {
		set[row] = true
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
