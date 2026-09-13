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
)

// selectedIndexes returns the selected row indexes of the list in ascending order.
func selectedIndexes[T any](l *unison.List[T]) []int {
	var out []int
	i := l.Selection.FirstSet()
	for i != -1 {
		out = append(out, i)
		i = l.Selection.NextSet(i + 1)
	}
	return out
}

func newTestList(values ...string) *unison.List[string] {
	l := unison.NewList[string]()
	l.Append(values...)
	return l
}

func TestListAppendAndDataAtIndex(t *testing.T) {
	c := check.New(t)
	l := newTestList("a", "b", "c")
	c.Equal(3, l.Count())
	c.Equal("b", l.DataAtIndex(1))
	// Out-of-range access yields the zero value rather than panicking.
	c.Equal("", l.DataAtIndex(-1))
	c.Equal("", l.DataAtIndex(99))
}

func TestListReplace(t *testing.T) {
	c := check.New(t)
	l := newTestList("a", "b", "c")
	l.Replace(1, "B")
	c.Equal("B", l.DataAtIndex(1))
	// Out-of-range replace is a no-op.
	l.Replace(99, "x")
	c.Equal(3, l.Count())
}

func TestListInsertShiftsSelection(t *testing.T) {
	c := check.New(t)
	l := newTestList("a", "b", "c")
	l.Select(false, 2) // select "c"
	l.Insert(1, "x", "y")
	c.Equal(5, l.Count())
	c.Equal("c", l.DataAtIndex(4))
	// The selection follows the moved item.
	c.Equal([]int{4}, selectedIndexes(l))
}

func TestListInsertClampsIndex(t *testing.T) {
	c := check.New(t)
	l := newTestList("a", "b")
	l.Insert(-1, "z") // negative clamps to append
	c.Equal("z", l.DataAtIndex(2))
}

func TestListRemoveShiftsSelection(t *testing.T) {
	c := check.New(t)
	l := newTestList("a", "b", "c", "d")
	l.Select(true, 1, 3)
	l.Remove(2) // remove "c"
	c.Equal(3, l.Count())
	// Index 1 ("b") stays; index 3 ("d") slides down to 2.
	c.Equal([]int{1, 2}, selectedIndexes(l))
	c.Equal("d", l.DataAtIndex(2))
}

func TestListRemoveRangeShiftsSelection(t *testing.T) {
	c := check.New(t)
	l := newTestList("a", "b", "c", "d", "e")
	l.SelectAll()
	l.RemoveRange(1, 2) // remove "b","c"
	c.Equal(3, l.Count())
	c.Equal([]int{0, 1, 2}, selectedIndexes(l))
	c.Equal("d", l.DataAtIndex(1))
}

func TestListInsertShiftsAnchor(t *testing.T) {
	c := check.New(t)
	l := newTestList("a", "b", "c")
	l.Select(false, 2) // anchor at "c"
	c.Equal(2, l.Anchor())
	l.Insert(1, "x", "y")
	// The anchor follows the moved item.
	c.Equal(4, l.Anchor())
	// Inserting after the anchor leaves it alone.
	l.Insert(5, "z")
	c.Equal(4, l.Anchor())
}

func TestListRemoveAdjustsAnchor(t *testing.T) {
	c := check.New(t)
	l := newTestList("a", "b", "c", "d")
	l.Select(false, 2) // anchor at "c"
	l.Remove(0)        // remove "a"; anchor slides down with its row
	c.Equal(1, l.Anchor())
	l.Remove(2) // remove "d", after the anchor; anchor unchanged
	c.Equal(1, l.Anchor())
	l.Remove(1) // remove the anchored row itself
	c.Equal(-1, l.Anchor())
}

func TestListRemoveRangeAdjustsAnchor(t *testing.T) {
	c := check.New(t)
	l := newTestList("a", "b", "c", "d", "e")
	l.Select(false, 4) // anchor at "e"
	l.RemoveRange(0, 1)
	c.Equal(2, l.Anchor())
	l.RemoveRange(1, 2) // range covers the anchored row
	c.Equal(-1, l.Anchor())
}

