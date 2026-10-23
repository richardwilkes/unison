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
	"github.com/richardwilkes/unison/enums/mod"
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

// TestListInsertShiftsSelection verifies that the selection follows the rows an insertion moves, and that inserting no
// rows leaves it alone, where moving the selected rows up by nothing used to clear them.
func TestListInsertShiftsSelection(t *testing.T) {
	for _, tc := range []struct {
		name     string
		selected []int
		values   []string
		want     []int
	}{
		{name: "the selection follows the moved rows", selected: []int{2}, values: []string{"x", "y"}, want: []int{4}},
		{name: "inserting nothing keeps the selection", selected: []int{1, 2}, want: []int{1, 2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			l := newTestList("a", "b", "c")
			l.Select(false, tc.selected...)
			l.Insert(1, tc.values...)
			c.Equal(3+len(tc.values), l.Count())
			c.Equal("c", l.DataAtIndex(2+len(tc.values)))
			c.Equal(tc.want, selectedIndexes(l))
		})
	}
}

// TestListInsertLeavesTheCallersSliceAlone verifies that inserting part of a slice with spare capacity does not write
// the rows that follow the insertion point into the caller's slice.
func TestListInsertLeavesTheCallersSliceAlone(t *testing.T) {
	c := check.New(t)
	l := newTestList("x", "y")
	all := []string{"a", "b", "c", "d"}
	l.Insert(1, all[:2]...)
	c.Equal([]string{"a", "b", "c", "d"}, all, "the caller's slice is left alone")
	c.Equal(4, l.Count())
	c.Equal("x", l.DataAtIndex(0))
	c.Equal("a", l.DataAtIndex(1))
	c.Equal("b", l.DataAtIndex(2))
	c.Equal("y", l.DataAtIndex(3))
}

func TestListInsertClampsIndex(t *testing.T) {
	c := check.New(t)
	l := newTestList("a", "b")
	l.Insert(-1, "z") // negative clamps to append
	c.Equal("z", l.DataAtIndex(2))
}

// TestListRemoveRangeShiftsSelection verifies that removing rows, as a range or one at a time with Remove, drops them
// from the selection and moves the selection past them down by as many.
func TestListRemoveRangeShiftsSelection(t *testing.T) {
	for _, tc := range []struct {
		remove   func(l *unison.List[string])
		name     string
		rows     []string
		selected []int
		want     []int
		count    int
		dAt      int // Where "d", the first row past the removed ones, ends up.
	}{
		{
			name:     "a range",
			rows:     []string{"a", "b", "c", "d", "e"},
			selected: []int{0, 1, 2, 3, 4},
			remove:   func(l *unison.List[string]) { l.RemoveRange(1, 2) },
			count:    3,
			want:     []int{0, 1, 2},
			dAt:      1,
		},
		{
			name:     "one row",
			rows:     []string{"a", "b", "c", "d"},
			selected: []int{1, 3},
			remove:   func(l *unison.List[string]) { l.Remove(2) },
			count:    3,
			want:     []int{1, 2},
			dAt:      2,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			l := newTestList(tc.rows...)
			l.Select(false, tc.selected...)
			tc.remove(l)
			c.Equal(tc.count, l.Count())
			c.Equal(tc.want, selectedIndexes(l))
			c.Equal("d", l.DataAtIndex(tc.dAt))
		})
	}
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

