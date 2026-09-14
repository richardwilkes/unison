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
	node := axMustNode(c, screen.AccessibilityNodeFor(list))
	c.Equal(role.List, node.Role)
	c.True(node.Multiselectable)
	c.Equal(rowCount, node.RowCount)

	rows := axChildNodes(tree, node)
	c.True(len(rows) < rowCount/2,
		"only the visible rows and the selection should have been described, got %d", len(rows))
	first := axMustNode(c, axNodeWithRowIndex(tree, node, 0))
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

	last := axMustNode(c, axNodeWithRowIndex(tree, node, rowCount-1),
		"a selected row is described however far out of sight it is")
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
	last = axMustNode(c, axNodeWithRowIndex(tree, node, rowCount-1))
	c.Equal(rowCount-1, last.RowIndex)
	c.False(last.Offscreen, "the last row should have been scrolled into view")
	first = axMustNode(c, axNodeWithRowIndex(tree, node, 0),
		"the first row is still selected, so it is still described")
	c.True(first.Offscreen, "the first row has been scrolled out of view")

	// Back to the top, where the selection can be moved between two rows that both stay on the screen — and therefore
	// stay described — for as long as the requests take.
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   first.ID,
		Action: accessibility.ScrollIntoView,
	}))
	tree = screen.AccessibilityTree(wnd)
	node = screen.AccessibilityNodeFor(list)
	firstRow := axMustNode(c, axNodeWithRowIndex(tree, node, 0))
	secondRow := axMustNode(c, axNodeWithRowIndex(tree, node, 1))

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

	// The rows just past the bottom of the view port are described even though they cannot be seen, so that an
	// assistive technology stepping down through the rows has one to step onto; selecting one scrolls it into view, as
	// the arrow keys would, so that the next description reaches further down.
	rows = axChildNodes(tree, node)
	visibleRows := 0
	for _, row := range rows {
		if !row.Offscreen {
			visibleRows++
		}
	}
	c.True(visibleRows > 0 && visibleRows < rowCount/4, "a handful of rows fit in the view port, got %d", visibleRows)
	beyond := axMustNode(c, axNodeWithRowIndex(tree, node, visibleRows),
		"the row just past the view port must be described for an assistive technology to reach")
	c.True(beyond.Offscreen, "the row just past the view port cannot be seen yet")
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   beyond.ID,
		Action: accessibility.Select,
	}))
	tree = screen.AccessibilityTree(wnd)
	node = screen.AccessibilityNodeFor(list)
	beyond = axMustNode(c, axNodeWithRowIndex(tree, node, visibleRows))
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
	node := axMustNode(c, screen.AccessibilityNodeFor(list))
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
	node := axMustNode(c, screen.AccessibilityNodeFor(table))
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
	rowsByName := axTableRows(tree, node)
	selected := rowsByName["row 2 col 0"]
	c.True(selected != nil, "the row that was selected should still be described")
	if selected != nil {
		c.True(selected.Selected, "the selected row should say so")
	}

	// Scrolling a cell into view is the one thing a request aimed at a cell does to the cell rather than to its row.
	second := rowsByName["row 1 col 0"]
	c.True(second != nil)
	if second != nil {
		secondCells := axChildNodes(tree, second)
		c.Equal(2, len(secondCells), "a row has one cell per column")
		if len(secondCells) == 2 {
			c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
				Node:   secondCells[1].ID,
				Action: accessibility.ScrollIntoView,
			}))
		}
	}
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
	firstBefore := axMustNode(c, before["r0"])
	lastBefore := axMustNode(c, before["r2"])
	firstID := firstBefore.ID
	lastID := lastBefore.ID
	c.True(firstID != 0)
	c.Equal(0, firstBefore.RowIndex)
	c.Equal(2, lastBefore.RowIndex)

	// Handing the model its rows in the opposite order and syncing is what sorting or filtering a table amounts to.
	screen.Do(func() { table.SetRootRows([]*tableTestRow{rows[2], rows[1], rows[0]}) })
	tree = screen.AccessibilityTree(wnd)
	node = screen.AccessibilityNodeFor(table)
	after := axTableRows(tree, node)
	c.Equal(3, len(after))
	firstAfter := axMustNode(c, after["r0"])
	lastAfter := axMustNode(c, after["r2"])
	c.Equal(firstID, firstAfter.ID, "a row keeps its node when the rows around it move")
	c.Equal(lastID, lastAfter.ID)
	c.Equal(2, firstAfter.RowIndex, "the row that was first is now last")
	c.Equal(0, lastAfter.RowIndex)
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
	node := axMustNode(c, screen.AccessibilityNodeFor(table))
	c.Equal(role.Tree, node.Role, "a table whose rows can have children is a tree")
	c.Equal(1, node.RowCount, "a closed row shows none of its children")
	parentNode := axMustNode(c, axTableRows(tree, node)["parent"])
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
		node = axMustNode(c, screen.AccessibilityNodeFor(table))
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
		node = axMustNode(c, screen.AccessibilityNodeFor(table))
		c.Equal(1, node.RowCount, "pressing it again should have closed the row")
		parentNode = axMustNode(c, axTableRows(tree, node)["parent"])
	}

	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   parentNode.ID,
		Action: accessibility.Expand,
	}))
	tree = screen.AccessibilityTree(wnd)
	node = axMustNode(c, screen.AccessibilityNodeFor(table))
	c.Equal(3, node.RowCount, "expanding the row should have brought its children into the table")
	rowNodes := axTableRows(tree, node)
	c.Equal(3, len(rowNodes))
	parentRow := axMustNode(c, rowNodes["parent"])
	c.True(parentRow.Expanded)
	c.Equal(1, parentRow.Level)
	childRow := axMustNode(c, rowNodes["child0"])
	c.Equal(2, childRow.Level, "a child row is one level deeper than its parent")
	c.Equal(1, childRow.RowIndex)

	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   parentNode.ID,
		Action: accessibility.Collapse,
	}))
	tree = screen.AccessibilityTree(wnd)
	node = axMustNode(c, screen.AccessibilityNodeFor(table))
	c.Equal(1, node.RowCount, "collapsing the row should have taken its children away again")
	c.False(axMustNode(c, axTableRows(tree, node)["parent"]).Expanded)
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
	node := axMustNode(c, screen.AccessibilityNodeFor(table))
	c.Equal(2, node.RowCount, "the filter shows the row that passed and the container holding it")
	parentNode := axMustNode(c, axTableRows(tree, node)["parent"])
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
	node = axMustNode(c, screen.AccessibilityNodeFor(table))
	c.Equal(2, node.RowCount, "the rows the filter shows must not have changed")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTableAccessibilityWithNoColumns verifies that a table holding rows but no columns never asks a row for the data
