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
// headers belong to a header panel of their own rather than to the table, so the header that describes a column has to
// be searched for; see [buildTableHeaders]. Rows have no headers at all in Unison, which is what GetRowHeader says.
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
			{Name: "GetColumnDescription", In: "i", Out: "s", Handle: o.getColumnDescription},
			{Name: "GetRowExtentAt", In: "ii", Out: "i", Handle: o.getCellExtent},
			{Name: "GetColumnExtentAt", In: "ii", Out: "i", Handle: o.getCellExtent},
			{Name: "GetRowHeader", In: "i", Out: objectRefSignature, Handle: replyNullReference},
			{Name: "GetColumnHeader", In: "i", Out: objectRefSignature, Handle: o.getColumnHeader},
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
// A cell's column header is the one describing its own column, which is what Orca announces as the user arrows across a
// table. Rows have no headers in Unison, so that list is always empty.
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
				Get:  func() (any, error) { return o.a.references(o.columnHeaderCells()), nil },
			},
			{
				Name: "RowHeaderCells",
				Sig:  objectRefArraySignature,
				Get:  func() (any, error) { return []dbus.ObjectRef{}, nil },
			},
		},
	}
}

// maxHeaderSearchDepth bounds how far the search for a table's header panel will walk, so that a malformed tree cannot
// make it run on forever. Real hierarchies are orders of magnitude shallower than this.
const maxHeaderSearchDepth = 512

// buildTableHeaders returns the column headers that describe each table or tree in a snapshot, keyed by the id of the
// table they belong to and in the order the header panel publishes them. It is worked out once per published snapshot,
// since an assistive technology asks for a column's header once per cell as the user arrows across a table, and the
// answer takes a walk of the whole tree plus a scan of one subtree per ancestor.
//
// Finding a header takes a search because a Unison table and its header are separate panels: the header goes into the
// column-header slot of the scroll panel whose content is the table, so it is nowhere inside the table's own subtree.
// The steps are the ones the other two adapters take for the same question — see uiaTableHeaderFor in internal/w32 and
// axTableHeaderOf in internal/cocoa — so that all three platforms name the same header for the same window:
//
//  1. A TableHeader the table's Controls name, or one whose own Controls name the table. An explicit link is the right
//     answer whenever a widget records one, and is the only thing that can be right when a window is laid out oddly.
//  2. Proximity: the nearest ancestor of the table that contains exactly one TableHeader and no table but this one.
//     That ancestor is the scroll panel in every layout Unison builds. Insisting that it hold only one table is what
//     stops two tables side by side from claiming each other's header; an ancestor that holds more than one ends the
//     search rather than widening it, since widening it can only make the ambiguity worse. The search also stops the
//     moment it reaches another table or tree, since everything beyond that belongs to the outer table.
//  3. Nothing, which leaves the table with no headers to report.
func buildTableHeaders(t *accessibility.Tree) map[accessibility.NodeID][]accessibility.NodeID {
	var tables []accessibility.NodeID
	linked := make(map[accessibility.NodeID]accessibility.NodeID)
	t.Walk(func(n *accessibility.Node) bool {
		switch {
		case n.Ignored:
			// An ignored node has no object, so it is neither a table anything is reported for nor a header anything
			// can be pointed at. What is inside it still counts, which is why the walk carries on.
		case supportsTable(n.Role):
			tables = append(tables, n.ID)
		case n.Role == role.TableHeader:
			for _, controlled := range n.Controls {
				if _, exists := linked[controlled]; !exists {
					linked[controlled] = n.ID
				}
			}
		}
		return true
	})
	if len(tables) == 0 {
		return nil
	}
	headers := make(map[accessibility.NodeID][]accessibility.NodeID, len(tables))
	for _, id := range tables {
		if header := tableHeaderFor(t, id, linked); header != 0 {
			columns := appendColumnHeaders(t, nil, header, make(map[accessibility.NodeID]bool), 0)
			if len(columns) != 0 {
				headers[id] = columns
			}
		}
	}
	if len(headers) == 0 {
		return nil
	}
	return headers
}

// tableHeaderFor returns the id of the header panel describing a table's columns, or zero when there is none to be
// found. linked holds the header each table is named by, for the headers that name one. See [buildTableHeaders].
func tableHeaderFor(t *accessibility.Tree, id accessibility.NodeID,
	linked map[accessibility.NodeID]accessibility.NodeID,
) accessibility.NodeID {
	table := t.Node(id)
	if table == nil {
		return 0
	}
	for _, controlled := range table.Controls {
		if n := t.Node(controlled); n != nil && n.Role == role.TableHeader && !n.Ignored {
			return controlled
		}
	}
	if header, exists := linked[id]; exists {
		return header
	}
	ancestor := t.UnignoredParent(id)
	for depth := 0; ancestor != 0 && depth < maxHeaderSearchDepth; depth++ {
		if n := t.Node(ancestor); n != nil && supportsTable(n.Role) {
			// Another table contains this one, which a table nested in a cell really is. Its header describes its own
			// columns, and everything further out belongs to it as well, so the search ends here rather than climbing
			// out of the container and announcing the outer table's column names over the inner table's cells.
			return 0
		}
		headers, tables := headersAndTablesWithin(t, ancestor, make(map[accessibility.NodeID]bool), 0)
		if tables > 1 {
			return 0
		}
		if len(headers) == 1 {
			return headers[0]
		}
		ancestor = t.UnignoredParent(ancestor)
	}
	return 0
}

