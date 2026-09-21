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
	"strings"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/role"
	"github.com/richardwilkes/unison/internal/dbus"
)

// tableWindow is the window the table tests publish.
const tableWindow WindowKey = 4

// tableTree is the window the table tests work over. The table holds four rows and describes two of them, which is what
// a Unison table does: only the rows that can be seen, plus the selection and the row the focus is in, are described,
// however many the model holds. Its column headers sit in a panel of their own beside it, which is where a Unison
// layout puts them and which is why finding them takes a search; see [buildTableHeaders].
//
//	60 window "Books"                (0,0 300x200)  active
//	├─ 69 table header               (0,0 300x20)
//	│  ├─ 70 column header "Title"   (0,0 160x20)   column 0
//	│  └─ 71 column header "Count"   (160,0 140x20) column 1
//	└─ 61 table "Ledger"             (0,0 300x200)  4 rows, 2 columns, multi-select
//	   ├─ 62 row "Alpha"             (0,20 300x20)  row 1, selected
//	   │  ├─ 63 disclosure triangle  (0,20 20x20)
//	   │  ├─ 64 cell "Alpha"         (20,20 140x20) row 1, column 0
//	   │  └─ 65 cell "10"            (160,20 140x20) row 1, column 1
//	   └─ 66 row "Beta"              (0,40 300x20)  row 2
//	      ├─ 67 cell "Beta"          (0,40 160x20)  row 2, column 0
//	      └─ 68 cell "20"            (160,40 140x20) row 2, column 1
func tableTree() *accessibility.Tree {
	return treeOf(1,
		&accessibility.Node{
			ID: 60, Role: role.Window, Name: "Books", Focused: true, Bounds: geom.NewRect(0, 0, 300, 200),
			Children: []accessibility.NodeID{69, 61},
		},
		&accessibility.Node{
			ID: 69, Parent: 60, Role: role.TableHeader, Bounds: geom.NewRect(0, 0, 300, 20),
			Children: []accessibility.NodeID{70, 71},
		},
		&accessibility.Node{
			ID: 70, Parent: 69, Role: role.ColumnHeader, Name: "Title", Bounds: geom.NewRect(0, 0, 160, 20),
		},
		&accessibility.Node{
			ID: 71, Parent: 69, Role: role.ColumnHeader, Name: "Count", ColumnIndex: 1,
			Bounds: geom.NewRect(160, 0, 140, 20),
		},
		&accessibility.Node{
			ID: 61, Parent: 60, Role: role.Table, Name: "Ledger", Multiselectable: true, RowCount: 4, ColumnCount: 2,
			Bounds: geom.NewRect(0, 0, 300, 200), Children: []accessibility.NodeID{62, 66},
		},
		&accessibility.Node{
			ID: 62, Parent: 61, Role: role.Row, Name: "Alpha", RowIndex: 1, Level: 1, Selectable: true, Selected: true,
			Bounds: geom.NewRect(0, 20, 300, 20), Children: []accessibility.NodeID{63, 64, 65},
			Actions: selectionActions(),
		},
		&accessibility.Node{
			ID: 63, Parent: 62, Role: role.DisclosureTriangle, Name: "Disclosure Triangle", RowIndex: 1,
			Bounds: geom.NewRect(0, 20, 20, 20),
		},
		&accessibility.Node{
			ID: 64, Parent: 62, Role: role.Cell, Name: "Alpha", RowIndex: 1, Bounds: geom.NewRect(20, 20, 140, 20),
		},
		&accessibility.Node{
			ID: 65, Parent: 62, Role: role.Cell, Name: "10", RowIndex: 1, ColumnIndex: 1,
			Bounds: geom.NewRect(160, 20, 140, 20),
		},
		&accessibility.Node{
			ID: 66, Parent: 61, Role: role.Row, Name: "Beta", RowIndex: 2, Level: 1, Selectable: true,
			Bounds: geom.NewRect(0, 40, 300, 20), Children: []accessibility.NodeID{67, 68},
			Actions: selectionActions(),
		},
		&accessibility.Node{
			ID: 67, Parent: 66, Role: role.Cell, Name: "Beta", RowIndex: 2, Bounds: geom.NewRect(0, 40, 160, 20),
		},
		&accessibility.Node{
			ID: 68, Parent: 66, Role: role.Cell, Name: "20", RowIndex: 2, ColumnIndex: 1,
			Bounds: geom.NewRect(160, 40, 140, 20),
		},
	)
}

// newTableAdapter starts an adapter and publishes the table window, leaving the signals both publishes sent unread,
// since none of the table tests are about them.
func newTableAdapter(t *testing.T) *testAdapter {
	t.Helper()
	ta := newTestAdapter(t)
	ta.Publish(tableWindow, tableTree(), nil, sampleGeometry())
	return ta
}