// of a column that does not exist. Every other caller of CellDataForSort is driven by a real column index, so a model
// is entitled to reach straight for the column it was handed.
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
	node := axMustNode(c, screen.AccessibilityNodeFor(table))
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
// that can be seen and caps how many it describes solely because they are selected. Nothing here focuses a cell; that a
// row out of view is described because it holds the focused one is TestTableAccessibilityFocusedCell's job.
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
	node := axMustNode(c, screen.AccessibilityNodeFor(table))
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
	beyond := axMustNode(c, axNodeWithRowIndex(tree, node, visible),
		"the row just past the view port must be described")
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
	beyond = axMustNode(c, axNodeWithRowIndex(tree, node, visible))
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
	node := axMustNode(c, screen.AccessibilityNodeFor(e.table))
	rows := axTableRows(tree, node)
	last := axMustNode(c, rows["r"+strconv.Itoa(rowCount-1)],
		"the row holding the focused cell must be described")
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
	node := axMustNode(c, screen.AccessibilityNodeFor(header))
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
	if len(columns) != 2 {
		return
	}
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
	if len(columns) != 2 {
		return
	}
	c.Equal(accessibility.SortDescending, columns[0].Sort, "pressing it again turned the sort around")

	// A column that is sorted on after the primary one is not reported as sorted: the order the rows visibly follow is
	// the primary column's, and it is the only one the header draws an indicator for.
	screen.Do(func() {
		header.ColumnHeaders[1].SetSortState(unison.SortState{Order: 1, Ascending: true, Sortable: true})
	})
	tree = screen.AccessibilityTree(wnd)
	node = screen.AccessibilityNodeFor(header)
	columns = axChildNodes(tree, node)
	c.Equal(2, len(columns))
	if len(columns) == 2 {
		c.Equal(accessibility.SortDescending, columns[0].Sort, "the primary sort column still says which way it runs")
		c.Equal(accessibility.SortNone, columns[1].Sort, "a secondary sort key is not what the rows are read in")
	}

	// A table and its header are separate panels, so the table says which header describes its columns; both platform
	// adapters look there before falling back to searching the window.
	tableNode := screen.AccessibilityNodeFor(table)
	c.True(tableNode != nil)
	if tableNode != nil {
		c.Equal(1, len(tableNode.Controls), "the table points at its header")
		if len(tableNode.Controls) == 1 {
			c.Equal(node.ID, tableNode.Controls[0])
		}
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// axButtonHeader is a table column header built around a button rather than a label, which is what a header with
// something in it to press looks like. It pairs a SetTitle with the String() every panel answers with the name of its
// own Go type, which is what a column header must not be announced as.
type axButtonHeader struct {
	*unison.Button
	sortState unison.SortState
}

// newAxButtonHeader returns a column header that is a button, counting the clicks it receives.
func newAxButtonHeader(title string, clicks *int) *axButtonHeader {
	h := &axButtonHeader{
		Button:    unison.NewButton(),
		sortState: unison.SortState{Order: -1, Ascending: true, Sortable: true},
	}
	h.Self = h
	h.ClickAnimationTime = 0
	h.SetTitle(title)
	h.ClickCallback = func() { *clicks++ }
	return h
}

// SortState implements unison.TableColumnHeader.
func (h *axButtonHeader) SortState() unison.SortState { return h.sortState }

// SetSortState implements unison.TableColumnHeader.
func (h *axButtonHeader) SetSortState(state unison.SortState) { h.sortState = state }

// Less implements unison.TableColumnHeader.
func (h *axButtonHeader) Less() func(a, b string) bool { return nil }

// TestTableHeaderAccessibilityCustomColumnHeader verifies what is said about a column header that is not simply a
// label. It must not be announced as the name of the Go type it was written as, which is what every panel answers
// String() with, and whatever it holds has to be reachable: a header with a button in it is a button an assistive
// technology can find and press, not a leaf with the button hidden inside it.
func TestTableHeaderAccessibilityCustomColumnHeader(t *testing.T) {
	c := check.New(t)
	var table *unison.Table[*tableTestRow]
	var header *unison.TableHeader[*tableTestRow]
	var wnd *unison.Window
	clicks := 0
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			table = axNewTable(flatRows(3)...)
			header = unison.NewTableHeader[*tableTestRow](table,
				newAxButtonHeader("Pick", &clicks),
				unison.NewTableColumnHeader[*tableTestRow]("Value", "", nil))
			scroller := axScroller(table, geom.NewSize(300, 200))
			scroller.SetColumnHeader(header)
			wnd = newHeadlessWindow(t, "custom header", geom.NewRect(10, 10, 400, 400), axColumn(scroller))
		}))
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	node := axMustNode(c, screen.AccessibilityNodeFor(header))
	columns := axChildNodes(tree, node)
	c.Equal(2, len(columns))
	if len(columns) != 2 {
		return
	}
	c.Equal(role.ColumnHeader, columns[0].Role)
	c.Equal("Pick", columns[0].Name, "a column header is named by what it holds, never by its Go type")
	c.Equal("Value", columns[1].Name, "a header that is nothing but a label is still named by its text")
	c.Equal(0, len(axUnignoredNodes(tree, columns[1])), "such a header has nothing within it to describe")

	inside := axUnignoredNodes(tree, columns[0])
	c.Equal(1, len(inside), "the button in the header should have been described: %v", axNodeNames(inside))
	if len(inside) != 1 {
		return
	}
	c.Equal(role.Button, inside[0].Role)
	c.Equal("Pick", inside[0].Name)
	c.True(inside[0].Actions.Has(accessibility.Press))
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   inside[0].ID,
		Action: accessibility.Press,
	}))
	var count int
	screen.Do(func() { count = clicks })
	c.Equal(1, count, "pressing the button in the header should have clicked it")

	// The column header itself still sorts the table, which is what pressing a column header does.
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   columns[0].ID,
		Action: accessibility.Press,
	}))
	tree = screen.AccessibilityTree(wnd)
	node = screen.AccessibilityNodeFor(header)
	columns = axChildNodes(tree, node)
	c.Equal(2, len(columns))
	if len(columns) == 2 {
		c.Equal(accessibility.SortAscending, columns[0].Sort, "pressing the header sorted the table on that column")
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// axLabelHeaderWithChild is a table column header written the documented way — by embedding a *Label and pointing Self
// at itself — that also holds a child panel. It is the shape a header that pairs a title with a control of its own
// takes, and the one that used to have the column's title described twice.
type axLabelHeaderWithChild struct {
	*unison.Label
	sortState unison.SortState
}

// newAxLabelHeaderWithChild returns a column header built around a label, holding a button beneath it.
func newAxLabelHeaderWithChild(title string) *axLabelHeaderWithChild {
	h := &axLabelHeaderWithChild{
		Label:     unison.NewLabel(),
		sortState: unison.SortState{Order: -1, Ascending: true, Sortable: true},
	}
	h.Self = h
	h.SetTitle(title)
	button := unison.NewButton()
	button.ClickAnimationTime = 0
	button.SetTitle("Filter")
	h.AddChild(button)
	return h
}

// SortState implements unison.TableColumnHeader.
func (h *axLabelHeaderWithChild) SortState() unison.SortState { return h.sortState }

// SetSortState implements unison.TableColumnHeader.
func (h *axLabelHeaderWithChild) SetSortState(state unison.SortState) { h.sortState = state }

// Less implements unison.TableColumnHeader.
func (h *axLabelHeaderWithChild) Less() func(a, b string) bool { return nil }

// TestTableHeaderAccessibilityLabelHeaderWithChildrenIsOneElement verifies that a column header built around a label is
// described as the single element the column header node already is, even when it has children. Such a header resolves
// to static text, which is one element however many panels it is built from, so the snapshot never visits what is
// beneath it; describing the header's own panel under the column would add a node carrying the column's title a second
// time — the title heard twice — while reaching none of the content that node was added for.
func TestTableHeaderAccessibilityLabelHeaderWithChildrenIsOneElement(t *testing.T) {
	c := check.New(t)
	var table *unison.Table[*tableTestRow]
	var header *unison.TableHeader[*tableTestRow]
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			table = axNewTable(flatRows(3)...)
			header = unison.NewTableHeader[*tableTestRow](table,
				newAxLabelHeaderWithChild("Named"),
				unison.NewTableColumnHeader[*tableTestRow]("Value", "", nil))
			scroller := axScroller(table, geom.NewSize(300, 200))
			scroller.SetColumnHeader(header)
			wnd = newHeadlessWindow(t, "label header", geom.NewRect(10, 10, 400, 400), axColumn(scroller))
		}))
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	node := axMustNode(c, screen.AccessibilityNodeFor(header))
	columns := axChildNodes(tree, node)
	c.Equal(2, len(columns))
	if len(columns) != 2 {
		return
	}
	c.Equal(role.ColumnHeader, columns[0].Role)
	c.Equal("Named", columns[0].Name, "the embedded label's text names the column")
	c.Equal(0, len(axChildNodes(tree, columns[0])),
		"a header that is described as static text has nothing beneath it worth describing: %v",
		axNodeNames(axChildNodes(tree, columns[0])))
	names := 0
	tree.Walk(func(n *accessibility.Node) bool {
		if n.Name == "Named" {
			names++
		}
		return true
	})
	c.Equal(1, names, "the column's title must be described once rather than on a node of its own as well")
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
	containerNode := axMustNode(c, screen.AccessibilityNodeFor(container))
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
	c.Equal("The first one, Modified", tabs[0].Description,
		"the marker a sighted person sees is said in words instead")
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
	if len(tabs) != 2 {
		return
	}
	c.False(tabs[0].Selected)
	c.True(tabs[1].Selected, "the selection should have moved with the current dockable")

	// Closing is the one thing the button on a tab does, and an assistive technology has nothing else to reach it by,
	// so pressing that node has to run it.
	closers := axChildNodes(tree, tabs[0])
	c.Equal(1, len(closers), "the tab still holds its close button")
	if len(closers) == 1 {
		c.Equal("Close", closers[0].Name)
		c.True(closers[0].Actions.Has(accessibility.Press))
		c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
			Node:   closers[0].ID,
			Action: accessibility.Press,
		}))
		var closed bool
		screen.Do(func() { closed = first.closed })
		c.True(closed, "pressing a tab's close button should have asked its dockable to close")
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
	node := axMustNode(c, screen.AccessibilityNodeFor(markdown))
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

