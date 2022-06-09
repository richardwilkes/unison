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
	"github.com/richardwilkes/toolbox"
	"github.com/richardwilkes/toolbox/log/jot"
	"github.com/richardwilkes/toolbox/xmath"
)

// TableDrop provides default support for dropping data into a table. This should only be instantiated by a call to
// Table.InstallDropSupport().
type TableDrop struct {
	Table            *Table
	DragKey          string
	TargetParent     TableRowData
	TargetIndex      int
	DropData         map[string]any
	DropTableData    *TableDragData
	originalDrawOver func(*Canvas, Rect)
	dropCallback     func(*TableDrop)
	top              float32
	left             float32
	inDragOver       bool
}

// DrawOverCallback handles drawing the drop zone feedback.
func (d *TableDrop) DrawOverCallback(gc *Canvas, rect Rect) {
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
func (d *TableDrop) DataDragOverCallback(where Point, data map[string]any) bool {
	if _, exists := data[d.DragKey]; exists {
		d.inDragOver = true
		last := d.Table.LastRowIndex()
		contentRect := d.Table.ContentRect(false)
		if where.Y >= contentRect.Bottom()-2 {
			// Over bottom edge, adding to end of top-level rows
			d.TargetParent = nil
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
				if row.CanHaveChildRows() {
					// Row is a container; add to container at index 0
					d.TargetParent = row
					d.TargetIndex = 0
					if d.Table.HierarchyColumnIndex != -1 {
						d.left += d.Table.HierarchyIndent
					}
				} else {
					// Row is not a container; add as sibling below this row
					d.TargetParent = row.ParentRow()
					row = d.Table.RowFromIndex(rowIndex + 1)
				}
			} else {
				// Over upper half of row; add to parent of this row at this row's index
				d.TargetParent = row.ParentRow()
				d.top = xmath.Max(rect.Y, 1)
				d.left = rect.X
			}
			if d.TargetIndex == -1 {
				var children []TableRowData
				if d.TargetParent == nil {
					children = d.Table.TopLevelRows()
				} else {
					children = d.TargetParent.ChildRows()
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
			d.Table.MarkForRedraw()
			return true
		}
		// Not over any row, adding to end of top-level rows
		d.TargetParent = nil
		d.TargetIndex = d.Table.TopLevelRowCount()
		rect := d.Table.RowFrame(last)
		d.top = xmath.Min(rect.Bottom()+1, contentRect.Bottom()-1)
		d.left, _ = d.Table.ColumnEdges(xmath.Max(d.Table.HierarchyColumnIndex, 0))
		d.Table.MarkForRedraw()
		return true
	}
	return false
}

// DataDragExitCallback handles resetting the state when a drag is no longer of interest.
func (d *TableDrop) DataDragExitCallback() {
	d.inDragOver = false
	d.TargetParent = nil
	d.Table.MarkForRedraw()
}

// DataDragDropCallback handles processing a drop.
func (d *TableDrop) DataDragDropCallback(where Point, data map[string]any) {
	d.inDragOver = false
	d.DropData = data
	var ok bool
	if d.DropTableData, ok = data[d.DragKey].(*TableDragData); !ok {
		jot.Warn("unable to extract table drag data from drag key: " + d.DragKey)
	}
	d.Table.MarkForRedraw()
	toolbox.Call(func() { d.dropCallback(d) })
	d.TargetParent = nil
}