// TestListSelectRangeClamps verifies that SelectRange clamps out-of-range indexes to the rows and anchors at the start
// of the range.
func TestListSelectRangeClamps(t *testing.T) {
	c := check.New(t)
	l := newTestList("a", "b", "c", "d", "e")
	l.SelectRange(-3, 99, false)
	c.Equal([]int{0, 1, 2, 3, 4}, selectedIndexes(l))
	l.SelectRange(1, 99, false)
	c.Equal([]int{1, 2, 3, 4}, selectedIndexes(l))
	c.Equal(1, l.Anchor())
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

// TestListAccessibilitySelectionSetsTheAnchor verifies that adding a row to a list's selection, or taking one out of
// it, through an assistive technology puts the anchor a later shift-click extends from on that row. Both stand for
// the ctrl-click that does the same thing, and DefaultMouseDown's DiscontiguousSelectionDown branch moves the anchor to
// the row it touched whichever way the flip went, so a shift-click after an assistive technology acted has to extend
// from the same place it would have after the click. Adding used to leave the anchor wherever the previous selection
// had put it, and removing used to clear it away entirely.
func TestListAccessibilitySelectionSetsTheAnchor(t *testing.T) {
	c := check.New(t)
	shiftClick := func(l *unison.List[string], row int) {
		where := geom.NewPoint(5, l.RowRect(row).CenterY())
		l.DefaultMouseDown(where, unison.ButtonLeft, 1, mod.Shift)
		l.DefaultMouseUp(where, unison.ButtonLeft, mod.Shift)
	}

	// Adding a row to the selection.
	l := newTestList("a", "b", "c", "d", "e")
	l.Factory = &unison.DefaultCellFactory{Height: 20}
	l.SetFrameRect(geom.NewRect(0, 0, 200, 100))
	l.Select(false, 0)
	c.Equal(0, l.Anchor())
	c.True(l.PerformAccessibilityAction(accessibility.ActionRequest{Key: 2, Action: accessibility.AddToSelection}))
	c.Equal([]int{0, 2}, selectedIndexes(l))
	c.Equal(2, l.Anchor(), "the row added is the one a shift-click extends from")
	shiftClick(l, 4)
	c.Equal([]int{0, 2, 3, 4}, selectedIndexes(l),
		"the shift-click should have extended from the row the request added")

	// Taking a row out of the selection.
	l = newTestList("a", "b", "c", "d", "e")
	l.Factory = &unison.DefaultCellFactory{Height: 20}
	l.SetFrameRect(geom.NewRect(0, 0, 200, 100))
	l.Select(false, 0, 2)
	c.Equal(0, l.Anchor())
	c.True(l.PerformAccessibilityAction(accessibility.ActionRequest{Key: 2, Action: accessibility.RemoveFromSelection}))
	c.Equal([]int{0}, selectedIndexes(l))
	c.Equal(2, l.Anchor(), "the row taken out is still the one a shift-click extends from")
	shiftClick(l, 4)
	c.Equal([]int{0, 2, 3, 4}, selectedIndexes(l),
		"the shift-click should have extended from the row the request removed")
}

// axListWindow is what the focus tests below drive: a list, another control to move the focus away to, and a menu bar
// with one menu, which is what the displacement test opens.
type axListWindow struct {
	screen *unison.HeadlessScreen
	wnd    *unison.Window
	list   *unison.List[string]
	other  *unison.Panel
	listID accessibility.NodeID
}

// newAXListWindow starts a session showing a list of rowCount rows, each of a fixed height, with nothing selected and
// nothing holding the keyboard focus yet.
func newAXListWindow(t *testing.T, title string, rowCount int, multiple bool) *axListWindow {
	t.Helper()
	const (
		menuID = unison.UserBaseID + iota
		cutID
	)
	out := &axListWindow{}
	out.screen = startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			out.list = unison.NewList[string]()
			out.list.Factory = &unison.DefaultCellFactory{Height: 20}
			out.list.SetAllowMultipleSelection(multiple)
			for i := range rowCount {
				out.list.Append("Row " + strconv.Itoa(i))
			}
			out.other = axFocusablePanel("Other")
			out.wnd = newHeadlessWindow(t, title, geom.NewRect(10, 10, 400, 400),
				axColumn(out.list, out.other))
			if out.wnd == nil {
				return
			}
			unison.DefaultMenuFactory().BarForWindow(out.wnd, func(bar unison.Menu) {
				f := bar.Factory()
				edit := f.NewMenu(menuID, "Edit", nil)
				edit.InsertItem(-1, f.NewItem(cutID, "Cut", unison.KeyBinding{}, nil, nil))
				bar.InsertMenu(-1, edit)
			})
			out.wnd.ToFront()
		}))
	out.screen.Sync()
	out.screen.EnableAccessibility()
	if node := out.screen.AccessibilityNodeFor(out.list); node != nil {
		out.listID = node.ID
	}
	return out
}

