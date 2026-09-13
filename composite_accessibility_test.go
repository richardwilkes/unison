// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison_test

import (
	"strconv"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/behavior"
	checkenum "github.com/richardwilkes/unison/enums/check"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/enums/role"
	"github.com/richardwilkes/unison/enums/side"
)

// These tests ask the widgets that are built out of other widgets, or out of no widgets at all, what they tell an
// assistive technology. The rows of a list, the rows and cells of a table and the headers of its columns have no panels
// of their own, so what is being checked for those is that they are described anyway, that only the ones worth
// describing are, and that acting on one reaches the row or column it named. A session owns most of the package's
// mutable globals while it runs, so none of these may call t.Parallel.

// axChildNodes returns the child nodes of a node, in order, skipping any the tree has no node for.
func axChildNodes(tree *accessibility.Tree, node *accessibility.Node) []*accessibility.Node {
	if tree == nil || node == nil {
		return nil
	}
	nodes := make([]*accessibility.Node, 0, len(node.Children))
	for _, id := range node.Children {
		if child := tree.Node(id); child != nil {
			nodes = append(nodes, child)
		}
	}
	return nodes
}

// axNodesWithRole returns every node in the tree with the given role, in the order the tree is walked.
func axNodesWithRole(tree *accessibility.Tree, r role.Enum) []*accessibility.Node {
	var nodes []*accessibility.Node
	tree.Walk(func(n *accessibility.Node) bool {
		if n.Role == r {
			nodes = append(nodes, n)
		}
		return true
	})
	return nodes
}

// axNodeWithRowIndex returns the first child of a node with the given row index, or nil if there is none.
func axNodeWithRowIndex(tree *accessibility.Tree, node *accessibility.Node, row int) *accessibility.Node {
	for _, child := range axChildNodes(tree, node) {
		if child.RowIndex == row {
			return child
		}
	}
	return nil
}

// axScroller returns a scroll panel showing content through a view port of the given size, for the tests that need
// more rows than can be seen at once.
func axScroller(content unison.Paneler, size geom.Size) *unison.ScrollPanel {
	scroller := unison.NewScrollPanel()
	scroller.SetContent(content, behavior.Fill, behavior.Unmodified)
	scroller.SetLayoutData(&unison.FlexLayoutData{
		SizeHint: size,
		HAlign:   align.Fill,
		VAlign:   align.Start,
	})
	return scroller
}

// TestListAccessibility verifies that a list describes its rows as virtual children keyed by index, that it describes
// only the rows that can be seen along with the ones that are selected, and that an assistive technology can move the
// selection from row to row.
func TestListAccessibility(t *testing.T) {
	c := check.New(t)
	const rowCount = 40
	var list *unison.List[string]
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			list = unison.NewList[string]()
			// A fixed cell height is what most lists have, and it is what lets the list work out where a row is without
			// walking the rows before it.
			list.Factory = &unison.DefaultCellFactory{Height: 20}
			for i := range rowCount {
				list.Append("Row " + strconv.Itoa(i))
			}
			list.Select(false, 0)
			list.Select(true, rowCount-1)

			// The view port is far shorter than the rows within it, so most of them cannot be seen.
			wnd = newHeadlessWindow(t, "list", geom.NewRect(10, 10, 300, 300),
				axColumn(axScroller(list, geom.NewSize(200, 100))))
		}))
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(list)
	c.True(node != nil)
	c.Equal(role.List, node.Role)
	c.True(node.Multiselectable)
	c.Equal(rowCount, node.RowCount)

	rows := axChildNodes(tree, node)
	c.True(len(rows) < rowCount/2,
		"only the visible rows and the selection should have been described, got %d", len(rows))
	first := axNodeWithRowIndex(tree, node, 0)
	c.True(first != nil)
	c.Equal(role.ListItem, first.Role)
	c.Equal("Row 0", first.Name, "a row is named by the text of the cell that draws it")
	c.True(first.Selectable)
	c.True(first.Selected)
	c.False(first.Offscreen)
	c.True(first.Actions.Has(accessibility.Select))
	c.True(first.Actions.Has(accessibility.AddToSelection))
	c.True(first.Actions.Has(accessibility.RemoveFromSelection))
	c.True(first.Actions.Has(accessibility.ScrollIntoView))
	c.True(first.Bounds.Height > 0)

	last := axNodeWithRowIndex(tree, node, rowCount-1)
	c.True(last != nil, "a selected row is described however far out of sight it is")
	c.Equal("Row 39", last.Name)
	c.True(last.Selected)
	c.True(last.Offscreen, "the last row is far below the view port")

	c.True(axNodeWithRowIndex(tree, node, rowCount/2) == nil,
		"a row that is neither visible nor selected is not described at all")

	// Scrolling the last row into view turns the viewport around without changing which node stands for which row.
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   last.ID,
		Action: accessibility.ScrollIntoView,
	}))
	tree = screen.AccessibilityTree(wnd)
	node = screen.AccessibilityNodeFor(list)
	last = axNodeWithRowIndex(tree, node, rowCount-1)
	c.True(last != nil)
	c.Equal(rowCount-1, last.RowIndex)
	c.False(last.Offscreen, "the last row should have been scrolled into view")
	first = axNodeWithRowIndex(tree, node, 0)
	c.True(first != nil, "the first row is still selected, so it is still described")
	c.True(first.Offscreen, "the first row has been scrolled out of view")

	// Back to the top, where the selection can be moved between two rows that both stay on the screen — and therefore
	// stay described — for as long as the requests take.
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   first.ID,
		Action: accessibility.ScrollIntoView,
	}))
	tree = screen.AccessibilityTree(wnd)
	node = screen.AccessibilityNodeFor(list)
	firstRow := axNodeWithRowIndex(tree, node, 0)
	secondRow := axNodeWithRowIndex(tree, node, 1)
	c.True(firstRow != nil)
	c.True(secondRow != nil)

	// Selecting one row replaces the selection; adding and removing adjust it.
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   secondRow.ID,
		Action: accessibility.Select,
	}))
	c.Equal([]int{1}, axSelection(screen, list))
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   firstRow.ID,
		Action: accessibility.AddToSelection,
	}))
	c.Equal([]int{0, 1}, axSelection(screen, list))
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   secondRow.ID,
		Action: accessibility.RemoveFromSelection,
	}))
	c.Equal([]int{0}, axSelection(screen, list))

	// The rows just past the bottom of the view port are described even though they cannot be seen, so that an assistive
	// technology stepping down through the rows has one to step onto; selecting one scrolls it into view, as the arrow
	// keys would, so that the next description reaches further down.
	rows = axChildNodes(tree, node)
	visibleRows := 0
	for _, row := range rows {
		if !row.Offscreen {
			visibleRows++
		}
	}
	c.True(visibleRows > 0 && visibleRows < rowCount/4, "a handful of rows fit in the view port, got %d", visibleRows)
	beyond := axNodeWithRowIndex(tree, node, visibleRows)
	c.True(beyond != nil, "the row just past the view port must be described for an assistive technology to reach")
	c.True(beyond.Offscreen, "the row just past the view port cannot be seen yet")
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   beyond.ID,
		Action: accessibility.Select,
	}))
	tree = screen.AccessibilityTree(wnd)
	node = screen.AccessibilityNodeFor(list)
	beyond = axNodeWithRowIndex(tree, node, visibleRows)
	c.True(beyond != nil)
	c.True(beyond.Selected, "the row should have been selected")
	c.False(beyond.Offscreen, "selecting the row should have scrolled it into view")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// axSelection returns the selected row indexes of a list, read on the thread that owns it.