func TestTableInterfacesAreOnlyThereForGrids(t *testing.T) {
	t.Parallel()
	ta := newTableAdapter(t)
	c := ta.c
	c.Equal([]string{InterfaceAccessible, InterfaceCollection, InterfaceComponent, InterfaceSelection, InterfaceTable},
		ta.one(NodePath(61), InterfaceAccessible, "GetInterfaces", ""))
	c.Equal([]string{InterfaceAccessible, InterfaceCollection, InterfaceComponent},
		ta.one(NodePath(62), InterfaceAccessible, "GetInterfaces", ""),
		"a row holds the cells rather than being one of them")
	c.Equal([]string{InterfaceAccessible, InterfaceCollection, InterfaceComponent, InterfaceTableCell},
		ta.one(NodePath(65), InterfaceAccessible, "GetInterfaces", ""))
	c.Equal(dbus.UnknownInterface, ta.errorName(NodePath(5), dbusPropertiesInterface, getMember, "ss", InterfaceTable,
		"NRows"), "a list box is not a grid")
	c.Equal(dbus.UnknownInterface, ta.errorName(NodePath(6), InterfaceTableCell, "GetRowColumnSpan", ""),
		"neither is one of its items")
}

func TestTableShape(t *testing.T) {
	t.Parallel()
	ta := newTableAdapter(t)
	c := ta.c
	c.Equal(int32(4), ta.peer.getProperty(NodePath(61), InterfaceTable, "NRows"),
		"the whole model rather than the rows being described")
	c.Equal(int32(2), ta.peer.getProperty(NodePath(61), InterfaceTable, "NColumns"))
	c.Equal(nullReference(), ta.peer.getProperty(NodePath(61), InterfaceTable, "Caption"))
	c.Equal(nullReference(), ta.peer.getProperty(NodePath(61), InterfaceTable, "Summary"))

	c.Equal(nodeRef(64), ta.one(NodePath(61), InterfaceTable, "GetAccessibleAt", "ii", int32(1), int32(0)))
	c.Equal(nodeRef(65), ta.one(NodePath(61), InterfaceTable, "GetAccessibleAt", "ii", int32(1), int32(1)))
	c.Equal(nodeRef(67), ta.one(NodePath(61), InterfaceTable, "GetAccessibleAt", "ii", int32(2), int32(0)))
	c.Equal(nullReference(), ta.one(NodePath(61), InterfaceTable, "GetAccessibleAt", "ii", int32(0), int32(0)),
		"a row the table is not describing has no object to hand back")
	c.Equal(nullReference(), ta.one(NodePath(61), InterfaceTable, "GetAccessibleAt", "ii", int32(1), int32(9)))

	// The index these four methods work in is a child index of the table, and a Unison table's children are its rows:
	// Orca feeds a cell's GetIndexInParent straight into GetRowAtIndex, so an index naming anything else sends it to
	// the wrong row. Row 1 is the first of the two the table is describing, so it is child 0.
	c.Equal(int32(0), ta.one(NodePath(61), InterfaceTable, "GetIndexAt", "ii", int32(1), int32(1)))
	c.Equal(int32(1), ta.one(NodePath(61), InterfaceTable, "GetIndexAt", "ii", int32(2), int32(0)))
	c.Equal(int32(1), ta.one(NodePath(61), InterfaceTable, "GetRowAtIndex", "i", int32(0)))
	c.Equal(int32(2), ta.one(NodePath(61), InterfaceTable, "GetRowAtIndex", "i", int32(1)))
	c.Equal(int32(0), ta.one(NodePath(61), InterfaceTable, "GetColumnAtIndex", "i", int32(1)),
		"a child of the table is a whole row, so it starts at the first column")
	c.Equal(nodeRef(62), ta.one(NodePath(61), InterfaceAccessible, "GetChildAtIndex", "i", int32(0)),
		"and the index really is one the table's own children answer to")
	for _, one := range []struct {
		member string
		sig    dbus.Signature
		args   []any
	}{
		{member: "GetIndexAt", sig: "ii", args: []any{int32(-1), int32(0)}},
		{member: "GetIndexAt", sig: "ii", args: []any{int32(0), int32(2)}},
		{member: "GetIndexAt", sig: "ii", args: []any{int32(0), int32(0)}},
		{member: "GetRowAtIndex", sig: "i", args: []any{int32(8)}},
		{member: "GetColumnAtIndex", sig: "i", args: []any{int32(-1)}},
		{member: "GetRowExtentAt", sig: "ii", args: []any{int32(9), int32(0)}},
		{member: "GetColumnExtentAt", sig: "ii", args: []any{int32(0), int32(9)}},
	} {
		c.Equal(int32(-1), ta.one(NodePath(61), InterfaceTable, one.member, one.sig, one.args...),
			"%s is being asked about something the table is not describing", one.member)
	}
	// Nothing Unison reports spans more than one row or column.
	c.Equal(int32(1), ta.one(NodePath(61), InterfaceTable, "GetRowExtentAt", "ii", int32(1), int32(0)))
	c.Equal(int32(1), ta.one(NodePath(61), InterfaceTable, "GetColumnExtentAt", "ii", int32(1), int32(0)))

	c.Equal("Alpha", ta.one(NodePath(61), InterfaceTable, "GetRowDescription", "i", int32(1)))
	c.Equal("", ta.one(NodePath(61), InterfaceTable, "GetRowDescription", "i", int32(0)))
	// Unison tables have no row headers at all, whatever they have in the way of column ones.
	c.Equal(nullReference(), ta.one(NodePath(61), InterfaceTable, "GetRowHeader", "i", int32(1)))
}