// TestMarkdownHeadingWithLinkAccessibility verifies that a heading holding a link is still one heading, named by the
// whole of its text, while the link within it stays something an assistive technology can move to, hear as a link and
// press. Folding a heading's content into its name is right for the text a heading usually is, and wrong for anything
// in it that has to be reached.
func TestMarkdownHeadingWithLinkAccessibility(t *testing.T) {
	c := check.New(t)
	const content = "## A [linked](https://example.com/x) heading\n\n### Plain heading\n"
	var markdown *unison.Markdown
	var wnd *unison.Window
	followed := ""
	screen := startHeadless(t, unison.HeadlessConfig{Width: 800, Height: 800},
		unison.StartupFinishedCallback(func() {
			markdown = unison.NewMarkdown(false)
			markdown.LinkHandler = func(_ unison.Paneler, target string) { followed = target }
			markdown.SetContent(content, 400)
			wnd = newHeadlessWindow(t, "heading links", geom.NewRect(10, 10, 600, 600), axColumn(markdown))
		}))
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	headings := axNodesWithRole(tree, role.Heading)
	c.Equal(2, len(headings))
	if len(headings) != 2 {
		return
	}
	c.Equal(2, headings[0].Level)
	c.Equal("A linked heading", headings[0].Name,
		"the heading reads as the whole of its text, with the link's words in their place and no seam where the "+
			"labels it is built from meet")
	c.Equal(0, len(headings[1].Children), "a heading of nothing but text is still one element")

	link := axNamed(tree, "linked")
	c.True(link != nil, "the link inside the heading should have been described")
	if link == nil {
		return
	}
	c.Equal(role.Link, link.Role)
	c.Equal("https://example.com/x", link.Description, "where the link leads is worth hearing")
	c.True(link.Actions.Has(accessibility.Press))
	c.Equal(headings[0].ID, tree.UnignoredParent(link.ID), "the link is reached through the heading it is in")

	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   link.ID,
		Action: accessibility.Press,
	}))
	var went string
	screen.Do(func() { went = followed })
	c.Equal("https://example.com/x", went, "pressing the link inside the heading should have followed it")
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
		c.False(never.Actions.Has(accessibility.Press), "and cannot be asked to do what it would refuse")
		c.True(never.Actions.Has(accessibility.ScrollIntoView), "though it can still be brought into view")
	}
	nested := byName["More"]
	c.True(nested != nil)
	if nested != nil {
		c.True(nested.Expandable, "an item with a sub-menu opens something")
	}
	c.True(len(axNodesWithRole(tree, role.Separator)) > 0, "the separator in the menu is described as one")

	// An item is highlighted by the pointer merely passing over it, whether or not it can be chosen: the item's panel
	// is never the thing that is disabled, so nothing stops the highlight. A disabled item must still not be reported
	// as holding the focus, which paired with saying it cannot take the focus is the one thing no tree may say — and
	// pointing the window's focus at it would offer an assistive technology a move to a node that would then refuse it.
	if never != nil {
		screen.MouseMove(axScreenPoint(screen, wnd, never), mod.None)
		tree = screen.AccessibilityTree(wnd)
		hoveredDisabled := tree.Node(never.ID)
		c.True(hoveredDisabled != nil)
		if hoveredDisabled != nil {
			c.True(hoveredDisabled.Disabled)
			c.False(hoveredDisabled.Focused, "a disabled item must not report that it holds the focus")
			c.False(hoveredDisabled.Focusable, "nor that it could take it")
			c.False(hoveredDisabled.Actions.Has(accessibility.Focus))
		}
		c.NotEqual(never.ID, tree.Focus, "and the window must not point the focus at it")
		focusedNodes := axFocusedNodes(tree)
		c.Equal(0, len(focusedNodes),
			"nothing else in this window can hold the focus, so nothing may claim it: %v", axNodeNames(focusedNodes))
	}

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

