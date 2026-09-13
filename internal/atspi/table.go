// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package atspi

import (
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/role"
	"github.com/richardwilkes/unison/internal/dbus"
)

// cellSpan is how many rows or columns one Unison cell covers. Nothing in the schema can say otherwise, so every cell
// covers exactly one of each, which is what the extent and span methods report.
const cellSpan = 1

// tableInterface returns the org.a11y.atspi.Table interface of a table or a tree, which is what an assistive
// technology's table navigation commands — move a row down, move a column across, read this row, read this column —
// are answered through. Without it a control with a table role is a list of anonymous children as far as a client is
// concerned, whatever its role claims.
//
// Two things about it follow from how a Unison table is described rather than from AT-SPI. A snapshot holds only the
// rows that can be seen, plus the selection and the row the focus is in, so most rows of a large table have no object
// at any moment: the methods that name a row that is not described answer with the null reference, an empty string or
// false, which is what a client that has scrolled ahead of the toolkit already has to cope with. And a table's column
// headers belong to a separate header panel rather than to the table, so GetRowHeader and GetColumnHeader have nothing
// to hand back; the headers are reported in their own right, where an assistive technology finds them by walking.
//
// Column selection is not part of the schema at all: Unison selects rows. The four methods that would change or report
// one answer as they would for a table that allows none.
func (o *nodeObject) tableInterface() *dbus.Interface {
	return &dbus.Interface{
		Name: InterfaceTable,
		Methods: []*dbus.Method{
			{Name: "GetAccessibleAt", In: "ii", Out: objectRefSignature, Handle: o.getAccessibleAt},
			{Name: "GetIndexAt", In: "ii", Out: "i", Handle: o.getIndexAt},
			{Name: "GetRowAtIndex", In: "i", Out: "i", Handle: o.getRowAtIndex},
			{Name: "GetColumnAtIndex", In: "i", Out: "i", Handle: o.getColumnAtIndex},
			{Name: "GetRowDescription", In: "i", Out: "s", Handle: o.getRowDescription},
			{Name: "GetColumnDescription", In: "i", Out: "s", Handle: replyEmptyString},
			{Name: "GetRowExtentAt", In: "ii", Out: "i", Handle: o.getCellExtent},
			{Name: "GetColumnExtentAt", In: "ii", Out: "i", Handle: o.getCellExtent},
			{Name: "GetRowHeader", In: "i", Out: objectRefSignature, Handle: replyNullReference},
			{Name: "GetColumnHeader", In: "i", Out: objectRefSignature, Handle: replyNullReference},
			{Name: "GetSelectedRows", Out: int32ArraySignature, Handle: o.getSelectedRows},
			{Name: "GetSelectedColumns", Out: int32ArraySignature, Handle: replyNoIndexes},
			{Name: "IsRowSelected", In: "i", Out: "b", Handle: o.isRowSelected},
			{Name: "IsColumnSelected", In: "i", Out: "b", Handle: replyFalse},
			{Name: "IsSelected", In: "ii", Out: "b", Handle: o.isCellSelected},
			{Name: "AddRowSelection", In: "i", Out: "b", Handle: o.addRowSelection},
			{Name: "AddColumnSelection", In: "i", Out: "b", Handle: replyFalse},
			{Name: "RemoveRowSelection", In: "i", Out: "b", Handle: o.removeRowSelection},
			{Name: "RemoveColumnSelection", In: "i", Out: "b", Handle: replyFalse},
			{
				Name: "GetRowColumnExtentsAtIndex", In: "i", Out: rowColumnExtentsSignature,
				Handle: o.getRowColumnExtentsAtIndex,
			},
		},
		Properties: []*dbus.Property{
			{Name: "NRows", Sig: "i", Get: func() (any, error) { return int32(o.node.RowCount), nil }},
			{Name: "NColumns", Sig: "i", Get: func() (any, error) { return int32(o.node.ColumnCount), nil }},
			{Name: "Caption", Sig: objectRefSignature, Get: func() (any, error) { return nullReference(), nil }},
			{Name: "Summary", Sig: objectRefSignature, Get: func() (any, error) { return nullReference(), nil }},
			{
				Name: "NSelectedRows",
				Sig:  "i",
				Get:  func() (any, error) { return int32(len(o.selectedRows())), nil },
			},
			{Name: "NSelectedColumns", Sig: "i", Get: func() (any, error) { return int32(0), nil }},
		},
	}
}