// TestTableColumnHeaders covers the one thing that makes a table readable as a table: the name of the column the user
// has arrowed into. A Unison table's headers sit in a panel beside it rather than inside it, so every route to them —
// the table's own GetColumnHeader and GetColumnDescription, and the cell's ColumnHeaderCells — goes through the same
// search, which is the one the other two adapters make.
func TestTableColumnHeaders(t *testing.T) {
	t.Parallel()
	ta := newTableAdapter(t)
	c := ta.c
	c.Equal(nodeRef(70), ta.one(NodePath(61), InterfaceTable, "GetColumnHeader", "i", int32(0)))
	c.Equal(nodeRef(71), ta.one(NodePath(61), InterfaceTable, "GetColumnHeader", "i", int32(1)))
	c.Equal(nullReference(), ta.one(NodePath(61), InterfaceTable, "GetColumnHeader", "i", int32(2)),
		"the table has no third column")
	c.Equal(nullReference(), ta.one(NodePath(61), InterfaceTable, "GetColumnHeader", "i", int32(-1)))
	c.Equal("Title", ta.one(NodePath(61), InterfaceTable, "GetColumnDescription", "i", int32(0)))
	c.Equal("Count", ta.one(NodePath(61), InterfaceTable, "GetColumnDescription", "i", int32(1)))
	c.Equal("", ta.one(NodePath(61), InterfaceTable, "GetColumnDescription", "i", int32(9)))
	c.Equal([]dbus.ObjectRef{nodeRef(70)},
		ta.peer.getProperty(NodePath(64), InterfaceTableCell, "ColumnHeaderCells"))
	c.Equal([]dbus.ObjectRef{nodeRef(71)},
		ta.peer.getProperty(NodePath(65), InterfaceTableCell, "ColumnHeaderCells"),
		"a cell reports the header of its own column")
	c.Equal([]dbus.ObjectRef{}, ta.peer.getProperty(NodePath(65), InterfaceTableCell, "RowHeaderCells"),
		"Unison tables have no row headers")

	// A header the table names, or that names the table, is used however the two are laid out, which is what an
	// explicit link is for.
	linked := tableTree()
	linked.Generation++
	linked.Node(60).Children = []accessibility.NodeID{61}
	linked.Node(61).Children = append(linked.Node(61).Children, 69)
	linked.Node(69).Parent = 61
	linked.Node(69).Controls = []accessibility.NodeID{61}
	ta.Publish(tableWindow, linked, nil, sampleGeometry())
	c.Equal(nodeRef(71), ta.one(NodePath(61), InterfaceTable, "GetColumnHeader", "i", int32(1)))

	// Two tables under one ancestor and no link between any of them is an ambiguity that cannot be resolved, and
	// guessing would announce one table's column names over the other's cells.
	ambiguous := tableTree()
	ambiguous.Generation += 2
	ambiguous.Node(60).Children = []accessibility.NodeID{69, 61, 72}
	ambiguous.Nodes[72] = &accessibility.Node{
		ID: 72, Parent: 60, Role: role.Table, Name: "Other", RowCount: 1, ColumnCount: 1,
		Bounds: geom.NewRect(0, 100, 300, 100),
	}
	ta.Publish(tableWindow, ambiguous, nil, sampleGeometry())
	c.Equal(nullReference(), ta.one(NodePath(61), InterfaceTable, "GetColumnHeader", "i", int32(0)))
	c.Equal("", ta.one(NodePath(61), InterfaceTable, "GetColumnDescription", "i", int32(0)))
}

// TestTableColumnHeadersWithGapsInTheirIndexes covers a header panel that reports column indexes, but not one per
// column. Answering such a column with the header that happens to sit at that position hands an assistive technology
// the wrong column name, which it reads out as the user arrows across; the positional answer is only ever right for a
// header panel that fills no indexes in at all.
func TestTableColumnHeadersWithGapsInTheirIndexes(t *testing.T) {
	t.Parallel()
	ta := newTableAdapter(t)
	c := ta.c
	gapped := tableTree()
	gapped.Generation++
	gapped.Node(61).ColumnCount = 3
	gapped.Node(71).ColumnIndex = 2
	ta.Publish(tableWindow, gapped, nil, sampleGeometry())
	c.Equal(nodeRef(70), ta.one(NodePath(61), InterfaceTable, "GetColumnHeader", "i", int32(0)))
	c.Equal(nodeRef(71), ta.one(NodePath(61), InterfaceTable, "GetColumnHeader", "i", int32(2)))
	c.Equal(nullReference(), ta.one(NodePath(61), InterfaceTable, "GetColumnHeader", "i", int32(1)),
		"the header panel names no header for the second column, and the third column's is not it")
	c.Equal("", ta.one(NodePath(61), InterfaceTable, "GetColumnDescription", "i", int32(1)))
	c.Equal([]dbus.ObjectRef{}, ta.peer.getProperty(NodePath(65), InterfaceTableCell, "ColumnHeaderCells"),
		"a cell in the column with no header reports none")

	// A header panel that fills no column index in at all is still answered by position, which is what the headers of
	// a table built the ordinary way look like before anything has been said about which column each one describes.
	positional := tableTree()
	positional.Generation += 2
	positional.Node(71).ColumnIndex = 0
	ta.Publish(tableWindow, positional, nil, sampleGeometry())
	c.Equal(nodeRef(70), ta.one(NodePath(61), InterfaceTable, "GetColumnHeader", "i", int32(0)))
	c.Equal(nodeRef(71), ta.one(NodePath(61), InterfaceTable, "GetColumnHeader", "i", int32(1)))
	c.Equal("Count", ta.one(NodePath(61), InterfaceTable, "GetColumnDescription", "i", int32(1)))
}