// axUnignoredNodes returns the nodes an assistive technology is shown beneath a node, which is what every adapter
// builds its child lists from: the children that are not ignored, with the children of the ones that are spliced in.
func axUnignoredNodes(tree *accessibility.Tree, node *accessibility.Node) []*accessibility.Node {
	if tree == nil || node == nil {
		return nil
	}
	ids := tree.UnignoredChildren(node.ID)
	nodes := make([]*accessibility.Node, 0, len(ids))
	for _, id := range ids {
		if n := tree.Node(id); n != nil {
			nodes = append(nodes, n)
		}
	}
	return nodes
}

// axMenuTestBar fills a menu bar with the menu the menu tests choose from: an ordinary item, a separator, an item with
// a check state that has been turned off, and an item with a sub-menu.
func axMenuTestBar(bar unison.Menu, baseID int, activated *int) {
	f := bar.Factory()
	edit := f.NewMenu(baseID, "Edit", nil)
	edit.InsertItem(-1, f.NewItem(baseID+1, "Cut", unison.KeyBinding{}, nil,
		func(_ unison.MenuItem) { *activated++ }))
	edit.InsertSeparator(-1, false)
	wrap := f.NewItem(baseID+2, "Wrap", unison.KeyBinding{}, nil, nil)
	wrap.SetCheckState(checkenum.Off)
	edit.InsertItem(-1, wrap)
	more := f.NewMenu(baseID+3, "More", nil)
	more.InsertItem(-1, f.NewItem(baseID+4, "Deeper", unison.KeyBinding{}, nil, nil))
	edit.InsertMenu(-1, more)
	bar.InsertMenu(-1, edit)
}