// tree describes the window as it is now.
func (a *axListWindow) tree() *accessibility.Tree {
	return a.screen.AccessibilityTree(a.wnd)
}

// rowNode returns the node describing a row of the list in the given tree, or nil if the row was not described.
func (a *axListWindow) rowNode(tree *accessibility.Tree, row int) *accessibility.Node {
	return axNodeWithRowIndex(tree, tree.Node(a.listID), row)
}

// axFocusedCount returns how many nodes other than the root report the keyboard focus, which may only ever be one.
func axFocusedCount(tree *accessibility.Tree) int {
	count := 0
	tree.Walk(func(n *accessibility.Node) bool {
		if n.Focused && n.ID != tree.Root {
			count++
		}
		return true
	})
	return count
}

// TestListAccessibilityFocusFollowsTheCurrentRow verifies that a list reports the keyboard focus on the row the person
// is on, and moves it with every gesture that moves the selection. Reporting it on the list itself and publishing
// nothing but a selection change per arrow key is what left a screen reader sitting on the list, silent, while the
// person moved through it.
func TestListAccessibilityFocusFollowsTheCurrentRow(t *testing.T) {
	c := check.New(t)
	a := newAXListWindow(t, "list focus", 6, true)
	c.NotNil(a.wnd)
	if a.wnd == nil {
		return
	}
	a.screen.Do(func() { a.list.RequestFocus() })
	tree := a.tree()
	c.True(tree != nil)
	if tree == nil {
		return
	}
	c.Equal(a.listID, tree.Focus, "a list with nothing selected keeps the focus on itself")
	a.screen.AccessibilityEvents(a.wnd)

	// Down selects the first row, which is where the person now is.
	previous := tree
	a.screen.KeyPress(unison.KeyDown, mod.None)
	tree = a.tree()
	c.True(tree != nil)
	if tree == nil {
		return
	}
	row := a.rowNode(tree, 0)
	c.True(row != nil)
	if row == nil {
		return
	}
	c.Equal(row.ID, tree.Focus, "the focus must be reported on the row the arrow key landed on")
	c.True(row.Focused)
	c.True(row.Focusable)
	c.True(row.Selected)
	c.True(row.Actions.Has(accessibility.Focus), "a row that can be moved to has to offer the move")
	c.False(tree.Node(a.listID).Focused, "the list itself stops reporting the focus it handed to the row")
	c.Equal(1, axFocusedCount(tree), "exactly one node other than the root may report the focus")
	c.True(axHasEvent(accessibility.Diff(previous, tree), accessibility.FocusChanged, row.ID),
		"the move has to reach an assistive technology as a focus change naming the row")
	c.Equal(0, a.list.Lead(), "the lead row is the row that was moved to")

	// And again, to the row after it.
	a.screen.KeyPress(unison.KeyDown, mod.None)
	tree = a.tree()
	c.True(tree != nil)
	if tree == nil {
		return
	}
	row = a.rowNode(tree, 1)
	c.True(row != nil && row.Focused)
	if row == nil {
		return
	}
	c.Equal(row.ID, tree.Focus)
	c.True(a.rowNode(tree, 0) != nil && !a.rowNode(tree, 0).Focused, "the row that was left gives the focus up")

	// Shift-Down adds the row below to the selection and moves the person onto it.
	a.screen.KeyPress(unison.KeyDown, mod.Shift)
	tree = a.tree()
	c.True(tree != nil)
	if tree == nil {
		return
	}
	c.Equal([]int{1, 2}, selectedIndexesOn(a.screen, a.list), "shift-down extends the selection")
	row = a.rowNode(tree, 2)
	c.True(row != nil)
	if row == nil {
		return
	}
	c.Equal(row.ID, tree.Focus, "the row the selection was extended onto is where the person is")
	c.Equal(1, axFocusedCount(tree))

	// A click puts the person on the row that was clicked.
	fourth := a.rowNode(tree, 4)
	c.True(fourth != nil)
	if fourth == nil {
		return
	}
	a.screen.Click(axScreenPoint(a.screen, a.wnd, fourth))
	tree = a.tree()
	c.True(tree != nil)
	if tree == nil {
		return
	}
	c.Equal([]int{4}, selectedIndexesOn(a.screen, a.list))
	row = a.rowNode(tree, 4)
	c.True(row != nil)
	if row == nil {
		return
	}
	c.Equal(row.ID, tree.Focus, "a click moves the reported focus onto the row it landed on")

	// With the selection gone there is no row to be on, so the list takes the focus back, as a native empty list box
	// holds it itself.
	a.screen.Do(func() { a.list.Select(false) })
	tree = a.tree()
	c.True(tree != nil)
	if tree == nil {
		return
	}
	c.Equal(a.listID, tree.Focus, "with nothing selected the focus goes back to the list")
	c.True(tree.Node(a.listID).Focused)
	c.Equal(1, axFocusedCount(tree))
	c.Equal(-1, a.list.Lead())
	c.Equal(0, len(a.screen.Errors()), "nothing should have panicked: %v", a.screen.Errors())
}