// TestTableHeaderSearchOfACyclicTree covers the defense [maxHeaderSearchDepth] claims to be: a tree whose nodes point
// back at each other. The depth bound alone does not provide it, since a cycle that branches multiplies the work at
// every level rather than merely deepening it, and [buildTableHeaders] runs on the UI thread from [Adapter.Publish], so
// a search that does not end is a hung application rather than a slow one. Both recursions the search makes are wound
// through a cycle here: the walk that looks for the header panel, and the scan of the panel that was found.
func TestTableHeaderSearchOfACyclicTree(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	tree := treeOf(1,
		&accessibility.Node{
			ID: 1, Role: role.Window, Name: "Cycles", Bounds: geom.NewRect(0, 0, 300, 200),
			Children: []accessibility.NodeID{2, 5, 6},
		},
		// 2, 3 and 4 are ordinary layout panels that hold each other: the walk reaches 2 from the window, 3 and 4 from
		// 2, and 2 again from each of them.
		&accessibility.Node{
			ID: 2, Parent: 1, Role: role.Group, Bounds: geom.NewRect(0, 0, 300, 20),
			Children: []accessibility.NodeID{3, 4},
		},
		&accessibility.Node{
			ID: 3, Parent: 2, Role: role.Group, Bounds: geom.NewRect(0, 0, 150, 20),
			Children: []accessibility.NodeID{2},
		},
		&accessibility.Node{
			ID: 4, Parent: 2, Role: role.Group, Bounds: geom.NewRect(150, 0, 150, 20),
			Children: []accessibility.NodeID{2},
		},
		&accessibility.Node{
			ID: 5, Parent: 1, Role: role.Table, Name: "Tangle", RowCount: 1, ColumnCount: 1,
			Bounds: geom.NewRect(0, 20, 300, 180),
		},
		// The header panel holds a cycle of its own, which is the second recursion.
		&accessibility.Node{
			ID: 6, Parent: 1, Role: role.TableHeader, Bounds: geom.NewRect(0, 0, 300, 20),
			Children: []accessibility.NodeID{7, 8},
		},
		&accessibility.Node{
			ID: 7, Parent: 6, Role: role.Group, Bounds: geom.NewRect(0, 0, 150, 20),
			Children: []accessibility.NodeID{9, 8},
		},
		&accessibility.Node{
			ID: 8, Parent: 6, Role: role.Group, Bounds: geom.NewRect(150, 0, 150, 20),
			Children: []accessibility.NodeID{7},
		},
		&accessibility.Node{
			ID: 9, Parent: 7, Role: role.ColumnHeader, Name: "Title", Bounds: geom.NewRect(0, 0, 150, 20),
		},
	)
	done := make(chan map[accessibility.NodeID][]accessibility.NodeID, 1)
	go func() { done <- buildTableHeaders(tree) }()
	select {
	case headers := <-done:
		c.Equal(map[accessibility.NodeID][]accessibility.NodeID{5: {9}}, headers,
			"the one header of the one table is still found, and is reported once")
	case <-time.After(testTimeout):
		t.Fatal("the header search did not end: a cycle in the tree must not be walked forever")
	}
}

func TestTableRowColumnExtentsAtIndex(t *testing.T) {
	t.Parallel()
	ta := newTableAdapter(t)
	c := ta.c
	// The index is a child index, so what it names is a row: it begins at the first column and spans them all.
	c.Equal([]any{true, int32(1), int32(0), int32(1), int32(2), true},
		ta.values(NodePath(61), InterfaceTable, "GetRowColumnExtentsAtIndex", "i", int32(0)))
	c.Equal([]any{true, int32(2), int32(0), int32(1), int32(2), false},
		ta.values(NodePath(61), InterfaceTable, "GetRowColumnExtentsAtIndex", "i", int32(1)))
	c.Equal([]any{false, int32(0), int32(0), int32(0), int32(0), false},
		ta.values(NodePath(61), InterfaceTable, "GetRowColumnExtentsAtIndex", "i", int32(99)))
}