// TestMenuAccessibilityChildrenAreTheItems verifies that what an assistive technology finds inside a menu is the items
// of that menu. Every menu panel lays its items out inside a scroll panel, for the menus too tall to fit, and an
// adapter builds its child list from the unignored children of the node, so a scroll area left in the way would be the
// only thing an assistive technology could find in any menu in the application. The popup menu's choices are the same
// thing arrived at another way, and while they are showing the popup says which menu it opened.
func TestMenuAccessibilityChildrenAreTheItems(t *testing.T) {
	c := check.New(t)
	const menuID = unison.UserBaseID + 100
	var popup *unison.PopupMenu[string]
	var wnd *unison.Window
	activated := 0
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			label := unison.NewLabel()
			label.SetTitle("Choices:")
			popup = unison.NewPopupMenu[string]()
			popup.AddItem("One")
			popup.AddItem("Two")
			popup.SelectIndex(0)
			wnd = newHeadlessWindow(t, "menu children", geom.NewRect(10, 10, 400, 300), axColumn(label, popup))
			if wnd == nil {
				return
			}
			unison.DefaultMenuFactory().BarForWindow(wnd, func(bar unison.Menu) {
				axMenuTestBar(bar, menuID, &activated)
			})
			wnd.ToFront()
		}))
	c.NotNil(wnd)
	screen.Sync()

	tree := screen.AccessibilityTree(wnd)
	bars := axNodesWithRole(tree, role.MenuBar)
	c.Equal(1, len(bars), "the window has an in-window menu bar")
	if len(bars) != 1 {
		return
	}
	titles := axUnignoredNodes(tree, bars[0])
	c.Equal(1, len(titles), "the bar holds the one menu that was added to it: %v", axNodeNames(titles))
	if len(titles) != 1 {
		return
	}
	c.Equal(role.MenuItem, titles[0].Role, "what an assistive technology finds in a menu bar is its titles")
	c.Equal("Edit", titles[0].Name)

	screen.Click(axScreenPoint(screen, wnd, titles[0]))
	tree = screen.AccessibilityTree(wnd)
	menus := axNodesWithRole(tree, role.Menu)
	c.Equal(1, len(menus), "the menu that opened should have been described")
	if len(menus) != 1 {
		return
	}
	items := axUnignoredNodes(tree, menus[0])
	c.Equal(4, len(items), "what an assistive technology finds in a menu is its items: %v", axNodeNames(items))
	if len(items) != 4 {
		return
	}
	c.Equal("Cut", items[0].Name)
	c.Equal(role.Separator, items[1].Role, "the separator keeps its place among the items")
	c.Equal("Wrap", items[2].Name)
	c.True(items[2].HasCheck, "an item that has been unchecked is still something with a check state")
	c.Equal(checkenum.Off, items[2].Checked)
	c.Equal("More", items[3].Name)
	c.True(items[3].Expandable)
	c.False(items[3].Expanded, "nothing has opened the sub-menu yet")
	c.True(items[3].Actions.Has(accessibility.Expand))
	c.True(items[3].Actions.Has(accessibility.Collapse),
		"an item with a sub-menu offers both, whatever state that sub-menu is in")

	// The sub-menu is a menu like any other, and what is in it is its own items.
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   items[3].ID,
		Action: accessibility.Expand,
	}))
	tree = screen.AccessibilityTree(wnd)
	menus = axNodesWithRole(tree, role.Menu)
	c.Equal(2, len(menus), "the sub-menu should have opened")
	var sub *accessibility.Node
	for _, one := range menus {
		if one.Name == "More" {
			sub = one
		}
	}
	c.True(sub != nil, "the sub-menu named after the item that opened it should be among %v", axNodeNames(menus))
	if sub == nil {
		return
	}
	c.Equal(role.Menu, sub.Role)
	deeper := axUnignoredNodes(tree, sub)
	c.Equal(1, len(deeper), "the sub-menu holds the one item it was given: %v", axNodeNames(deeper))
	if len(deeper) == 1 {
		c.Equal("Deeper", deeper[0].Name)
	}

	// The item that opened it now says its sub-menu is showing, and still offers the collapse that closes it again,
	// which is the state the offer actually matters in.
	openItem := axMustNode(c, tree.Node(items[3].ID), "the item that opened the sub-menu keeps its id")
	c.True(openItem.Expanded, "an item whose sub-menu is showing says so")
	c.True(openItem.Actions.Has(accessibility.Collapse), "an open sub-menu has to be closable again")

	// Collapsing the item that opened it takes the sub-menu away again, leaving the menu it belongs to open.
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   items[3].ID,
		Action: accessibility.Collapse,
	}))
	tree = screen.AccessibilityTree(wnd)
	c.Equal(1, len(axNodesWithRole(tree, role.Menu)), "collapsing the item should have closed its sub-menu")

	screen.KeyPress(unison.KeyEscape, mod.None)
	tree = screen.AccessibilityTree(wnd)
	c.Equal(0, len(axNodesWithRole(tree, role.Menu)), "the menus should all have closed")

	// A popup menu's choices are shown in a menu of the same kind, so they are reached the same way.
	popupNode := screen.AccessibilityNodeFor(popup)
	c.True(popupNode != nil)
	if popupNode == nil {
		return
	}
	c.True(popupNode.Expandable, "a popup with choices to show can be expanded")
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   popupNode.ID,
		Action: accessibility.Expand,
	}))
	tree = screen.AccessibilityTree(wnd)
	menus = axNodesWithRole(tree, role.Menu)
	c.Equal(1, len(menus), "the popup's choices should have been described")
	if len(menus) != 1 {
		return
	}
	c.Equal("Choices", menus[0].Name, "the menu a popup opens is named after the popup")
	choices := axUnignoredNodes(tree, menus[0])
	c.Equal(2, len(choices), "what an assistive technology finds in a popup's menu is its choices: %v",
		axNodeNames(choices))
	if len(choices) == 2 {
		c.Equal("One", choices[0].Name)
		c.Equal("Two", choices[1].Name)
	}
	popupNode = screen.AccessibilityNodeFor(popup)
	c.True(popupNode != nil)
	if popupNode != nil {
		c.Equal(1, len(popupNode.Controls), "the popup says which menu it opened")
		if len(popupNode.Controls) == 1 {
			c.Equal(menus[0].ID, popupNode.Controls[0])
		}
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestMenuAccessibilityKeyboardNavigation verifies that choosing from a menu with the arrow keys moves what the window
// reports as the focus, item by item, with exactly one node focused at a time. The mouse path and the keyboard path
// move the highlight through different code — the keys move the menu panel's own index and flip each item's over flag
// as they go — and only the highlight is what an assistive technology is told the person is choosing from.
func TestMenuAccessibilityKeyboardNavigation(t *testing.T) {
	c := check.New(t)
	const menuID = unison.UserBaseID + 200
	var wnd *unison.Window
	activated := 0
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			wnd = newHeadlessWindow(t, "menu keys", geom.NewRect(10, 10, 400, 300), unison.NewPanel())
			if wnd == nil {
				return
			}
			unison.DefaultMenuFactory().BarForWindow(wnd, func(bar unison.Menu) {
				axMenuTestBar(bar, menuID, &activated)
			})
			wnd.ToFront()
		}))
	c.NotNil(wnd)
	screen.Sync()

	tree := screen.AccessibilityTree(wnd)
	title := axNamed(tree, "Edit")
	c.True(title != nil)
	if title == nil {
		return
	}
	screen.Click(axScreenPoint(screen, wnd, title))
	tree = screen.AccessibilityTree(wnd)
	c.Equal(1, len(axNodesWithRole(tree, role.Menu)), "the menu should have opened")
	c.Equal(title.ID, tree.Focus, "with nothing in the menu pointed at yet, the title is what is being chosen from")

	// Down moves onto the first item, and each step is the one node in the window that reports being focused.
	axPressMenuKey(c, screen, wnd, unison.KeyDown, "Cut")
	// The separator is not something that can be chosen, so it is passed over rather than landed on.
	axPressMenuKey(c, screen, wnd, unison.KeyDown, "Wrap")
	axPressMenuKey(c, screen, wnd, unison.KeyUp, "Cut")
	axPressMenuKey(c, screen, wnd, unison.KeyDown, "Wrap")

	// Moving onto an item with a sub-menu opens it and points at what is inside, which is where the focus goes.
	screen.KeyPress(unison.KeyDown, mod.None)
	tree = screen.AccessibilityTree(wnd)
	c.Equal(2, len(axNodesWithRole(tree, role.Menu)), "arrowing onto an item with a sub-menu opens it")
	deeper := axNamed(tree, "Deeper")
	c.True(deeper != nil)
	if deeper == nil {
		return
	}
	focused := axFocusedNodes(tree)
	c.Equal(1, len(focused), "only one node in a window may report being focused: %v", axNodeNames(focused))
	c.Equal(deeper.ID, tree.Focus, "the item the newest menu is pointing at is what is being chosen from")

	// Escape closes the sub-menu, and what is being chosen from is the item it was opened from.
	screen.KeyPress(unison.KeyEscape, mod.None)
	tree = screen.AccessibilityTree(wnd)
	c.Equal(1, len(axNodesWithRole(tree, role.Menu)), "the sub-menu should have closed")
	focused = axFocusedNodes(tree)
	c.Equal(1, len(focused), "only one node in a window may report being focused: %v", axNodeNames(focused))
	if len(focused) == 1 {
		c.Equal("More", focused[0].Name)
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// axPressMenuKey presses a key against an open menu and checks that the item it moved onto is the one node the window
// reports as focused.
func axPressMenuKey(c check.Checker, screen *unison.HeadlessScreen, wnd *unison.Window, code unison.KeyCode,
	expected string,
) {
	screen.KeyPress(code, mod.None)
	tree := screen.AccessibilityTree(wnd)
	focused := axFocusedNodes(tree)
	c.Equal(1, len(focused), "only one node in a window may report being focused: %v", axNodeNames(focused))
	if len(focused) != 1 {
		return
	}
	c.Equal(expected, focused[0].Name, "the item the menu is pointing at is what is being chosen from")
	c.Equal(focused[0].ID, tree.Focus)
}

// axFocusedNodes returns every node in a tree that reports being focused, other than the window itself, whose Focused
// says only that the window is the active one.
func axFocusedNodes(tree *accessibility.Tree) []*accessibility.Node {
	var nodes []*accessibility.Node
	tree.Walk(func(n *accessibility.Node) bool {
		if n.Focused && n.ID != tree.Root {
			nodes = append(nodes, n)
		}
		return true
	})
	return nodes
}

// axNodeNames returns the name and role of each node, so that a failure says which nodes were found rather than where
// they happen to sit in memory.
func axNodeNames(nodes []*accessibility.Node) []string {
	names := make([]string, 0, len(nodes))
	for _, n := range nodes {
		names = append(names, n.Role.String()+" "+strconv.Quote(n.Name))
	}
	return names
}

// TestMenuBarAccessibilityFocus verifies that the pointer crossing a menu bar leaves the keyboard focus where it is,
// and that once a menu is open exactly one node stands for the focus, whether that is an item of the menu or the title
// on the bar that opened it. A menu item is highlighted by the pointer merely passing over it, so without that rule
// hovering a title would have the window report two focused nodes — the title and whatever really holds the focus —
// which is a tree an assistive technology cannot make sense of.
func TestMenuBarAccessibilityFocus(t *testing.T) {
	c := check.New(t)
	const (
		menuID = unison.UserBaseID + iota
		cutID
	)
	var field *unison.Field
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			field = unison.NewField()
			wnd = newHeadlessWindow(t, "menu focus", geom.NewRect(10, 10, 400, 300), axColumn(field))
			if wnd == nil {
				return
			}
			unison.DefaultMenuFactory().BarForWindow(wnd, func(bar unison.Menu) {
				f := bar.Factory()
				edit := f.NewMenu(menuID, "Edit", nil)
				edit.InsertItem(-1, f.NewItem(cutID, "Cut", unison.KeyBinding{}, nil, nil))
				bar.InsertMenu(-1, edit)
			})
			wnd.ToFront()
		}))
	c.NotNil(wnd)
	screen.Sync()

	screen.Click(screen.PanelCenter(field))
	tree := screen.AccessibilityTree(wnd)
	fieldNode := screen.AccessibilityNodeFor(field)
	c.True(fieldNode != nil)
	if fieldNode == nil {
		return
	}
	c.Equal(fieldNode.ID, tree.Focus, "the field holds the keyboard focus")
	title := axNamed(tree, "Edit")
	c.True(title != nil)
	if title == nil {
		return
	}

	// Merely moving the pointer onto the bar highlights the title, which is not a focus change: nothing has been opened
	// and the field still has the keys.
	screen.MouseMove(axScreenPoint(screen, wnd, title), mod.None)
	tree = screen.AccessibilityTree(wnd)
	c.Equal(fieldNode.ID, tree.Focus, "hovering a menu bar title must not move the focus")
	focused := axFocusedNodes(tree)
	c.Equal(1, len(focused), "only one node in a window may report being focused: %v", axNodeNames(focused))
	if len(focused) == 1 {
		c.Equal(fieldNode.ID, focused[0].ID)
	}

	// Clicking it opens the menu with the pointer still on the title and nothing in the menu highlighted, which is when
	// the title itself stands for what the person is choosing from.
	screen.Click(axScreenPoint(screen, wnd, title))
	tree = screen.AccessibilityTree(wnd)
	c.Equal(1, len(axNodesWithRole(tree, role.Menu)), "the menu should have opened")
	focused = axFocusedNodes(tree)
	c.Equal(1, len(focused), "only one node in a window may report being focused: %v", axNodeNames(focused))
	if len(focused) == 1 {
		c.Equal(title.ID, focused[0].ID, "the title whose menu is open is what is being chosen from")
		c.True(focused[0].Focusable, "a node that says it is focused must say it can be")
		c.Equal(title.ID, tree.Focus)
	}

	// Moving onto an item of the open menu hands the focus to it, and the title gives it up.
	cut := axNamed(tree, "Cut")
	c.True(cut != nil)
	if cut == nil {
		return
	}
	screen.MouseMove(axScreenPoint(screen, wnd, cut), mod.None)
	tree = screen.AccessibilityTree(wnd)
	focused = axFocusedNodes(tree)
	c.Equal(1, len(focused), "only one node in a window may report being focused: %v", axNodeNames(focused))
	if len(focused) == 1 {
		c.Equal(cut.ID, focused[0].ID, "the item the menu is pointing at is what is being chosen from")
		c.True(focused[0].Focusable)
	}
	c.Equal(cut.ID, tree.Focus)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTooltipAccessibility verifies that a tooltip, while it is showing, is described as one — a child of the window