// selectedIndexesOn returns the selected row indexes of a list, read on the thread that owns it.
func selectedIndexesOn[T any](screen *unison.HeadlessScreen, l *unison.List[T]) []int {
	var out []int
	screen.Do(func() { out = selectedIndexes(l) })
	return out
}

// TestListAccessibilityFocusActionSelectsTheRow verifies that a request to put the focus on a row does what clicking
// that row does: the list takes the keyboard focus and the row becomes the whole of the selection, which is what the
// focus being reported on a row means.
func TestListAccessibilityFocusActionSelectsTheRow(t *testing.T) {
	c := check.New(t)
	a := newAXListWindow(t, "list focus action", 6, true)
	c.NotNil(a.wnd)
	if a.wnd == nil {
		return
	}
	a.screen.Do(func() { a.wnd.SetFocus(a.other) })
	tree := a.tree()
	c.True(tree != nil)
	if tree == nil {
		return
	}
	row := a.rowNode(tree, 3)
	c.True(row != nil)
	if row == nil {
		return
	}
	c.True(row.Actions.Has(accessibility.Focus))
	c.True(a.screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   row.ID,
		Action: accessibility.Focus,
	}), "a row offering the focus must be able to take it")
	c.Equal([]int{3}, selectedIndexesOn(a.screen, a.list), "moving onto a row selects it")
	var holdsFocus bool
	a.screen.Do(func() { holdsFocus = a.list.Is(a.wnd.CurrentFocus()) })
	c.True(holdsFocus, "the list is the one tab stop, so the keyboard focus goes there")

	tree = a.tree()
	c.True(tree != nil)
	if tree == nil {
		return
	}
	row = a.rowNode(tree, 3)
	c.True(row != nil && row.Focused)
	if row != nil {
		c.Equal(row.ID, tree.Focus)
	}
	c.Equal(1, axFocusedCount(tree))
	c.Equal(0, len(a.screen.Errors()), "nothing should have panicked: %v", a.screen.Errors())
}