func axSelection[T any](screen *unison.HeadlessScreen, list *unison.List[T]) []int {
	var selection []int
	screen.Do(func() { selection = selectedIndexes(list) })
	return selection
}

// TestListAccessibilityWithVaryingRowHeights verifies that a list whose rows are each their own height describes them
// too, since where those rows sit can only be had by measuring the ones before them.
func TestListAccessibilityWithVaryingRowHeights(t *testing.T) {
	c := check.New(t)
	var list *unison.List[string]
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 500},
		unison.StartupFinishedCallback(func() {
			label := unison.NewLabel()
			label.SetTitle("Choices:")
			list = unison.NewList[string]()
			list.Append("alpha", "beta", "gamma")
			wnd = newHeadlessWindow(t, "varying", geom.NewRect(10, 10, 300, 300), axColumn(label, list))
		}))
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(list)
	c.True(node != nil)
	c.Equal("Choices", node.Name, "the label before it names it, minus the colon")
	c.Equal(3, node.RowCount)
	rows := axChildNodes(tree, node)
	c.Equal(3, len(rows), "every row is visible, so every row is described")
	if len(rows) == 3 {
		c.Equal("alpha", rows[0].Name)
		c.Equal("gamma", rows[2].Name)
		c.True(rows[1].Bounds.Y > rows[0].Bounds.Y, "the rows stack downwards")
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// axTableRows returns the row nodes of a table's description, keyed by the name each row reports.
func axTableRows(tree *accessibility.Tree, node *accessibility.Node) map[string]*accessibility.Node {
	rows := make(map[string]*accessibility.Node)
	for _, child := range axChildNodes(tree, node) {
		if child.Role == role.Row {
			rows[child.Name] = child
		}
	}
	return rows
}

// axNewTable returns a table of the given rows with two columns, synced and ready to be put in a window.
func axNewTable(rows ...*tableTestRow) *unison.Table[*tableTestRow] {
	model := &unison.SimpleTableModel[*tableTestRow]{}
	model.SetRootRows(rows)
	table := unison.NewTable[*tableTestRow](model)
	table.Columns = []unison.ColumnInfo{
		{ID: 0, Current: 100},
		{ID: 1, Current: 100},
	}
	table.SyncToModel()
	return table
}

// TestTableAccessibility verifies what a table says about its rows and cells: each row is described as a virtual child
// keyed by its model id, with its cells beneath it, and acting on one reaches the row it named.
func TestTableAccessibility(t *testing.T) {
	c := check.New(t)
	var table *unison.Table[*tableTestRow]
	var rows []*tableTestRow
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			rows = make([]*tableTestRow, 3)
			for i := range rows {
				rows[i] = newTableTestRow("r" + strconv.Itoa(i))
				name := "row " + strconv.Itoa(i)
				rows[i].cellData = func(col int) string { return name + " col " + strconv.Itoa(col) }
			}
			table = axNewTable(rows...)
			wnd = newHeadlessWindow(t, "table", geom.NewRect(10, 10, 400, 400), axColumn(table))
		}))
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(table)
	c.True(node != nil)
	c.Equal(role.Table, node.Role, "a table whose rows cannot have children is a table rather than a tree")
	c.True(node.Multiselectable)
	c.Equal(3, node.RowCount)
	c.Equal(2, node.ColumnCount)

	rowNodes := axChildNodes(tree, node)
	c.Equal(3, len(rowNodes), "every row fits in the window, so every row is described")
	if len(rowNodes) != 3 {
		return
	}
	first := rowNodes[0]
	c.Equal(role.Row, first.Role)
	c.Equal("row 0 col 0", first.Name, "a row is named by the data of its first column")
	c.Equal(0, first.RowIndex)
	c.Equal(1, first.Level, "a row with no ancestors is at the top level")
	c.True(first.Selectable)
	c.False(first.Selected)
	c.False(first.Expandable, "a row that cannot have children cannot be expanded")
	c.True(first.Actions.Has(accessibility.Select))
	c.True(first.Bounds.Height > 0)
	c.True(rowNodes[1].Bounds.Y > first.Bounds.Y, "the rows stack downwards")

	cells := axChildNodes(tree, first)
	c.Equal(2, len(cells), "a row has one cell per column")
	if len(cells) == 2 {
		c.Equal(role.Cell, cells[0].Role)
		c.Equal("row 0 col 0", cells[0].Name)
		c.Equal("row 0 col 1", cells[1].Name)
		c.Equal(0, cells[0].ColumnIndex)
		c.Equal(1, cells[1].ColumnIndex)
		c.Equal(0, cells[1].RowIndex)
		c.True(cells[1].Bounds.X > cells[0].Bounds.X, "the cells run across the row")
	}

	// Selecting a row replaces the selection; adding and removing adjust it.
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   rowNodes[1].ID,
		Action: accessibility.Select,
	}))
	c.Equal([]int{1}, axSelectedRowIndexes(screen, table))
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   rowNodes[2].ID,
		Action: accessibility.AddToSelection,
	}))
	c.Equal([]int{1, 2}, axSelectedRowIndexes(screen, table))
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   rowNodes[1].ID,
		Action: accessibility.RemoveFromSelection,
	}))
	c.Equal([]int{2}, axSelectedRowIndexes(screen, table))
	tree = screen.AccessibilityTree(wnd)
	node = screen.AccessibilityNodeFor(table)
	c.True(axTableRows(tree, node)["row 2 col 0"].Selected, "the selected row should say so")

	// Scrolling a cell into view is the one thing a request aimed at a cell does to the cell rather than to its row.
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   axChildNodes(tree, axTableRows(tree, node)["row 1 col 0"])[1].ID,
		Action: accessibility.ScrollIntoView,
	}))
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// axSelectedRowIndexes returns the indexes of a table's selected rows, read on the thread that owns it.
func axSelectedRowIndexes(screen *unison.HeadlessScreen, table *unison.Table[*tableTestRow]) []int {
	var indexes []int
	screen.Do(func() {
		for i := range table.RowHeights() {
			if table.IsRowSelected(i) {
				indexes = append(indexes, i)
			}
		}
	})
	return indexes
}

