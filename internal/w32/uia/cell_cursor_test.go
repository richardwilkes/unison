// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package uia

import (
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/role"
)

// cellCursorTree builds what a table publishes once it has a cell cursor: tableTree with every row and cell focusable
// and offering the Focus action, the rows named with the whole of what they hold, and one cell named by the label
// inside it rather than by any name of its own. The keyboard focus starts on the table, which is what a table nobody
// has arrowed through yet reports.
//
//	6 table "Files"                        focusable, 5 rows, 2 columns
//	├─ 7 row  "unison, Size 4 KB"          row 1, focusable, selected
//	│  ├─ 8  cell "unison"                 row 1 column 0, focusable
//	│  └─ 9  cell "4 KB"                   row 1 column 1, focusable
//	└─ 10 row "doc.go, Size 1 KB"          row 2, focusable
//	   ├─ 11 cell "doc.go"                 row 2 column 0, focusable
//	   └─ 12 cell (named by its content)   row 2 column 1, focusable
//	      └─ 16 label "1 KB"
//
// The columns are headed by 4 ("Name") and 5 ("Size"); see tableTree.
func cellCursorTree() *accessibility.Tree {
	tree := tableTree()
	tree.Node(6).Focusable = true
	for _, id := range []accessibility.NodeID{7, 10} {
		n := tree.Node(id)
		n.Focusable = true
		n.Actions = n.Actions.With(accessibility.Focus)
	}
	tree.Node(7).Name = "unison, Size 4 KB"
	tree.Node(10).Name = "doc.go, Size 1 KB"
	for _, id := range []accessibility.NodeID{8, 9, 11, 12} {
		n := tree.Node(id)
		n.Focusable = true
		n.Actions = n.Actions.With(accessibility.ScrollIntoView, accessibility.Focus)
	}
	// A cell whose content describes itself has its name cleared by the builder and is named by that content here;
	// see Table.axAddRow and namedByItsContent.
	tree.Node(12).Name = ""
	tree.Node(12).Children = []accessibility.NodeID{16}
	tree.Nodes[16] = &accessibility.Node{
		ID: 16, Parent: 12, Role: role.Label, Name: "1 KB", Bounds: geom.NewRect(100, 40, 100, 20),
	}
	tree.Focus = 6
	tree.Node(6).Focused = true
	return tree
}

// focusCell puts the cell cursor on one cell of one row, which is what the table publishes while the person is at cell
// level: the row is selected but hands the focus on to the cell within it, so that the cell alone claims the keyboard.
// See Table.ProvideAccessibility and AccessibilityBuilder.FocusChild.
func focusCell(tree *accessibility.Tree, container, row, cell accessibility.NodeID) {
	focusRow(tree, container, row)
	tree.Node(row).Focused = false
	tree.Node(cell).Focused = true
	tree.Focus = cell
}

// TestHasKeyboardFocusOnCell verifies that the cell the cursor is on is the one element of the table claiming the
// keyboard, and that it claims being focusable alongside it. NVDA's shouldAllowUIAFocusEvent drops a focus event whose
// element does not report HasKeyboardFocus, so a cell that did not claim it would be moved onto in silence.
func TestHasKeyboardFocusOnCell(t *testing.T) {
	c := check.New(t)
	tree := cellCursorTree()
	focusCell(tree, 6, 7, 9)
	c.True(HasKeyboardFocus(tree, tree.Node(9)), "the cell the tree's Focus names")
	c.False(HasKeyboardFocus(tree, tree.Node(7)), "the row that handed the focus on to the cell")
	c.False(HasKeyboardFocus(tree, tree.Node(8)), "another cell of the same row")
	c.False(HasKeyboardFocus(tree, tree.Node(6)), "the table that handed the focus on to the row")
	c.True(tree.Node(9).Focusable)
	c.True(ReportsProperty(tree, tree.Node(9), HasKeyboardFocusPropertyId))
	c.True(ReportsProperty(tree, tree.Node(9), IsKeyboardFocusablePropertyId))
	c.Equal(1, focusedCount(tree), "at most one element of a fragment claims the keyboard")

	// Moving the cursor to the next column moves the claim with it, and moving back out to the row hands it back.
	focusCell(tree, 6, 7, 8)
	c.True(HasKeyboardFocus(tree, tree.Node(8)))
	c.False(HasKeyboardFocus(tree, tree.Node(9)))
	focusRow(tree, 6, 7)
	c.True(HasKeyboardFocus(tree, tree.Node(7)), "back at row level the row claims the keyboard again")
	c.False(HasKeyboardFocus(tree, tree.Node(8)))

	// An inactive window holds the keyboard nowhere, cell or not.
	inactive := cellCursorTree()
	focusCell(inactive, 6, 7, 9)
	inactive.Node(1).Focused = false
	c.False(HasKeyboardFocus(inactive, inactive.Node(9)))
}

// focusedCount returns how many elements of a snapshot other than the root claim the keyboard focus.
func focusedCount(tree *accessibility.Tree) int {
	count := 0
	for _, n := range tree.Nodes {
		if n.ID != tree.Root && HasKeyboardFocus(tree, n) {
			count++
		}
	}
	return count
}