func TestTableSelection(t *testing.T) {
	t.Parallel()
	ta := newTableAdapter(t)
	c := ta.c
	c.Equal(int32(1), ta.peer.getProperty(NodePath(61), InterfaceTable, "NSelectedRows"))
	c.Equal([]int32{1}, ta.one(NodePath(61), InterfaceTable, "GetSelectedRows", ""))
	c.Equal(true, ta.one(NodePath(61), InterfaceTable, "IsRowSelected", "i", int32(1)))
	c.Equal(false, ta.one(NodePath(61), InterfaceTable, "IsRowSelected", "i", int32(2)))
	c.Equal(false, ta.one(NodePath(61), InterfaceTable, "IsRowSelected", "i", int32(0)),
		"a row the table is not describing cannot be reported as selected")
	c.Equal(true, ta.one(NodePath(61), InterfaceTable, "IsSelected", "ii", int32(1), int32(0)),
		"a cell is selected when its row is: Unison selects whole rows")
	c.Equal(false, ta.one(NodePath(61), InterfaceTable, "IsSelected", "ii", int32(1), int32(9)))
	c.Equal(false, ta.one(NodePath(61), InterfaceTable, "IsSelected", "ii", int32(2), int32(0)))

	// Unison has no notion of a selected column, so nothing about one is reported and nothing can be done to one.
	c.Equal(int32(0), ta.peer.getProperty(NodePath(61), InterfaceTable, "NSelectedColumns"))
	c.Equal([]int32{}, ta.one(NodePath(61), InterfaceTable, "GetSelectedColumns", ""))
	c.Equal(false, ta.one(NodePath(61), InterfaceTable, "IsColumnSelected", "i", int32(0)))
	c.Equal(false, ta.one(NodePath(61), InterfaceTable, "AddColumnSelection", "i", int32(0)))
	c.Equal(false, ta.one(NodePath(61), InterfaceTable, "RemoveColumnSelection", "i", int32(0)))
	ta.noRequest(t)

	// The table allows more than one selected row, so asking for another one adds to the selection.
	c.Equal(true, ta.one(NodePath(61), InterfaceTable, "AddRowSelection", "i", int32(2)))
	c.Equal(accessibility.ActionRequest{Node: 66, Action: accessibility.AddToSelection}, ta.nextRequest(t))
	c.Equal(true, ta.one(NodePath(61), InterfaceTable, "RemoveRowSelection", "i", int32(1)))
	c.Equal(accessibility.ActionRequest{Node: 62, Action: accessibility.RemoveFromSelection}, ta.nextRequest(t))
	c.Equal(false, ta.one(NodePath(61), InterfaceTable, "AddRowSelection", "i", int32(0)),
		"a row with no object cannot be asked to do anything")
	c.Equal(false, ta.one(NodePath(61), InterfaceTable, "RemoveRowSelection", "i", int32(3)))
	ta.noRequest(t)

	// A table that cannot hold more than one selected row refuses to add to its selection, which is ATK's contract for
	// it. Replacing the selection instead would throw away the row the user had selected and report success.
	single := tableTree()
	single.Generation++
	single.Node(61).Multiselectable = false
	ta.Publish(tableWindow, single, nil, sampleGeometry())
	c.Equal(false, ta.one(NodePath(61), InterfaceTable, "AddRowSelection", "i", int32(2)))
	ta.noRequest(t)
	c.Equal(true, ta.one(NodePath(61), InterfaceSelection, "SelectChild", "i", int32(1)),
		"a client that means to replace the selection has SelectChild to ask with")
	c.Equal(accessibility.ActionRequest{Node: 66, Action: accessibility.Select}, ta.nextRequest(t))
}

func TestTableCellInterface(t *testing.T) {
	t.Parallel()
	ta := newTableAdapter(t)
	c := ta.c
	c.Equal(dbus.Struct{int32(1), int32(1)}, ta.peer.getProperty(NodePath(65), InterfaceTableCell, "Position"))
	c.Equal(dbus.Struct{int32(2), int32(0)}, ta.peer.getProperty(NodePath(67), InterfaceTableCell, "Position"))
	c.Equal(int32(1), ta.peer.getProperty(NodePath(65), InterfaceTableCell, "RowSpan"))
	c.Equal(int32(1), ta.peer.getProperty(NodePath(65), InterfaceTableCell, "ColumnSpan"))
	c.Equal(nodeRef(61), ta.peer.getProperty(NodePath(65), InterfaceTableCell, "Table"),
		"the table is the cell's grandparent, since the row sits between them")
	// The header lists are covered by TestTableColumnHeaders; here it is enough that the cell reports one column header
	// and no row header at all.
	c.Equal([]dbus.ObjectRef{nodeRef(71)},
		ta.peer.getProperty(NodePath(65), InterfaceTableCell, "ColumnHeaderCells"))
	c.Equal([]dbus.ObjectRef{}, ta.peer.getProperty(NodePath(65), InterfaceTableCell, "RowHeaderCells"))
	c.Equal([]any{true, int32(1), int32(1), int32(1), int32(1)},
		ta.values(NodePath(65), InterfaceTableCell, "GetRowColumnSpan", ""))
}