// TestTableAccessibilityRowsSurviveReorder verifies that a row is described under the same node from one description to
// the next however the rows around it move, which is what keying a row by its model id rather than by its index buys.
func TestTableAccessibilityRowsSurviveReorder(t *testing.T) {
	c := check.New(t)
	var table *unison.Table[*tableTestRow]
	var rows []*tableTestRow
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			rows = flatRows(3)
			table = axNewTable(rows...)
			wnd = newHeadlessWindow(t, "reorder", geom.NewRect(10, 10, 400, 400), axColumn(table))
		}))
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(table)
	before := axTableRows(tree, node)
	c.Equal(3, len(before))
	firstID := before["r0"].ID
	lastID := before["r2"].ID
	c.True(firstID != 0)
	c.Equal(0, before["r0"].RowIndex)
	c.Equal(2, before["r2"].RowIndex)

	// Handing the model its rows in the opposite order and syncing is what sorting or filtering a table amounts to.
	screen.Do(func() { table.SetRootRows([]*tableTestRow{rows[2], rows[1], rows[0]}) })
	tree = screen.AccessibilityTree(wnd)
	node = screen.AccessibilityNodeFor(table)
	after := axTableRows(tree, node)
	c.Equal(3, len(after))
	c.Equal(firstID, after["r0"].ID, "a row keeps its node when the rows around it move")
	c.Equal(lastID, after["r2"].ID)
	c.Equal(2, after["r0"].RowIndex, "the row that was first is now last")
	c.Equal(0, after["r2"].RowIndex)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTableAccessibilityHierarchy verifies that a table whose rows can have children is a tree, that a row says whether
// it can be expanded and whether it is, and that expanding one through an assistive technology brings the table up to
// date with the rows that appeared.
func TestTableAccessibilityHierarchy(t *testing.T) {
	c := check.New(t)
	var table *unison.Table[*tableTestRow]
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			parent := newTableTestRow("parent")
			parent.SetChildren([]*tableTestRow{newTableTestRow("child0"), newTableTestRow("child1")})
			table = axNewTable(parent)
			wnd = newHeadlessWindow(t, "tree", geom.NewRect(10, 10, 400, 400), axColumn(table))
		}))
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(table)
	c.True(node != nil)
	c.Equal(role.Tree, node.Role, "a table whose rows can have children is a tree")
	c.Equal(1, node.RowCount, "a closed row shows none of its children")
	parentNode := axTableRows(tree, node)["parent"]
	c.True(parentNode != nil)
	c.True(parentNode.Expandable)
	c.False(parentNode.Expanded)
	c.True(parentNode.Actions.Has(accessibility.Expand))
	c.True(parentNode.Actions.Has(accessibility.Collapse))

	// The triangle the table draws to open the row is described as the row's first child, and pressing it opens and
	// closes the row as clicking it would.
	parentChildren := axChildNodes(tree, parentNode)
	c.True(len(parentChildren) > 0, "the row should hold a disclosure triangle and its cells")
	if len(parentChildren) > 0 {
		disclosure := parentChildren[0]
		c.Equal(role.DisclosureTriangle, disclosure.Role, "the row's first child is its disclosure triangle")
		c.False(disclosure.Pressed)
		c.True(disclosure.Expandable)
		c.True(disclosure.Actions.Has(accessibility.Press))
		c.True(disclosure.Bounds.Width > 0 && disclosure.Bounds.Height > 0)
		c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
			Node:   disclosure.ID,
			Action: accessibility.Press,
		}))
		tree = screen.AccessibilityTree(wnd)
		node = screen.AccessibilityNodeFor(table)
		c.Equal(3, node.RowCount, "pressing the disclosure triangle should have opened the row")
		disclosure = tree.Node(disclosure.ID)
		c.True(disclosure != nil, "the disclosure triangle keeps its id")
		if disclosure != nil {
			c.True(disclosure.Pressed, "the open row's triangle is pressed")
			c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
				Node:   disclosure.ID,
				Action: accessibility.Press,
			}))
		}
		tree = screen.AccessibilityTree(wnd)
		node = screen.AccessibilityNodeFor(table)
		c.Equal(1, node.RowCount, "pressing it again should have closed the row")
		parentNode = axTableRows(tree, node)["parent"]
	}

	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   parentNode.ID,
		Action: accessibility.Expand,
	}))
	tree = screen.AccessibilityTree(wnd)
	node = screen.AccessibilityNodeFor(table)
	c.Equal(3, node.RowCount, "expanding the row should have brought its children into the table")
	rowNodes := axTableRows(tree, node)
	c.Equal(3, len(rowNodes))
	c.True(rowNodes["parent"].Expanded)
	c.Equal(1, rowNodes["parent"].Level)
	c.True(rowNodes["child0"] != nil)
	c.Equal(2, rowNodes["child0"].Level, "a child row is one level deeper than its parent")
	c.Equal(1, rowNodes["child0"].RowIndex)

	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   parentNode.ID,
		Action: accessibility.Collapse,
	}))
	tree = screen.AccessibilityTree(wnd)
	node = screen.AccessibilityNodeFor(table)
	c.Equal(1, node.RowCount, "collapsing the row should have taken its children away again")
	c.False(axTableRows(tree, node)["parent"].Expanded)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTableAccessibilityHierarchicalFilterRefusesOpenStateChanges verifies that an assistive technology is given no way