// tableCellInterface returns the org.a11y.atspi.TableCell interface of one cell, which is the half of the table
// protocol that answers from the cell rather than from the container. It is what lets an assistive technology say
// "row 4, column 2" about whatever the user has just landed on without walking back up to the table and searching it.
//
// The header cell lists are empty for the same reason GetRowHeader and GetColumnHeader hand back nothing: a Unison
// table's column headers belong to a header panel of their own rather than to the table.
func (o *nodeObject) tableCellInterface() *dbus.Interface {
	return &dbus.Interface{
		Name: InterfaceTableCell,
		Methods: []*dbus.Method{
			{Name: "GetRowColumnSpan", Out: rowColumnSpanSignature, Handle: o.getRowColumnSpan},
		},
		Properties: []*dbus.Property{
			{Name: "ColumnSpan", Sig: "i", Get: func() (any, error) { return int32(cellSpan), nil }},
			{
				Name: "Position",
				Sig:  intPairSignature,
				Get: func() (any, error) {
					return dbus.Struct{int32(o.node.RowIndex), int32(o.node.ColumnIndex)}, nil
				},
			},
			{Name: "RowSpan", Sig: "i", Get: func() (any, error) { return int32(cellSpan), nil }},
			{
				Name: "Table",
				Sig:  objectRefSignature,
				Get:  func() (any, error) { return o.a.reference(o.tableOf()), nil },
			},
			{
				Name: "ColumnHeaderCells",
				Sig:  objectRefArraySignature,
				Get:  func() (any, error) { return []dbus.ObjectRef{}, nil },
			},
			{
				Name: "RowHeaderCells",
				Sig:  objectRefArraySignature,
				Get:  func() (any, error) { return []dbus.ObjectRef{}, nil },
			},
		},
	}
}

// replyEmptyString answers a method whose answer Unison has nothing to put in.
func replyEmptyString(call *dbus.Call) {
	call.Reply("")
}

// replyNullReference answers a method that names an object Unison cannot point at.
func replyNullReference(call *dbus.Call) {
	call.Reply(nullReference())
}

// replyNoIndexes answers a method that lists indexes Unison never has any of.
func replyNoIndexes(call *dbus.Call) {
	call.Reply([]int32{})
}

// rowNode returns the described row of this table at the given row index, or nil when the table does not hold that row
// or is not describing it at the moment.
func (o *nodeObject) rowNode(row int) *accessibility.Node {
	if row < 0 || row >= o.node.RowCount {
		return nil
	}
	for _, id := range o.children() {
		if n := o.data.node(id); n != nil && n.Role == role.Row && n.RowIndex == row {
			return n
		}
	}
	return nil
}

// cellNode returns the described cell of this table at the given row and column, or nil when there is none. The cells
// are the children of the row rather than of the table, and a row holds more than cells — the disclosure triangle of a
// tree is one of its children too — so the column is matched against the cells alone.
func (o *nodeObject) cellNode(row, col int) *accessibility.Node {
	rowNode := o.rowNode(row)
	if rowNode == nil || col < 0 || col >= o.node.ColumnCount {
		return nil
	}
	for _, id := range o.data.unignoredChildren(rowNode.ID) {
		if n := o.data.node(id); n != nil && n.Role == role.Cell && n.ColumnIndex == col {
			return n
		}
	}
	return nil
}

// selectedRows returns the indexes of the selected rows of this table, in ascending order, out of the rows it is
// describing. A snapshot describes the selection however far out of sight it is, up to a limit, so this is the whole
// selection of all but a table with a very large one.
func (o *nodeObject) selectedRows() []int32 {
	children := o.children()
	rows := make([]int32, 0, len(children))
	for _, id := range children {
		if n := o.data.node(id); n != nil && n.Role == role.Row && n.Selected {
			rows = append(rows, int32(n.RowIndex))
		}
	}
	return rows
}

// tableOf returns the id of the table or tree a cell belongs to, or zero if it is not in one. It is the nearest
// reported ancestor with a table role, which is the cell's grandparent in a well-formed table: the cell sits in a row,
// and the row in the table.
func (o *nodeObject) tableOf() accessibility.NodeID {
	for id := o.data.parent(o.node.ID); id != 0; id = o.data.parent(id) {
		n := o.data.node(id)
		if n == nil {
			return 0
		}
		if supportsTable(n.Role) {
			return id
		}
	}
	return 0
}

// cellIndex returns the flattened index of a cell, which is what AT-SPI's child index of a table means: the cells
// counted row by row, as ATK's own implementation counts them. It is not an index into anything Unison reports — a
// Unison table's children are its rows — but it is the only arithmetic the methods that convert between the two forms
// can agree on, and GetAccessibleAt is what a client reaches an actual object through.
func (o *nodeObject) cellIndex(row, col int) int {
	return row*o.node.ColumnCount + col
}

// getAccessibleAt implements org.a11y.atspi.Table.GetAccessibleAt.
func (o *nodeObject) getAccessibleAt(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	cell := o.cellNode(int(int32Arg(args, 0)), int(int32Arg(args, 1)))
	if cell == nil {
		call.Reply(nullReference())
		return
	}
	call.Reply(o.a.reference(cell.ID))
}

// getIndexAt implements org.a11y.atspi.Table.GetIndexAt. A row or column outside the table has no index, which AT-SPI
// spells as -1.
func (o *nodeObject) getIndexAt(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	row := int(int32Arg(args, 0))
	col := int(int32Arg(args, 1))
	if !o.inTable(row, col) {
		call.Reply(int32(-1))
		return
	}
	call.Reply(int32(o.cellIndex(row, col)))
}