// rather than of the panel it belongs to, since that is where it is drawn — and that the whole of what a tooltip says,
// secondary text included, is what describes the panel it belongs to, which is the only way that text is ever heard by
// someone who cannot see the tooltip itself.
func TestTooltipAccessibility(t *testing.T) {
	c := check.New(t)
	var button *unison.Button
	var label *unison.Label
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			button = unison.NewButton()
			button.SetTitle("Save")
			button.Tooltip = unison.NewTooltipWithText("Write the file out")
			// Without this the tooltip would not appear until the delay a person's pause has to last.
			button.TooltipImmediate = true
			label = unison.NewLabel()
			label.SetTitle("Total")
			label.Tooltip = unison.NewTooltipWithSecondaryText("Primary tip", "Secondary tip")
			wnd = newHeadlessWindow(t, "tips", geom.NewRect(10, 10, 300, 200), axColumn(button, label))
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
		c.Equal(0, len(tips[0].Children),
			"the labels the text is drawn with are hidden, so it is not announced a second time")
	}
	c.Equal("Write the file out", axMustNode(c, screen.AccessibilityNodeFor(button)).Description,
		"the tooltip is the button's description as well")
	labelNode := axMustNode(c, screen.AccessibilityNodeFor(label))
	c.Equal("Primary tip\nSecondary tip", labelNode.Description,
		"the secondary text is part of what the tooltip says, so it is part of the description")
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
	node := axMustNode(c, screen.AccessibilityNodeFor(table))
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