// to open or close a row while a hierarchical filter is applied. Such a filter shows every container it kept as open,
// whatever the container's own open state, so the disclosure triangle is drawn without a hit rect, the left and right
// arrow keys do nothing and DiscloseRow reports that it changed nothing; letting a request through here would have
// flipped open states the filter hides, with nothing on the screen to show for it.
func TestTableAccessibilityHierarchicalFilterRefusesOpenStateChanges(t *testing.T) {
	c := check.New(t)
	var table *unison.Table[*tableTestRow]
	var parent *tableTestRow
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			parent = newTableTestRow("parent")
			parent.SetChildren([]*tableTestRow{newTableTestRow("child0"), newTableTestRow("child1")})
			table = axNewTable(parent)
			// The filter keeps the rows it returns false for, so only child0 passes; parent is shown as the context it
			// sits in, open despite never having been opened.
			table.ApplyHierarchicalFilter(func(row *tableTestRow) bool { return row.ID() != "child0" })
			wnd = newHeadlessWindow(t, "filtered tree", geom.NewRect(10, 10, 400, 400), axColumn(table))
		}))
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(table)
	c.True(node != nil)
	c.Equal(2, node.RowCount, "the filter shows the row that passed and the container holding it")
	parentNode := axTableRows(tree, node)["parent"]
	c.True(parentNode != nil)
	c.True(parentNode.Expandable)
	c.True(parentNode.Expanded, "a hierarchical filter shows every container it kept as open")
	c.False(parentNode.Actions.Has(accessibility.Expand), "the open state cannot be changed behind the filter")
	c.False(parentNode.Actions.Has(accessibility.Collapse), "the open state cannot be changed behind the filter")
	for _, child := range axChildNodes(tree, parentNode) {
		c.NotEqual(role.DisclosureTriangle, child.Role,
			"the triangle the filter draws is not something that can be pressed")
	}

	// Even a request that names the row directly, as one built from an older description would, is refused.
	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   parentNode.ID,
		Action: accessibility.Expand,
	}))
	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   parentNode.ID,
		Action: accessibility.Collapse,
	}))
	var open bool
	screen.Do(func() { open = parent.IsOpen() })
	c.False(open, "the row's own open state must have been left alone")
	node = screen.AccessibilityNodeFor(table)
	c.Equal(2, node.RowCount, "the rows the filter shows must not have changed")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTableAccessibilityWithNoColumns verifies that a table holding rows but no columns never asks a row for the data of
// a column that does not exist. Every other caller of CellDataForSort is driven by a real column index, so a model is
// entitled to reach straight for the column it was handed.
func TestTableAccessibilityWithNoColumns(t *testing.T) {
	c := check.New(t)
	var table *unison.Table[*tableTestRow]
	var wnd *unison.Window
	var asked []int
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			row := newTableTestRow("only")
			row.cellData = func(col int) string {
				asked = append(asked, col)
				return "data"
			}
			model := &unison.SimpleTableModel[*tableTestRow]{}
			model.SetRootRows([]*tableTestRow{row})
			table = unison.NewTable[*tableTestRow](model)
			table.SetLayoutData(&unison.FlexLayoutData{HAlign: align.Fill, HGrab: true})
			table.SyncToModel()
			wnd = newHeadlessWindow(t, "columnless table", geom.NewRect(10, 10, 300, 200), axColumn(table))
		}))
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(table)
	c.True(node != nil)
	c.Equal(1, node.RowCount)
	c.Equal(0, node.ColumnCount)
	rows := axChildNodes(tree, node)
	c.Equal(1, len(rows), "the row is still described, even with no columns to describe within it")
	if len(rows) == 1 {
		c.Equal(role.Row, rows[0].Role)
		c.Equal("", rows[0].Name, "there is no column for a name to come from")
		c.Equal(0, len(rows[0].Children), "a row with no columns holds no cells")
	}
	var calls []int
	screen.Do(func() { calls = asked })
	c.Equal(0, len(calls), "no cell data should have been asked for, but columns %v were", calls)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTableAccessibilityBoundsSelectionAndFocus verifies that a table far taller than its view port describes the rows