// headersAndTablesWithin returns the header panels within the subtree rooted at a node, along with how many tables it
// holds. Ignored nodes are walked through — a header can sit inside a layout panel — but an ignored node is never
// reported as a header, since an assistive technology is never shown one.
//
// visited holds the ids already descended into, and is what keeps a malformed tree from being scanned forever. The
// depth bound alone does not: a cycle that branches multiplies the work at every level rather than merely deepening it,
// so 512 levels of a tree whose nodes point back at each other is more work than any machine will finish, and this runs
// on the UI thread from [Adapter.Publish]. It is the same defense [accessibility.Tree.Walk], Tree.HitTest and
// Tree.UnignoredChildren carry, for the same reason. Counting each node once also keeps a table reachable by two paths
// from reading as two tables, which would end the caller's search rather than answering it.
func headersAndTablesWithin(t *accessibility.Tree, id accessibility.NodeID, visited map[accessibility.NodeID]bool,
	depth int,
) (headers []accessibility.NodeID, tables int) {
	n := t.Node(id)
	if n == nil || depth >= maxHeaderSearchDepth || visited[id] {
		return nil, 0
	}
	visited[id] = true
	switch {
	case n.Role == role.TableHeader:
		if !n.Ignored {
			headers = append(headers, id)
		}
		// A header's own subtree holds nothing but its column headers, so there is no reason to walk into it.
		return headers, 0
	case supportsTable(n.Role):
		// Likewise a table's subtree holds its rows and cells. Counting it and stopping also keeps a table nested
		// inside another table's cell from being counted twice.
		return nil, 1
	default:
	}
	for _, child := range n.Children {
		childHeaders, childTables := headersAndTablesWithin(t, child, visited, depth+1)
		headers = append(headers, childHeaders...)
		tables += childTables
	}
	return headers, tables
}