// TestTableFlatFilterIgnoresTheOpenAndCloseKeys verifies that the left and right arrow keys leave the rows' open states
// alone while a flat filter is applied. Such a filter shows nothing beneath any row, so no disclosure triangle is drawn
// and there is nothing for opening or closing a row to show; the keys used to change those states regardless, which
// only became visible once the filter was lifted, while the same request from an assistive technology was refused and
// ApplyFilter's own promise is that no modifications to the row data are performed while one is in force.
func TestTableFlatFilterIgnoresTheOpenAndCloseKeys(t *testing.T) {
	c := check.New(t)
	var table *unison.Table[*tableTestRow]
	var closedRow, openRow *tableTestRow
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 400},
		unison.StartupFinishedCallback(func() {
			closedRow = newTableTestRow("closed")
			closedRow.SetChildren([]*tableTestRow{newTableTestRow("closedChild")})
			openRow = newTableTestRow("open")
			openRow.SetChildren([]*tableTestRow{newTableTestRow("openChild")})
			openRow.SetOpen(true)
			table = axNewTable(closedRow, openRow)
			// The filter keeps the rows it returns false for, so both containers are shown, side by side in a flat
			// list with neither one's children beneath it.
			table.ApplyFilter(func(row *tableTestRow) bool {
				return row.ID() != "closed" && row.ID() != "open"
			})
			wnd = newHeadlessWindow(t, "flat filter keys", geom.NewRect(10, 10, 400, 300), axColumn(table))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() {
		wnd.ToFront()
		table.RequestFocus()
		table.SelectByIndex(0, 1)
	}))
	var rowCount int
	screen.Do(func() { rowCount = table.LastRowIndex() + 1 })
	c.Equal(2, rowCount, "a flat filter shows the rows that passed and nothing beneath any of them")

	screen.KeyPress(unison.KeyRight, mod.None)
	var isOpen bool
	screen.Do(func() { isOpen = closedRow.IsOpen() })
	c.False(isOpen, "the right arrow must not open a row whose children the filter is hiding anyway")

	screen.KeyPress(unison.KeyLeft, mod.None)
	screen.Do(func() { isOpen = openRow.IsOpen() })
	c.True(isOpen, "nor may the left arrow close one")

	// Lifting the filter shows that nothing was changed behind it.
	screen.Do(func() { table.ApplyFilter(nil) })
	screen.Do(func() { rowCount = table.LastRowIndex() + 1 })
	c.Equal(3, rowCount, "the open row still shows its child and the closed one still shows none")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTableAccessibilityCellPressReachesTheWidgetOfferingIt verifies that a press aimed at a cell holding a field and a
// button reaches the button. Which panels within a cell may be handed the request is decided by their shape — a *Field
// sets both halves of a click, so it looks pressable — while the press the cell advertises comes from what its content
// was described as offering, and a field takes Press out of its own description. The field comes first within the cell,
// so a press handed out by position alone landed on it: nothing would have been pressed, the request would still have
// been reported as carried out, and the field would have been left installed as the table's focused cell, quietly
// starting an editing session nobody asked for.
func TestTableAccessibilityCellPressReachesTheWidgetOfferingIt(t *testing.T) {
	c := check.New(t)
	var table *unison.Table[*tableTestRow]
	var wnd *unison.Window
	clicks := 0
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 400},
		unison.StartupFinishedCallback(func() {
			row := newTableTestRow("r0")
			row.cellFactory = func(_, _ int) unison.Paneler {
				cell := unison.NewPanel()
				cell.SetLayout(&unison.FlexLayout{Columns: 2, HSpacing: unison.StdHSpacing})
				field := unison.NewField()
				field.SetText("editable")
				cell.AddChild(field)
				button := unison.NewButton()
				button.ClickAnimationTime = 0
				button.SetTitle("Go")
				button.ClickCallback = func() { clicks++ }
				cell.AddChild(button)
				return cell
			}
			table = axNewTable(row)
			wnd = newHeadlessWindow(t, "cell press", geom.NewRect(10, 10, 500, 200), table)
		}))
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	node := axMustNode(c, screen.AccessibilityNodeFor(table))
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
	content := axUnignoredNodes(tree, cells[0])
	c.Equal(2, len(content), "the field and the button are both described within the cell: %v", axNodeNames(content))
	if len(content) != 2 {
		return
	}
	c.Equal(role.TextField, content[0].Role, "the field is the first thing in the cell")
	c.False(content[0].Actions.Has(accessibility.Press), "a field withdraws the press, since it has nothing to press")
	c.Equal(role.Button, content[1].Role)
	c.True(content[1].Actions.Has(accessibility.Press))
	c.True(cells[0].Actions.Has(accessibility.Press), "the button is why the cell offers a press at all")

	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   cells[0].ID,
		Action: accessibility.Press,
	}))
	var count, focusedRow, focusedCol int
	screen.Do(func() {
		count = clicks
		focusedRow, focusedCol = table.FocusedCell()
	})
	c.Equal(1, count, "the press should have gone to the button rather than the field ahead of it")
	c.Equal(-1, focusedRow, "nothing should have been left installed as the focused cell")
	c.Equal(-1, focusedCol)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestListAccessibilitySelectionCallbacksAndRangePublish verifies two things a list used to get wrong about its