// that can be seen, caps how many it describes solely because they are selected, and always describes the row holding
// the cell that has the keyboard focus, however far out of view it has been scrolled.
func TestTableAccessibilityBoundsSelectionAndFocus(t *testing.T) {
	c := check.New(t)
	const rowCount = 300
	var table *unison.Table[*tableTestRow]
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			rows := make([]*tableTestRow, rowCount)
			for i := range rows {
				rows[i] = newTableTestRow("r" + strconv.Itoa(i))
			}
			table = axNewTable(rows...)
			wnd = newHeadlessWindow(t, "bounded", geom.NewRect(10, 10, 400, 400),
				axColumn(axScroller(table, geom.NewSize(300, 100))))
		}))
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(table)
	c.True(node != nil)
	c.Equal(rowCount, node.RowCount)
	described := len(axChildNodes(tree, node))
	c.True(described > 0 && described < 40,
		"only the rows in and just past the view port should have been described, got %d", described)
	visible := 0
	for _, row := range axChildNodes(tree, node) {
		if !row.Offscreen {
			visible++
		}
	}
	c.True(visible > 0 && visible < described, "the rows past the view port are described but cannot be seen")

	// Selecting the first row past the view port, as an assistive technology stepping down through the rows does,
	// scrolls it into view so that the rows after it come within reach.
	beyond := axNodeWithRowIndex(tree, node, visible)
	c.True(beyond != nil, "the row just past the view port must be described")
	c.True(beyond.Offscreen)
	screen.AccessibilityEvents(wnd) // drain, so that only what the request produces is seen below
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   beyond.ID,
		Action: accessibility.Select,
	}))
	// The request is followed at once by a fresh description, without waiting for a redraw, since an assistive
	// technology reads the result straight after asking.
	selectedNow := false
	for _, e := range screen.AccessibilityEvents(wnd) {
		if e.Kind == accessibility.StateChanged && e.State == accessibility.StateSelected && e.Node == beyond.ID {
			selectedNow = true
		}
	}
	c.True(selectedNow, "the selection change should have been published as soon as the request was carried out")
	tree = screen.AccessibilityTree(wnd)
	node = screen.AccessibilityNodeFor(table)
	beyond = axNodeWithRowIndex(tree, node, visible)
	c.True(beyond != nil)
	c.True(beyond.Selected)
	c.False(beyond.Offscreen, "selecting the row should have scrolled it into view")
	visible = len(axChildNodes(tree, node))

	screen.Do(func() { table.SelectAll() })
	tree = screen.AccessibilityTree(wnd)
	node = screen.AccessibilityNodeFor(table)
	described = len(axChildNodes(tree, node))
	c.True(described > visible, "the selected rows should have been described as well, got %d", described)
	c.True(described < rowCount, "how many rows the selection alone can add is capped, got %d", described)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTableAccessibilityFocusedCell verifies that the row holding the cell that has the keyboard focus is described
// however far out of view it has been scrolled, and that the widget the person is working in is described within that
// cell — the one cell that is a panel of the table for longer than a single event.
func TestTableAccessibilityFocusedCell(t *testing.T) {
	c := check.New(t)
	const rowCount = 20
	var e *editTable
	var wnd *unison.Window
	var scroll *unison.ScrollPanel
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			e = newEditTable(rowCount, 2, "typed", false)
			wnd, scroll = newEditScrollWindow(t, e)
		}))
	c.NotNil(wnd)

	var focused bool
	screen.Do(func() { focused = e.table.FocusCell(rowCount-1, 0) })
	c.True(focused, "the field in the last row's first cell should have taken the focus")
	// Focusing the cell scrolled it into view; the table is taken back to the top so that the row holding it is out of
	// sight while it still holds the focus.
	screen.Do(func() { scroll.SetPosition(0, 0) })

	tree := screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(e.table)
	c.True(node != nil)
	rows := axTableRows(tree, node)
	last := rows["r"+strconv.Itoa(rowCount-1)]
	c.True(last != nil, "the row holding the focused cell must be described")
	c.True(last.Offscreen, "that row has been scrolled out of view")
	c.True(rows["r0"] != nil, "the rows that can be seen are described too")

	cells := axChildNodes(tree, last)
	c.Equal(3, len(cells))
	if len(cells) != 3 {
		return
	}
	field := screen.AccessibilityNodeFor(e.fields[rowCount-1][0])
	c.True(field != nil, "the widget in the focused cell should have been described")
	if field == nil {
		return
	}
	c.Equal(role.TextField, field.Role)
	c.Equal("typed", field.Value)
	c.Equal(cells[0].ID, field.Parent, "the widget is described within the cell it lives in")
	c.Equal(field.ID, tree.Focus, "the tree should point at the widget holding the focus")
	c.Equal(1, len(axChildNodes(tree, cells[0])), "the cell holds the one widget and nothing else")
	others := tree.UnignoredChildren(cells[1].ID)
	c.True(len(others) != 0, "the other cell describes its own content too")
	for _, id := range others {
		c.False(tree.Node(id).Focused, "only the widget in the focused cell holds the focus")
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTableHeaderAccessibility verifies that a table's header describes each of its column headers, says which column
// the rows are sorted on and in which direction, and sorts the table when one of them is pressed.
func TestTableHeaderAccessibility(t *testing.T) {
	c := check.New(t)
	var table *unison.Table[*tableTestRow]
	var header *unison.TableHeader[*tableTestRow]
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			table = axNewTable(flatRows(3)...)
			header = unison.NewTableHeader[*tableTestRow](table,
				unison.NewTableColumnHeader[*tableTestRow]("Name", "What it is called", nil),
				unison.NewTableColumnHeader[*tableTestRow]("Value", "", nil))
			scroller := axScroller(table, geom.NewSize(300, 200))
			scroller.SetColumnHeader(header)
			wnd = newHeadlessWindow(t, "header", geom.NewRect(10, 10, 400, 400), axColumn(scroller))
		}))
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(header)
	c.True(node != nil)
	c.Equal(role.TableHeader, node.Role)
	c.Equal(2, node.ColumnCount)

	columns := axChildNodes(tree, node)
	c.Equal(2, len(columns), "the header describes one element per column header")
	if len(columns) != 2 {
		return
	}
	c.Equal(role.ColumnHeader, columns[0].Role)
	c.Equal("Name", columns[0].Name, "a column header is named by its own text")
	c.Equal("What it is called", columns[0].Description, "the column header's tooltip describes it")
	c.Equal(0, columns[0].ColumnIndex)
	c.Equal(1, columns[1].ColumnIndex)
	c.Equal("Value", columns[1].Name)
	c.Equal(accessibility.SortNone, columns[0].Sort, "nothing is sorted yet")
	c.True(columns[0].Actions.Has(accessibility.Press))
	c.True(columns[0].Bounds.Width > 0)
	c.True(columns[1].Bounds.X > columns[0].Bounds.X, "the headers run across the table")

	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   columns[0].ID,
		Action: accessibility.Press,
	}))
	tree = screen.AccessibilityTree(wnd)
	node = screen.AccessibilityNodeFor(header)
	columns = axChildNodes(tree, node)
	c.Equal(2, len(columns))
	c.Equal(accessibility.SortAscending, columns[0].Sort, "pressing the header sorted the table on that column")
	c.Equal(accessibility.SortNone, columns[1].Sort, "only the primary sort column is reported as sorted")

	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   columns[0].ID,
		Action: accessibility.Press,
	}))
	tree = screen.AccessibilityTree(wnd)
	node = screen.AccessibilityNodeFor(header)
	columns = axChildNodes(tree, node)
	c.Equal(2, len(columns))
	c.Equal(accessibility.SortDescending, columns[0].Sort, "pressing it again turned the sort around")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// axDockable is a dockable for these tests: a panel with a title, a tooltip, and the ability to be closed, which is
