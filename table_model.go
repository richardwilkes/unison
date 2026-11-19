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
	"github.com/richardwilkes/toolbox/v2/tid"
)

// TableModel provides access to the root nodes of the table's data underlying model.
type TableModel[T TableRowConstraint[T]] interface {
	// RootRowCount returns the number of top-level rows.
	RootRowCount() int
	// RootRows returns the top-level rows. Do not alter the returned list.
	RootRows() []T
	// SetRootRows sets the top-level rows this table will display. After calling this method, any tables using this
	// model should have their SyncToModel() method called.
	SetRootRows(rows []T)
}

// TableRowData provides information about a single row of data.
type TableRowData[T any] interface {
	// CloneForTarget creates a duplicate of this row with its parent set to 'newParent'. 'target' is the table, or the
	// view of it (see Table.NewView) that took the drop, that the row will be placed within. Limitations in the way
	// generics work in Go prevent this from being declared as a *Table.
	CloneForTarget(target Paneler, newParent T) T
	// ID returns the ID of this data.
	ID() tid.TID
	// Parent returns the parent of this row, or nil if it is a root node.
	Parent() T
	// SetParent sets the parent of this row. parent will be nil if this is a top-level row.
	SetParent(parent T)
	// CanHaveChildren returns true if this row can have children, even if it currently does not have any.
	CanHaveChildren() bool
	// Children returns the child rows.
	Children() []T
	// SetChildren sets the children of this row.
	SetChildren(children []T)
	// CellDataForSort returns the string that represents the data in the specified cell.
	CellDataForSort(col int) string
	// ColumnCell returns the panel that should be placed at the position of the cell for the given column index.
	// 'focused' is true when the table, or one of its cells, holds the keyboard focus. Any widget that retains state --
	// which includes anything that takes the keyboard focus so it can be edited -- must be the same instance on every
	// call rather than a newly created one, and must never be shared between rows or between tables, which also means
	// CloneForTarget() must not copy it into the clone.
	ColumnCell(row, col int, foreground, background Ink, selected, indirectlySelected, focused bool) Paneler
	// IsOpen returns true if the row can have children and is currently showing its children.
	IsOpen() bool
	// SetOpen sets the row's open state.
	SetOpen(open bool)
}

// TableRowAccessibleText is implemented by a row whose sort text is not what a person should hear. A table names each
// row and cell for an assistive technology from CellDataForSort, which is usually the text the cell shows. A row that
// puts something ahead of that text so that it sorts where it should -- a marker that groups containers first, a rank
// that orders kinds of row -- would have that marker read out in front of every row, so such a row implements this to
// hand back the words on their own. Where a row does not implement it, CellDataForSort is used.
type TableRowAccessibleText interface {
	// CellDataForAccessibility returns what an assistive technology should call the specified cell.
	CellDataForAccessibility(col int) string
}

// TableRowCellSizes is implemented by a row that keeps the preferred sizes of its cells, so that a table can measure it
// without its cells. A table measures every row it shows, but asks only for the cells of those in view to draw them,
// so such a row need not keep the cell of a row out of view once it has been measured, nor build it to be measured
// again. A row that does not implement this is asked for each of its cells every time it is measured.
//
// The table never drops the sizes a row keeps, not even when it measures every row again, so the row must drop them
// itself whenever its cells may change size: when what they show changes, and when the theme changes, since the fonts
// may have changed with it (see ThemeChangedCallback).
type TableRowCellSizes interface {
	// KeptCellSize returns the preferred size the row keeps for its cell in the given column when the cell is as wide
	// as 'width', or as wide as it likes when that is 0 or less, and whether it keeps one. 'row' is the row's index.
	KeptCellSize(row, col int, width float32) (size geom.Size, kept bool)
	// KeepCellSize hands the row the preferred size the table measured, at that width, for the cell ColumnCell just
	// returned for the column.
	KeepCellSize(row, col int, width float32, size geom.Size)
}

// TableRowConstraint defines the constraints required of the data type used for data rows in tables.
type TableRowConstraint[T any] interface {
	comparable
	TableRowData[T]
}

// SimpleTableModel is a simple implementation of TableModel.
type SimpleTableModel[T TableRowConstraint[T]] struct {
	roots []T
}

// RootRowCount implements TableModel.
func (m *SimpleTableModel[T]) RootRowCount() int {
	return len(m.roots)
}

// RootRows implements TableModel.
func (m *SimpleTableModel[T]) RootRows() []T {
	return m.roots
}

// SetRootRows implements TableModel.
func (m *SimpleTableModel[T]) SetRootRows(rows []T) {
	m.roots = rows
}

// CollectIDsFromRow returns a map containing the IDs of the provided node and all of its descendants.
func CollectIDsFromRow[T TableRowConstraint[T]](node T, ids map[tid.TID]bool) {
	ids[node.ID()] = true
	for _, child := range node.Children() {
		CollectIDsFromRow(child, ids)
	}
}
