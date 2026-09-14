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
	c.Equal([]string{InterfaceAccessible, InterfaceComponent, InterfaceSelection, InterfaceTable},
		ta.one(NodePath(61), InterfaceAccessible, "GetInterfaces", ""))
	c.Equal([]string{InterfaceAccessible, InterfaceComponent},
		ta.one(NodePath(62), InterfaceAccessible, "GetInterfaces", ""),
		"a row holds the cells rather than being one of them")
	c.Equal([]string{InterfaceAccessible, InterfaceComponent, InterfaceTableCell},
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