func TestTablePositionAttributes(t *testing.T) {
	t.Parallel()
	ta := newTableAdapter(t)
	c := ta.c
	// An assistive technology that has not asked for the table interfaces still has to be able to say "row 2 of 4".
	c.Equal(dbus.Dict{
		{Key: toolkitAttribute, Value: toolkitName},
		{Key: levelAttribute, Value: "1"},
		{Key: rowIndexAttribute, Value: "2"},
		{Key: positionInSetAttribute, Value: "2"},
		{Key: setSizeAttribute, Value: "4"},
	}, ta.one(NodePath(62), InterfaceAccessible, "GetAttributes", ""))
	c.Equal(dbus.Dict{
		{Key: toolkitAttribute, Value: toolkitName},
		{Key: rowIndexAttribute, Value: "2"},
		{Key: columnIndexAttribute, Value: "2"},
	}, ta.one(NodePath(65), InterfaceAccessible, "GetAttributes", ""))
}

func TestTableIntrospection(t *testing.T) {
	t.Parallel()
	ta := newTableAdapter(t)
	c := ta.c
	for _, one := range []struct {
		path     dbus.ObjectPath
		expected []string
	}{
		{
			path: NodePath(61),
			expected: []string{
				InterfaceTable,
				`<property name="NRows" type="i" access="read"/>`,
				`<method name="GetAccessibleAt">`,
				`<arg type="b" direction="out"/>`,
			},
		},
		{
			path: NodePath(65),
			expected: []string{
				InterfaceTableCell,
				`<property name="Position" type="(ii)" access="read"/>`,
				`<method name="GetRowColumnSpan">`,
			},
		},
	} {
		xml, ok := ta.one(one.path, "org.freedesktop.DBus.Introspectable", "Introspect", "").(string)
		c.True(ok)
		for _, want := range one.expected {
			c.True(strings.Contains(xml, want), "the introspection of %s must mention %s", one.path, want)
		}
		// What the node says it implements has to be what is there, in the same order, so that a client that walks the
		// introspection and one that asks the shorter question see the same object. See
		// TestTheObjectsAgreeWithWhatIsAdvertised, which covers the rest of the roles.
		advertised, ok := ta.one(one.path, InterfaceAccessible, "GetInterfaces", "").([]string)
		c.True(ok)
		c.Equal(advertised, atspiInterfacesIn(xml), "%s advertises interfaces it does not implement", one.path)
	}
}

// atspiInterfacesIn returns the AT-SPI interfaces an introspection document declares, in the order it declares them.
func atspiInterfacesIn(xml string) []string {
	var found []string
	for _, line := range strings.Split(xml, "\n") {
		name, ok := strings.CutPrefix(strings.TrimSpace(line), `<interface name="`)
		if !ok {
			continue
		}
		if name, _, ok = strings.Cut(name, `"`); ok && strings.HasPrefix(name, "org.a11y.") {
			found = append(found, name)
		}
	}
	return found
}

// tableCursorTree returns the table window with the keyboard focus where a Unison table puts it: on the row the person
// is on, or, once they have moved the cell cursor out along that row, on one of its cells. focus names the row or the
// cell holding it. generation is how many publishes have gone before, so that each snapshot is newer than the last.
//
// Every row and every cell can take the focus and offers the request that moves the person to it, which is what a table
// publishes whether or not the cursor is out at the moment; only where the focus is reported changes. The row the focus
// is in is the selected one, since a cell cursor never exists without a selected lead row.
func tableCursorTree(generation uint64, focus accessibility.NodeID) *accessibility.Tree {
	tree := tableTree()
	tree.Generation += generation
	tree.Node(61).Focusable = true
	for _, id := range []accessibility.NodeID{62, 66} {
		row := tree.Node(id)
		row.Focusable = true
		row.Actions = row.Actions.With(accessibility.Focus)
		row.Focused = false
		row.Selected = false
	}
	for _, id := range []accessibility.NodeID{64, 65, 67, 68} {
		cell := tree.Node(id)
		cell.Focusable = true
		cell.Actions = cell.Actions.With(accessibility.ScrollIntoView, accessibility.Focus)
	}
	node := tree.Node(focus)
	node.Focused = true
	tree.Focus = focus
	row := node
	if node.Role == role.Cell {
		row = tree.Node(node.Parent)
	}
	row.Selected = true
	return tree
}

// drainSignals reads everything the publishes so far have sent, which is what a test that is only about the next
// publish wants. The announcement is a marker: it is queued behind them all, so reaching it means they have all been
// read.
func (ta *testAdapter) drainSignals(marker string) {
	ta.Announce(marker)
	for ta.peer.nextSignal().member != signalAnnouncement {
		// Everything sent before the marker is passed over.
	}
}