// appendColumnHeaders appends the ColumnHeader descendants of a node, in reading order. A column header holds nothing
// that is itself a column header, so the walk stops at each one it finds.
//
// visited holds the ids already reached, for the same reason [headersAndTablesWithin] carries one: a branching cycle
// would otherwise be walked forever, and a node reachable by two paths would be reported as two columns.
func appendColumnHeaders(t *accessibility.Tree, ids []accessibility.NodeID, id accessibility.NodeID,
	visited map[accessibility.NodeID]bool, depth int,
) []accessibility.NodeID {
	if depth >= maxHeaderSearchDepth || visited[id] {
		return ids
	}
	visited[id] = true
	for _, child := range t.UnignoredChildren(id) {
		n := t.Node(child)
		if n == nil || visited[child] {
			continue
		}
		if n.Role == role.ColumnHeader {
			visited[child] = true
			ids = append(ids, child)
			continue
		}
		ids = appendColumnHeaders(t, ids, child, visited, depth+1)
	}
	return ids
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

// rowAtChildIndex returns the described row at a child index of this table, or nil when the index names no child or
// names one that is not a row.
//
// The index AT-SPI's GetIndexAt hands out, and that GetRowAtIndex, GetColumnAtIndex and GetRowColumnExtentsAtIndex take
// back, is a child index of the table, so these four methods work over the table's children — which in a Unison table
// are its rows, since the cells are children of the rows rather than of the table. No index can name a cell here at
// all. The flattened row*columns+column index that ATK's own implementation hands out is not an option either: it names
// cells that are not children of anything this table reports, and a client that fed one back would be asking about an
// object the table does not have.
func (o *nodeObject) rowAtChildIndex(index int) *accessibility.Node {
	children := o.children()
	if index < 0 || index >= len(children) {
		return nil
	}
	n := o.data.node(children[index])
	if n == nil || n.Role != role.Row {
		return nil
	}
	return n
}

// childIndexOfRow returns the child index of the described row with the given row index, or -1 when the table is not
// describing that row, which is the case for most rows of a large table at any moment.
func (o *nodeObject) childIndexOfRow(row int) int {
	for i, id := range o.children() {
		if n := o.data.node(id); n != nil && n.Role == role.Row && n.RowIndex == row {
			return i
		}
	}
	return -1
}

// columnHeader returns the node describing a column of this table, or nil when there is none. The header whose own
// ColumnIndex matches is the answer; a snapshot that never filled those in — they would all read zero — is answered by
// position instead, which is right whenever the header panel publishes one element per column in column order.
//
// That fallback is offered only when no header carries a column index at all, which is the case it describes. A header
// panel whose elements do report indexes, but not one per column — say 0 and 2 of three columns — is answered with
// nothing for the columns it leaves out, rather than with whichever header happens to sit at that position: naming the
// third column's header over the second column would have an assistive technology read the wrong column name out as
// the user arrows across, which is worse than reading none.
func (o *nodeObject) columnHeader(col int) *accessibility.Node {
	if col < 0 || col >= o.node.ColumnCount {
		return nil
	}
	headers := o.data.columnHeaders(o.node.ID)
	indexed := false
	for _, id := range headers {
		n := o.data.node(id)
		if n == nil {
			continue
		}
		if n.ColumnIndex == col {
			return n
		}
		indexed = indexed || n.ColumnIndex != 0
	}
	if !indexed && col < len(headers) {
		return o.data.node(headers[col])
	}
	return nil
}

// columnHeaderCells returns the header describing the column this cell is in, as the list of at most one that
// org.a11y.atspi.TableCell.ColumnHeaderCells reports.
func (o *nodeObject) columnHeaderCells() []accessibility.NodeID {
	table := o.tableOf()
	if table == 0 {
		return nil
	}
	header := (&nodeObject{a: o.a, data: o.data, node: o.data.node(table)}).columnHeader(o.node.ColumnIndex)
	if header == nil {
		return nil
	}
	return []accessibility.NodeID{header.ID}
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

// getIndexAt implements org.a11y.atspi.Table.GetIndexAt, which answers with the child index of the row holding the cell
// at a row and column; see [nodeObject.rowAtChildIndex] for why it is the row rather than the cell. A row or column
// outside the table, and a row the table is not describing at the moment, have no index, which AT-SPI spells as -1.
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
	call.Reply(int32(o.childIndexOfRow(row)))
}

// getRowAtIndex implements org.a11y.atspi.Table.GetRowAtIndex, which answers with the row a child of this table
// occupies in the model, undoing what [nodeObject.getIndexAt] does.
func (o *nodeObject) getRowAtIndex(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	row := o.rowAtChildIndex(int(int32Arg(args, 0)))
	if row == nil {
		call.Reply(int32(-1))
		return
	}
	call.Reply(int32(row.RowIndex))
}

// getColumnAtIndex implements org.a11y.atspi.Table.GetColumnAtIndex. A child of this table is a whole row, so it starts
// at the first column; the cells within it say which column each of them is in through org.a11y.atspi.TableCell and the
// colindex attribute.
func (o *nodeObject) getColumnAtIndex(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	if o.rowAtChildIndex(int(int32Arg(args, 0))) == nil {
		call.Reply(int32(-1))
		return
	}
	call.Reply(int32(0))
}

// getColumnDescription implements org.a11y.atspi.Table.GetColumnDescription, which is what an assistive technology
// announces as the user arrows into a column. Unison has no description of a column beyond what its header is named.
func (o *nodeObject) getColumnDescription(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	header := o.columnHeader(int(int32Arg(args, 0)))
	if header == nil {
		call.Reply("")
		return
	}
	call.Reply(header.Name)
}

// getColumnHeader implements org.a11y.atspi.Table.GetColumnHeader.
func (o *nodeObject) getColumnHeader(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	header := o.columnHeader(int(int32Arg(args, 0)))
	if header == nil {
		call.Reply(nullReference())
		return
	}
	call.Reply(o.a.reference(header.ID))
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

// addRowSelection implements org.a11y.atspi.Table.AddRowSelection, which asks for a row to join the selection rather
// than to become it. The answer is optimistic, as every request is.
//
// A table that cannot hold more than one selected row refuses, which is ATK's contract for it. Replacing the selection
// instead would throw away the row the user had selected while telling the client that what it asked for was done, and
// a client that wanted the selection replaced has Selection.SelectChild to ask with.
func (o *nodeObject) addRowSelection(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	row := o.rowNode(int(int32Arg(args, 0)))
	if row == nil || !o.node.Multiselectable {
		call.Reply(false)
		return
	}
	call.Reply(o.requestOnChild(row, accessibility.AddToSelection))
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
// everything the child at an index has to say about where it is. That child is a row, so it begins at the first column
// and spans them all; see [nodeObject.rowAtChildIndex].
func (o *nodeObject) getRowColumnExtentsAtIndex(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	row := o.rowAtChildIndex(int(int32Arg(args, 0)))
	if row == nil {
		call.Reply(false, int32(0), int32(0), int32(0), int32(0), false)
		return
	}
	call.Reply(true, int32(row.RowIndex), int32(0), int32(cellSpan), int32(o.node.ColumnCount), row.Selected)
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