func TestListRemoveRangeNoPhantomSelection(t *testing.T) {
	c := check.New(t)
	l := newTestList("a", "b", "c", "d", "e", "f", "g", "h", "i", "j")
	l.Select(false, 9)  // anchor at the last row
	l.RemoveRange(5, 9) // the anchored row is gone; the anchor must not point past the new row count
	c.Equal(-1, l.Anchor())
	// A subsequent range selection anchors afresh instead of extending from the stale index, so the selection can
	// never include rows beyond the current count and CanSelectAll stays consistent.
	l.SelectRange(2, 2, true)
	c.Equal([]int{2}, selectedIndexes(l))
	c.Equal(2, l.Anchor())
	c.True(l.Selection.Count() <= l.Count())
	c.True(l.CanSelectAll())
}

func TestListSelectReplaceVsAdd(t *testing.T) {
	c := check.New(t)
	l := newTestList("a", "b", "c", "d")
	l.Select(false, 1)
	c.Equal([]int{1}, selectedIndexes(l))
	// add=false replaces.
	l.Select(false, 3)
	c.Equal([]int{3}, selectedIndexes(l))
	// add=true augments; the existing anchor (3) is retained.
	l.Select(true, 0)
	c.Equal([]int{0, 3}, selectedIndexes(l))
	c.Equal(3, l.Anchor())
}

func TestListSelectRangeClamps(t *testing.T) {
	c := check.New(t)
	l := newTestList("a", "b", "c", "d", "e")
	l.SelectRange(-3, 99, false)
	c.Equal([]int{0, 1, 2, 3, 4}, selectedIndexes(l))
}

func TestListSelectAllAndCanSelectAll(t *testing.T) {
	c := check.New(t)
	l := newTestList("a", "b", "c")
	c.True(l.CanSelectAll())
	l.SelectAll()
	c.Equal([]int{0, 1, 2}, selectedIndexes(l))
	c.False(l.CanSelectAll())
}

func TestListClearResetsSelectionAndAnchor(t *testing.T) {
	c := check.New(t)
	l := newTestList("a", "b", "c")
	l.SelectAll()
	l.Clear()
	c.Equal(0, l.Count())
	c.Equal(0, l.Selection.Count())
	c.Equal(-1, l.Anchor())
}

func TestListSetAllowMultipleSelectionCollapses(t *testing.T) {
	c := check.New(t)
	l := newTestList("a", "b", "c", "d")
	c.True(l.AllowMultipleSelection())
	l.Select(true, 1, 2, 3)
	c.Equal(3, l.Selection.Count())

	// Disabling multiple selection collapses to a single anchored row.
	l.SetAllowMultipleSelection(false)
	c.False(l.AllowMultipleSelection())
	c.Equal(1, l.Selection.Count())

	// With multiple disabled, Select keeps only the last requested index.
	l.Select(false, 0, 2)
	c.Equal([]int{2}, selectedIndexes(l))
}

// TestListAccessibilityPressOpensRow verifies that pressing a row runs the double-click callback, which is the list's
// "open this row" gesture and was unreachable from an assistive technology. The row is selected first when it was not
// already, since both of the gestures the press stands for act on the selection.
func TestListAccessibilityPressOpensRow(t *testing.T) {
	c := check.New(t)
	l := newTestList("a", "b", "c")
	opens := 0
	selections := 0
	l.DoubleClickCallback = func() { opens++ }
	l.NewSelectionCallback = func() { selections++ }

	c.True(l.PerformAccessibilityAction(accessibility.ActionRequest{Key: 1, Action: accessibility.Press}))
	c.Equal(1, opens, "pressing a row opens it")
	c.Equal([]int{1}, selectedIndexes(l), "the row the callback acts on has to be selected")
	c.Equal(1, selections)

	// Pressing the row that is already selected leaves the selection alone.
	c.True(l.PerformAccessibilityAction(accessibility.ActionRequest{Key: 1, Action: accessibility.Press}))
	c.Equal(2, opens)
	c.Equal(1, selections, "a row that was already selected is not selected again")

	// A row out of range, and a list with nothing to open, are both refused.
	c.False(l.PerformAccessibilityAction(accessibility.ActionRequest{Key: 99, Action: accessibility.Press}))
	l.DoubleClickCallback = nil
	c.False(l.PerformAccessibilityAction(accessibility.ActionRequest{Key: 0, Action: accessibility.Press}))
}