// getRowAtIndex implements org.a11y.atspi.Table.GetRowAtIndex, which undoes the arithmetic [nodeObject.cellIndex] does.
func (o *nodeObject) getRowAtIndex(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	index := int(int32Arg(args, 0))
	if !o.isCellIndex(index) {
		call.Reply(int32(-1))
		return
	}
	call.Reply(int32(index / o.node.ColumnCount))
}

// getColumnAtIndex implements org.a11y.atspi.Table.GetColumnAtIndex.
func (o *nodeObject) getColumnAtIndex(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	index := int(int32Arg(args, 0))
	if !o.isCellIndex(index) {
		call.Reply(int32(-1))
		return
	}
	call.Reply(int32(index % o.node.ColumnCount))
}

// getRowDescription implements org.a11y.atspi.Table.GetRowDescription. Unison has no description of a row beyond what
// the row itself is named, which is what a client reads from the row's own object, so a row that is being described
// hands back its name and anything else hands back nothing.
func (o *nodeObject) getRowDescription(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	row := o.rowNode(int(int32Arg(args, 0)))
	if row == nil {
		call.Reply("")
		return
	}
	call.Reply(row.Name)
}

// getCellExtent implements both org.a11y.atspi.Table.GetRowExtentAt and GetColumnExtentAt. Nothing Unison reports
// spans more than one row or column, so the only question is whether there is a cell there at all; AT-SPI's answer for
// one there is not is -1.
func (o *nodeObject) getCellExtent(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	if !o.inTable(int(int32Arg(args, 0)), int(int32Arg(args, 1))) {
		call.Reply(int32(-1))
		return
	}
	call.Reply(int32(cellSpan))
}

// getSelectedRows implements org.a11y.atspi.Table.GetSelectedRows.
func (o *nodeObject) getSelectedRows(call *dbus.Call) {
	call.Reply(o.selectedRows())
}

// isRowSelected implements org.a11y.atspi.Table.IsRowSelected.
func (o *nodeObject) isRowSelected(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	row := o.rowNode(int(int32Arg(args, 0)))
	call.Reply(row != nil && row.Selected)
}

// isCellSelected implements org.a11y.atspi.Table.IsSelected. A cell is selected when its row is: Unison selects whole
// rows.
func (o *nodeObject) isCellSelected(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	col := int(int32Arg(args, 1))
	row := o.rowNode(int(int32Arg(args, 0)))
	call.Reply(row != nil && row.Selected && col >= 0 && col < o.node.ColumnCount)
}

// addRowSelection implements org.a11y.atspi.Table.AddRowSelection. In a table that allows more than one selected row
// the row joins the selection, since that is what a client asking for another one means; anywhere else it becomes the
// selection. The answer is optimistic, as every request is.
func (o *nodeObject) addRowSelection(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	row := o.rowNode(int(int32Arg(args, 0)))
	if row == nil {
		call.Reply(false)
		return
	}
	action := accessibility.Select
	if o.node.Multiselectable && row.Actions.Has(accessibility.AddToSelection) {
		action = accessibility.AddToSelection
	}
	call.Reply(o.requestOnChild(row, action))
}

// removeRowSelection implements org.a11y.atspi.Table.RemoveRowSelection.
func (o *nodeObject) removeRowSelection(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	call.Reply(o.requestOnChild(o.rowNode(int(int32Arg(args, 0))), accessibility.RemoveFromSelection))
}

// getRowColumnExtentsAtIndex implements org.a11y.atspi.Table.GetRowColumnExtentsAtIndex, which answers in one call
// everything the cell at a flattened index has to say about where it is.
func (o *nodeObject) getRowColumnExtentsAtIndex(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	index := int(int32Arg(args, 0))
	if !o.isCellIndex(index) {
		call.Reply(false, int32(0), int32(0), int32(0), int32(0), false)
		return
	}
	row := index / o.node.ColumnCount
	rowNode := o.rowNode(row)
	call.Reply(true, int32(row), int32(index%o.node.ColumnCount), int32(cellSpan), int32(cellSpan),
		rowNode != nil && rowNode.Selected)
}

// getRowColumnSpan implements org.a11y.atspi.TableCell.GetRowColumnSpan.
func (o *nodeObject) getRowColumnSpan(call *dbus.Call) {
	call.Reply(true, int32(o.node.RowIndex), int32(o.node.ColumnIndex), int32(cellSpan), int32(cellSpan))
}

// inTable returns true if a row and column pair names a cell of this table, whether or not that cell is one of the
// ones being described.
func (o *nodeObject) inTable(row, col int) bool {
	return row >= 0 && row < o.node.RowCount && col >= 0 && col < o.node.ColumnCount
}

// isCellIndex returns true if a flattened index names a cell of this table. See [nodeObject.cellIndex].
func (o *nodeObject) isCellIndex(index int) bool {
	return o.node.ColumnCount > 0 && index >= 0 && index < o.node.RowCount*o.node.ColumnCount
}