// TestFocusedCellReportsItsPlaceInTheGrid verifies that the cell the cursor is on tells a client everything it needs to
// speak it: the control type a screen reader reads a grid item as, the Grid and Table item patterns, the grid it
// belongs to, its row and column, and the header of its own column. NVDA reads exactly these for a focused element —
// name, then the column header and "column N" from ITableItemProvider::GetColumnHeaderItems and
// IGridItemProvider::get_Column, then "row N" from get_Row — so a cell missing any of them is spoken as a bare name.
func TestFocusedCellReportsItsPlaceInTheGrid(t *testing.T) {
	c := check.New(t)
	tree := cellCursorTree()
	focusCell(tree, 6, 7, 9)
	cell := tree.Node(9)
	c.Equal(DataItemControlTypeId, ControlType(tree, cell))
	c.True(Patterns(cell).Has(PatternGridItem))
	c.True(Patterns(cell).Has(PatternTableItem))
	c.True(Patterns(cell).Has(PatternScrollItem), "the cursor can be asked to bring the cell into view")
	c.Equal(accessibility.NodeID(6), ContainingGrid(tree, 9))
	c.Equal(1, cell.RowIndex)
	c.Equal(1, cell.ColumnIndex)
	c.Equal(accessibility.NodeID(5), ColumnHeaderItem(tree, 9), "the header of the cell's own column")
	c.Equal(accessibility.NodeID(4), ColumnHeaderItem(tree, 8))
	c.Equal(accessibility.NodeID(9), GridItem(tree, 6, 1, 1), "the grid hands the same cell back")

	// A cell whose content describes itself is named by that content, which is the only name a client is given for it.
	c.Equal("1 KB", NameString(tree, tree.Node(12)))
	c.Equal("4 KB", NameString(tree, cell), "a cell with a name of its own keeps it")

	// The position properties a row reports are not a cell's: a cell is not one of a numbered set of rows, and
	// reporting one would have a screen reader say "2 of 5" over the row's own position.
	position, size := PositionInSet(tree, cell)
	c.Equal(0, position)
	c.Equal(0, size)
}

// TestCellCursorLeavesTheRowNameAlone verifies that the whole-row name the table composes is what a client is told,
// unchanged by anything the adapter would otherwise add: a row is not named by the cells beneath it, so the name is
// spoken once as the person arrows down the rows rather than once for the row and again for its content.
func TestCellCursorLeavesTheRowNameAlone(t *testing.T) {
	c := check.New(t)
	tree := cellCursorTree()
	c.Equal("unison, Size 4 KB", NameString(tree, tree.Node(7)))
	c.Equal("doc.go, Size 1 KB", NameString(tree, tree.Node(10)),
		"the row holding a cell named by its content is still named by the whole row")
	c.False(namedByItsContent(tree, tree.Node(7)), "a row is never named by the cells within it")
	focusCell(tree, 6, 10, 12)
	c.Equal("doc.go, Size 1 KB", NameString(tree, tree.Node(10)), "and the cursor does not change it")
	c.Equal("1 KB", NameString(tree, tree.Node(12)))
}

// TestDecideRaisesCellCursorEntered verifies that moving from the row onto the first cell of it is reported as a focus
// change on the cell. A screen reader moves its cursor to the element a focus event names, so the cell has to be
// named: the row is where the user was, not where they are.
func TestDecideRaisesCellCursorEntered(t *testing.T) {
	c := check.New(t)
	old := cellCursorTree()
	focusRow(old, 6, 7)
	cur := cellCursorTree()
	focusCell(cur, 6, 7, 8)
	c.Equal([]Raise{
		raiseEvent(8, AutomationFocusChangedEventId),
	}, DecideRaises(old, cur, accessibility.Diff(old, cur)))
}

// TestDecideRaisesCellCursorAcrossColumns verifies that Left and Right within one row are reported as a focus change on
// the cell moved onto, and on nothing else: the row did not change, so its selection did not either.
func TestDecideRaisesCellCursorAcrossColumns(t *testing.T) {
	c := check.New(t)
	old := cellCursorTree()
	focusCell(old, 6, 7, 8)
	cur := cellCursorTree()
	focusCell(cur, 6, 7, 9)
	c.Equal([]Raise{
		raiseEvent(9, AutomationFocusChangedEventId),
	}, DecideRaises(old, cur, accessibility.Diff(old, cur)))
}

// TestDecideRaisesCellCursorAcrossRows verifies that Up and Down at cell level report the focus on the cell of the row
// moved onto, while the rows still report what happened to the selection. Both are needed: no screen reader treats a
// selection change as the user having moved, and a client tracking the selection is not told by the focus event which
// rows joined and left it.
func TestDecideRaisesCellCursorAcrossRows(t *testing.T) {
	c := check.New(t)
	old := cellCursorTree()
	focusCell(old, 6, 7, 9)
	cur := cellCursorTree()
	focusCell(cur, 6, 10, 12)
	c.Equal([]Raise{
		raiseProperty(7, SelectionItemIsSelectedPropertyId),
		raiseEvent(7, SelectionItem_ElementRemovedFromSelectionEventId),
		raiseProperty(10, SelectionItemIsSelectedPropertyId),
		raiseEvent(10, SelectionItem_ElementAddedToSelectionEventId),
		raiseEvent(12, AutomationFocusChangedEventId),
	}, DecideRaises(old, cur, accessibility.Diff(old, cur)))

	// A table that allows only one selection at a time reports the row that became the selection instead, which is
	// what a client treats as everything else having been deselected.
	single := cellCursorTree()
	focusCell(single, 6, 10, 12)
	single.Node(6).Multiselectable = false
	before := cellCursorTree()
	focusCell(before, 6, 7, 9)
	before.Node(6).Multiselectable = false
	c.Equal([]Raise{
		raiseProperty(7, SelectionItemIsSelectedPropertyId),
		raiseProperty(10, SelectionItemIsSelectedPropertyId),
		raiseEvent(10, SelectionItem_ElementSelectedEventId),
		raiseEvent(12, AutomationFocusChangedEventId),
	}, DecideRaises(before, single, accessibility.Diff(before, single)))
}