// TestListAccessibilityRowsOfAListThatCannotTakeTheFocus verifies that the rows of a list that cannot take the
// keyboard focus neither offer it nor say they can take it, and that a request for it anyway is refused with nothing
// changed: the focus stays where it was and the selection is left alone.
func TestListAccessibilityRowsOfAListThatCannotTakeTheFocus(t *testing.T) {
	c := check.New(t)
	var list *unison.List[string]
	var field *unison.Field
	var wnd *unison.Window
	selections := 0
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			list = unison.NewList[string]()
			list.Factory = &unison.DefaultCellFactory{Height: 20}
			list.SetFocusable(false)
			list.NewSelectionCallback = func() { selections++ }
			for i := range 4 {
				list.Append("Row " + strconv.Itoa(i))
			}
			field = unison.NewField()
			if wnd = newHeadlessWindow(t, "list no focus", geom.NewRect(10, 10, 400, 400), axColumn(field, list)); wnd != nil {
				wnd.ToFront()
				wnd.SetFocus(field)
			}
		}))
	c.NotNil(wnd)
	tree := screen.AccessibilityTree(wnd)
	c.NotNil(tree)
	row := axNodeWithRowIndex(tree, screen.AccessibilityNodeFor(list), 2)
	row = axMustNode(c, row, "the row is described")
	c.False(row.Focusable)
	c.False(row.Actions.Has(accessibility.Focus))
	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{Node: row.ID, Action: accessibility.Focus}),
		"a request for the focus is refused")
	var focus *unison.Panel
	var count int
	c.True(screen.Do(func() {
		focus = wnd.CurrentFocus()
		count = selections
	}))
	c.True(focus == field.AsPanel(), "the field keeps the focus")
	c.Equal([]int(nil), selectedIndexesOn(screen, list), "and the selection is left alone")
	c.Equal(0, count)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestListKeyDownReportsOnlyChangedSelections verifies that the arrow, Home and End keys report a new selection only
// when the selection changed, as a click does, and that they do nothing on an empty list.
func TestListKeyDownReportsOnlyChangedSelections(t *testing.T) {
	c := check.New(t)
	l := newTestList("a", "b", "c")
	selections := 0
	l.NewSelectionCallback = func() { selections++ }
	c.True(l.DefaultKeyDown(unison.KeyDown, mod.None, false))
	c.Equal([]int{0}, selectedIndexes(l))
	c.Equal(1, selections)
	c.True(l.DefaultKeyDown(unison.KeyEnd, mod.None, false))
	c.Equal([]int{2}, selectedIndexes(l))
	c.Equal(2, selections)
	c.True(l.DefaultKeyDown(unison.KeyDown, mod.None, false))
	c.Equal([]int{2}, selectedIndexes(l))
	c.Equal(2, selections, "Down at the last row changes nothing")
	c.True(l.DefaultKeyDown(unison.KeyEnd, mod.None, false))
	c.Equal(2, selections, "and neither does End there")
	c.True(l.DefaultKeyDown(unison.KeyHome, mod.None, false))
	c.Equal([]int{0}, selectedIndexes(l))
	c.Equal(3, selections)
	c.True(l.DefaultKeyDown(unison.KeyUp, mod.None, false))
	c.Equal(3, selections, "Up at the first row changes nothing")
	empty := newTestList()
	empty.NewSelectionCallback = func() { selections++ }
	c.False(empty.DefaultKeyDown(unison.KeyDown, mod.None, false), "nothing to move through")
	c.False(empty.DefaultKeyDown(unison.KeyUp, mod.None, false))
	c.False(empty.DefaultKeyDown(unison.KeyHome, mod.None, false))
	c.False(empty.DefaultKeyDown(unison.KeyEnd, mod.None, false))
	c.Equal(3, selections)
	c.Equal(-1, empty.Lead())
}

// TestListAccessibilityFocusDisplacedByAnOpenMenu verifies that an open menu takes the focus away from a list's
// current row exactly as it takes it away from any other control: the keys go to the menu, so that is where the person
// is, and a row still claiming the focus would be a second focused object in the window.
func TestListAccessibilityFocusDisplacedByAnOpenMenu(t *testing.T) {
	c := check.New(t)
	a := newAXListWindow(t, "list focus menu", 6, true)
	c.NotNil(a.wnd)
	if a.wnd == nil {
		return
	}
	a.screen.Do(func() {
		a.list.Select(false, 1)
		a.list.RequestFocus()
	})
	tree := a.tree()
	c.True(tree != nil)
	if tree == nil {
		return
	}
	row := a.rowNode(tree, 1)
	c.True(row != nil && row.Focused)
	title := axNamed(tree, "Edit")
	c.True(title != nil)
	if title == nil {
		return
	}

	a.screen.Click(axScreenPoint(a.screen, a.wnd, title))
	tree = a.tree()
	c.True(tree != nil)
	if tree == nil {
		return
	}
	row = a.rowNode(tree, 1)
	c.True(row != nil)
	if row == nil {
		return
	}
	c.False(row.Focused, "an open menu displaces the focus a list handed to its row")
	c.False(tree.Node(a.listID).Focused)
	c.True(tree.Focus != row.ID && tree.Focus != a.listID, "the focus must be reported within the menu")
	c.Equal(1, axFocusedCount(tree))
	c.Equal(0, len(a.screen.Errors()), "nothing should have panicked: %v", a.screen.Errors())
}