// what makes its tab show a close button.
type axDockable struct {
	title string
	tip   string
	unison.Panel
	modified bool
	closed   bool
}

// newAxDockable returns a dockable with the given title and tooltip.
func newAxDockable(title, tip string) *axDockable {
	d := &axDockable{title: title, tip: tip}
	d.Self = d
	return d
}

// TitleIcon implements unison.Dockable.
func (d *axDockable) TitleIcon(_ geom.Size) unison.Drawable { return nil }

// Title implements unison.Dockable.
func (d *axDockable) Title() string { return d.title }

// Tooltip implements unison.Dockable.
func (d *axDockable) Tooltip() string { return d.tip }

// Modified implements unison.Dockable.
func (d *axDockable) Modified() bool { return d.modified }

// MayAttemptClose implements unison.TabCloser.
func (d *axDockable) MayAttemptClose() bool { return true }

// AttemptClose implements unison.TabCloser.
func (d *axDockable) AttemptClose() bool {
	d.closed = true
	return true
}

// TestDockAccessibility verifies that a dock container describes itself as a group holding a list of tabs and the
// content of the current one, that each tab is named after its dockable without the marker that says it has unsaved
// changes, and that pressing a tab brings its dockable to the front.
func TestDockAccessibility(t *testing.T) {
	c := check.New(t)
	var first, second *axDockable
	var container *unison.DockContainer
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 800, Height: 600},
		unison.StartupFinishedCallback(func() {
			first = newAxDockable("First", "The first one")
			first.modified = true
			second = newAxDockable("Second", "")
			dock := unison.NewDock()
			dock.DockTo(first, nil, side.Left)
			container = unison.Ancestor[*unison.DockContainer](first)
			if container != nil {
				container.Stack(second, -1)
				container.SetCurrentDockable(first)
			}
			wnd = newHeadlessWindow(t, "dock", geom.NewRect(10, 10, 600, 400), dock)
		}))
	c.NotNil(wnd)
	c.True(container != nil)

	tree := screen.AccessibilityTree(wnd)
	containerNode := screen.AccessibilityNodeFor(container)
	c.True(containerNode != nil)
	c.Equal(role.Group, containerNode.Role)
	c.Equal("First", containerNode.Name, "the group is named after the dockable it is showing")
	c.False(containerNode.Ignored, "a named group is worth reporting")

	tabList := axNodesWithRole(tree, role.TabList)
	c.Equal(1, len(tabList))
	tabs := axNodesWithRole(tree, role.Tab)
	c.Equal(2, len(tabs), "there is one tab per dockable")
	if len(tabs) != 2 {
		return
	}
	c.Equal("First", tabs[0].Name, "the marker that says the dockable is modified is not part of its name")
	c.Equal("The first one", tabs[0].Description)
	c.True(tabs[0].Selectable)
	c.True(tabs[0].Selected, "the first dockable is the current one")
	c.Equal("Second", tabs[1].Name)
	c.False(tabs[1].Selected)
	c.True(tabs[1].Actions.Has(accessibility.Press))
	c.True(tabs[1].Actions.Has(accessibility.Select))

	// The tab's own label would only repeat the title, so the tab's children are the buttons on it.
	c.Equal(1, len(axChildNodes(tree, tabs[0])), "a tab holds its close button and nothing else")
	c.True(axNamed(tree, "Close") != nil, "the button that closes a tab says what it does")
	c.True(axNamed(tree, "Maximize") != nil, "so does the one that maximizes the container")

	panels := axNodesWithRole(tree, role.TabPanel)
	c.Equal(1, len(panels))
	if len(panels) == 1 {
		c.Equal("First", panels[0].Name, "the tab panel is named after the dockable it is showing")
		c.Equal("The first one", panels[0].Description)
	}

	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   tabs[1].ID,
		Action: accessibility.Press,
	}))
	var current unison.Dockable
	screen.Do(func() { current = container.CurrentDockable() })
	c.True(current == unison.Dockable(second), "pressing a tab should have brought its dockable to the front")
	tree = screen.AccessibilityTree(wnd)
	tabs = axNodesWithRole(tree, role.Tab)
	c.Equal(2, len(tabs))
	if len(tabs) == 2 {
		c.False(tabs[0].Selected)
		c.True(tabs[1].Selected, "the selection should have moved with the current dockable")
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestMarkdownAccessibility verifies that rendered markdown is described as a document: a heading is one element that
// knows how deep it is, a link says where it leads, and an image is named by its alternative text.
func TestMarkdownAccessibility(t *testing.T) {
	c := check.New(t)
	const content = "# Title\n\n## Subtitle\n\nBody text with a [Docs](https://example.com/docs) link.\n\n" +
		"![A cat](missing-image-for-test.png)\n"
	var markdown *unison.Markdown
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 800, Height: 800},
		unison.StartupFinishedCallback(func() {
			markdown = unison.NewMarkdown(false)
			markdown.SetContent(content, 400)
			wnd = newHeadlessWindow(t, "markdown", geom.NewRect(10, 10, 600, 600), axColumn(markdown))
		}))
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(markdown)
	c.True(node != nil)
	c.Equal(role.Document, node.Role, "rendered markdown is readable content rather than a set of controls")

	headings := axNodesWithRole(tree, role.Heading)
	c.Equal(2, len(headings))
	if len(headings) == 2 {
		c.Equal("Title", headings[0].Name, "a heading's text is its name, however many labels it is built from")
		c.Equal(1, headings[0].Level)
		c.Equal(0, len(headings[0].Children), "a heading is one element rather than a group of labels")
		c.Equal("Subtitle", headings[1].Name)
		c.Equal(2, headings[1].Level, "a second-level heading says so")
	}

	link := axNamed(tree, "Docs")
	c.True(link != nil, "the link should have been described")
	if link != nil {
		c.Equal(role.Link, link.Role)
		c.Equal("https://example.com/docs", link.Description, "where the link leads is worth hearing")
		c.True(link.Actions.Has(accessibility.Press))
	}

	image := axNamed(tree, "A cat")
	c.True(image != nil, "the image should have been named by its alternative text")
	if image != nil {
		c.Equal(role.Image, image.Role)
		c.False(image.Ignored, "an image something has described is worth reporting")
	}

	c.True(axNamed(tree, "Body text with a ") != nil, "the text of a paragraph is still described")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// axScreenPoint returns the point on the screen at the center of a node's bounds, which is where a test aims the mouse