// TestListAccessibilityActionsMatchWhatItCanDo verifies what a list and its rows offer to do: the list itself is not
// something to press, since that would click the middle of its frame and replace the selection with whatever row sits
// there, a list that holds one row at a time does not offer to add to or take away from its selection, and a row offers
// to be pressed only when there is something for pressing it to do.
func TestListAccessibilityActionsMatchWhatItCanDo(t *testing.T) {
	c := check.New(t)
	const rowCount = 6
	var single, multiple *unison.List[string]
	var wnd *unison.Window
	opens := 0
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			single = unison.NewList[string]()
			single.Factory = &unison.DefaultCellFactory{Height: 20}
			single.SetAllowMultipleSelection(false)
			single.DoubleClickCallback = func() { opens++ }

			multiple = unison.NewList[string]()
			multiple.Factory = &unison.DefaultCellFactory{Height: 20}
			for i := range rowCount {
				single.Append("Row " + strconv.Itoa(i))
				multiple.Append("Row " + strconv.Itoa(i))
			}
			wnd = newHeadlessWindow(t, "list actions", geom.NewRect(10, 10, 400, 400),
				axColumn(single, multiple))
		}))
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(single)
	c.True(node != nil)
	c.False(node.Multiselectable)
	c.False(node.Actions.Has(accessibility.Press), "pressing the list would select whatever row is in the middle")

	row := axNodeWithRowIndex(tree, node, 1)
	c.True(row != nil)
	if row == nil {
		return
	}
	c.True(row.Actions.Has(accessibility.Select))
	c.False(row.Actions.Has(accessibility.AddToSelection),
		"a list that holds one row at a time has nothing to add a row to")
	c.False(row.Actions.Has(accessibility.RemoveFromSelection))
	c.True(row.Actions.Has(accessibility.Press), "the list has something to open with")

	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   row.ID,
		Action: accessibility.Press,
	}))
	var count int
	screen.Do(func() { count = opens })
	c.Equal(1, count, "pressing the row should have opened it")

	multiNode := screen.AccessibilityNodeFor(multiple)
	c.True(multiNode != nil)
	c.True(multiNode.Multiselectable)
	multiRow := axNodeWithRowIndex(tree, multiNode, 1)
	c.True(multiRow != nil)
	if multiRow != nil {
		c.True(multiRow.Actions.Has(accessibility.AddToSelection), "a multiple-selection list can add to its selection")
		c.True(multiRow.Actions.Has(accessibility.RemoveFromSelection))
		c.False(multiRow.Actions.Has(accessibility.Press), "nothing has been given to open these rows with")
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestListAccessibilityVaryingRowsKeepsTheSelectionInSight verifies that stopping the walk through a list whose rows
// are each their own height does not drop a selected row that lies far below what can be seen, since what is selected
// is worth describing however far out of sight it is.
func TestListAccessibilityVaryingRowsKeepsTheSelectionInSight(t *testing.T) {
	c := check.New(t)
	const rowCount = 60
	var list *unison.List[string]
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			list = unison.NewList[string]()
			// No fixed cell height, so every row has to be measured to know where the ones after it sit.
			list.Factory = &unison.DefaultCellFactory{}
			for i := range rowCount {
				list.Append("Row " + strconv.Itoa(i))
			}
			list.Select(false, rowCount-1)
			wnd = newHeadlessWindow(t, "varying rows", geom.NewRect(10, 10, 300, 300),
				axColumn(axScroller(list, geom.NewSize(200, 60))))
		}))
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(list)
	c.True(node != nil)
	c.Equal(rowCount, node.RowCount)
	rows := axChildNodes(tree, node)
	c.True(len(rows) < rowCount/2, "only the rows worth describing should have been described, got %d", len(rows))
	last := axNodeWithRowIndex(tree, node, rowCount-1)
	c.True(last != nil, "the selected row is described however far out of sight it is")
	if last != nil {
		c.True(last.Selected)
	}
	c.True(axNodeWithRowIndex(tree, node, rowCount/2) == nil,
		"a row that is neither visible nor selected is not described at all")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