// TestListAccessibilityFocusOnARowOutOfSight verifies that the row the focus is reported on is described however far
// out of view it is, and that describing it as nothing but itself — which is all a row nobody can see is described as
// — is no obstacle to the focus being reported on it.
func TestListAccessibilityFocusOnARowOutOfSight(t *testing.T) {
	c := check.New(t)
	const rowCount = 60
	var list *unison.List[string]
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			list = unison.NewList[string]()
			list.Factory = &unison.DefaultCellFactory{Height: 20}
			for i := range rowCount {
				list.Append("Row " + strconv.Itoa(i))
			}
			wnd = newHeadlessWindow(t, "row out of sight", geom.NewRect(10, 10, 300, 300),
				axColumn(axScroller(list, geom.NewSize(200, 60))))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	if wnd == nil {
		return
	}
	screen.Do(func() {
		list.Select(false, rowCount-1)
		list.RequestFocus()
	})

	tree := screen.AccessibilityTree(wnd)
	c.True(tree != nil)
	if tree == nil {
		return
	}
	node := screen.AccessibilityNodeFor(list)
	c.True(node != nil)
	if node == nil {
		return
	}
	row := axNodeWithRowIndex(tree, node, rowCount-1)
	c.True(row != nil, "the row the focus is on must be described however far out of sight it is")
	if row == nil {
		return
	}
	c.Equal(0, len(row.Children), "a row nobody can see is described as nothing but itself")
	c.True(row.Focused)
	c.Equal(row.ID, tree.Focus)
	c.Equal(1, axFocusedCount(tree))
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestListLeadMovesWithTheRows verifies that the row the person is on, and the anchor a shift-click extends from, are
// each kept in step with the rows around them as they are added and removed, and let go of when their row itself goes
// away.
func TestListLeadMovesWithTheRows(t *testing.T) {
	c := check.New(t)
	l := newTestList("a", "b", "c", "d", "e")
	c.Equal(-1, l.Lead(), "a list nobody has been in has no lead row")
	l.Select(false, 2)
	c.Equal(2, l.Lead())
	c.Equal(2, l.Anchor())

	l.Insert(0, "x", "y")
	c.Equal(4, l.Lead(), "rows added above move the lead row down with the row it names")
	c.Equal(4, l.Anchor(), "rows added above move the anchor down with the row it names")
	l.Insert(6, "z")
	c.Equal(4, l.Lead(), "rows added below leave it where it is")
	c.Equal(4, l.Anchor(), "rows added below leave the anchor where it is")

	l.Remove(0)
	c.Equal(3, l.Lead())
	c.Equal(3, l.Anchor())
	l.Remove(5)
	c.Equal(3, l.Lead(), "a row removed below leaves the lead row where it is")
	c.Equal(3, l.Anchor(), "a row removed below leaves the anchor where it is")
	l.Remove(3)
	c.Equal(-1, l.Lead(), "the lead row is let go of when the row it names is removed")
	c.Equal(-1, l.Anchor(), "the anchor is let go of when the row it names is removed")

	l.Select(false, 4)
	c.Equal(4, l.Lead())
	c.Equal(4, l.Anchor())
	l.RemoveRange(0, 1)
	c.Equal(2, l.Lead())
	c.Equal(2, l.Anchor())
	l.RemoveRange(1, 2)
	c.Equal(-1, l.Lead())
	c.Equal(-1, l.Anchor())

	l.Select(false, 0)
	l.Clear()
	c.Equal(-1, l.Lead(), "a list with no rows has no lead row")

	l = newTestList("a", "b", "c", "d", "e")
	l.SelectRange(1, 3, false)
	c.Equal(3, l.Lead(), "the end of the range is where the selection arrived")
	l.SelectAll()
	c.Equal(3, l.Lead(), "selecting everything does not move the person")
	l.Select(false, 4)
	c.Equal(4, l.Lead())
	l.Select(false)
	c.Equal(-1, l.Lead(), "a selection that names nothing leaves nowhere to be")
	l.SelectAll()
	c.Equal(0, l.Lead(), "with nowhere to be, selecting everything reads from the first row")
}

// TestListRowsChangedDuringAPress verifies that rows inserted or removed while a press is in progress move the row the
// release narrows the selection to, and the selection saved at the press, which a drag extends from and the release
// compares against, along with the selection itself, and that clearing the rows forgets both.
func TestListRowsChangedDuringAPress(t *testing.T) {
	c := check.New(t)
	l := newTestList("A", "B", "C", "D", "E")
	l.Factory = &unison.DefaultCellFactory{Height: 20}
	l.SetFrameRect(geom.NewRect(0, 0, 200, 200))
	selections := 0
	l.NewSelectionCallback = func() { selections++ }
	rowPoint := func(row int) geom.Point { return geom.NewPoint(5, l.RowRect(row).CenterY()) }

	l.Select(false, 1)
	l.DefaultMouseDown(rowPoint(1), unison.ButtonLeft, 1, mod.None)
	l.Insert(0, "X")
	l.DefaultMouseUp(rowPoint(2), unison.ButtonLeft, mod.None)
	c.Equal([]int{2}, selectedIndexes(l), "B, now at 2, is what the release narrows the selection to")
	c.Equal(2, l.Lead())
	c.Equal(0, selections, "and nothing changed")

	l.DefaultMouseDown(rowPoint(2), unison.ButtonLeft, 1, mod.None)
	l.Remove(0)
	l.DefaultMouseUp(rowPoint(1), unison.ButtonLeft, mod.None)
	c.Equal([]int{1}, selectedIndexes(l), "B, back at 1, after a removal before it")
	c.Equal(1, l.Lead())
	c.Equal(0, selections)

	l.Select(false, 3)
	l.DefaultMouseDown(rowPoint(3), unison.ButtonLeft, 1, mod.None)
	l.RemoveRange(0, 1)
	l.DefaultMouseUp(rowPoint(1), unison.ButtonLeft, mod.None)
	c.Equal([]int{1}, selectedIndexes(l), "D, now at 1, after a removal of a range before it")
	c.Equal(1, l.Lead())
	c.Equal(0, selections)

	l.Select(false, 0)
	l.DefaultMouseDown(rowPoint(0), unison.ButtonLeft, 1, mod.None)
	l.Insert(0, "X")
	l.DefaultMouseDrag(rowPoint(2), unison.ButtonLeft, mod.Shift)
	c.Equal([]int{1, 2}, selectedIndexes(l), "a shift-drag extends from the saved selection as moved")
	l.DefaultMouseUp(rowPoint(2), unison.ButtonLeft, mod.Shift)
	c.Equal([]int{1, 2}, selectedIndexes(l))
	c.Equal(1, selections)

	l.Select(false, 1)
	l.DefaultMouseDown(rowPoint(1), unison.ButtonLeft, 1, mod.None)
	l.Clear()
	l.Append("Z")
	l.DefaultMouseUp(rowPoint(0), unison.ButtonLeft, mod.None)
	c.Equal([]int(nil), selectedIndexes(l), "nothing is selected among the new rows")
	c.Equal(-1, l.Lead())
	c.Equal(1, selections)
}