// at something that has no panel of its own to ask.
func axScreenPoint(screen *unison.HeadlessScreen, wnd *unison.Window, node *accessibility.Node) geom.Point {
	var origin geom.Point
	screen.Do(func() { origin = wnd.ContentRect().Point })
	return node.Bounds.Center().Add(origin)
}

// TestInWindowMenuAccessibility verifies what the pure-Go menus say about themselves: the bar and the menus opened
// from it, each item's title, key binding, check state and enablement, and that the item a menu is pointing at is
// reported as the focused one, since that is what a person choosing from an open menu is on.
func TestInWindowMenuAccessibility(t *testing.T) {
	c := check.New(t)
	const (
		menuID = unison.UserBaseID + iota
		cutID
		wrapID
		neverID
		nestedID
		deeperID
	)
	var wnd *unison.Window
	activated := 0
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			wnd = newHeadlessWindow(t, "menus", geom.NewRect(10, 10, 400, 300), unison.NewPanel())
			if wnd == nil {
				return
			}
			unison.DefaultMenuFactory().BarForWindow(wnd, func(bar unison.Menu) {
				f := bar.Factory()
				edit := f.NewMenu(menuID, "Edit", nil)
				edit.InsertItem(-1, f.NewItem(cutID, "Cut",
					unison.KeyBinding{KeyCode: unison.KeyX, Modifiers: mod.OSMenuCommand()}, nil,
					func(_ unison.MenuItem) { activated++ }))
				edit.InsertSeparator(-1, false)
				wrap := f.NewItem(wrapID, "Wrap", unison.KeyBinding{}, nil, nil)
				wrap.SetCheckState(checkenum.On)
				edit.InsertItem(-1, wrap)
				edit.InsertItem(-1, f.NewItem(neverID, "Never", unison.KeyBinding{},
					func(_ unison.MenuItem) bool { return false }, nil))
				more := f.NewMenu(nestedID, "More", nil)
				more.InsertItem(-1, f.NewItem(deeperID, "Deeper", unison.KeyBinding{}, nil, nil))
				edit.InsertMenu(-1, more)
				bar.InsertMenu(-1, edit)
			})
			wnd.ToFront()
		}))
	c.NotNil(wnd)
	screen.Sync()

	tree := screen.AccessibilityTree(wnd)
	bars := axNodesWithRole(tree, role.MenuBar)
	c.Equal(1, len(bars), "the window has an in-window menu bar")
	title := axNamed(tree, "Edit")
	c.True(title != nil, "the bar carries the one menu that was added to it")
	if title == nil {
		return
	}
	c.Equal(role.MenuItem, title.Role)
	c.True(title.Expandable, "a menu on the bar opens something")
	c.False(title.Expanded, "nothing is open yet")

	// Clicking the title opens the menu, which becomes part of the window's description.
	screen.Click(axScreenPoint(screen, wnd, title))
	tree = screen.AccessibilityTree(wnd)
	menus := axNodesWithRole(tree, role.Menu)
	c.Equal(1, len(menus), "the menu that opened should have been described")
	if len(menus) != 1 {
		return
	}
	c.Equal("Edit", menus[0].Name)
	items := axNodesWithRole(tree, role.MenuItem)
	byName := make(map[string]*accessibility.Node, len(items))
	for _, item := range items {
		byName[item.Name] = item
	}
	cut := byName["Cut"]
	c.True(cut != nil)
	if cut == nil {
		return
	}
	c.True(cut.Shortcut != "", "an item with a key binding says what it is")
	c.False(cut.HasCheck, "an ordinary item is not announced as something that could be checked")
	c.False(cut.Disabled)
	c.True(cut.Actions.Has(accessibility.Press))

	wrapItem := byName["Wrap"]
	c.True(wrapItem != nil)
	if wrapItem != nil {
		c.True(wrapItem.HasCheck)
		c.Equal(checkenum.On, wrapItem.Checked)
	}
	never := byName["Never"]
	c.True(never != nil)
	if never != nil {
		c.True(never.Disabled, "an item whose validator refuses it is disabled")
	}
	nested := byName["More"]
	c.True(nested != nil)
	if nested != nil {
		c.True(nested.Expandable, "an item with a sub-menu opens something")
	}
	c.True(len(axNodesWithRole(tree, role.Separator)) > 0, "the separator in the menu is described as one")

	// Moving the mouse over an item is what a person choosing from a menu does, and what the menu is pointing at is
	// where an assistive technology must be told the focus is.
	screen.MouseMove(axScreenPoint(screen, wnd, cut), mod.None)
	tree = screen.AccessibilityTree(wnd)
	c.Equal(cut.ID, tree.Focus, "the item the menu is pointing at is the focused one")
	hovered := tree.Node(cut.ID)
	c.True(hovered != nil)
	if hovered != nil {
		c.True(hovered.Focused)
	}

	// Pressing the item chooses it, which runs its handler from the event loop and closes the menu.
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   cut.ID,
		Action: accessibility.Press,
	}))
	screen.Sync()
	var count int
	screen.Do(func() { count = activated })
	c.Equal(1, count, "choosing the item should have run its handler exactly once")
	tree = screen.AccessibilityTree(wnd)
	c.Equal(0, len(axNodesWithRole(tree, role.Menu)), "choosing an item should have closed the menu")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTooltipAccessibility verifies that a tooltip, while it is showing, is described as one — a child of the window