// TestTheCellTheCursorIsOnIsTheFocus covers what a client is handed when the person has moved the cell cursor out along
// a row: the cell says it can take the focus and holds it, the row it sits in says it can take the focus and does not,
// and the cell answers the half of the table protocol that says where it is. Orca reads all of this off the object
// after its locus of focus has been moved there, so a cell that says nothing about its place would be presented with no
// idea which column the person had arrowed onto.
func TestTheCellTheCursorIsOnIsTheFocus(t *testing.T) {
	t.Parallel()
	ta := newTestAdapter(t)
	c := ta.c
	ta.Publish(tableWindow, tableCursorTree(0, 65), nil, sampleGeometry())

	c.True(ta.statesOf(65).Has(StateFocusable), "the cell the cursor is on can take the focus")
	c.True(ta.statesOf(65).Has(StateFocused), "and holds it")
	c.True(ta.statesOf(64).Has(StateFocusable), "so can the cell beside it")
	c.False(ta.statesOf(64).Has(StateFocused))
	c.True(ta.statesOf(62).Has(StateFocusable), "the row is still something the focus can be given to")
	c.False(ta.statesOf(62).Has(StateFocused), "but the focus it holds is reported on the cell")
	c.True(ta.statesOf(62).Has(StateSelected), "the row the cursor is in is the selected one")
	c.False(ta.statesOf(61).Has(StateFocused))

	c.Equal([]string{InterfaceAccessible, InterfaceCollection, InterfaceComponent, InterfaceTableCell},
		ta.one(NodePath(65), InterfaceAccessible, "GetInterfaces", ""),
		"moving to a cell and scrolling it into view are asked for through Component, not through Action")
	c.Equal(true, ta.one(NodePath(65), InterfaceComponent, "GrabFocus", ""),
		"a cell the person can be moved to accepts being asked to take the focus")
	c.Equal(accessibility.ActionRequest{Node: 65, Action: accessibility.Focus}, ta.nextRequest(t))
	c.Equal(dbus.Struct{int32(1), int32(1)}, ta.peer.getProperty(NodePath(65), InterfaceTableCell, "Position"),
		"the cell says which row and column it is in")
	c.Equal(nodeRef(61), ta.peer.getProperty(NodePath(65), InterfaceTableCell, "Table"))
	c.Equal(nodeRefs(71), ta.peer.getProperty(NodePath(65), InterfaceTableCell, "ColumnHeaderCells"),
		"and which header describes the column, which is what Orca announces as the cursor crosses into it")
	c.Equal(nodeRef(65), ta.one(NodePath(61), InterfaceTable, "GetAccessibleAt", "ii", int32(1), int32(1)),
		"and the table finds the same cell from the other end")
}

// TestTheCellCursorMovingIsAFocusMoveOntoEachCell covers the three moves the cursor makes: out of the row onto a cell,
// across to the next cell of the same row, and down to a cell of another row. Each is the plain pair of signals that
// says the focus left one object and arrived at another, since that is what Orca follows; the row that had the focus
// gives up the state as the first cell takes it, and the selection moves with the cursor when it changes rows.
func TestTheCellCursorMovingIsAFocusMoveOntoEachCell(t *testing.T) {
	t.Parallel()
	ta := newTestAdapter(t)
	c := ta.c
	onRow := tableCursorTree(0, 62)
	ta.Publish(tableWindow, onRow, nil, sampleGeometry())
	ta.drainSignals("The table is up")

	// Right at row level puts the cursor on the first cell of the row.
	firstCell := tableCursorTree(1, 64)
	events := accessibility.Diff(onRow, firstCell)
	c.Equal([]accessibility.Event{{Kind: accessibility.FocusChanged, Node: 64}}, events,
		"the cursor moving out along the row is nothing but a focus move")
	ta.Publish(tableWindow, firstCell, events, sampleGeometry())
	c.Equal([]signalRecord{
		stateEvent(62, stateNameFocused, false),
		stateEvent(64, stateNameFocused, true),
		focusEvent(64),
	}, ta.peer.nextSignals(3))

	// Right again moves it to the next column of the same row, which is a different object each time, since a cell's
	// node id is its own.
	nextCell := tableCursorTree(2, 65)
	events = accessibility.Diff(firstCell, nextCell)
	ta.Publish(tableWindow, nextCell, events, sampleGeometry())
	c.Equal([]signalRecord{
		stateEvent(64, stateNameFocused, false),
		stateEvent(65, stateNameFocused, true),
		focusEvent(65),
	}, ta.peer.nextSignals(3))

	// Down keeps the column and takes the selection to the row below, so the two rows say what happened to them and the
	// table is told its selection moved, all around the same focus move.
	rowBelow := tableCursorTree(3, 68)
	events = accessibility.Diff(nextCell, rowBelow)
	ta.Publish(tableWindow, rowBelow, events, sampleGeometry())
	c.Equal([]signalRecord{
		stateEvent(62, stateNameSelected, false),
		stateEvent(66, stateNameSelected, true),
		stateEvent(65, stateNameFocused, false),
		stateEvent(68, stateNameFocused, true),
		focusEvent(68),
		objectEvent(61, signalSelectionChanged, "", 0, 0, variantInt32(0)),
	}, ta.peer.nextSignals(6))
	ta.Announce("Nothing more about the cell cursor")
	c.Equal(signalAnnouncement, ta.peer.nextSignal().member)
}