// selection: re-selecting the row that is already the whole of the selection does not tell the application that the
// selection changed, which is what a click on that same row has always done, and changing whether more than one row may
// be selected reaches an assistive technology rather than waiting for something unrelated to redraw the window.
func TestListAccessibilitySelectionCallbacksAndRangePublish(t *testing.T) {
	c := check.New(t)
	var list *unison.List[string]
	var wnd *unison.Window
	changes := 0
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			list = unison.NewList[string]()
			list.Factory = &unison.DefaultCellFactory{Height: 20}
			for i := range 4 {
				list.Append("Row " + strconv.Itoa(i))
			}
			list.SetAllowMultipleSelection(false)
			list.Select(false, 0)
			list.NewSelectionCallback = func() { changes++ }
			wnd = newHeadlessWindow(t, "list selection", geom.NewRect(10, 10, 300, 300), axColumn(list))
		}))
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(list)
	c.True(node != nil)
	if node == nil {
		return
	}
	c.False(node.Multiselectable, "a list holds one row at a time until it is told otherwise")
	first := axNodeWithRowIndex(tree, node, 0)
	second := axNodeWithRowIndex(tree, node, 1)
	c.True(first != nil && second != nil)
	if first == nil || second == nil {
		return
	}
	c.False(first.Actions.Has(accessibility.AddToSelection),
		"a list that holds one row at a time has nothing to add to")

	// The row is already the whole of the selection, so selecting it again changes nothing and says nothing.
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   first.ID,
		Action: accessibility.Select,
	}))
	var count int
	screen.Do(func() { count = changes })
	c.Equal(0, count, "re-selecting the row that is already selected is not a change of selection")

	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   second.ID,
		Action: accessibility.Select,
	}))
	screen.Do(func() { count = changes })
	c.Equal(1, count, "moving the selection to another row is")

	// Allowing more than one row to be selected changes what the list and every row of it offer, so it has to be
	// published without waiting for something else to redraw the window.
	screen.AccessibilityEvents(wnd)
	screen.Do(func() { list.SetAllowMultipleSelection(true) })
	published := false
	for _, e := range screen.AccessibilityEvents(wnd) {
		if e.Kind == accessibility.StateChanged && e.Node == node.ID &&
			e.State == accessibility.StateMultiselectable && e.New == "true" {
			published = true
		}
	}
	c.True(published, "the change should have been described without anything else happening")
	tree = screen.AccessibilityTree(wnd)
	node = screen.AccessibilityNodeFor(list)
	c.True(node != nil)
	if node == nil {
		return
	}
	c.True(node.Multiselectable)
	second = axNodeWithRowIndex(tree, node, 1)
	c.True(second != nil)
	if second != nil {
		c.True(second.Actions.Has(accessibility.AddToSelection), "every row now offers to be added to the selection")
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestPopupMenuAccessibilityWithNothingToChooseFrom verifies that a popup menu holding nothing that could be chosen
// says so: Click refuses to open one, so advertising that it expands, and describing it afterwards as expanded, would
// leave an assistive technology waiting for choices that are never going to appear.
func TestPopupMenuAccessibilityWithNothingToChooseFrom(t *testing.T) {
	c := check.New(t)
	var empty, separatorsOnly, filled *unison.PopupMenu[string]
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			empty = unison.NewPopupMenu[string]()
			separatorsOnly = unison.NewPopupMenu[string]()
			separatorsOnly.AddSeparator()
			filled = unison.NewPopupMenu[string]()
			filled.AddItem("One")
			filled.SelectIndex(0)
			wnd = newHeadlessWindow(t, "empty popup", geom.NewRect(10, 10, 300, 200),
				axColumn(empty, separatorsOnly, filled))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()

	screen.AccessibilityTree(wnd)
	for _, one := range []*unison.PopupMenu[string]{empty, separatorsOnly} {
		node := screen.AccessibilityNodeFor(one)
		c.True(node != nil)
		if node == nil {
			continue
		}
		c.False(node.Expandable, "a popup with nothing to choose from opens nothing")
		c.False(node.Actions.Has(accessibility.Expand))
		c.False(node.Actions.Has(accessibility.Collapse))
		// The click that would open it is queued rather than performed while the answer is awaited, so the request is
		// taken whatever will come of it. What did come of it is what the next description of the popup says, and for
		// one with nothing in it that is the same thing it said before.
		c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
			Node:   node.ID,
			Action: accessibility.Expand,
		}))
		c.Equal(0, len(axNodesWithRole(screen.AccessibilityTree(wnd), role.Menu)), "nothing should have opened")
		c.False(axMustNode(c, screen.AccessibilityNodeFor(one)).Expanded,
			"a popup that opened nothing goes on saying its choices are not showing")
	}

	filledNode := screen.AccessibilityNodeFor(filled)
	c.True(filledNode != nil)
	if filledNode == nil {
		return
	}
	c.True(filledNode.Expandable, "a popup with a choice in it does open something")
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   filledNode.ID,
		Action: accessibility.Expand,
	}))
	c.Equal(1, len(axNodesWithRole(screen.AccessibilityTree(wnd), role.Menu)), "its choices should have been shown")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