// rather than of the panel it belongs to, since that is where it is drawn.
func TestTooltipAccessibility(t *testing.T) {
	c := check.New(t)
	var button *unison.Button
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			button = unison.NewButton()
			button.SetTitle("Save")
			button.Tooltip = unison.NewTooltipWithText("Write the file out")
			// Without this the tooltip would not appear until the delay a person's pause has to last.
			button.TooltipImmediate = true
			wnd = newHeadlessWindow(t, "tips", geom.NewRect(10, 10, 300, 200), axColumn(button))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() { wnd.ToFront() }))

	tree := screen.AccessibilityTree(wnd)
	c.Equal(0, len(axNodesWithRole(tree, role.Tooltip)), "no tooltip is showing yet")

	screen.MouseMove(screen.PanelCenter(button), mod.None)
	tree = screen.AccessibilityTree(wnd)
	tips := axNodesWithRole(tree, role.Tooltip)
	c.Equal(1, len(tips), "the tooltip that is showing should have been described")
	if len(tips) == 1 {
		c.Equal("Write the file out", tips[0].Name, "the tooltip's text is its name, line breaks and all")
		c.Equal(tree.Root, tips[0].Parent, "a tooltip is drawn by the window rather than by the panel it explains")
	}
	c.Equal("Write the file out", screen.AccessibilityNodeFor(button).Description,
		"the tooltip is the button's description as well")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTableAccessibilityCellContent verifies that whatever a row hands back for a cell is described within the cell,
// and that a request aimed at it reaches the widget: a check box the row keeps from one call to the next, and a button
// it builds afresh each time, which only its position within the cell can identify. The nodes keep their ids from one
// description to the next either way.
func TestTableAccessibilityCellContent(t *testing.T) {
	c := check.New(t)
	var table *unison.Table[*tableTestRow]
	var wnd *unison.Window
	var box *unison.CheckBox
	clicks := 0
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			box = unison.NewCheckBox()
			box.SetTitle("Done")
			row := newTableTestRow("r0")
			row.cellFactory = func(_, col int) unison.Paneler {
				if col == 0 {
					return box
				}
				button := unison.NewButton()
				button.SetTitle("Go")
				button.ClickCallback = func() { clicks++ }
				return button
			}
			table = axNewTable(row)
			wnd = newHeadlessWindow(t, "cells", geom.NewRect(10, 10, 400, 200), table)
		}))
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(table)
	c.True(node != nil)
	rows := axChildNodes(tree, node)
	c.Equal(1, len(rows))
	if len(rows) != 1 {
		return
	}
	cells := axChildNodes(tree, rows[0])
	c.Equal(2, len(cells))
	if len(cells) != 2 {
		return
	}
	c.Equal("", cells[0].Name, "a cell with content of its own is not named by its sort text on top of that")
	boxes := axChildNodes(tree, cells[0])
	c.Equal(1, len(boxes), "the check box is described within its cell")
	buttons := axChildNodes(tree, cells[1])
	c.Equal(1, len(buttons), "the button is described within its cell")
	if len(boxes) != 1 || len(buttons) != 1 {
		return
	}
	boxNode, buttonNode := boxes[0], buttons[0]
	c.Equal(role.CheckBox, boxNode.Role)
	c.Equal("Done", boxNode.Name)
	c.True(boxNode.HasCheck)
	c.Equal(checkenum.Off, boxNode.Checked)
	c.True(boxNode.Actions.Has(accessibility.Press))
	c.True(boxNode.Bounds.Width > 0 && boxNode.Bounds.Height > 0)
	c.True(boxNode.Bounds.Y >= cells[0].Bounds.Y && boxNode.Bounds.X >= cells[0].Bounds.X,
		"the widget sits within its cell")
	c.Equal(role.Button, buttonNode.Role)
	c.Equal("Go", buttonNode.Name)
	c.True(cells[0].Actions.Has(accessibility.Press), "a cell passes a press on to the check box within it")
	c.True(cells[0].Actions.Has(accessibility.Toggle))
	c.True(cells[1].Actions.Has(accessibility.Press), "a cell passes a press on to the button within it")
	c.False(cells[1].Actions.Has(accessibility.Toggle), "a button cannot be toggled")

	// A screen reader treats the cell as the unit: it presses the cell, and it reports what changed by reading the
	// cell's value again, so the check box's state is the cell's value and a change to it is reported on the cell.
	c.Equal("Unchecked", cells[0].Value, "a cell holding one check box reports its state as its value")
	c.Equal("", cells[1].Value, "a button has no state to report")
	screen.AccessibilityEvents(wnd)
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   cells[0].ID,
		Action: accessibility.Press,
	}))
	var state checkenum.Enum
	screen.Do(func() { state = box.State })
	c.Equal(checkenum.On, state, "pressing the cell should have toggled the check box within it")
	cellValueChanged := false
	for _, e := range screen.AccessibilityEvents(wnd) {
		if e.Kind == accessibility.ValueChanged && e.Node == cells[0].ID && e.New == "Checked" {
			cellValueChanged = true
		}
	}
	c.True(cellValueChanged, "the change should have been reported as a change to the cell's value")
	screen.Do(func() { box.State = checkenum.Off })

	// Pressing the check box through the table toggles it, and the next description says so under the same id.
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   boxNode.ID,
		Action: accessibility.Press,
	}))
	screen.Do(func() { state = box.State })
	c.Equal(checkenum.On, state, "the press should have toggled the check box")
	tree = screen.AccessibilityTree(wnd)
	again := tree.Node(boxNode.ID)
	c.True(again != nil, "the check box keeps its node id from one description to the next")
	if again != nil {
		c.Equal(checkenum.On, again.Checked)
	}

	// The button is a new panel every time the row is asked, so only its position within the cell can identify it; the
	// press reaches whichever one has been built to handle it.
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   buttonNode.ID,
		Action: accessibility.Press,
	}))
	c.Equal(1, clicks, "the press should have reached the button")
	tree = screen.AccessibilityTree(wnd)
	c.True(tree.Node(buttonNode.ID) != nil, "the button keeps its node id from one description to the next")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
