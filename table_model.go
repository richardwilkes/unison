// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import "github.com/google/uuid"

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
	// CloneForTarget creates a duplicate of this row with its parent set to 'newParent'. 'target' is the table that the
	// row will be placed within. Limitations in the way generics work in Go prevent this from being declared as a
	// *Table.
	CloneForTarget(target Paneler, newParent T) T
	// UUID returns the UUID of this data.
	UUID() uuid.UUID
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
	// ColumnCell returns the panel that should be placed at the position of the cell for the given column index. If you
	// need for the cell to retain widget state, make sure to return the same widget each time rather than creating a
	// new one.
	ColumnCell(row, col int, foreground, background Ink, selected, indirectlySelected, focused bool) Paneler
	// IsOpen returns true if the row can have children and is currently showing its children.
	IsOpen() bool
	// SetOpen sets the row's open state.
	SetOpen(open bool)
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

// CollectUUIDsFromRow returns a map containing the UUIDs of the provided node and all of its descendants.
func CollectUUIDsFromRow[T TableRowConstraint[T]](node T, ids map[uuid.UUID]bool) {
	ids[node.UUID()] = true
	for _, child := range node.Children() {
		CollectUUIDsFromRow(child, ids)
	}
}