// TestALargeTableNamesTheFocusedCellAsItsCurrentDescendant covers the one route left to a client that has been told not
// to walk or cache what is inside a table: the cell the cursor is on is named as the table's current descendant, both
// in the signal and to a client that asks instead of listening. Orca moves its locus of focus straight onto whatever
// the event carries, so a cell is as good an answer here as a row.
func TestALargeTableNamesTheFocusedCellAsItsCurrentDescendant(t *testing.T) {
	t.Parallel()
	ta := newTestAdapter(t)
	c := ta.c
	large := tableCursorTree(0, 64)
	large.Node(61).RowCount = manageDescendantsRowThreshold + 100
	ta.Publish(tableWindow, large, nil, sampleGeometry())
	ta.drainSignals("The large table is up")
	c.True(ManagesDescendants(large.Node(61)), "a table this large manages its own descendants")
	c.Equal(ta.reference(64), ta.one(NodePath(61), InterfaceCollection, "GetActiveDescendant", ""),
		"the cell the cursor is on is the current descendant")

	moved := tableCursorTree(1, 65)
	moved.Node(61).RowCount = manageDescendantsRowThreshold + 100
	events := accessibility.Diff(large, moved)
	ta.Publish(tableWindow, moved, events, sampleGeometry())
	c.Equal([]signalRecord{
		stateEvent(64, stateNameFocused, false),
		stateEvent(65, stateNameFocused, true),
		focusEvent(65),
		// The index is the cell's place among its own parent's children, which is what ATK's bridge sends: the
		// disclosure triangle is the row's first child, so the second column's cell is its third.
		objectEvent(61, signalActiveDescendantChanged, "", 2, 0, variantRef(nodeRef(65))),
	}, ta.peer.nextSignals(4))
	ta.Announce("Nothing more about the current descendant")
	c.Equal(signalAnnouncement, ta.peer.nextSignal().member)
	c.Equal(ta.reference(65), ta.one(NodePath(61), InterfaceCollection, "GetActiveDescendant", ""))
}

// wholeRowName is what a Unison row is called once every column has been folded into it: the first column on its own,
// then each further column announced with the title of its header.
const wholeRowName = "Alpha, Count 10"

// TestARowIsNamedByTheWholeRowAndNothingIsInventedForACellsContent covers the naming half of the cell cursor, which is
// the half that decides how often the person hears a row read out.
//
// The row's name is published exactly as the table composed it. Orca's speech_generator._generate_table_row reads a
// row's name and its position and never walks the row's cells, so the whole row is spoken once as the person arrows
// down the table; and generator._combine_cell_results presents a named, non-layout row as the row object itself rather
// than as its cells whenever the row has changed, so arrowing down at cell level says the same thing rather than a
// second, cell-by-cell version of it. Neither would hold if the row were left nameless, which is why nothing here
// strips or shortens the name and why the row carries no text of its own for a client to read a second copy from.
//
// A cell whose content is described within it keeps the empty name the table gave it. The other two adapters name such
// a cell from its content; this one must not, since Orca takes the words from the descendant holding them
// (generator._generate_real_active_descendant_displayed_text, ax_utilities.active_descendant) and a name here would be
// a second copy of the same words.
func TestARowIsNamedByTheWholeRowAndNothingIsInventedForACellsContent(t *testing.T) {
	t.Parallel()
	ta := newTestAdapter(t)
	c := ta.c
	const countText = "10"
	tree := tableCursorTree(0, 65)
	tree.Node(62).Name = wholeRowName
	described := tree.Node(65)
	described.Name = ""
	described.Children = []accessibility.NodeID{72}
	tree.Nodes[72] = &accessibility.Node{
		ID: 72, Parent: 65, Role: role.Label, Name: countText, Bounds: geom.NewRect(160, 20, 140, 20),
		Text: &accessibility.TextInfo{
			Text:  countText,
			Lines: []accessibility.Line{{End: len(countText), Bounds: geom.NewRect(0, 0, 20, 20)}},
		},
	}
	ta.Publish(tableWindow, tree, nil, sampleGeometry())

	c.Equal(wholeRowName, ta.peer.getProperty(NodePath(62), InterfaceAccessible, "Name"),
		"the row is called the whole of what is in it, word for word")
	c.Equal([]string{InterfaceAccessible, InterfaceCollection, InterfaceComponent},
		ta.one(NodePath(62), InterfaceAccessible, "GetInterfaces", ""),
		"a row carries no text of its own, so its name is the only place the words are")
	c.Equal(nodeRefs(63, 64, 65), ta.one(NodePath(62), InterfaceAccessible, "GetChildren", ""),
		"the cells are still there for the cursor to land on, whatever the row is called")

	c.Equal("", ta.peer.getProperty(NodePath(65), InterfaceAccessible, "Name"),
		"a cell whose content speaks for itself is left nameless rather than named from that content")
	c.Equal("Alpha", ta.peer.getProperty(NodePath(64), InterfaceAccessible, "Name"),
		"a cell with nothing inside it keeps the text the table sorts it by")
	c.Equal(nodeRefs(72), ta.one(NodePath(65), InterfaceAccessible, "GetChildren", ""))
	c.Equal(countText, ta.one(NodePath(72), InterfaceText, "GetText", "ii", int32(0), int32(-1)),
		"the words are on the object inside the cell, which is where Orca reads them from")
}
