// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"strconv"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/behavior"
	checkenum "github.com/richardwilkes/unison/enums/check"
	"github.com/richardwilkes/unison/enums/gradienttype"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/enums/role"
	"github.com/richardwilkes/unison/internal/testenv"
)

// These tests reach the parts of the accessibility core that a widget uses rather than an application: the builder a
// widget describes itself with, the virtual children a collection invents for rows it has no panels for, and the
// geometry the platforms that work in screen pixels convert with. A session owns most of the package's mutable globals
// while it runs, so none of these may call t.Parallel.

const (
	// axTestRowHeight is how tall each of axTestList's rows is, in its own coordinates.
	axTestRowHeight = 10
	// axTestColumns is how many cells each of axTestList's rows holds.
	axTestColumns = 2
)

// axTestCellKey identifies one cell of axTestList, standing in for the accessibility.CellKey a real table uses.
type axTestCellKey struct {
	row int
	col int
}

// axTestList is a widget that has rows but no panel per row, which is how a list or a table is built. It describes each
// row as a virtual child, keyed by the row's index so that the same row keeps the same node id however the rows around
// it change, and gives each row cells of its own so that more than one level of virtual children is exercised.
type axTestList struct {
	window *Window
	rows   []string
	acted  []accessibility.ActionRequest
	Panel
	visible geom.Rect
	focused bool
}

// newAXTestList creates a list of numbered rows.
func newAXTestList(count int) *axTestList {
	l := &axTestList{}
	l.Self = l
	l.rows = make([]string, count)
	for i := range l.rows {
		l.rows[i] = "row " + strconv.Itoa(i)
	}
	l.SetSizer(l.sizes)
	return l
}

func (l *axTestList) sizes(_ geom.Size) (minSize, prefSize, maxSize geom.Size) {
	size := geom.NewSize(100, axTestRowHeight*float32(len(l.rows)))
	return size, size, size
}

// ProvideAccessibility describes the list and each of its rows.
func (l *axTestList) ProvideAccessibility(b *AccessibilityBuilder) {
	node := b.Node()
	node.Role = role.List
	node.Name = "Rows"
	node.RowCount = len(l.rows)
	node.ColumnCount = axTestColumns
	node.Multiselectable = true
	// Recorded so the test can check that the builder answers these for the panel being described.
	l.visible = b.VisibleRect()
	l.window = b.Window()
	l.focused = b.Focused()
	for i, text := range l.rows {
		rowID := b.AddVirtualChild(i, func(n *accessibility.Node) {
			n.Role = role.ListItem
			n.Name = text
			n.RowIndex = i
			n.Bounds = geom.NewRect(0, axTestRowHeight*float32(i), 100, axTestRowHeight)
			n.Actions = n.Actions.With(accessibility.Select)
		})
		for col := range axTestColumns {
			b.AddVirtualChildOf(rowID, axTestCellKey{row: i, col: col}, func(n *accessibility.Node) {
				n.Role = role.Cell
				n.Name = text
				n.ColumnIndex = col
				n.Bounds = geom.NewRect(50*float32(col), axTestRowHeight*float32(i), 50, axTestRowHeight)
			})
		}
	}
}

// PerformAccessibilityAction records the selections it is asked for and refuses everything else.
func (l *axTestList) PerformAccessibilityAction(req accessibility.ActionRequest) bool {
	if req.Action != accessibility.Select {
		return false
	}
	l.acted = append(l.acted, req)
	return true
}

// axNewTestWindow creates a window whose content is the given panel, laid out to fill it, and shows it.
func axNewTestWindow(t *testing.T, title string, rect geom.Rect, panel Paneler) *Window {
	t.Helper()
	w, err := NewWindow(title)
	if err != nil {
		t.Errorf("unable to create window %q: %v", title, err)
		return nil
	}
	w.Content().SetLayout(&FlexLayout{
		Columns:  1,
		HSpacing: StdHSpacing,
		VSpacing: StdVSpacing,
	})
	w.Content().AddChild(panel)
	w.SetContentRect(rect)
	w.Show()
	return w
}

// TestAccessibilityVirtualChildren verifies what a collection widget can do: describe rows it has no panels for, have
// each row keep its identity from one description to the next, receive requests aimed at one particular row, and have
// the ids of rows it has stopped using reclaimed.
func TestAccessibilityVirtualChildren(t *testing.T) {
	c := check.New(t)
	const initialRows = 30
	var list *axTestList
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 400, Height: 500},
		StartupFinishedCallback(func() {
			list = newAXTestList(initialRows)
			wnd = axNewTestWindow(t, "virtual", geom.NewRect(10, 10, 300, 480), list)
		}))
	c.NotNil(list)
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	c.True(tree != nil)
	listNode := screen.AccessibilityNodeFor(list)
	c.True(listNode != nil)
	c.Equal(role.List, listNode.Role)
	c.Equal("Rows", listNode.Name)
	c.Equal(initialRows, listNode.RowCount)
	c.Equal(initialRows, len(listNode.Children), "every row should be a child of the list")

	firstRow := tree.Node(listNode.Children[0])
	c.True(firstRow != nil)
	c.Equal(role.ListItem, firstRow.Role)
	c.Equal("row 0", firstRow.Name)
	c.Equal(listNode.ID, firstRow.Parent)
	c.Equal(listNode.Bounds.Point, firstRow.Bounds.Point,
		"a virtual child's bounds should have been converted out of the panel's own coordinates")
	c.Equal(float32(axTestRowHeight), firstRow.Bounds.Height)
	c.Equal(axTestColumns, len(firstRow.Children), "the row's cells should be children of the row")
	cell := tree.Node(firstRow.Children[0])
	c.True(cell != nil)
	c.Equal(role.Cell, cell.Role)
	c.Equal(firstRow.ID, cell.Parent)

	var visible geom.Rect
	var describedWindow *Window
	var focusedWhenDescribed bool
	screen.Do(func() {
		visible = list.visible
		describedWindow = list.window
		focusedWhenDescribed = list.focused
	})
	c.False(visible.Empty(), "the list is on the screen, so some of it is visible")
	c.True(describedWindow == wnd, "the builder should report the window being described")
	c.False(focusedWhenDescribed, "nothing gave the list the focus")

	// Asking again must reuse the ids, since an assistive technology's notion of a row has to survive the snapshot it
	// was found in.
	screen.AccessibilityTree(wnd)
	again := screen.AccessibilityNodeFor(list)
	c.True(again != nil)
	c.Equal(listNode.Children, again.Children, "the rows should have kept their ids")

	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   firstRow.ID,
		Action: accessibility.Select,
	}), "the list should have handled the selection")
	var acted []accessibility.ActionRequest
	screen.Do(func() { acted = list.acted })
	c.Equal(1, len(acted))
	if len(acted) == 1 {
		c.Equal(0, acted[0].Key, "the request should name the row's own key")
		c.Equal(firstRow.ID, acted[0].Node)
	}
	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   firstRow.ID,
		Action: accessibility.Focus,
	}), "a virtual child gets none of the default behaviors, since those act on a panel")

	// Dropping most of the rows leaves the ids of the ones that are gone to be reclaimed, which happens the next time
	// the list is described. Each row accounts for one id of its own plus one per cell.
	screen.Do(func() { list.rows = list.rows[:3] })
	screen.AccessibilityTree(wnd)
	var held int
	screen.Do(func() { held = len(list.Accessibility.virtual) })
	c.Equal(3*(1+axTestColumns), held, "the ids of the rows that are gone should have been reclaimed")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// axDuplicateKeyList is a widget that misuses the builder by handing the same key back more than once while describing
// itself, which is what a table whose rows carry duplicate tid.TIDs, or a widget that adds one key under two parents,
// does. It records what each attempt was given so the test can see which of them were refused.
type axDuplicateKeyList struct {
	Panel
	first    accessibility.NodeID
	repeat   accessibility.NodeID
	nested   accessibility.NodeID
	distinct accessibility.NodeID
}

// newAXDuplicateKeyList creates the widget, sized so that it has a frame to describe children within.
func newAXDuplicateKeyList() *axDuplicateKeyList {
	l := &axDuplicateKeyList{}
	l.Self = l
	l.SetSizer(func(_ geom.Size) (minSize, prefSize, maxSize geom.Size) {
		size := geom.NewSize(100, 2*axTestRowHeight)
		return size, size, size
	})
	return l
}

// ProvideAccessibility adds one row, then tries to add it again under the same parent and under itself, and finally
// adds a row with a key of its own to show that the refusals cost the description nothing else.
func (l *axDuplicateKeyList) ProvideAccessibility(b *AccessibilityBuilder) {
	node := b.Node()
	node.Role = role.List
	node.Name = "Rows"
	l.first = b.AddVirtualChild("row", func(n *accessibility.Node) {
		n.Role = role.ListItem
		n.Name = "first"
		n.Bounds = geom.NewRect(0, 0, 100, axTestRowHeight)
	})
	l.repeat = b.AddVirtualChild("row", func(n *accessibility.Node) {
		n.Role = role.ListItem
		n.Name = "second"
		n.Bounds = geom.NewRect(0, axTestRowHeight, 100, axTestRowHeight)
	})
	l.nested = b.AddVirtualChildOf(l.first, "row", func(n *accessibility.Node) {
		n.Role = role.Cell
		n.Name = "nested"
		n.Bounds = geom.NewRect(0, 0, 50, axTestRowHeight)
	})
	l.distinct = b.AddVirtualChild("other", func(n *accessibility.Node) {
		n.Role = role.ListItem
		n.Name = "other"
		n.Bounds = geom.NewRect(0, axTestRowHeight, 100, axTestRowHeight)
	})
}

// TestAccessibilityDuplicateVirtualKeyRefused verifies that a key which has already been used during one description
// adds nothing the second time. Letting it through would replace what the first node said and list its id twice among
// its parent's children, which everything counted out of that list — the index within the parent, Tree.PositionInSet
// and the events the next Diff produces — would then be wrong about.
func TestAccessibilityDuplicateVirtualKeyRefused(t *testing.T) {
	c := check.New(t)
	var list *axDuplicateKeyList
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 400, Height: 300},
		StartupFinishedCallback(func() {
			list = newAXDuplicateKeyList()
			wnd = axNewTestWindow(t, "duplicate keys", geom.NewRect(10, 10, 200, 150), list)
		}))
	c.NotNil(list)
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	c.True(tree != nil)
	listNode := screen.AccessibilityNodeFor(list)
	c.True(listNode != nil)
	if listNode == nil {
		return
	}
	var first, repeat, nested, distinct accessibility.NodeID
	screen.Do(func() {
		first, repeat, nested, distinct = list.first, list.repeat, list.nested, list.distinct
	})
	c.True(first != 0, "the first use of a key adds the child")
	c.Equal(accessibility.NodeID(0), repeat, "the same key a second time must be refused")
	c.Equal(accessibility.NodeID(0), nested, "the same key under another parent must be refused")
	c.True(distinct != 0, "a key of its own must still be honored after a refusal")
	c.Equal([]accessibility.NodeID{first, distinct}, listNode.Children,
		"the refused attempts must have left the list with one child per key")

	firstNode := tree.Node(first)
	c.True(firstNode != nil)
	if firstNode != nil {
		c.Equal("first", firstNode.Name, "the node must say what it said the first time")
		c.Equal(0, len(firstNode.Children), "the refused attempt must not have become a child of it either")
	}
	pos, size := tree.PositionInSet(distinct)
	c.Equal(2, pos, "the second child is the second of two")
	c.Equal(2, size)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestAccessibilityGeometry verifies the conversion the platforms whose accessibility APIs work in physical screen
// pixels apply to the window-local, logical-unit bounds a tree holds.
func TestAccessibilityGeometry(t *testing.T) {
	c := check.New(t)
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 800, Height: 600, Scale: 2},
		StartupFinishedCallback(func() {
			wnd = axNewTestWindow(t, "geometry", geom.NewRect(20, 30, 200, 100), NewPanel())
		}))
	c.NotNil(wnd)

	var origin, scale, contentOrigin geom.Point
	screen.Do(func() {
		origin, scale = wnd.accessibilityGeometry()
		contentOrigin = wnd.ContentRect().Point
		// Nothing observable follows from this on a headless window, but the wrapper is what the platform windows call
		// when they move, resize or change backing scale, so it must survive being called.
		wnd.apiAccessibilityGeometryChanged()
	})
	c.Equal(geom.NewPoint(2, 2), scale, "the scale is the window's backing scale")
	c.Equal(geom.NewPoint(20, 30), contentOrigin)
	c.Equal(contentOrigin.MulPt(scale), origin, "the origin is the content area's position in physical pixels")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestAccessibilityDeactivationReleasesState verifies that turning accessibility support off drops everything it was
// holding, so that a screen reader going away costs an application nothing thereafter.
func TestAccessibilityDeactivationReleasesState(t *testing.T) {
	c := check.New(t)
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 400, Height: 300},
		StartupFinishedCallback(func() {
			wnd = axNewTestWindow(t, "deactivate", geom.NewRect(20, 20, 240, 120), NewPanel())
		}))
	c.NotNil(wnd)

	c.True(screen.AccessibilityTree(wnd) != nil)
	var held bool
	screen.Do(func() { held = wnd.ax != nil })
	c.True(held, "the window should be holding the description it published")

	screen.Do(deactivateAccessibility)
	c.False(IsAccessibilityActive())
	screen.Do(func() { held = wnd.ax != nil })
	c.False(held, "deactivation should have freed what the window was holding")
	var tree *accessibility.Tree
	screen.Do(func() {
		if hw := headlessWindowFor(wnd); hw != nil {
			tree = hw.axTree
		}
	})
	c.True(tree == nil, "the adapter should have been shut down")

	// Redrawing must not describe anything now that support is off.
	var before, after uint64
	screen.Do(func() {
		before = axSnapshotCount
		wnd.MarkForRedraw()
	})
	screen.Do(func() { after = axSnapshotCount })
	c.Equal(before, after, "no snapshot may be built while accessibility support is off")
}

// TestAccessibilityDisposalReleasesState verifies the teardown a window performs on its way out. Window.destroy shuts
// the adapter down before the platform window goes away, since an adapter's teardown talks to that window, and
// everything the window was holding for an assistive technology — the tree every query was answered from, and any
// events nothing had drained — goes with it.
func TestAccessibilityDisposalReleasesState(t *testing.T) {
	c := check.New(t)
	var doomed, other *Window
	var panel *Panel
	presses := 0
	screen := startHeadlessTest(t, HeadlessConfig{Width: 400, Height: 300},
		StartupFinishedCallback(func() {
			// The second window is there so that disposing of the first does not close the last window and end the
			// session.
			other = axNewTestWindow(t, "other", geom.NewRect(280, 20, 100, 100), NewPanel())
			panel = axNewClickable(&presses)
			panel.Accessibility.Name = "Before"
			doomed = axNewTestWindow(t, "doomed", geom.NewRect(20, 20, 240, 120), panel)
		}))
	c.NotNil(doomed)
	c.NotNil(other)

	c.True(screen.AccessibilityTree(doomed) != nil)
	// Renaming something and describing the window again leaves events nothing has drained, which is the state the
	// teardown has to cope with.
	screen.Do(func() { panel.Accessibility.Name = "After" })
	c.True(screen.AccessibilityTree(doomed) != nil)
	var tree *accessibility.Tree
	var events int
	var held bool
	read := func() {
		screen.Do(func() {
			held = doomed.ax != nil
			tree = nil
			events = 0
			if hw := headlessWindowFor(doomed); hw != nil {
				tree = hw.axTree
				events = len(hw.axEvents)
			}
		})
	}
	read()
	c.True(held, "the window should be holding the description it published")
	c.True(tree != nil, "the adapter should be holding the tree it answers from")
	c.True(events > 0, "the rename should have left an event nothing has drained")

	c.True(screen.Do(func() { doomed.Dispose() }))
	read()
	c.False(held, "disposal should have freed what the window was holding")
	c.True(tree == nil, "the adapter should have been shut down")
	c.Equal(0, events, "events nothing drained should have gone with the window")
	c.True(screen.AccessibilityTree(doomed) == nil, "a window that is gone has nothing to describe")

	// The window that is left is unaffected: support is still on and it is still described.
	c.True(IsAccessibilityActive())
	c.True(screen.AccessibilityTree(other) != nil)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// axNewClickable returns a panel that counts the clicks it is given, which is what an assistive technology's Press
// synthesizes for a panel that has no other notion of being activated.
func axNewClickable(presses *int) *Panel {
	panel := NewPanel()
	panel.SetSizer(func(_ geom.Size) (minSize, prefSize, maxSize geom.Size) {
		size := geom.NewSize(80, 24)
		return size, size, size
	})
	panel.SetFocusable(true)
	panel.Accessibility.Role = role.Button
	panel.MouseDownCallback = func(_ geom.Point, _, _ int, _ mod.Modifiers) bool { return true }
	panel.MouseUpCallback = func(_ geom.Point, _ int, _ mod.Modifiers) bool {
		*presses++
		return true
	}
	return panel
}

// TestAccessibilityActionsRefusedWhileBlockedByAModal verifies that a window another window's modal is blocking refuses
// requests from an assistive technology, as it refuses mouse events. Such a window is still described — only the top
// modal window's root says it is modal — so without this an assistive technology could press buttons and move the focus
// in a window the person cannot touch.
func TestAccessibilityActionsRefusedWhileBlockedByAModal(t *testing.T) {
	c := check.New(t)
	var blockedPresses, modalPresses int
	var blockedPanel, modalPanel *Panel
	var blocked, modal *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 500, Height: 400},
		StartupFinishedCallback(func() {
			blockedPanel = axNewClickable(&blockedPresses)
			blocked = axNewTestWindow(t, "blocked", geom.NewRect(10, 10, 240, 120), blockedPanel)
			modalPanel = axNewClickable(&modalPresses)
			modal = axNewTestWindow(t, "modal", geom.NewRect(60, 60, 200, 100), modalPanel)
		}))
	c.NotNil(blocked)
	c.NotNil(modal)
	t.Cleanup(func() { screen.Do(func() { modalStack = nil }) })

	screen.AccessibilityTree(blocked)
	screen.AccessibilityTree(modal)
	blockedNode := screen.AccessibilityNodeFor(blockedPanel)
	modalNode := screen.AccessibilityNodeFor(modalPanel)
	c.True(blockedNode != nil)
	c.True(modalNode != nil)
	if blockedNode == nil || modalNode == nil {
		return
	}
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   blockedNode.ID,
		Action: accessibility.Press,
	}), "nothing is modal yet, so the window may be acted on")

	// That press moved the focus onto the panel, as a person's click would have. It is taken away again so that the
	// focus a refused request must not produce cannot be one the panel already had.
	screen.Do(func() {
		blocked.SetFocus(nil)
		modalStack = append(modalStack, modal)
	})
	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   blockedNode.ID,
		Action: accessibility.Press,
	}), "a window blocked by a modal must refuse to be pressed")
	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   blockedNode.ID,
		Action: accessibility.Focus,
	}), "a window blocked by a modal must refuse to move its focus")
	var presses int
	var focus *Panel
	screen.Do(func() {
		presses = blockedPresses
		focus = blocked.CurrentFocus()
	})
	c.Equal(1, presses, "nothing in the blocked window may have run")
	c.True(focus == nil || !focus.Is(blockedPanel), "the blocked window's focus must not have moved")

	// The modal itself is still fully usable, and the blocked window is still described, since an assistive technology
	// is shown what is on the screen.
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   modalNode.ID,
		Action: accessibility.Press,
	}), "the top modal window accepts requests")
	screen.Do(func() { presses = modalPresses })
	c.Equal(1, presses)
	c.True(screen.AccessibilityTree(blocked) != nil, "a blocked window is still described")

	screen.Do(func() { modalStack = nil })
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   blockedNode.ID,
		Action: accessibility.Press,
	}), "the window may be acted on again once the modal has gone")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestAccessibilityInfoIdentityIsNotShared verifies what assigning AccessibilityInfo as a whole does. The identity a
// panel carries in it — its node id and the ids it has handed out for its virtual children — belongs to the one panel
// holding it, so copying one panel's information onto another must not leave two live panels describing themselves
// under the same ids, which would overwrite one another in the tree and make each a child of two different parents.
func TestAccessibilityInfoIdentityIsNotShared(t *testing.T) {
	c := check.New(t)
	const rows = 3
	var first, second *axTestList
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 400, Height: 500},
		StartupFinishedCallback(func() {
			first = newAXTestList(rows)
			second = newAXTestList(rows)
			content := NewPanel()
			content.SetLayout(&FlexLayout{Columns: 1, HSpacing: StdHSpacing, VSpacing: StdVSpacing})
			content.AddChild(first)
			content.AddChild(second)
			wnd = axNewTestWindow(t, "identity", geom.NewRect(10, 10, 300, 480), content)
		}))
	c.NotNil(wnd)

	screen.AccessibilityTree(wnd)
	var firstID, secondID accessibility.NodeID
	screen.Do(func() {
		firstID = first.Accessibility.id
		secondID = second.Accessibility.id
		// What an application might write meaning only to copy the name across.
		second.Accessibility = first.Accessibility
	})
	c.True(firstID != 0 && secondID != 0)

	tree := screen.AccessibilityTree(wnd)
	c.True(tree != nil)
	if tree == nil {
		return
	}
	var firstAfter, secondAfter accessibility.NodeID
	var sharedVirtual bool
	screen.Do(func() {
		firstAfter = first.Accessibility.id
		secondAfter = second.Accessibility.id
		sharedVirtual = len(first.Accessibility.virtual) != 0 &&
			first.Accessibility.virtual[0].id == second.Accessibility.virtual[0].id
	})
	c.Equal(firstID, firstAfter, "the panel the identity belongs to keeps it")
	c.True(secondAfter != 0 && secondAfter != firstAfter, "the panel it was copied onto must be given one of its own")
	c.False(sharedVirtual, "the ids handed out for virtual children must not be shared either")

	firstNode := tree.Node(firstAfter)
	secondNode := tree.Node(secondAfter)
	c.True(firstNode != nil, "the first list should still be in the tree")
	c.True(secondNode != nil, "the second list should still be in the tree")
	if firstNode == nil || secondNode == nil {
		return
	}
	c.Equal(rows, len(firstNode.Children))
	c.Equal(rows, len(secondNode.Children))
	for i, id := range firstNode.Children {
		c.True(id != secondNode.Children[i], "the two lists' rows must have ids of their own")
	}
	c.Equal(firstNode.Parent, secondNode.Parent)
	parent := tree.Node(firstNode.Parent)
	c.True(parent != nil)
	if parent != nil {
		c.Equal(2, len(parent.Children), "each list should appear once beneath the panel holding them")
	}

	// Assigning a fresh struct costs the panel its identity, which is what the documentation warns of, but must still
	// leave a well-formed tree behind.
	screen.Do(func() { second.Accessibility = AccessibilityInfo{Description: "Replaced"} })
	tree = screen.AccessibilityTree(wnd)
	var replacedID accessibility.NodeID
	screen.Do(func() { replacedID = second.Accessibility.id })
	c.True(replacedID != 0 && replacedID != secondAfter, "the panel should have been given a new identity")
	replaced := tree.Node(replacedID)
	c.True(replaced != nil, "the panel should still be described, under the identity it was given")
	if replaced != nil {
		c.Equal("Replaced", replaced.Description)
		c.Equal(rows, len(replaced.Children), "its rows should have been given ids of their own too")
	}
	c.True(tree.Node(secondAfter) == nil, "the identity it gave up must not still be in the tree")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// axSweepList describes one virtual child per key it is holding, and then makes a number of further attempts with the
// first of those keys, which the builder refuses. It is what shows that a refused attempt costs the panel nothing: the
// count of children a panel is actually using is what the sweep of abandoned ids measures its holdings against, so
// counting refusals as uses would keep ids alive that nothing is using any more.
//
// The children are also what shows that a virtual child is given a concrete role: the first is described as role.None
// and the rest are left with no role at all, neither of which may reach a published tree.
type axSweepList struct {
	keys []string
	Panel
	repeats int
}

// newAXSweepList creates a list holding the given number of keys.
func newAXSweepList(count int) *axSweepList {
	l := &axSweepList{}
	l.Self = l
	l.keys = make([]string, count)
	for i := range l.keys {
		l.keys[i] = "k" + strconv.Itoa(i)
	}
	l.SetSizer(func(_ geom.Size) (minSize, prefSize, maxSize geom.Size) {
		size := geom.NewSize(100, axTestRowHeight*float32(max(count, 1)))
		return size, size, size
	})
	return l
}

// ProvideAccessibility adds one child per key, then repeats the first key.
func (l *axSweepList) ProvideAccessibility(b *AccessibilityBuilder) {
	node := b.Node()
	node.Role = role.List
	node.Name = "Sweep"
	for i, key := range l.keys {
		b.AddVirtualChild(key, func(n *accessibility.Node) {
			if i == 0 {
				n.Role = role.None
			}
			n.Name = key
			n.Bounds = geom.NewRect(0, axTestRowHeight*float32(i), 100, axTestRowHeight)
		})
	}
	for range l.repeats {
		b.AddVirtualChild(l.keys[0], func(n *accessibility.Node) { n.Name = "repeat" })
	}
}

// TestAccessibilityVirtualChildRoleIsResolved verifies that a virtual child reaches a published tree with a role it can
// have: neither role.Auto, which for a child a widget invented there is nothing to work out from, nor role.None, which
// is an instruction about a panel. Adapters are told a published tree holds neither.
func TestAccessibilityVirtualChildRoleIsResolved(t *testing.T) {
	c := check.New(t)
	var list *axSweepList
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 400, Height: 300},
		StartupFinishedCallback(func() {
			list = newAXSweepList(2)
			wnd = axNewTestWindow(t, "virtual roles", geom.NewRect(10, 10, 200, 150), list)
		}))
	c.NotNil(list)
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	c.True(tree != nil)
	listNode := screen.AccessibilityNodeFor(list)
	c.True(listNode != nil)
	if listNode == nil {
		return
	}
	c.Equal(2, len(listNode.Children))
	for i, id := range listNode.Children {
		child := tree.Node(id)
		c.True(child != nil)
		if child != nil {
			c.Equal(role.Group, child.Role, "child %d", i)
		}
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestAccessibilityRefusedVirtualKeyIsNotAUse verifies that a key the builder refused does not make the panel look like
// it is using more children than it is. The ids of children a panel has stopped using are reclaimed once it is holding
// appreciably more of them than it needs, and a refusal counted as a use raises that threshold — which is the opposite
// of what refusing the key is saying.
func TestAccessibilityRefusedVirtualKeyIsNotAUse(t *testing.T) {
	c := check.New(t)
	const initialKeys = 40
	var list *axSweepList
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 400, Height: 600},
		StartupFinishedCallback(func() {
			list = newAXSweepList(initialKeys)
			wnd = axNewTestWindow(t, "sweep", geom.NewRect(10, 10, 300, 560), list)
		}))
	c.NotNil(list)
	c.NotNil(wnd)

	screen.AccessibilityTree(wnd)
	var held int
	screen.Do(func() { held = len(list.Accessibility.virtual) })
	c.Equal(initialKeys, held, "each key should have been given an id of its own")

	// One key, and a pile of attempts to use it again. The panel is using one child, so the ids of the other
	// thirty-nine have nothing keeping them alive.
	screen.Do(func() {
		list.keys = list.keys[:1]
		list.repeats = initialKeys / 2
	})
	screen.AccessibilityTree(wnd)
	screen.Do(func() { held = len(list.Accessibility.virtual) })
	c.Equal(1, held, "the ids of the keys that are gone should have been reclaimed")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestAccessibilityPublishThrottle covers the throttle that bounds how often a window that redraws continuously is
// described. Headless sessions bypass it so that a test asking for a tree is given the current one, which leaves it
// with nothing else that could drive it, so it is turned on here by hand.
func TestAccessibilityPublishThrottle(t *testing.T) {
	testenv.SkipTimingSensitive(t)
	c := check.New(t)
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 400, Height: 300},
		StartupFinishedCallback(func() {
			wnd = axNewTestWindow(t, "throttle", geom.NewRect(20, 20, 240, 120), NewPanel())
		}))
	c.NotNil(wnd)
	screen.Do(func() { axThrottleHeadless = true })
	t.Cleanup(func() { screen.Do(func() { axThrottleHeadless = false }) })

	// The first publish of a window is never throttled, whatever the session, so that an adapter which activates on its
	// first query has something to answer with before it returns.
	c.True(screen.AccessibilityTree(wnd) != nil)
	var generation uint64
	var queued bool
	screen.Do(func() {
		generation = wnd.ax.generation
		wnd.publishAccessibility()
		queued = wnd.ax.publishQueued
	})
	c.True(queued, "a publish that came too soon after the last one should have been queued")
	var after uint64
	screen.Do(func() { after = wnd.ax.generation })
	c.Equal(generation, after, "and should not have described the window")

	// The queued task fires on a timer, which Sync deliberately does not wait for, so it is waited for here.
	c.True(axWaitForPublish(t, screen, wnd, generation), "the queued publish should have described the window")
	screen.Do(func() { queued = wnd.ax.publishQueued })
	c.False(queued, "and should have left nothing queued behind it")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestAccessibilityPublishThrottleSurvivesDeactivation covers what becomes of a queued publish when the support it was
// queued for goes away before the task runs, which is what deactivating support, or destroying the window, does: the
// state the task would publish from is dropped while the task is still in flight.
func TestAccessibilityPublishThrottleSurvivesDeactivation(t *testing.T) {
	testenv.SkipTimingSensitive(t)
	c := check.New(t)
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 400, Height: 300},
		StartupFinishedCallback(func() {
			wnd = axNewTestWindow(t, "throttle off", geom.NewRect(20, 20, 240, 120), NewPanel())
		}))
	c.NotNil(wnd)
	screen.Do(func() { axThrottleHeadless = true })
	t.Cleanup(func() { screen.Do(func() { axThrottleHeadless = false }) })

	c.True(screen.AccessibilityTree(wnd) != nil)
	var queued bool
	screen.Do(func() {
		wnd.publishAccessibility()
		queued = wnd.ax.publishQueued
	})
	c.True(queued)

	screen.Do(deactivateAccessibility)
	var held bool
	screen.Do(func() { held = wnd.ax != nil })
	c.False(held, "deactivation should have dropped what the queued publish would have published from")

	// The task still runs, and must find nothing rather than a window it can describe.
	time.Sleep(2 * axPublishThrottle)
	screen.Sync()
	screen.Do(func() { held = wnd.ax != nil })
	c.False(held, "the queued publish must not have brought the state back")
	c.False(IsAccessibilityActive())
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// axWaitForPublish waits for the window to be described again, which a queued publish does on a timer that Sync does
// not wait for. Reports whether it happened before waiting went on too long.
func axWaitForPublish(t *testing.T, screen *HeadlessScreen, w *Window, was uint64) bool {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		var generation uint64
		screen.Do(func() {
			if w.ax != nil {
				generation = w.ax.generation
			}
		})
		if generation > was {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(time.Millisecond)
	}
}

// axNodesOf returns the node each request names, which is what a test asserts a set of them by: the requests the widget
// was given carry the key the dispatcher filled in for the virtual child, and the ones it was asked for do not.
func axNodesOf(reqs []accessibility.ActionRequest) []accessibility.NodeID {
	ids := make([]accessibility.NodeID, 0, len(reqs))
	for _, req := range reqs {
		ids = append(ids, req.Node)
	}
	return ids
}

// TestAccessibilityBatchedActionsPublishOnce verifies that a set of requests an assistive technology made as one is
// carried out as one: every request reaches the widget, and the window is described a single time at the end rather
// than once per request.
//
// Setting which rows of a table are selected is the only thing that arrives this way. The schema has no request that
// replaces a selection wholesale, so the macOS adapter turns one of those into a Select and an AddToSelection apiece
// and hands the set over together; carried out one at a time, each would lay the window out and build, diff and publish
// a complete snapshot, so naming k rows would cost k of them — and publish k intermediate selections an assistive
// technology may read — inside the single callback it is waiting on. The single-request path is measured alongside it,
// so what the test pins is the difference rather than a number.
func TestAccessibilityBatchedActionsPublishOnce(t *testing.T) {
	c := check.New(t)
	const rows = 5
	var list *axTestList
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 400, Height: 300},
		StartupFinishedCallback(func() {
			list = newAXTestList(rows)
			wnd = axNewTestWindow(t, "batched", geom.NewRect(10, 10, 300, 280), list)
		}))
	c.NotNil(list)
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	c.True(tree != nil)
	listNode := screen.AccessibilityNodeFor(list)
	c.True(listNode != nil)
	if listNode == nil {
		return
	}
	c.Equal(rows, len(listNode.Children))
	reqs := make([]accessibility.ActionRequest, 0, 3)
	for _, id := range listNode.Children[:3] {
		reqs = append(reqs, accessibility.ActionRequest{Node: id, Action: accessibility.Select})
	}

	// Read on either side of the call within the same visit to the UI thread, so that nothing else — a redraw, or the
	// task a throttled publish left behind — can be counted as part of what the requests cost.
	var carried bool
	var before, after uint64
	var acted []accessibility.ActionRequest
	screen.Do(func() {
		list.acted = nil
		before = wnd.ax.generation
		carried = wnd.performAccessibilityActions(reqs)
		after = wnd.ax.generation
		acted = list.acted
	})
	c.True(carried, "the list handles every one of these, so the set was carried out")
	c.Equal(axNodesOf(reqs), axNodesOf(acted),
		"every request of the set should have reached the widget, in the order it was given")
	c.Equal(uint64(1), after-before, "a set of three requests should have described the window exactly once")
	if len(acted) == len(reqs) {
		for i, req := range acted {
			c.Equal(i, req.Key, "each request of a set should name the row it was aimed at, as a single one does")
		}
	}

	// The same three one at a time, which is what the set is there to avoid: one description apiece.
	screen.Do(func() {
		list.acted = nil
		before = wnd.ax.generation
		for _, req := range reqs {
			wnd.performAccessibilityAction(req)
		}
		after = wnd.ax.generation
		acted = list.acted
	})
	c.Equal(axNodesOf(reqs), axNodesOf(acted))
	c.Equal(uint64(len(reqs)), after-before, "one at a time, each request describes the window for itself")

	// A set holding nothing the window will act on describes it not at all: the publish follows the requests that were
	// carried out, exactly as it does for a single one.
	screen.Do(func() {
		list.acted = nil
		before = wnd.ax.generation
		carried = wnd.performAccessibilityActions([]accessibility.ActionRequest{
			{Node: listNode.Children[0], Action: accessibility.Press},
			{Node: 0, Action: accessibility.Select},
		})
		after = wnd.ax.generation
		acted = list.acted
	})
	c.False(carried, "neither request could be carried out")
	c.Equal(0, len(acted))
	c.Equal(uint64(0), after-before, "a set that achieved nothing should not have described the window")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// axCaretDoc stands in for a Markdown: a focusable panel with a document's role, holding blocks of its own.
type axCaretDoc struct {
	Panel
}

// newAXCaretDoc creates a document holding two blocks, the first of which claims to hold the reading caret.
func newAXCaretDoc() (doc *axCaretDoc, caretBlock, otherBlock *axCaretBlock) {
	doc = &axCaretDoc{}
	doc.Self = doc
	doc.SetFocusable(true)
	doc.SetLayout(&FlexLayout{Columns: 1})
	doc.Accessibility.Role = role.Document
	caretBlock = newAXCaretBlock(true)
	otherBlock = newAXCaretBlock(false)
	doc.AddChild(caretBlock)
	doc.AddChild(otherBlock)
	return doc, caretBlock, otherBlock
}

func (d *axCaretDoc) ProvideAccessibility(b *AccessibilityBuilder) {
	b.Node().Name = "Document"
}

// axCaretBlock stands in for one block of a document's content, which reports the focus for itself when it is the block
// the reading caret is in. See axCaretBlockReportsFocus, which is what decides whether that claim stands.
type axCaretBlock struct {
	Panel
	holdsCaret bool
}

// newAXCaretBlock creates one block, which claims the focus only if it is the one holding the caret.
func newAXCaretBlock(holdsCaret bool) *axCaretBlock {
	b := &axCaretBlock{holdsCaret: holdsCaret}
	b.Self = b
	b.SetSizer(func(_ geom.Size) (minSize, prefSize, maxSize geom.Size) {
		size := geom.NewSize(100, 20)
		return size, size, size
	})
	return b
}

func (b *axCaretBlock) ProvideAccessibility(builder *AccessibilityBuilder) {
	node := builder.Node()
	node.Role = role.Paragraph
	node.Name = "Block"
	node.ReadOnly = true
	node.Text = &accessibility.TextInfo{Text: "block"}
	if b.holdsCaret {
		// Claimed whatever the platform does with it, so that what the builder makes of the claim is what this pins.
		node.Focused = true
	}
}

// TestAccessibilityCaretBlockReportsFocus covers the second claim on the focus a document's block makes while it holds
// the reading caret: on the platforms whose screen readers need it the claim stands, and the document is still what
// Tree.Focus names, while everywhere else the builder takes it away. Two elements claiming the focus is ordinary on
// AT-SPI, which is how Orca presents a caret the application moved, and wrong on UI Automation and AppKit, which have
// one focused element apiece.
//
// Both answers are pinned whichever platform this runs on, since the widget that will rely on this is built once for
// all three.
func TestAccessibilityCaretBlockReportsFocus(t *testing.T) {
	c := check.New(t)
	var doc *axCaretDoc
	var caretBlock, otherBlock *axCaretBlock
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 400, Height: 400},
		StartupFinishedCallback(func() {
			doc, caretBlock, otherBlock = newAXCaretDoc()
			wnd = axNewTestWindow(t, "caret focus", geom.NewRect(10, 10, 300, 300), doc)
			if wnd != nil {
				wnd.ToFront()
				wnd.SetFocus(doc)
			}
		}))
	c.NotNil(wnd)
	if wnd == nil {
		return
	}
	saved := axCaretBlockReportsFocus
	defer screen.Do(func() { axCaretBlockReportsFocus = saved })

	for _, reports := range []bool{true, false} {
		// Written on the UI thread, which is the only thread that reads it, since a snapshot is built there.
		screen.Do(func() { axCaretBlockReportsFocus = reports })
		tree := screen.AccessibilityTree(wnd)
		c.NotNil(tree)
		docNode := screen.AccessibilityNodeFor(doc)
		caretNode := screen.AccessibilityNodeFor(caretBlock)
		otherNode := screen.AccessibilityNodeFor(otherBlock)
		c.NotNil(docNode)
		c.NotNil(caretNode)
		c.NotNil(otherNode)
		if tree == nil || docNode == nil || caretNode == nil || otherNode == nil {
			return
		}
		c.True(docNode.Focused, "the document holds the keyboard focus whatever its blocks say")
		c.Equal(docNode.ID, tree.Focus, "and is what the tree reports the focus on")
		c.Equal(reports, caretNode.Focused,
			"the block holding the caret reports the focus only where a screen reader needs it (%v)", reports)
		c.False(otherNode.Focused, "no other block ever does")
	}

	// A claim made while the focus is somewhere else entirely is a mistake rather than a caret, so it is taken away even
	// where a document's own block would have kept it.
	screen.Do(func() {
		axCaretBlockReportsFocus = true
		field := NewField()
		wnd.Content().AddChild(field)
		wnd.Content().MarkForLayoutAndRedraw()
		wnd.SetFocus(field)
	})
	tree := screen.AccessibilityTree(wnd)
	c.NotNil(tree)
	caretNode := screen.AccessibilityNodeFor(caretBlock)
	c.NotNil(caretNode)
	if tree != nil && caretNode != nil {
		c.False(caretNode.Focused, "a block outside the panel that holds the focus does not get to claim it")
		c.NotEqual(caretNode.ID, tree.Focus)
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// axVirtualRefPanel stands in for a document that records which node occupies each part of its content: while it is
// being described it asks for the id of a virtual child of a panel that has not been described yet, which is what a
// document holding a table does for the table's rows.
type axVirtualRefPanel struct {
	target *axVirtualRowTable
	Panel
	recorded accessibility.NodeID
}

// newAXVirtualRefPanel creates a panel that will ask for the id of the given table's first row.
func newAXVirtualRefPanel(target *axVirtualRowTable) *axVirtualRefPanel {
	p := &axVirtualRefPanel{target: target}
	p.Self = p
	p.SetSizer(func(_ geom.Size) (minSize, prefSize, maxSize geom.Size) {
		size := geom.NewSize(100, 20)
		return size, size, size
	})
	return p
}

func (p *axVirtualRefPanel) ProvideAccessibility(b *AccessibilityBuilder) {
	node := b.Node()
	node.Role = role.Document
	node.Name = "Doc"
	p.recorded = b.virtualIDOf(p.target.AsPanel(), axVirtualRowKey{row: 0})
}

// TestAccessibilityVirtualIDOfSurvivesTheOwnerBeingDescribed covers pointing at a virtual child of a panel that has not
// been described yet, which is how a document says which node occupies a stretch of its stream when that node is a row
// a table has yet to invent. The id handed out then has to be the id the table itself hands out afterwards: a span
// naming anything else names a node that is not in the tree, so everything an assistive technology would do with it —
// reading the row the caret is in, moving to it, reporting where it is — finds nothing.
//
// The panel doing the asking is described first, so the table really has no identity of its own at that point. Since
// establishing one throws away whatever map of virtual keys the panel was carrying, asking has to establish it rather
// than write into a map about to be discarded.
func TestAccessibilityVirtualIDOfSurvivesTheOwnerBeingDescribed(t *testing.T) {
	c := check.New(t)
	var table *axVirtualRowTable
	var ref *axVirtualRefPanel
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 400, Height: 300},
		StartupFinishedCallback(func() {
			table = newAXVirtualRowTable("One", "Two")
			ref = newAXVirtualRefPanel(table)
			wnd = axNewTestWindow(t, "virtual ids", geom.NewRect(10, 10, 300, 200), ref)
		}))
	c.NotNil(wnd)
	if wnd == nil {
		return
	}
	c.NotNil(screen.AccessibilityTree(wnd), "a first tree, which the table is not part of")

	// The table is added and the window described within one visit to the UI thread, so nothing can describe the window
	// in between: this really is the first description the table is part of, and it has no identity of its own at the
	// moment the panel before it asks about one of its keys. Both are read here too, since a later description would
	// find the table already identified and so could not tell the two apart.
	var tree *accessibility.Tree
	var recorded accessibility.NodeID
	screen.Do(func() {
		wnd.Content().AddChild(table)
		wnd.Content().ValidateLayout()
		wnd.publishAccessibility()
		recorded = ref.recorded
		if hw := headlessWindowFor(wnd); hw != nil {
			tree = hw.axTree
		}
	})
	c.NotNil(tree)
	if tree == nil {
		return
	}
	tableNode := tree.Node(table.Accessibility.id)
	c.NotNil(tableNode)
	if tableNode == nil {
		return
	}
	c.NotEqual(accessibility.NodeID(0), recorded, "the panel was given an id to point at")
	c.Equal(1, len(tableNode.Children), "the table described its one row")
	c.Equal(tableNode.Children[0], recorded, "the row the table handed out is the node that was pointed at")
	c.NotNil(tree.Node(recorded), "and it is a node that is actually in the tree")

	// The same holds on the next description, where the table does have an identity to begin with, so nothing about the
	// first one's answer was accidental.
	tree = screen.AccessibilityTree(wnd)
	tableNode = screen.AccessibilityNodeFor(table)
	c.NotNil(tree)
	c.NotNil(tableNode)
	if tree == nil || tableNode == nil {
		return
	}
	c.Equal(recorded, ref.recorded, "a row keeps its identity from one description to the next")
	c.Equal(tableNode.Children[0], ref.recorded)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestAccessibilityCaretBlockLosesFocusToAnOpenMenu covers what becomes of a document block's claim on the focus while
// an open menu has taken the focus over. Exactly one node of a window may report the focus, and while a menu is open
// that node is what the person is choosing from: a caret block still claiming it would be the second focused object
// that axSnapshot.clearDisplacedFocus exists to prevent, which is what Orca would announce instead of the menu. Nothing
// is lost by dropping it, since the caret has not moved and the claim comes back with the focus.
func TestAccessibilityCaretBlockLosesFocusToAnOpenMenu(t *testing.T) {
	c := check.New(t)
	var doc *axCaretDoc
	var caretBlock *axCaretBlock
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 400, Height: 400},
		StartupFinishedCallback(func() {
			doc, caretBlock, _ = newAXCaretDoc()
			wnd = axNewTestWindow(t, "caret focus and menus", geom.NewRect(10, 10, 300, 300), doc)
			if wnd != nil {
				wnd.ToFront()
				wnd.SetFocus(doc)
			}
		}))
	c.NotNil(wnd)
	if wnd == nil {
		return
	}
	saved := axCaretBlockReportsFocus
	defer screen.Do(func() { axCaretBlockReportsFocus = saved })
	// The claim only exists on the platforms whose screen readers need it, so it is turned on here whichever platform
	// this runs on: what is being pinned is that an open menu takes it away even there.
	screen.Do(func() { axCaretBlockReportsFocus = true })

	tree := screen.AccessibilityTree(wnd)
	docNode := screen.AccessibilityNodeFor(doc)
	caretNode := screen.AccessibilityNodeFor(caretBlock)
	c.NotNil(tree)
	c.NotNil(docNode)
	c.NotNil(caretNode)
	if tree == nil || docNode == nil || caretNode == nil {
		return
	}
	c.True(docNode.Focused, "with no menu open the document holds the focus")
	c.True(caretNode.Focused, "and the block holding the caret reports it as well")

	var mp *menuPanel
	screen.Do(func() {
		mp = newTestMenuPanel(20, 20, 120, 80)
		mp.Accessibility.Role = role.Menu
		mp.Accessibility.Name = "Edit"
		newTestPopupWindow(wnd, mp)
		wnd.root.insertMenu(mp)
		wnd.MarkForRedraw()
	})
	tree = screen.AccessibilityTree(wnd)
	menuNode := screen.AccessibilityNodeFor(mp)
	c.NotNil(tree)
	c.NotNil(menuNode)
	if tree == nil || menuNode == nil {
		return
	}
	c.Equal(menuNode.ID, tree.Focus, "the open menu is where the person is, so that is where the focus is reported")
	c.False(tree.Node(docNode.ID).Focused, "the document the menu displaced no longer reports the focus")
	c.False(tree.Node(caretNode.ID).Focused, "nor does the block inside it")
	var focused []string
	tree.Walk(func(n *accessibility.Node) bool {
		if n.Focused && n.ID != tree.Root {
			focused = append(focused, n.Name)
		}
		return true
	})
	c.Equal(1, len(focused), "exactly one node in the window may report being focused: %v", focused)

	// Closing the menu hands the focus back, and the caret — which never moved — is reported again.
	screen.Do(func() {
		wnd.root.removeMenu(mp)
		wnd.MarkForRedraw()
	})
	tree = screen.AccessibilityTree(wnd)
	c.NotNil(tree)
	if tree == nil {
		return
	}
	c.Equal(docNode.ID, tree.Focus)
	c.True(tree.Node(docNode.ID).Focused)
	c.True(tree.Node(caretNode.ID).Focused, "the claim comes back with the focus")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// axVirtualFocusClaim is a widget whose one virtual child claims the keyboard focus, which is what a widget publishing
// a second focused node looks like: the child has no panel of its own, so it cannot be the thing the window's focus is
// on.
type axVirtualFocusClaim struct {
	Panel
}

// newAXVirtualFocusClaim creates a focusable widget describing one virtual child that says it holds the focus.
func newAXVirtualFocusClaim() *axVirtualFocusClaim {
	t := &axVirtualFocusClaim{}
	t.Self = t
	t.SetFocusable(true)
	t.SetSizer(func(_ geom.Size) (minSize, prefSize, maxSize geom.Size) {
		size := geom.NewSize(100, 40)
		return size, size, size
	})
	return t
}

func (t *axVirtualFocusClaim) ProvideAccessibility(b *AccessibilityBuilder) {
	node := b.Node()
	node.Role = role.Table
	node.Name = "Grid"
	node.RowCount = 1
	b.AddVirtualChild(axVirtualRowKey{row: 0}, func(n *accessibility.Node) {
		n.Role = role.Row
		n.RowIndex = 0
		n.Bounds = t.ContentRect(true)
		// Claimed whatever the platform does with it, so that what the builder makes of the claim is what this pins.
		n.Focused = true
	})
}

// TestAccessibilityVirtualChildFocusClaim covers a claim on the focus made by a node a widget invented. It is decided
// by the same rule a claim from a real panel is: it is a companion to the focus rather than the focus itself, so it
// stands only where a screen reader needs a second focused object and only while the panel that described it is the one
// the window reports the focus on. Without that a widget could publish a second focused node on every platform,
// whatever that platform's assistive technology makes of one.
func TestAccessibilityVirtualChildFocusClaim(t *testing.T) {
	c := check.New(t)
	var claimer *axVirtualFocusClaim
	var field *Field
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 400, Height: 300},
		StartupFinishedCallback(func() {
			claimer = newAXVirtualFocusClaim()
			field = NewField()
			wnd = axNewTestWindow(t, "virtual focus claims", geom.NewRect(10, 10, 300, 200), claimer)
			if wnd != nil {
				wnd.Content().AddChild(field)
				wnd.Content().MarkForLayoutAndRedraw()
				wnd.ToFront()
				wnd.SetFocus(claimer)
			}
		}))
	c.NotNil(wnd)
	if wnd == nil {
		return
	}
	saved := axCaretBlockReportsFocus
	defer screen.Do(func() { axCaretBlockReportsFocus = saved })

	for _, reports := range []bool{true, false} {
		screen.Do(func() { axCaretBlockReportsFocus = reports })
		tree := screen.AccessibilityTree(wnd)
		node := screen.AccessibilityNodeFor(claimer)
		c.NotNil(tree)
		c.NotNil(node)
		if tree == nil || node == nil {
			return
		}
		c.Equal(1, len(node.Children), "the widget described its one row")
		if len(node.Children) != 1 {
			return
		}
		row := tree.Node(node.Children[0])
		c.NotNil(row)
		if row == nil {
			return
		}
		c.True(node.Focused, "the widget holds the keyboard focus whatever its rows say")
		c.Equal(node.ID, tree.Focus, "and is what the tree reports the focus on")
		c.Equal(reports, row.Focused,
			"a virtual child inside the focus panel reports the focus only where one is needed (%v)", reports)
	}

	// A claim from a widget that does not hold the focus is a mistake rather than a caret, so it goes even where a
	// second focused object is what the platform wants.
	screen.Do(func() {
		axCaretBlockReportsFocus = true
		wnd.SetFocus(field)
	})
	tree := screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(claimer)
	c.NotNil(tree)
	c.NotNil(node)
	if tree == nil || node == nil || len(node.Children) != 1 {
		return
	}
	row := tree.Node(node.Children[0])
	c.NotNil(row)
	if row != nil {
		c.False(row.Focused, "a virtual child outside the panel that holds the focus does not get to claim it")
		c.NotEqual(row.ID, tree.Focus)
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// axVirtualRowKey identifies the one row axVirtualRowTable invents, standing in for the key a document's table uses.
type axVirtualRowKey struct {
	row int
}

// axVirtualRowTable is a widget shaped like a document's table: the cells are real panels it holds as children, and the
// row between them and itself exists only because it described one. See AccessibilityBuilder.describeChildrenUnder.
type axVirtualRowTable struct {
	cells []*Label
	Panel
}

// newAXVirtualRowTable creates a table of one row holding the given cell titles.
func newAXVirtualRowTable(titles ...string) *axVirtualRowTable {
	t := &axVirtualRowTable{}
	t.Self = t
	t.SetLayout(&FlexLayout{Columns: len(titles)})
	for _, title := range titles {
		cell := NewLabel()
		cell.SetTitle(title)
		t.cells = append(t.cells, cell)
		t.AddChild(cell)
	}
	return t
}

// ProvideAccessibility describes the table, the one virtual row it invents, and the real cell panels beneath that row.
func (t *axVirtualRowTable) ProvideAccessibility(b *AccessibilityBuilder) {
	node := b.Node()
	node.Role = role.Table
	node.Name = "Grid"
	node.RowCount = 1
	node.ColumnCount = len(t.cells)
	rowID := b.AddVirtualChild(axVirtualRowKey{row: 0}, func(n *accessibility.Node) {
		n.Role = role.Row
		n.RowIndex = 0
		n.Bounds = t.ContentRect(true)
	})
	children := make([]axChildAt, 0, len(t.cells)+3)
	for i, cell := range t.cells {
		children = append(children, axChildAt{panel: cell.AsPanel(), index: i})
	}
	// A nil in the list is skipped rather than describing anything, since a widget building this list from its own
	// content may well have a hole in it, and so is an entry whose index says the panel is not one the widget holds:
	// pushing a negative index onto a cell's path would make a key no request could resolve.
	children = append(children,
		axChildAt{index: 0},
		axChildAt{panel: t.cells[0].AsPanel(), index: -1},
		// The first cell again, which is already in this tree: describing it a second time would list its id beneath the
		// row twice and leave every position counted out of that list wrong.
		axChildAt{panel: t.cells[0].AsPanel(), index: 0},
	)
	b.describeChildrenUnder(rowID, children)
	// Nothing must describe the cells a second time as children of the table itself, which is what this says.
	b.snapshot.markChildrenDescribed(node.ID)
	// A parent that is not in this tree describes nothing at all.
	b.describeChildrenUnder(0, children)
}

// TestAccessibilityDescribeChildrenUnder covers the shape a document's table is described in: the widget invents a row
// that has no panel of its own, and the real panels holding the cells are described beneath that row rather than
// beneath the widget. Each cell keeps its own identity, so what an assistive technology is told about is the panel a
// request about it will reach.
func TestAccessibilityDescribeChildrenUnder(t *testing.T) {
	c := check.New(t)
	var table *axVirtualRowTable
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 400, Height: 300},
		StartupFinishedCallback(func() {
			table = newAXVirtualRowTable("One", "Two")
			wnd = axNewTestWindow(t, "virtual rows", geom.NewRect(10, 10, 300, 200), table)
		}))
	c.NotNil(wnd)
	tree := screen.AccessibilityTree(wnd)
	c.NotNil(tree)
	tableNode := screen.AccessibilityNodeFor(table)
	c.NotNil(tableNode)
	if tree == nil || tableNode == nil {
		return
	}
	c.Equal(1, len(tableNode.Children), "the row is the table's only child")
	row := tree.Node(tableNode.Children[0])
	c.NotNil(row)
	if row == nil {
		return
	}
	c.Equal(role.Row, row.Role)
	c.Equal(len(table.cells), len(row.Children),
		"every cell panel is described beneath the row, and none of them twice: %v", row.Children)
	for i, id := range row.Children {
		cellNode := tree.Node(id)
		c.NotNil(cellNode)
		if cellNode == nil {
			continue
		}
		c.Equal(row.ID, cellNode.Parent)
		c.Equal(role.Label, cellNode.Role)
		c.Equal(table.cells[i].String(), cellNode.Name)
		c.Equal(screen.AccessibilityNodeFor(table.cells[i]).ID, id, "a cell keeps its own identity")
	}
	// The repeated cell, the entry with no panel and the one with a negative index were all refused, so the cell that was
	// handed over twice is listed once.
	first := screen.AccessibilityNodeFor(table.cells[0])
	c.NotNil(first)
	if first != nil {
		listed := 0
		for _, id := range row.Children {
			if id == first.ID {
				listed++
			}
		}
		c.Equal(1, listed, "a cell already described in this tree is not described a second time")
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestActivateAccessibilitySeedsAFocusThatLeftTheWindow verifies that a screen reader arriving at a window whose
// focused panel has since been removed from it -- so that the window's focus names a panel no longer in it, exactly as
// if nothing held the focus -- has the focus seeded for it, as a window with nothing focused does, rather than being
// left with nowhere to start because the stale name was taken for a focus.
func TestActivateAccessibilitySeedsAFocusThatLeftTheWindow(t *testing.T) {
	c := check.New(t)
	var wnd *Window
	var first, second *Field
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			first = NewField()
			second = NewField()
			column := NewPanel()
			column.SetLayout(&FlexLayout{Columns: 1})
			column.AddChild(first)
			column.AddChild(second)
			wnd = axNewTestWindow(t, "seed on activation", geom.NewRect(10, 10, 500, 500), column)
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()
	focus := func() *Panel {
		var p *Panel
		screen.Do(func() { p = wnd.CurrentFocus() })
		return p
	}

	screen.Do(func() {
		wnd.SetFocus(second)
		second.RemoveFromParent()
	})
	c.Nil(focus(), "the panel that held the focus is no longer in the window")
	screen.EnableAccessibility()
	c.True(focus().Is(first), "so the screen reader's arrival seeds the focus, as it would with nothing focused")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// These tests cover the keyboard focus standalone static text takes for a screen reader's sake; see SetFocusForReading,
// AccessibilityInfo.FocusForReading and Panel.axStaticTextTakesFocus. They use plain labels rather than headings, since
// a headless session answers as Linux does and lets a heading take the focus through an arm of its own;
// TestStaticTextFocusByRole turns that arm off. A session owns most of the package's mutable globals while it runs, so
// none of these may call t.Parallel.

func staticTextLabel(title string) *Label {
	l := NewLabel()
	l.SetTitle(title)
	return l
}

// staticTextField returns a field whose caret never blinks, since a blink timer outlives the session that armed it and
// would enqueue a stray task into whichever test runs next.
func staticTextField() *Field {
	f := NewField()
	f.BlinkRate = time.Hour
	return f
}

// staticTextNamedField returns a field that names itself, so the label before it stays standalone text rather than
// becoming its caption.
func staticTextNamedField(name string) *Field {
	f := staticTextField()
	f.Accessibility.Name = name
	return f
}

func staticTextColumn(panels ...Paneler) *Panel {
	column := NewPanel()
	column.SetLayout(&FlexLayout{Columns: 1})
	for _, p := range panels {
		column.AddChild(p)
	}
	return column
}

func restoreStaticTextFocus(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { focusForReading.Store(false) })
}

func staticTextFocusable(screen *HeadlessScreen, p Paneler) bool {
	var focusable bool
	screen.Do(func() { focusable = p.AsPanel().Focusable() })
	return focusable
}

func staticTextSnapshots(screen *HeadlessScreen) uint64 {
	var count uint64
	screen.Do(func() { count = axSnapshotCount })
	return count
}

// staticTextCheckNode checks, without describing the window again, that the last published node for p has Focusable
// and the focus action both equal to want.
func staticTextCheckNode(c check.Checker, screen *HeadlessScreen, p Paneler, want bool, what string) {
	node := screen.AccessibilityNodeFor(p)
	c.NotNil(node, "%s should have been described", what)
	if node == nil {
		return
	}
	c.Equal(want, node.Focusable, "%s: Focusable", what)
	c.Equal(want, node.Actions.Has(accessibility.Focus), "%s: offers the focus action", what)
}

// TestStaticTextFocusIsOffByDefault verifies that a standalone label is neither a tab stop nor described as one while
// an assistive technology is being served but the switch is off.
func TestStaticTextFocusIsOffByDefault(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	var text *Label
	var field *Field
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			text = staticTextLabel("Changes are saved as you type.")
			field = staticTextNamedField("Search")
			wnd = axNewTestWindow(t, "off by default", geom.NewRect(10, 10, 500, 300), staticTextColumn(text, field))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()

	c.False(FocusForReading(), "the switch is off until the application turns it on")
	screen.EnableAccessibility()
	c.True(IsAccessibilityActive())
	c.False(staticTextFocusable(screen, text), "a standalone label is not a tab stop by default")
	c.NotNil(screen.AccessibilityTree(wnd))
	staticTextCheckNode(c, screen, text, false, "the standalone label")

	screen.Do(func() { wnd.SetFocus(field) })
	screen.KeyPress(KeyTab, mod.None)
	var focus *Panel
	screen.Do(func() { focus = wnd.CurrentFocus() })
	c.True(focus.Is(field), "Tab wraps around onto the field, the only tab stop there is")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestStaticTextFocusNeedsAnAssistiveTechnology verifies that the switch changes nothing while no assistive technology
// is being served: no label becomes a tab stop, the window still opens in its field, and nothing is described.
func TestStaticTextFocusNeedsAnAssistiveTechnology(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	var text *Label
	var field *Field
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			text = staticTextLabel("Changes are saved as you type.")
			field = staticTextNamedField("Search")
			wnd = axNewTestWindow(t, "no assistive technology", geom.NewRect(10, 10, 500, 300),
				staticTextColumn(text, field))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()

	SetFocusForReading(true)
	screen.Sync()
	c.True(FocusForReading(), "the switch reports what it was set to")
	c.False(IsAccessibilityActive(), "turning the switch on must not start accessibility support")
	c.False(staticTextFocusable(screen, text), "with nothing listening, a label is not a tab stop")

	var seeded, next *Panel
	screen.Do(func() { wnd.SetFocus(nil) })
	screen.KeyPress(KeyTab, mod.None)
	screen.Do(func() { seeded = wnd.CurrentFocus() })
	screen.KeyPress(KeyTab, mod.None)
	screen.Do(func() { next = wnd.CurrentFocus() })
	c.True(seeded.Is(field), "the window opens in its field")
	c.True(next.Is(field), "and Tab never stops at the label")
	c.Equal(uint64(0), staticTextSnapshots(screen), "nothing may be described while nothing is listening")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestStaticTextTakesFocusExceptCaptions verifies the switch while an assistive technology is being served: standalone
// labels become tab stops, are described as such and are given the focus when a screen reader asks, while a caption,
// whether placed before a field or pointed at through LabeledBy, is neither. The description published right after the
// switch is turned on is checked without describing the window again, since the caption before its field is described
// before the field makes it one; see axSnapshot.settleCaptionFocus.
func TestStaticTextTakesFocusExceptCaptions(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	var first, caption, trailingCaption, last *Label
	var named, pointedAt *Field
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			first = staticTextLabel("Both names are optional.")
			caption = staticTextLabel("Name:")
			named = staticTextField()
			pointedAt = staticTextField()
			// Named by a label that comes after it, so the builder records the caption before describing it.
			trailingCaption = staticTextLabel("Nickname")
			pointedAt.Accessibility.LabeledBy = trailingCaption
			last = staticTextLabel("Changes are saved when the window closes.")
			wnd = axNewTestWindow(t, "captions", geom.NewRect(10, 10, 500, 400),
				staticTextColumn(first, caption, named, pointedAt, trailingCaption, last))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()

	focus := func() *Panel {
		var p *Panel
		screen.Do(func() { p = wnd.CurrentFocus() })
		return p
	}

	// Described once before the switch, so the captions were recorded in an earlier description than the one the
	// switch causes.
	screen.EnableAccessibility()
	c.NotNil(screen.AccessibilityTree(wnd))
	before := staticTextSnapshots(screen)
	SetFocusForReading(true)
	screen.Sync()
	c.True(staticTextSnapshots(screen) > before, "turning the switch on describes the window again")

	staticTextCheckNode(c, screen, first, true, "the standalone label at the top")
	staticTextCheckNode(c, screen, last, true, "the standalone label at the bottom")
	staticTextCheckNode(c, screen, caption, false, "the caption described before its field")
	staticTextCheckNode(c, screen, trailingCaption, false, "the caption described after its field")
	c.True(staticTextFocusable(screen, first))
	c.True(staticTextFocusable(screen, last))
	c.False(staticTextFocusable(screen, caption), "a caption is spoken with its field and is not a tab stop")
	c.False(staticTextFocusable(screen, trailingCaption), "whichever way it names the field")

	screen.Do(func() { wnd.SetFocus(pointedAt) })
	screen.KeyPress(KeyTab, mod.None)
	c.True(focus().Is(last), "Tab passes over the caption after the field and lands on the standalone label")
	screen.KeyPress(KeyTab, mod.None)
	c.True(focus().Is(first), "and wraps around onto the label at the top of the window")
	screen.KeyPress(KeyTab, mod.None)
	c.True(focus().Is(named), "then passes over the caption that names the first field")
	screen.KeyPress(KeyTab, mod.Shift)
	c.True(focus().Is(first), "and Shift-Tab passes over it again on the way back")

	captionNode := screen.AccessibilityNodeFor(caption)
	firstNode := screen.AccessibilityNodeFor(first)
	c.NotNil(captionNode)
	c.NotNil(firstNode)
	if captionNode == nil || firstNode == nil {
		return
	}
	screen.Do(func() { wnd.SetFocus(named) })
	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   captionNode.ID,
		Action: accessibility.Focus,
	}), "a caption does not offer the focus, so a request for it is refused")
	c.True(focus().Is(named), "and the focus stays where it was")
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   firstNode.ID,
		Action: accessibility.Focus,
	}), "a screen reader asking for the focus on standalone text gets it")
	c.True(focus().Is(first))
	tree := screen.AccessibilityTree(wnd)
	c.NotNil(tree)
	if tree != nil {
		c.Equal(firstNode.ID, tree.Focus, "and the description reports the focus there")
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestStaticTextFocusExclusions verifies that, with the switch on, text outside the window's content stays out of the
// tab order: a label inside a document or a control, one hidden from assistive technologies, one holding only a
// drawable, one installed as a cell of a Table, List or TableHeader, and one in the tooltip. Labels inside a group and
// a scroll panel are the window's text, and are tab stops.
func TestStaticTextFocusExclusions(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	var inReader, inButton, hidden, icon, inGroup, inScroller *Label
	var md *Markdown
	var table *Table[*axHeaderRow]
	var header *TableHeader[*axHeaderRow]
	var list *List[string]
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 800, Height: 800},
		StartupFinishedCallback(func() {
			md = NewMarkdown(false)
			md.SetContent("# Title\n\nSome body text.\n", 400)
			// A stand-in for a document holding a plain label, which a real Markdown never does, so that the ancestor
			// rule is the only thing keeping the label out.
			reader := NewPanel()
			reader.SetLayout(&FlexLayout{Columns: 1})
			reader.axFocusable = true
			inReader = staticTextLabel("Inside a document")
			reader.AddChild(inReader)

			button := NewPanel()
			button.SetLayout(&FlexLayout{Columns: 1})
			button.Accessibility.Role = role.Button
			inButton = staticTextLabel("Go")
			button.AddChild(inButton)

			hidden = staticTextLabel("Hidden from screen readers")
			hidden.Accessibility.Role = role.None

			icon = NewLabel()
			icon.Drawable = &axLabelTestDrawable{size: geom.NewSize(16, 16)}
			icon.Tooltip = NewTooltipWithText("Printer")

			group := NewPanel()
			group.SetLayout(&FlexLayout{Columns: 1})
			group.Accessibility.Role = role.Group
			inGroup = staticTextLabel("In a group")
			group.AddChild(inGroup)

			scroller := NewScrollPanel()
			inScroller = staticTextLabel("In a scroll panel")
			scroller.SetContent(inScroller, behavior.Fill, behavior.Fill)

			model := &SimpleTableModel[*axHeaderRow]{}
			model.SetRootRows([]*axHeaderRow{{id: "a"}})
			table = NewTable[*axHeaderRow](model)
			table.Columns = []ColumnInfo{{ID: 0, Current: 100}}
			header = NewTableHeader(table, NewTableColumnHeader[*axHeaderRow]("Column", "", nil))
			table.SyncToModel()
			list = NewList[string]()
			list.Append("row")

			wnd = axNewTestWindow(t, "exclusions", geom.NewRect(10, 10, 700, 700),
				staticTextColumn(md, reader, button, hidden, icon, group, scroller, header, table, list))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()
	screen.EnableAccessibility()
	SetFocusForReading(true)
	screen.Sync()

	c.False(staticTextFocusable(screen, inReader), "a document reads its own content")
	c.False(staticTextFocusable(screen, inButton), "text inside a control is that control's")
	c.False(staticTextFocusable(screen, hidden), "text hidden from assistive technologies is not somewhere to land")
	c.False(staticTextFocusable(screen, icon), "a label holding only a drawable is an image rather than text")
	c.True(staticTextFocusable(screen, inGroup), "a grouping panel leaves its text as the window's")
	c.True(staticTextFocusable(screen, inScroller), "and so does a scroll panel")

	var inDocument []*Panel
	var inTable, inHeader, inList, inTooltip, documentFocusable bool
	screen.Do(func() {
		documentFocusable = md.AsPanel().Focusable()
		var walk func(p *Panel)
		walk = func(p *Panel) {
			for _, child := range p.Children() {
				if child.Focusable() {
					inDocument = append(inDocument, child)
				}
				walk(child)
			}
		}
		walk(md.AsPanel())

		// A cell is attached only while drawn or described, so each is asked while installed.
		rect := geom.NewRect(0, 0, 100, 20)
		cellFocusable := func(install, uninstall func(p *Panel)) bool {
			cell := staticTextLabel("In a cell").AsPanel()
			install(cell)
			defer uninstall(cell)
			return cell.Focusable()
		}
		inTable = cellFocusable(func(p *Panel) { table.installCell(p, rect) }, func(p *Panel) { p.parent = nil })
		inHeader = cellFocusable(func(p *Panel) { header.installCell(p, rect) }, header.uninstallCell)
		inList = cellFocusable(func(p *Panel) { list.installCell(p, rect) }, list.uninstallCell)

		tip := staticTextColumn()
		tipText := staticTextLabel("Tip text")
		tip.AddChild(tipText)
		wnd.root.setTooltip(tip)
		inTooltip = tipText.AsPanel().Focusable()
		wnd.root.setTooltip(nil)
	})
	c.True(documentFocusable, "the document itself takes the focus for a screen reader's sake")
	c.Equal(0, len(inDocument), "but nothing inside it does: %v", inDocument)
	c.False(inTable, "a table cell is reached through the table's cell cursor")
	c.False(inHeader, "a column header is part of the table's description")
	c.False(inList, "a list row is reached through the list's current row")
	c.False(inTooltip, "a tooltip comes and goes and is never in the tab order")

	c.NotNil(screen.AccessibilityTree(wnd))
	staticTextCheckNode(c, screen, inButton, false, "the label inside the button")
	staticTextCheckNode(c, screen, icon, false, "the drawable-only label")
	staticTextCheckNode(c, screen, inGroup, true, "the label inside the group")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestStaticTextSeedsTheFocusLast verifies that static text is the last choice when a window with nothing focused
// chooses where to put the person: a window holding a field opens in the field from either end, while a window of
// nothing but text opens in its first label.
func TestStaticTextSeedsTheFocusLast(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	var lead, trailing, firstText, lastText *Label
	var field *Field
	var wnd, textOnly *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			lead = staticTextLabel("Search the catalog.")
			field = staticTextNamedField("Search")
			trailing = staticTextLabel("Results appear below.")
			wnd = axNewTestWindow(t, "text and a field", geom.NewRect(10, 10, 500, 300),
				staticTextColumn(lead, field, trailing))

			firstText = staticTextLabel("The operation finished.")
			lastText = staticTextLabel("Nothing was changed.")
			textOnly = axNewTestWindow(t, "text only", geom.NewRect(60, 60, 300, 200),
				staticTextColumn(firstText, lastText))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	c.NotNil(textOnly)
	screen.Sync()
	screen.EnableAccessibility()
	SetFocusForReading(true)
	screen.Sync()
	c.True(staticTextFocusable(screen, lead), "the label before the field is standalone text, and a tab stop")
	c.True(staticTextFocusable(screen, trailing))

	var seeded, seededBackward, seededText, seededTextBackward *Panel
	screen.Do(func() { wnd.SetFocus(nil) })
	screen.KeyPress(KeyTab, mod.None)
	screen.Do(func() {
		seeded = wnd.CurrentFocus()
		wnd.SetFocus(nil)
	})
	screen.KeyPress(KeyTab, mod.Shift)
	screen.Do(func() {
		seededBackward = wnd.CurrentFocus()
		textOnly.SetFocus(nil)
		textOnly.FocusNext()
		seededText = textOnly.CurrentFocus()
		textOnly.SetFocus(nil)
		textOnly.FocusPrevious()
		seededTextBackward = textOnly.CurrentFocus()
	})
	c.True(seeded.Is(field), "the window opens in its field rather than on the label above it")
	c.True(seededBackward.Is(field), "and Shift-Tab passes over the label below it just the same")
	c.True(seededText.Is(firstText), "a window of nothing but text opens in its first label")
	c.True(seededTextBackward.Is(lastText), "or its last, moving the other way")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestStaticTextFocusRuntimeToggle verifies that the switch may be flipped while the application runs, from a goroutine
// other than the UI thread: turning it off leaves the focus on the label that holds it, stops the label being a tab
// stop and describes the window again, and turning it back on restores both. The label sits between two fields so that
// the next Tab landing on the window's first tab stop, and Shift-Tab on its last, is told apart from the stops beside
// it.
func TestStaticTextFocusRuntimeToggle(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	var text *Label
	var before, after *Field
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			before = staticTextNamedField("Search")
			text = staticTextLabel("Changes are saved as you type.")
			after = staticTextNamedField("Replace")
			wnd = axNewTestWindow(t, "toggle", geom.NewRect(10, 10, 500, 300), staticTextColumn(before, text, after))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()
	focus := func() *Panel {
		var p *Panel
		screen.Do(func() { p = wnd.CurrentFocus() })
		return p
	}

	screen.EnableAccessibility()
	SetFocusForReading(true)
	screen.Sync()
	screen.Do(func() { wnd.SetFocus(text) })
	c.True(focus().Is(text), "the label takes the focus while the switch is on")
	c.NotNil(screen.AccessibilityTree(wnd))
	staticTextCheckNode(c, screen, text, true, "the label with the switch on")

	// Called off the UI thread, as a worker goroutine would.
	snapshots := staticTextSnapshots(screen)
	SetFocusForReading(false)
	screen.Sync()
	c.False(FocusForReading())
	c.True(focus().Is(text), "turning the switch off leaves the focus where it is")
	c.False(staticTextFocusable(screen, text), "but the label is no longer a tab stop")
	c.True(staticTextSnapshots(screen) > snapshots, "the window is described again to say so")
	staticTextCheckNode(c, screen, text, false, "the label with the switch off")
	// The description keeps reporting the focus on the label, which still holds it, rather than falling back to an
	// ancestor as it does for a disabled focus panel; see axSnapshot.settleCaptionFocus.
	node := screen.AccessibilityNodeFor(text)
	tree := screen.AccessibilityTree(wnd)
	c.NotNil(node)
	c.NotNil(tree)
	if node != nil && tree != nil {
		c.True(node.Focused, "the label still says it holds the focus")
		c.Equal(node.ID, tree.Focus, "and the window reports the focus there")
	}
	screen.KeyPress(KeyTab, mod.None)
	c.True(focus().Is(before), "so Tab lands on the window's first tab stop, not the field after the label")

	snapshots = staticTextSnapshots(screen)
	SetFocusForReading(true)
	screen.Sync()
	c.True(staticTextFocusable(screen, text), "turning it back on makes the label a tab stop again")
	c.True(staticTextSnapshots(screen) > snapshots, "and describes the window again")
	staticTextCheckNode(c, screen, text, true, "the label with the switch back on")

	screen.Do(func() { wnd.SetFocus(text) })
	SetFocusForReading(false)
	screen.Sync()
	c.True(focus().Is(text))
	screen.KeyPress(KeyTab, mod.Shift)
	c.True(focus().Is(after), "and Shift-Tab lands on the window's last tab stop, not the field before the label")
	SetFocusForReading(true)
	screen.Sync()

	snapshots = staticTextSnapshots(screen)
	SetFocusForReading(true)
	screen.Sync()
	c.Equal(snapshots, staticTextSnapshots(screen), "setting the value it already has describes nothing")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestStaticTextFocusByRole verifies the arm of Panel.axIsReadableText the Windows and macOS builds rely on, where a
// heading never takes the focus for its own sake: a panel marked up as role.Label takes the focus when it has a name
// and not otherwise, and a heading takes it as static text once the heading arm, which a headless session answers as
// Linux does, is turned off.
func TestStaticTextFocusByRole(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	var named, nameless *Panel
	var heading *Label
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			named = NewPanel()
			named.Accessibility.Role = role.Label
			named.Accessibility.Name = "Built from a plain panel"
			nameless = NewPanel()
			nameless.Accessibility.Role = role.Label
			heading = staticTextLabel("Section")
			heading.Accessibility.Role = role.Heading
			heading.Accessibility.Level = 1
			wnd = axNewTestWindow(t, "by role", geom.NewRect(10, 10, 500, 300),
				staticTextColumn(named, nameless, heading, staticTextNamedField("Search")))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()

	var saved bool
	screen.Do(func() {
		saved = axHeadingsTakeFocus
		axHeadingsTakeFocus = false
	})
	defer screen.Do(func() { axHeadingsTakeFocus = saved })

	screen.EnableAccessibility()
	c.NotNil(screen.AccessibilityTree(wnd))
	c.False(staticTextFocusable(screen, heading), "where headings do not take the focus, one is not a tab stop by default")
	c.False(staticTextFocusable(screen, named), "nor is a panel marked up as a label")

	SetFocusForReading(true)
	screen.Sync()
	c.True(staticTextFocusable(screen, named), "a panel marked up as a label takes the focus on the strength of its name")
	c.False(staticTextFocusable(screen, nameless), "one with nothing to announce is no place to land")
	c.True(staticTextFocusable(screen, heading), "a heading takes the focus as static text where the heading arm is off")
	staticTextCheckNode(c, screen, named, true, "the named label panel")
	staticTextCheckNode(c, screen, heading, true, "the heading")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestFocusForReadingOptsInOnePanel verifies the per-panel opt-in: with the switch off, a label that sets
// FocusForReading takes the focus while an assistive technology is being served, the label beside it does not, and a
// caption that sets it is still not a tab stop.
func TestFocusForReadingOptsInOnePanel(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	var reading, plain, caption *Label
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			reading = staticTextLabel("Read this before continuing.")
			reading.Accessibility.FocusForReading = true
			plain = staticTextLabel("Some other text.")
			caption = staticTextLabel("Name:")
			caption.Accessibility.FocusForReading = true
			wnd = axNewTestWindow(t, "opt in", geom.NewRect(10, 10, 500, 300),
				staticTextColumn(reading, plain, caption, staticTextField()))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()

	c.False(staticTextFocusable(screen, reading), "with nothing listening, even an opted-in label is not a tab stop")
	screen.EnableAccessibility()
	c.NotNil(screen.AccessibilityTree(wnd))
	c.False(FocusForReading(), "the switch itself is still off")
	c.True(staticTextFocusable(screen, reading), "the label that asked takes the focus")
	c.False(staticTextFocusable(screen, plain), "the one beside it, which did not ask, does not")
	c.False(staticTextFocusable(screen, caption), "and a caption that asked is still spoken with its field instead")
	staticTextCheckNode(c, screen, reading, true, "the opted-in label")
	staticTextCheckNode(c, screen, plain, false, "the plain label")
	staticTextCheckNode(c, screen, caption, false, "the opted-in caption")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestStaticTextFocusDoesNotOutliveTheSession verifies that a headless session puts the switch back when it ends.
func TestStaticTextFocusDoesNotOutliveTheSession(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	screen := startHeadlessTest(t, HeadlessConfig{Width: 200, Height: 200})
	SetFocusForReading(true)
	c.True(FocusForReading())
	c.True(screen.Quit(), "the session should end")
	next := startHeadlessTest(t, HeadlessConfig{Width: 200, Height: 200})
	c.True(next.Running())
	c.False(FocusForReading(), "the next session starts with the switch off")
}

// These tests cover the keyboard focus a disabled control takes for a screen reader's sake while staying disabled in
// every other respect; see SetFocusForReading and Panel.axDisabledTakesFocus. They share the helpers of the static
// text focus tests above. A session owns most of the package's mutable globals while it runs, so none of these may
// call t.Parallel.

// disabledField returns a disabled field that names itself, so that no label is needed to caption it.
func disabledField(name, text string) *Field {
	f := staticTextNamedField(name)
	f.SetText(text)
	f.SetEnabled(false)
	return f
}

func disabledButton(title string, clicked *int) *Button {
	b := NewButton()
	b.SetTitle(title)
	b.ClickCallback = func() { *clicked++ }
	b.SetEnabled(false)
	return b
}

func disabledCheckBox(title string) *CheckBox {
	cb := NewCheckBox()
	cb.SetTitle(title)
	cb.SetEnabled(false)
	return cb
}

func disabledFieldText(screen *HeadlessScreen, f *Field) string {
	var text string
	screen.Do(func() { text = f.Text() })
	return text
}

func disabledFocus(screen *HeadlessScreen, wnd *Window) *Panel {
	var p *Panel
	screen.Do(func() { p = wnd.CurrentFocus() })
	return p
}

// disabledCheckStaysDisabled checks that a panel reports that it is disabled.
func disabledCheckStaysDisabled(c check.Checker, screen *HeadlessScreen, p Paneler, what string) {
	var enabled bool
	screen.Do(func() { enabled = p.AsPanel().Enabled() })
	c.False(enabled, "%s: Enabled", what)
}

// disabledNode returns the last published node for p, without describing the window again, failing the test if there
// is none.
func disabledNode(c check.Checker, screen *HeadlessScreen, p Paneler, what string) *accessibility.Node {
	node := screen.AccessibilityNodeFor(p)
	c.NotNil(node, "%s should have been described", what)
	return node
}

// disabledStdMenus gives a window the standard menus, whose commands and key equivalents are routed to whatever holds
// the keyboard focus ahead of the keys themselves; see RouteActionToFocusExecuteFunc.
func disabledStdMenus(wnd *Window) {
	if wnd != nil {
		DefaultMenuFactory().BarForWindow(wnd, func(m Menu) { InsertStdMenus(m, nil, nil, nil) })
	}
}

// disabledCheckNode checks, without describing the window again, that the last published node for p is disabled and
// offers only what a disabled node may, and that it is focusable and offers the focus exactly when reachable is true.
func disabledCheckNode(c check.Checker, screen *HeadlessScreen, p Paneler, reachable bool, what string) {
	node := disabledNode(c, screen, p, what)
	if node == nil {
		return
	}
	c.True(node.Disabled, "%s: Disabled", what)
	c.Equal(reachable, node.Focusable, "%s: Focusable", what)
	want := axDisabledActions
	if reachable {
		want = want.With(accessibility.Focus)
	}
	c.Equal(want, node.Actions, "%s: actions offered", what)
}

// TestDisabledControlsAreOutOfReachByDefault verifies that a disabled control takes no focus while no assistive
// technology is being served, whatever the switch says, and while one is being served but the switch is off.
func TestDisabledControlsAreOutOfReachByDefault(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	clicked := 0
	var editable, field *Field
	var button *Button
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			editable = staticTextNamedField("Search")
			field = disabledField("Total", "42")
			button = disabledButton("Apply", &clicked)
			wnd = axNewTestWindow(t, "out of reach", geom.NewRect(10, 10, 500, 300),
				staticTextColumn(editable, field, button))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()

	c.False(staticTextFocusable(screen, field), "a disabled field is not a tab stop")
	c.False(staticTextFocusable(screen, button), "nor is a disabled button")

	SetFocusForReading(true)
	screen.Sync()
	c.False(IsAccessibilityActive(), "turning the switch on must not start accessibility support")
	c.False(staticTextFocusable(screen, field), "the switch alone changes nothing while nothing is listening")
	c.False(staticTextFocusable(screen, button))
	screen.Do(func() { wnd.SetFocus(editable) })
	screen.KeyPress(KeyTab, mod.None)
	c.True(disabledFocus(screen, wnd).Is(editable), "and Tab never stops at either")
	c.Equal(uint64(0), staticTextSnapshots(screen), "nothing may be described while nothing is listening")
	SetFocusForReading(false)
	screen.Sync()

	screen.EnableAccessibility()
	c.NotNil(screen.AccessibilityTree(wnd))
	c.False(staticTextFocusable(screen, field), "with the switch off, being listened to changes nothing either")
	c.False(staticTextFocusable(screen, button))
	disabledCheckNode(c, screen, field, false, "the disabled field with the switch off")
	disabledCheckNode(c, screen, button, false, "the disabled button with the switch off")
	if node := disabledNode(c, screen, field, "the disabled field"); node != nil {
		c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
			Node:   node.ID,
			Action: accessibility.Focus,
		}), "a request for the focus is refused")
	}
	c.True(disabledFocus(screen, wnd).Is(editable))
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestDisabledControlsTakeTheFocusForReading verifies the switch while an assistive technology is being served: each
// kind of disabled control becomes a tab stop in the order it sits in the window, is published as disabled but able to
// take the focus, reports the focus when it holds it, and is given the focus when a screen reader asks. None of them
// stops reporting that it is disabled.
func TestDisabledControlsTakeTheFocusForReading(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	clicked := 0
	var editable, field, combo *Field
	var numeric *NumericField[int]
	var button *Button
	var checkBox *CheckBox
	var radio *RadioButton
	var popup *PopupMenu[string]
	var list *List[string]
	var wnd *Window
	first := "First"
	second := "Second"
	screen := startHeadlessTest(t, HeadlessConfig{Width: 800, Height: 800},
		StartupFinishedCallback(func() {
			editable = staticTextNamedField("Search")
			field = disabledField("Total", "42")
			numeric = NewNumericField(5, 0, 10, strconv.Itoa, strconv.Atoi, nil)
			numeric.BlinkRate = editable.BlinkRate
			numeric.Accessibility.Name = "Count"
			numeric.SetEnabled(false)
			combo = NewComboField([]*string{&first, &second}, &first, nil)
			combo.BlinkRate = editable.BlinkRate
			combo.Accessibility.Name = "Choice"
			combo.SetEnabled(false)
			button = disabledButton("Apply", &clicked)
			checkBox = disabledCheckBox("Remember")
			radio = NewRadioButton()
			radio.SetTitle("Always")
			radio.SetEnabled(false)
			popup = NewPopupMenu[string]()
			popup.AddItem("One", "Two")
			popup.SelectIndex(0)
			popup.Accessibility.Name = "Number"
			popup.SetEnabled(false)
			list = NewList[string]()
			list.Append("Row one", "Row two")
			list.Accessibility.Name = "Entries"
			list.SetEnabled(false)
			wnd = axNewTestWindow(t, "for reading", geom.NewRect(10, 10, 700, 700),
				staticTextColumn(editable, field, numeric, combo, button, checkBox, radio, popup, list))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()

	controls := []struct {
		panel Paneler
		what  string
		role  role.Enum
	}{
		{panel: field, what: "the disabled field", role: role.TextField},
		{panel: numeric, what: "the disabled numeric field", role: role.SpinButton},
		{panel: combo, what: "the disabled combo field", role: role.ComboBox},
		{panel: button, what: "the disabled button", role: role.Button},
		{panel: checkBox, what: "the disabled check box", role: role.CheckBox},
		{panel: radio, what: "the disabled radio button", role: role.RadioButton},
		{panel: popup, what: "the disabled popup menu", role: role.PopupButton},
		{panel: list, what: "the disabled list", role: role.List},
	}

	// Described once before the switch, so that turning it on is what describes the window again.
	screen.EnableAccessibility()
	c.NotNil(screen.AccessibilityTree(wnd))
	before := staticTextSnapshots(screen)
	SetFocusForReading(true)
	screen.Sync()
	c.True(staticTextSnapshots(screen) > before, "turning the switch on describes the window again")

	for _, one := range controls {
		c.True(staticTextFocusable(screen, one.panel), "%s can take the focus", one.what)
		disabledCheckStaysDisabled(c, screen, one.panel, one.what)
		disabledCheckNode(c, screen, one.panel, true, one.what)
		if node := disabledNode(c, screen, one.panel, one.what); node != nil {
			c.Equal(one.role, node.Role, "%s: Role", one.what)
			c.False(node.Focused, "%s does not hold the focus yet", one.what)
		}
	}
	if node := disabledNode(c, screen, field, "the disabled field"); node != nil {
		c.Equal("42", node.Value, "what the field holds is there to be read")
	}

	screen.Do(func() { wnd.SetFocus(editable) })
	for _, one := range controls {
		screen.KeyPress(KeyTab, mod.None)
		c.True(disabledFocus(screen, wnd).Is(one.panel), "Tab stops at %s", one.what)
		tree := screen.AccessibilityTree(wnd)
		node := screen.AccessibilityNodeFor(one.panel)
		c.NotNil(tree)
		c.NotNil(node)
		if tree != nil && node != nil {
			c.True(node.Focused, "%s says it holds the focus", one.what)
			c.Equal(node.ID, tree.Focus, "and the window reports the focus on %s", one.what)
			c.True(node.Disabled, "%s is still published as disabled", one.what)
		}
	}
	screen.KeyPress(KeyTab, mod.None)
	c.True(disabledFocus(screen, wnd).Is(editable), "and Tab wraps around onto the field that can be typed into")
	screen.KeyPress(KeyTab, mod.Shift)
	c.True(disabledFocus(screen, wnd).Is(list), "Shift-Tab goes back the other way")

	screen.Do(func() { wnd.SetFocus(editable) })
	c.NotNil(screen.AccessibilityTree(wnd))
	if node := disabledNode(c, screen, button, "the disabled button"); node != nil {
		c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
			Node:   node.ID,
			Action: accessibility.Focus,
		}), "a screen reader asking for the focus on a disabled control gets it")
		c.True(disabledFocus(screen, wnd).Is(button))
		c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
			Node:   node.ID,
			Action: accessibility.ScrollIntoView,
		}), "and may scroll it into view, as it always could")
	}
	c.Equal(0, clicked)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestDisabledControlsStayDisabled verifies that holding the focus changes nothing else about a disabled control:
// nothing typed, pressed or clicked reaches it, the keys pressed while it holds the focus go to the panel it is inside,
// the commands of the menus and their key equivalents pass over it, a click does not move the focus onto it, and an
// assistive technology is refused everything that would act on it.
func TestDisabledControlsStayDisabled(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	const original = "forty-two"
	clicked := 0
	modified := 0
	var keys []KeyCode
	var runes []rune
	var editable, field *Field
	var numeric *NumericField[int]
	var button *Button
	var checkBox *CheckBox
	var list *List[string]
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 800, Height: 800},
		StartupFinishedCallback(func() {
			editable = staticTextNamedField("Search")
			field = disabledField("Total", original)
			field.ModifiedCallback = func(_, _ *FieldState) { modified++ }
			numeric = NewNumericField(5, 0, 10, strconv.Itoa, strconv.Atoi, nil)
			numeric.BlinkRate = editable.BlinkRate
			numeric.Accessibility.Name = "Count"
			numeric.SetEnabled(false)
			button = disabledButton("Apply", &clicked)
			checkBox = disabledCheckBox("Remember")
			list = NewList[string]()
			list.Append("Row one", "Row two")
			list.Accessibility.Name = "Entries"
			list.SetEnabled(false)
			column := staticTextColumn(editable, field, numeric, button, checkBox, list)
			column.KeyDownCallback = func(keyCode KeyCode, _ mod.Modifiers, _ bool) bool {
				if keyCode == KeyTab {
					return false
				}
				keys = append(keys, keyCode)
				return true
			}
			column.RuneTypedCallback = func(ch rune) bool {
				runes = append(runes, ch)
				return true
			}
			wnd = axNewTestWindow(t, "stays disabled", geom.NewRect(10, 10, 700, 700), column)
			disabledStdMenus(wnd)
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()
	screen.EnableAccessibility()
	SetFocusForReading(true)
	screen.Sync()

	screen.Do(func() { wnd.SetFocus(field) })
	c.True(disabledFocus(screen, wnd).Is(field))
	var start, end int
	screen.Do(func() { start, end = field.Selection() })
	c.Equal(len(original), start, "taking the focus selects nothing in a disabled field")
	c.Equal(len(original), end)

	screen.Type("x")
	screen.KeyPress(KeyBackspace, mod.None)
	screen.KeyPress(KeyLeft, mod.None)
	screen.Do(func() { ClipboardSetText("pasted") })
	screen.KeyPress(KeyV, mod.OSMenuCommand())
	c.Equal(original, disabledFieldText(screen, field), "nothing typed or pressed changes the field")
	screen.Do(func() { start, end = field.Selection() })
	c.Equal(len(original), start, "or moves its caret")
	c.Equal(len(original), end)
	c.Equal([]rune{'x'}, runes, "what is typed goes to the panel the field is inside, as for any disabled focus")
	c.Equal([]KeyCode{KeyX, KeyBackspace, KeyLeft, KeyV}, keys, "and so do the keys")
	c.Equal(0, modified)

	// The commands of the menus and their key equivalents pass over the disabled field, which has handlers installed
	// for every one of them.
	var can []bool
	var clipboard string
	screen.Do(func() {
		for _, action := range []*Action{CutAction(), CopyAction(), PasteAction(), DeleteAction(), SelectAllAction()} {
			can = append(can, action.Enabled(nil))
		}
	})
	c.Equal([]bool{false, false, false, false, false}, can, "no command is offered for a disabled field")
	screen.KeyPress(KeyA, mod.OSMenuCommand())
	screen.Do(func() { start, end = field.Selection() })
	c.Equal(len(original), start, "select all selects nothing in it")
	c.Equal(len(original), end)
	screen.Do(func() {
		field.SetSelection(0, len(original))
		modified = 0
	})
	screen.KeyPress(KeyX, mod.OSMenuCommand())
	screen.KeyPress(KeyV, mod.OSMenuCommand())
	screen.Do(func() {
		DeleteAction().Execute(nil)
		PasteAction().Execute(nil)
		CutAction().Execute(nil)
		field.PerformCmd(nil, PasteItemID)
		clipboard = ClipboardGetText()
		field.SetSelectionToEnd()
	})
	c.Equal(original, disabledFieldText(screen, field), "nothing is cut from, pasted into or deleted from the field")
	c.Equal("pasted", clipboard, "and nothing of it was put on the clipboard")
	c.Equal(0, modified)

	screen.Do(func() {
		list.Select(false, 0)
		wnd.SetFocus(list)
	})
	c.True(disabledFocus(screen, wnd).Is(list))
	screen.KeyPress(KeyA, mod.OSMenuCommand())
	var selected int
	screen.Do(func() {
		SelectAllAction().Execute(nil)
		selected = list.Selection.Count()
	})
	c.Equal(1, selected, "select all does not select the rows of a disabled list")

	screen.Do(func() { wnd.SetFocus(button) })
	screen.KeyPress(KeySpace, mod.None)
	c.Equal(0, clicked, "the key that clicks a button does nothing for a disabled one that holds the focus")
	screen.Do(func() { wnd.SetFocus(editable) })
	screen.Click(screen.PanelCenter(button))
	c.Equal(0, clicked, "nor does a click")
	c.True(disabledFocus(screen, wnd).Is(editable), "which does not move the focus there either")
	screen.Click(screen.PanelCenter(checkBox))
	var state checkenum.Enum
	screen.Do(func() { state = checkBox.State })
	c.Equal(checkenum.Off, state, "a disabled check box is not checked by a click")
	screen.Click(screen.PanelCenter(field))
	c.True(disabledFocus(screen, wnd).Is(editable), "and a click on a disabled field leaves the focus where it was")

	tree := screen.AccessibilityTree(wnd)
	c.NotNil(tree)
	refused := func(p Paneler, what string, requests ...accessibility.ActionRequest) {
		node := screen.AccessibilityNodeFor(p)
		c.NotNil(node, "%s should have been described", what)
		if node == nil {
			return
		}
		for _, req := range requests {
			req.Node = node.ID
			c.False(screen.PerformAccessibilityAction(req), "%s refuses %s", what, req.Action)
		}
	}
	refused(field, "the disabled field",
		accessibility.ActionRequest{Action: accessibility.SetValue, Value: "replaced"},
		accessibility.ActionRequest{Action: accessibility.ReplaceText, Start: 0, End: 2, Value: "replaced"},
		accessibility.ActionRequest{Action: accessibility.SetTextSelection, Start: 1, End: 4},
		accessibility.ActionRequest{Action: accessibility.ShowContextMenu})
	refused(numeric, "the disabled numeric field",
		accessibility.ActionRequest{Action: accessibility.Increment},
		accessibility.ActionRequest{Action: accessibility.Decrement},
		accessibility.ActionRequest{Action: accessibility.SetValue, Value: "7", Number: 7})
	refused(button, "the disabled button", accessibility.ActionRequest{Action: accessibility.Press})
	refused(checkBox, "the disabled check box",
		accessibility.ActionRequest{Action: accessibility.Press},
		accessibility.ActionRequest{Action: accessibility.Toggle})
	c.Equal(original, disabledFieldText(screen, field))
	c.Equal("5", disabledFieldText(screen, numeric.Field))
	c.Equal(0, clicked)
	screen.Do(func() { state = checkBox.State })
	c.Equal(checkenum.Off, state)

	// The rows of a disabled list are no more reachable than they were: the list itself is what takes the focus.
	if tree != nil {
		rows := 0
		tree.Walk(func(n *accessibility.Node) bool {
			if n.Role == role.ListItem {
				rows++
				c.True(n.Disabled, "a row of a disabled list is disabled")
				c.False(n.Actions.Has(accessibility.Focus), "and does not offer the focus")
				c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
					Node:   n.ID,
					Action: accessibility.Focus,
				}), "which it refuses")
			}
			return true
		})
		c.Equal(2, rows, "both rows of the list should have been described")
	}
	c.True(disabledFocus(screen, wnd).Is(editable))

	// Out of range, which a field being edited would bring back into range as it lost the focus.
	screen.Do(func() {
		numeric.SetText("12")
		wnd.SetFocus(numeric)
	})
	screen.KeyPress(KeyTab, mod.None)
	c.True(disabledFocus(screen, wnd).Is(button))
	c.Equal("12", disabledFieldText(screen, numeric.Field), "losing the focus leaves what the application put there")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestDisabledControlsSeedTheFocusLast verifies that a disabled control is the last choice when a window with nothing
// focused chooses where to put the person, as static text is: a window that also holds a control that can be used
// opens there from either end, a container is resolved the same way, and a window holding nothing else opens in its
// first disabled control.
func TestDisabledControlsSeedTheFocusLast(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	clicked := 0
	var lead, editable, only *Field
	var trailing, last *Button
	var column *Panel
	var wnd, disabledWindow *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			lead = disabledField("Subtotal", "40")
			editable = staticTextNamedField("Discount")
			trailing = disabledButton("Apply", &clicked)
			column = staticTextColumn(lead, editable, trailing)
			wnd = axNewTestWindow(t, "disabled and enabled", geom.NewRect(10, 10, 500, 300), column)

			only = disabledField("Total", "42")
			last = disabledButton("Recalculate", &clicked)
			disabledWindow = axNewTestWindow(t, "disabled alone", geom.NewRect(60, 60, 300, 200),
				staticTextColumn(only, last))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	c.NotNil(disabledWindow)
	screen.Sync()
	screen.EnableAccessibility()
	SetFocusForReading(true)
	screen.Sync()
	c.True(staticTextFocusable(screen, lead))
	c.True(staticTextFocusable(screen, trailing))

	var seeded, seededBackward, firstChild, lastChild, seededAlone, seededAloneBackward *Panel
	screen.Do(func() {
		wnd.SetFocus(nil)
		wnd.FocusNext()
		seeded = wnd.CurrentFocus()
		wnd.SetFocus(nil)
		wnd.FocusPrevious()
		seededBackward = wnd.CurrentFocus()
		firstChild = column.FirstFocusableChild()
		lastChild = column.LastFocusableChild()
		disabledWindow.SetFocus(nil)
		disabledWindow.FocusNext()
		seededAlone = disabledWindow.CurrentFocus()
		disabledWindow.SetFocus(nil)
		disabledWindow.FocusPrevious()
		seededAloneBackward = disabledWindow.CurrentFocus()
	})
	c.True(seeded.Is(editable), "the window opens in the field that can be typed into")
	c.True(seededBackward.Is(editable), "from either end")
	c.True(firstChild.Is(editable), "and a container is resolved the same way")
	c.True(lastChild.Is(editable))
	c.True(seededAlone.Is(only), "a window of nothing but disabled controls opens in its first")
	c.True(seededAloneBackward.Is(last), "or its last, moving the other way")

	screen.Do(func() { wnd.SetFocus(editable) })
	screen.KeyPress(KeyTab, mod.None)
	c.True(disabledFocus(screen, wnd).Is(trailing), "Tab from a real focus stops at the disabled control after it")
	screen.KeyPress(KeyTab, mod.None)
	c.True(disabledFocus(screen, wnd).Is(lead), "and wraps around onto the one at the top")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestDisabledControlFocusRuntimeChanges verifies that whether a disabled control takes the focus follows the switch,
// the control and the assistive technology as they change: turning the switch off leaves the focus on the control but
// takes it out of the tab order and stops the focus being reported there, enabling the control gives back one that can
// be used, a hidden one takes no focus, and the assistive technology going away puts it out of reach again.
func TestDisabledControlFocusRuntimeChanges(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	var before, field, after *Field
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			before = staticTextNamedField("Search")
			field = disabledField("Total", "42")
			after = staticTextNamedField("Replace")
			wnd = axNewTestWindow(t, "runtime", geom.NewRect(10, 10, 500, 300),
				staticTextColumn(before, field, after))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()
	screen.EnableAccessibility()
	SetFocusForReading(true)
	screen.Sync()
	screen.Do(func() { wnd.SetFocus(field) })
	c.True(disabledFocus(screen, wnd).Is(field))

	// Called off the UI thread, as a worker goroutine would.
	snapshots := staticTextSnapshots(screen)
	SetFocusForReading(false)
	screen.Sync()
	c.True(disabledFocus(screen, wnd).Is(field), "turning the switch off leaves the focus where it is")
	c.False(staticTextFocusable(screen, field), "but the field is no longer a tab stop")
	c.True(staticTextSnapshots(screen) > snapshots, "the window is described again to say so")
	disabledCheckNode(c, screen, field, false, "the field with the switch turned off")
	tree := screen.AccessibilityTree(wnd)
	c.NotNil(tree)
	if node := disabledNode(c, screen, field, "the field with the switch turned off"); node != nil && tree != nil {
		c.False(node.Focused, "a disabled control that cannot take the focus is published without it")
		c.NotEqual(node.ID, tree.Focus, "and the focus is reported on something that can be shown")
	}
	screen.KeyPress(KeyTab, mod.None)
	c.True(disabledFocus(screen, wnd).Is(before), "so Tab lands on the window's first tab stop")
	screen.KeyPress(KeyTab, mod.None)
	c.True(disabledFocus(screen, wnd).Is(after), "and passes over the field from then on")

	SetFocusForReading(true)
	screen.Sync()
	c.True(staticTextFocusable(screen, field), "turning it back on makes the field a tab stop again")
	disabledCheckNode(c, screen, field, true, "the field with the switch back on")

	screen.Do(func() { field.Hidden = true })
	c.False(staticTextFocusable(screen, field), "a hidden control takes no focus")
	screen.Do(func() { field.Hidden = false })
	c.True(staticTextFocusable(screen, field))

	screen.Do(func() {
		field.SetEnabled(true)
		wnd.SetFocus(field)
		field.SetSelectionToEnd()
	})
	screen.Type("0")
	c.Equal("420", disabledFieldText(screen, field), "a control that is enabled again is used as any other")
	c.NotNil(screen.AccessibilityTree(wnd))
	if node := disabledNode(c, screen, field, "the field that was enabled again"); node != nil {
		c.False(node.Disabled)
		c.True(node.Focusable)
		c.True(node.Actions.Has(accessibility.SetValue))
	}

	screen.Do(func() {
		field.SetEnabled(false)
		SetAccessibilityEnabled(false)
	})
	c.False(IsAccessibilityActive())
	c.True(FocusForReading(), "the switch is still on")
	c.False(staticTextFocusable(screen, field), "but with nothing listening any more, the field is out of reach")
	screen.Do(func() { SetAccessibilityEnabled(true) })
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestFocusForReadingOptsInOneDisabledControl verifies the per-panel opt-in: with the switch off, a disabled control
// that sets FocusForReading takes the focus while an assistive technology is being served, the one beside it does not,
// and the label before the first still captions it rather than becoming a tab stop.
func TestFocusForReadingOptsInOneDisabledControl(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	var reading, plain *Field
	var caption *Label
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			caption = staticTextLabel("Total:")
			reading = staticTextField()
			reading.SetText("42")
			reading.SetEnabled(false)
			reading.Accessibility.FocusForReading = true
			plain = disabledField("Subtotal", "40")
			wnd = axNewTestWindow(t, "opt in", geom.NewRect(10, 10, 500, 300),
				staticTextColumn(caption, reading, plain, staticTextNamedField("Discount")))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()

	c.False(staticTextFocusable(screen, reading), "with nothing listening, even an opted-in control is out of reach")
	screen.EnableAccessibility()
	c.NotNil(screen.AccessibilityTree(wnd))
	c.False(FocusForReading(), "the switch itself is still off")
	c.True(staticTextFocusable(screen, reading), "the control that asked takes the focus")
	c.False(staticTextFocusable(screen, plain), "the one beside it, which did not ask, does not")
	disabledCheckNode(c, screen, reading, true, "the opted-in field")
	disabledCheckNode(c, screen, plain, false, "the field that did not opt in")
	if node := disabledNode(c, screen, reading, "the opted-in field"); node != nil {
		c.Equal("Total", node.Name, "the label before the field still captions it")
	}
	c.False(staticTextFocusable(screen, caption), "and is spoken with it rather than being a tab stop")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestDisabledControlFocusExclusions verifies what the switch leaves alone: a disabled panel that would not be a tab
// stop were it enabled, a disabled control in a cell of a Table or List, which those reach through a cursor of their
// own, one inside a document, one inside a heading or a label, and one that is in no window. A disabled control inside
// a panel marked up as any other kind of element is its own element and takes the focus, and so does standalone static
// text that is disabled.
func TestDisabledControlFocusExclusions(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	clicked := 0
	var plain, reader *Panel
	var inReader, inToolbar, detached *Field
	var inHeading, inLabel *Button
	var text, caption *Label
	var captioned *Field
	var table *Table[*axHeaderRow]
	var list *List[string]
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 800, Height: 800},
		StartupFinishedCallback(func() {
			plain = NewPanel()
			plain.Accessibility.Name = "Just a panel"
			plain.SetEnabled(false)

			// A stand-in for a document holding a field, which a real Markdown never does, so that the ancestor rule is
			// the only thing keeping the field out.
			reader = NewPanel()
			reader.SetLayout(&FlexLayout{Columns: 1})
			reader.axFocusable = true
			inReader = disabledField("Inside a document", "1")
			reader.AddChild(inReader)

			toolbar := NewPanel()
			toolbar.SetLayout(&FlexLayout{Columns: 1})
			toolbar.Accessibility.Role = role.Toolbar
			inToolbar = disabledField("Inside a toolbar", "2")
			toolbar.AddChild(inToolbar)

			// What is inside static text is folded into its name and never described, so a control there has no element
			// for the focus to be reported on.
			heading := NewPanel()
			heading.SetLayout(&FlexLayout{Columns: 2})
			heading.Accessibility.Role = role.Heading
			heading.Accessibility.Level = 1
			heading.AddChild(staticTextLabel("Section"))
			inHeading = disabledButton("Edit", &clicked)
			heading.AddChild(inHeading)
			marked := NewPanel()
			marked.SetLayout(&FlexLayout{Columns: 2})
			marked.Accessibility.Role = role.Label
			marked.AddChild(staticTextLabel("Status"))
			inLabel = disabledButton("Refresh", &clicked)
			marked.AddChild(inLabel)

			text = staticTextLabel("This section cannot be changed.")
			text.SetEnabled(false)
			caption = staticTextLabel("Total:")
			caption.SetEnabled(false)
			captioned = staticTextField()
			captioned.SetText("42")
			captioned.SetEnabled(false)
			detached = disabledField("Elsewhere", "7")

			model := &SimpleTableModel[*axHeaderRow]{}
			model.SetRootRows([]*axHeaderRow{{id: "a"}})
			table = NewTable(model)
			table.Columns = []ColumnInfo{{ID: 0, Current: 100}}
			table.SyncToModel()
			list = NewList[string]()
			list.Append("row")

			wnd = axNewTestWindow(t, "exclusions", geom.NewRect(10, 10, 700, 700),
				staticTextColumn(plain, reader, toolbar, heading, marked, text, caption, captioned, table, list,
					disabledButton("Apply", &clicked)))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()
	screen.EnableAccessibility()
	SetFocusForReading(true)
	screen.Sync()
	// Described again, so that the caption has been recorded as one; see Panel.axStaticTextTakesFocus.
	c.NotNil(screen.AccessibilityTree(wnd))

	c.False(staticTextFocusable(screen, plain), "a panel that is no tab stop when enabled is none when disabled")
	c.False(staticTextFocusable(screen, inReader), "a document reads its own content")
	c.False(staticTextFocusable(screen, detached), "a control in no window has nothing to take the focus in")
	c.True(staticTextFocusable(screen, inToolbar), "a control inside some other kind of element is still its own")
	c.False(staticTextFocusable(screen, inHeading), "one inside a heading is folded into it and never described")
	c.False(staticTextFocusable(screen, inLabel), "and so is one inside a panel marked up as a label")
	c.Nil(screen.AccessibilityNodeFor(inHeading), "which is why it must not be a tab stop")
	c.Nil(screen.AccessibilityNodeFor(inLabel))
	c.True(staticTextFocusable(screen, text), "standalone static text takes the focus whether or not it is disabled")
	c.False(staticTextFocusable(screen, caption), "a caption is spoken with its control, disabled or not")
	c.True(staticTextFocusable(screen, captioned), "and its control is what takes the focus")
	disabledCheckNode(c, screen, inToolbar, true, "the field inside the toolbar")
	disabledCheckNode(c, screen, text, true, "the disabled label")
	disabledCheckNode(c, screen, caption, false, "the disabled caption")
	disabledCheckNode(c, screen, plain, false, "the disabled plain panel")
	if node := disabledNode(c, screen, captioned, "the captioned field"); node != nil {
		c.Equal("Total", node.Name, "the disabled caption names the disabled field")
	}

	var inTable, inList bool
	screen.Do(func() {
		// A cell is attached only while drawn or described, so each is asked while installed.
		rect := geom.NewRect(0, 0, 100, 20)
		cellFocusable := func(install, uninstall func(p *Panel)) bool {
			cell := disabledField("In a cell", "9").AsPanel()
			install(cell)
			defer uninstall(cell)
			return cell.Focusable()
		}
		inTable = cellFocusable(func(p *Panel) { table.installCell(p, rect) }, func(p *Panel) { p.parent = nil })
		inList = cellFocusable(func(p *Panel) { list.installCell(p, rect) }, list.uninstallCell)
	})
	c.False(inTable, "a disabled control in a table cell is reached through the table's cell cursor")
	c.False(inList, "and one in a list row through the list's current row")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestDisabledFocusGetsNoKeyUps verifies that the release of a key is not delivered to a disabled control that holds
// the focus, which was passed over for the press, nor to a control that was disabled between the two.
func TestDisabledFocusGetsNoKeyUps(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	disabledUps := 0
	enabledUps := 0
	var editable, field *Field
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			editable = staticTextNamedField("Search")
			editable.KeyUpCallback = func(_ KeyCode, _ mod.Modifiers) bool {
				enabledUps++
				return true
			}
			field = disabledField("Total", "42")
			field.KeyUpCallback = func(_ KeyCode, _ mod.Modifiers) bool {
				disabledUps++
				return true
			}
			wnd = axNewTestWindow(t, "key ups", geom.NewRect(10, 10, 500, 300), staticTextColumn(editable, field))
			disabledStdMenus(wnd)
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()
	screen.EnableAccessibility()
	SetFocusForReading(true)
	screen.Sync()

	screen.Do(func() { wnd.SetFocus(field) })
	c.True(disabledFocus(screen, wnd).Is(field))
	screen.KeyPress(KeyX, mod.None)
	screen.KeyPress(KeyLeft, mod.None)
	c.Equal(0, disabledUps, "no key release is delivered to a disabled control that holds the focus")

	screen.Do(func() { wnd.SetFocus(editable) })
	screen.KeyPress(KeyLeft, mod.None)
	c.Equal(1, enabledUps, "a control that can be used still hears of them")
	screen.KeyDown(KeyLeft, mod.None)
	screen.Do(func() { editable.SetEnabled(false) })
	screen.KeyUp(KeyLeft, mod.None)
	c.Equal(1, enabledUps, "but not of the release of a key it was disabled while holding down")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// disabledPixels returns the pixels the window last presented within the frame of p, drawing the window first.
func disabledPixels(screen *HeadlessScreen, wnd *Window, p Paneler) []byte {
	screen.Sync()
	img := screen.CaptureWindow(wnd)
	if img == nil {
		return nil
	}
	var rect geom.Rect
	screen.Do(func() { rect = p.AsPanel().RectToRoot(p.AsPanel().ContentRect(true)) })
	scale := screen.Scale()
	var pixels []byte
	for y := int(rect.Y * scale); y < int(rect.Bottom()*scale); y++ {
		for x := int(rect.X * scale); x < int(rect.Right()*scale); x++ {
			px := img.NRGBAAt(x, y)
			pixels = append(pixels, px.R, px.G, px.B, px.A)
		}
	}
	return pixels
}

// TestDisabledControlsShowTheFocus verifies that each kind of disabled control is drawn differently while it holds the
// focus, so that a person who can see the window can tell where a screen reader is, and is drawn as it was once the
// focus has gone.
func TestDisabledControlsShowTheFocus(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	clicked := 0
	var editable, field, combo *Field
	var numeric *NumericField[int]
	var button *Button
	var checkBox *CheckBox
	var radio *RadioButton
	var popup *PopupMenu[string]
	var list *List[string]
	var link *Label
	var wnd *Window
	first := "Earliest"
	screen := startHeadlessTest(t, HeadlessConfig{Width: 800, Height: 800},
		StartupFinishedCallback(func() {
			editable = staticTextNamedField("Search")
			field = disabledField("Total", "42")
			numeric = NewNumericField(5, 0, 10, strconv.Itoa, strconv.Atoi, nil)
			numeric.BlinkRate = editable.BlinkRate
			numeric.Accessibility.Name = "Amount"
			numeric.SetEnabled(false)
			combo = NewComboField([]*string{&first}, &first, nil)
			combo.BlinkRate = editable.BlinkRate
			combo.Accessibility.Name = "Choice"
			combo.SetEnabled(false)
			button = disabledButton("Apply", &clicked)
			checkBox = disabledCheckBox("Remember")
			radio = NewRadioButton()
			radio.SetTitle("Always")
			radio.SetEnabled(false)
			popup = NewPopupMenu[string]()
			popup.AddItem("One", "Two")
			popup.SelectIndex(0)
			popup.Accessibility.Name = "Number"
			popup.SetEnabled(false)
			list = NewList[string]()
			list.Append("Row one", "Row two")
			list.Select(false, 0)
			list.Accessibility.Name = "Records"
			list.SetEnabled(false)
			link = NewLink("Documentation", "", "https://example.com/docs", nil, nil)
			link.SetEnabled(false)
			wnd = axNewTestWindow(t, "shows the focus", geom.NewRect(10, 10, 700, 700),
				staticTextColumn(editable, field, numeric, combo, button, checkBox, radio, popup, list, link))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()
	screen.EnableAccessibility()
	SetFocusForReading(true)
	screen.Sync()

	for _, one := range []struct {
		panel Paneler
		what  string
	}{
		{panel: field, what: "the disabled field"},
		{panel: numeric, what: "the disabled numeric field"},
		{panel: combo, what: "the disabled combo field"},
		{panel: button, what: "the disabled button"},
		{panel: checkBox, what: "the disabled check box"},
		{panel: radio, what: "the disabled radio button"},
		{panel: popup, what: "the disabled popup menu"},
		{panel: list, what: "the disabled list"},
		{panel: link, what: "the disabled link"},
	} {
		screen.Do(func() { wnd.SetFocus(editable) })
		unfocused := disabledPixels(screen, wnd, one.panel)
		c.True(len(unfocused) > 0, "%s should have been drawn", one.what)
		screen.Do(func() { wnd.SetFocus(one.panel) })
		c.True(disabledFocus(screen, wnd).Is(one.panel), "%s takes the focus", one.what)
		c.NotEqual(unfocused, disabledPixels(screen, wnd, one.panel), "%s shows that it holds the focus", one.what)
		screen.Do(func() { wnd.SetFocus(editable) })
		c.Equal(unfocused, disabledPixels(screen, wnd, one.panel), "%s is drawn as it was once the focus has gone",
			one.what)
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestDisabledComboFieldButtonDoesNothing verifies that the dropdown button of a disabled combo field, which is not
// disabled along with the field, opens no menu and does not move the focus onto the field.
func TestDisabledComboFieldButtonDoesNothing(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	changed := 0
	first := "Primary"
	second := "Secondary"
	var editable, combo *Field
	var dropdown *Panel
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			editable = staticTextNamedField("Search")
			combo = NewComboField([]*string{&first, &second}, &second, func(_ *string) { changed++ })
			combo.BlinkRate = editable.BlinkRate
			combo.Accessibility.Name = "Choice"
			combo.SetEnabled(false)
			if children := combo.Children(); len(children) == 1 {
				dropdown = children[0]
			}
			wnd = axNewTestWindow(t, "combo", geom.NewRect(10, 10, 500, 300), staticTextColumn(editable, combo))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	c.NotNil(dropdown, "the combo field should hold its dropdown button")
	screen.Sync()
	if dropdown == nil {
		return
	}

	click := func(what string) {
		screen.Do(func() { wnd.SetFocus(editable) })
		screen.Click(screen.PanelCenter(dropdown))
		var open int
		screen.Do(func() { open = len(wnd.root.openMenuPanels) })
		c.Equal(0, open, "%s: no menu is opened", what)
		c.True(disabledFocus(screen, wnd).Is(editable), "%s: the focus stays where it was", what)
		c.Equal(second, disabledFieldText(screen, combo), "%s: the value is left alone", what)
		c.Equal(0, changed, "%s: nothing is reported as changed", what)
	}
	click("with nothing listening")
	screen.EnableAccessibility()
	click("with the switch off")
	SetFocusForReading(true)
	screen.Sync()
	click("with the switch on")

	// The key that opens the choices goes to the panel the field is inside, as every key does.
	screen.Do(func() { wnd.SetFocus(combo) })
	c.True(disabledFocus(screen, wnd).Is(combo))
	screen.KeyPress(KeyDown, mod.None)
	var open int
	screen.Do(func() { open = len(wnd.root.openMenuPanels) })
	c.Equal(0, open)

	screen.Do(func() {
		combo.SetEnabled(true)
		wnd.SetFocus(editable)
	})
	screen.Click(screen.PanelCenter(dropdown))
	screen.Do(func() { open = len(wnd.root.openMenuPanels) })
	c.Equal(1, open, "the button of a combo field that is enabled again opens its choices")
	c.True(disabledFocus(screen, wnd).Is(combo), "and gives the field the focus")
	screen.KeyPress(KeyEscape, mod.None)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestDisabledDocumentTakesTheFocusLast verifies that a disabled document is a disabled control like any other: it
// takes the focus so that it can be read, answers no keys, is published as disabled, and is passed over for a document
// that can be read from when a window or a container chooses where to put the focus.
func TestDisabledDocumentTakesTheFocusLast(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	clicked := 0
	var button *Button
	var disabled, enabled *Markdown
	var column *Panel
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			button = disabledButton("Apply", &clicked)
			disabled = NewMarkdown(false)
			disabled.SetContent("This part cannot be used.\n", 400)
			disabled.SetEnabled(false)
			enabled = NewMarkdown(false)
			enabled.SetContent("This part can be read.\n", 400)
			column = staticTextColumn(button, disabled, enabled)
			wnd = axNewTestWindow(t, "documents", geom.NewRect(10, 10, 500, 400), column)
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()
	screen.EnableAccessibility()
	c.NotNil(screen.AccessibilityTree(wnd))
	c.True(staticTextFocusable(screen, enabled), "a document takes the focus while a screen reader is listening")
	c.False(staticTextFocusable(screen, disabled), "a disabled one does not while the switch is off")

	SetFocusForReading(true)
	screen.Sync()
	c.True(staticTextFocusable(screen, disabled), "with the switch on, a disabled document takes the focus")
	disabledCheckStaysDisabled(c, screen, disabled, "the disabled document")
	disabledCheckNode(c, screen, disabled, true, "the disabled document")

	var seeded, seededBackward, firstChild, lastChild *Panel
	screen.Do(func() {
		wnd.SetFocus(nil)
		wnd.FocusNext()
		seeded = wnd.CurrentFocus()
		wnd.SetFocus(nil)
		wnd.FocusPrevious()
		seededBackward = wnd.CurrentFocus()
		firstChild = column.FirstFocusableChild()
		lastChild = column.LastFocusableChild()
	})
	c.True(seeded.Is(enabled), "the window opens in the document that can be read from")
	c.True(seededBackward.Is(enabled), "from either end")
	c.True(firstChild.Is(enabled), "and a container is resolved the same way")
	c.True(lastChild.Is(enabled))

	screen.Do(func() { wnd.SetFocus(disabled) })
	c.True(disabledFocus(screen, wnd).Is(disabled))
	var before, after int
	screen.Do(func() { before = disabled.axCaret() })
	screen.KeyPress(KeyRight, mod.None)
	screen.KeyPress(KeyEnd, mod.None)
	screen.Do(func() { after = disabled.axCaret() })
	c.Equal(before, after, "the keys that move a document's reading caret do nothing in a disabled one")
	screen.Click(screen.PanelCenter(enabled))
	screen.Click(screen.PanelCenter(disabled))
	c.True(disabledFocus(screen, wnd).Is(enabled), "and a click does not move the focus onto it")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestNumericFieldBringsIntoRangeOnlyWhatWasTyped verifies that a disabled numeric field that was only read is left
// showing what the application put there as the focus leaves, even though the switch was turned off, or the assistive
// technology went away, before it left, and that a field that was typed into and then disabled is brought into range
// whether the switch is on or off.
func TestNumericFieldBringsIntoRangeOnlyWhatWasTyped(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	modified := 0
	var editable *Field
	var reading, typing *NumericField[int]
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			editable = staticTextNamedField("Search")
			reading = NewNumericField(5, 0, 10, strconv.Itoa, strconv.Atoi, nil)
			reading.BlinkRate = editable.BlinkRate
			reading.Accessibility.Name = "Quantity"
			reading.SetText("12")
			reading.SetEnabled(false)
			reading.ModifiedCallback = func(_, _ *FieldState) { modified++ }
			typing = NewNumericField(5, 0, 10, strconv.Itoa, strconv.Atoi, nil)
			typing.BlinkRate = editable.BlinkRate
			typing.Accessibility.Name = "Copies"
			wnd = axNewTestWindow(t, "numeric", geom.NewRect(10, 10, 500, 300),
				staticTextColumn(editable, reading, typing))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()
	screen.EnableAccessibility()
	SetFocusForReading(true)
	screen.Sync()

	screen.Do(func() { wnd.SetFocus(reading) })
	c.True(disabledFocus(screen, wnd).Is(reading))
	SetFocusForReading(false)
	screen.Sync()
	c.True(disabledFocus(screen, wnd).Is(reading), "turning the switch off leaves the focus where it is")
	screen.KeyPress(KeyTab, mod.None)
	c.False(disabledFocus(screen, wnd).Is(reading))
	c.Equal("12", disabledFieldText(screen, reading.Field), "a field that was only read is left as it was")
	c.Equal(0, modified, "and reports no modification")

	SetFocusForReading(true)
	screen.Sync()
	screen.Do(func() {
		wnd.SetFocus(reading)
		SetAccessibilityEnabled(false)
	})
	c.True(disabledFocus(screen, wnd).Is(reading))
	screen.KeyPress(KeyTab, mod.None)
	c.False(disabledFocus(screen, wnd).Is(reading))
	c.Equal("12", disabledFieldText(screen, reading.Field), "as it is when the assistive technology has gone away")
	c.Equal(0, modified)
	screen.Do(func() { SetAccessibilityEnabled(true) })
	screen.EnableAccessibility()

	for _, on := range []bool{true, false} {
		SetFocusForReading(on)
		screen.Sync()
		screen.Do(func() {
			typing.SetEnabled(true)
			typing.SetText("5")
			wnd.SetFocus(typing)
			typing.SelectAll()
		})
		screen.Type("12")
		c.Equal("12", disabledFieldText(screen, typing.Field))
		screen.Do(func() { typing.SetEnabled(false) })
		c.True(disabledFocus(screen, wnd).Is(typing), "disabling a field leaves the focus on it")
		screen.Do(func() { wnd.SetFocus(editable) })
		c.Equal("10", disabledFieldText(screen, typing.Field),
			"what was typed is brought into range though the field was disabled since (switch on: %v)", on)
	}

	// A field that was disabled when the focus arrived and enabled while it held it may have been typed into.
	SetFocusForReading(true)
	screen.Sync()
	screen.Do(func() {
		typing.SetEnabled(false)
		typing.SetText("12")
		wnd.SetFocus(typing)
		typing.SetEnabled(true)
		wnd.SetFocus(editable)
	})
	c.Equal("10", disabledFieldText(screen, typing.Field))
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestDisabledListRowRefusesAStaleFocusRequest verifies that a request for the focus aimed at a row is refused once the
// list has been disabled, even though the row was published, while the list was enabled, as offering it and the window
// has not been described since.
func TestDisabledListRowRefusesAStaleFocusRequest(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	var editable *Field
	var list *List[string]
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			editable = staticTextNamedField("Search")
			list = NewList[string]()
			list.Append("Row one", "Row two")
			list.Accessibility.Name = "Items"
			wnd = axNewTestWindow(t, "stale row", geom.NewRect(10, 10, 500, 300), staticTextColumn(editable, list))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()
	screen.EnableAccessibility()
	SetFocusForReading(true)
	screen.Sync()
	screen.Do(func() { wnd.SetFocus(editable) })

	tree := screen.AccessibilityTree(wnd)
	c.NotNil(tree)
	if tree == nil {
		return
	}
	var row *accessibility.Node
	tree.Walk(func(n *accessibility.Node) bool {
		if row == nil && n.Role == role.ListItem {
			row = n
		}
		return true
	})
	c.NotNil(row, "the rows of the list should have been described")
	if row == nil {
		return
	}
	c.False(row.Disabled)
	c.True(row.Actions.Has(accessibility.Focus), "a row of a list that can be used offers the focus")

	// Disabled and asked within the one task, so that the window cannot have been described in between.
	var carriedOut, listTakesFocus bool
	var focus *Panel
	screen.Do(func() {
		list.SetEnabled(false)
		listTakesFocus = list.Focusable()
		carriedOut = wnd.performAccessibilityAction(accessibility.ActionRequest{
			Node:   row.ID,
			Action: accessibility.Focus,
		})
		focus = wnd.CurrentFocus()
	})
	c.True(listTakesFocus, "the disabled list itself can take the focus for reading")
	c.False(carriedOut, "but a request for the focus aimed at one of its rows is refused")
	c.True(focus.Is(editable), "and the focus stays where it was")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestDisabledCaptionGivesUpTheFocus verifies that a disabled label that holds the focus and then becomes the caption
// of a control is published without the focus, as every disabled node that cannot take the focus is, with the focus
// reported on something that can be shown in its place.
func TestDisabledCaptionGivesUpTheFocus(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	var editable *Field
	var label *Label
	var column *Panel
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			editable = staticTextNamedField("Search")
			label = staticTextLabel("Total:")
			label.SetEnabled(false)
			column = staticTextColumn(editable, label)
			wnd = axNewTestWindow(t, "disabled caption", geom.NewRect(10, 10, 500, 300), column)
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()
	screen.EnableAccessibility()
	SetFocusForReading(true)
	screen.Sync()
	c.NotNil(screen.AccessibilityTree(wnd))

	screen.Do(func() { wnd.SetFocus(label) })
	c.True(disabledFocus(screen, wnd).Is(label), "a disabled standalone label takes the focus")
	tree := screen.AccessibilityTree(wnd)
	c.NotNil(tree)
	if node := disabledNode(c, screen, label, "the disabled label"); node != nil && tree != nil {
		c.True(node.Focused)
		c.Equal(node.ID, tree.Focus)
	}

	screen.Do(func() {
		captioned := staticTextField()
		captioned.SetText("42")
		column.AddChild(captioned)
		column.MarkForLayoutAndRedraw()
	})
	tree = screen.AccessibilityTree(wnd)
	c.NotNil(tree)
	c.True(disabledFocus(screen, wnd).Is(label), "the window leaves the focus where it is")
	if node := disabledNode(c, screen, label, "the disabled caption"); node != nil && tree != nil {
		c.True(node.Disabled)
		c.False(node.Focusable, "a caption cannot take the focus")
		c.False(node.Actions.Has(accessibility.Focus))
		c.False(node.Focused, "and a disabled node that cannot take the focus is published without it")
		c.NotEqual(node.ID, tree.Focus)
		c.NotEqual(accessibility.NodeID(0), tree.Focus, "the focus is reported on something that can be shown")
		focused := 0
		tree.Walk(func(n *accessibility.Node) bool {
			if n.Focused && n.ID != tree.Root {
				focused++
				c.Equal(tree.Focus, n.ID)
			}
			return true
		})
		c.Equal(1, focused, "exactly one node reports the focus")
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestDisabledContainerHandsTheFocusOn verifies that a disabled panel that is focusable and holds a tab stop hands the
// focus an application gives it to that tab stop, as it does while the switch is off, and is still a tab stop of its
// own that Tab lands on and leaves and that a screen reader can be taken to.
func TestDisabledContainerHandsTheFocusOn(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	var editable, inner *Field
	var container *Panel
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			editable = staticTextNamedField("Search")
			container = NewPanel()
			container.SetLayout(&FlexLayout{Columns: 1})
			container.Accessibility.Name = "Details"
			container.SetFocusable(true)
			inner = staticTextNamedField("Inner")
			container.AddChild(inner)
			container.SetEnabled(false)
			wnd = axNewTestWindow(t, "container", geom.NewRect(10, 10, 500, 300),
				staticTextColumn(editable, container))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()
	screen.EnableAccessibility()

	for _, on := range []bool{false, true} {
		SetFocusForReading(on)
		screen.Sync()
		screen.Do(func() {
			wnd.SetFocus(editable)
			wnd.SetFocus(container)
		})
		c.True(disabledFocus(screen, wnd).Is(inner),
			"the focus given to the container goes to the field inside it (switch on: %v)", on)
	}

	c.True(staticTextFocusable(screen, container))
	screen.Do(func() { wnd.SetFocus(editable) })
	screen.KeyPress(KeyTab, mod.None)
	c.True(disabledFocus(screen, wnd).Is(container), "Tab stops at the disabled container itself")
	screen.KeyPress(KeyTab, mod.None)
	c.True(disabledFocus(screen, wnd).Is(inner), "and goes on to the field inside it")
	screen.KeyPress(KeyTab, mod.Shift)
	c.True(disabledFocus(screen, wnd).Is(container), "Shift-Tab goes back onto the container")
	screen.KeyPress(KeyTab, mod.Shift)
	c.True(disabledFocus(screen, wnd).Is(editable), "and past it")

	c.NotNil(screen.AccessibilityTree(wnd))
	if node := disabledNode(c, screen, container, "the disabled container"); node != nil {
		c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
			Node:   node.ID,
			Action: accessibility.Focus,
		}), "a screen reader asking for the focus on the container gets it")
		c.True(disabledFocus(screen, wnd).Is(container), "on the container itself")
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestDisabledFieldIsNotScrolledByTheFocus verifies that a disabled field whose text does not fit is left showing the
// start of it when it takes the focus, while a field that can be typed into is scrolled to its caret.
func TestDisabledFieldIsNotScrolledByTheFocus(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	const long = "A sentence that is a good deal longer than the field it has been put into can show"
	var editable, field, typed *Field
	var wnd *Window
	narrow := func(f *Field) *Field {
		f.SetText(long)
		f.NoSelectAllOnFocus = true
		f.SetLayoutData(&FlexLayoutData{SizeHint: geom.NewSize(80, 0)})
		return f
	}
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			editable = staticTextNamedField("Search")
			field = narrow(disabledField("Note", ""))
			typed = narrow(staticTextNamedField("Comment"))
			wnd = axNewTestWindow(t, "scroll", geom.NewRect(10, 10, 500, 300),
				staticTextColumn(editable, field, typed))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()
	screen.EnableAccessibility()
	SetFocusForReading(true)
	screen.Sync()

	offsets := func(f *Field) (before, during, after geom.Point, width float32) {
		screen.Do(func() {
			wnd.SetFocus(editable)
			f.SetScrollOffset(geom.Point{})
			before = f.ScrollOffset()
			width = f.ContentRect(false).Width
			wnd.SetFocus(f)
			during = f.ScrollOffset()
			wnd.SetFocus(editable)
			after = f.ScrollOffset()
		})
		return before, during, after, width
	}
	before, during, after, width := offsets(field)
	c.True(width < 200, "the field should be too narrow for its text, but is %v wide", width)
	c.Equal(geom.Point{}, before)
	c.Equal(geom.Point{}, during, "taking the focus does not scroll the text of a disabled field")
	c.Equal(geom.Point{}, after)
	_, during, _, _ = offsets(typed)
	c.True(during.X < 0, "while a field that can be typed into is scrolled to its caret, not left at %v", during)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestGradientEditorUpdatesADisabledFieldThatHoldsTheFocus verifies that a field of the gradient editor that holds the
// focus while disabled, which is one being read rather than typed into, is brought up to date with the gradient.
func TestGradientEditorUpdatesADisabledFieldThatHoldsTheFocus(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	var editor *GradientEditor
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 800, Height: 800},
		StartupFinishedCallback(func() {
			editor = NewGradientEditor(&Gradient{
				Kind:  gradienttype.Linear,
				EndPt: geom.NewPoint(1, 0),
				Stops: NewEvenlySpacedGradientStopsForColors(Black, White),
			})
			for _, f := range []*Field{
				editor.posField, editor.startXField, editor.startYField, editor.endXField, editor.endYField,
				editor.startRadiusField, editor.endRadiusField, editor.startAngleField, editor.endAngleField,
			} {
				if f != nil {
					f.BlinkRate = time.Hour
				}
			}
			wnd = axNewTestWindow(t, "gradient", geom.NewRect(10, 10, 700, 700), editor)
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()
	screen.EnableAccessibility()
	SetFocusForReading(true)
	screen.Sync()

	var enabled bool
	screen.Do(func() {
		enabled = editor.startRadiusField.Enabled()
		wnd.SetFocus(editor.startRadiusField)
	})
	c.False(enabled, "a linear gradient has no radius, so the field is disabled")
	c.True(disabledFocus(screen, wnd).Is(editor.startRadiusField), "and takes the focus so that it can be read")
	c.Equal("0", disabledFieldText(screen, editor.startRadiusField))
	screen.Do(func() {
		g := editor.Gradient()
		g.Radius.Start = 42
		editor.SetGradient(g)
	})
	c.Equal("42", disabledFieldText(screen, editor.startRadiusField),
		"a disabled field that holds the focus is not being edited, so it is brought up to date")

	// A field being typed into is still left alone.
	screen.Do(func() {
		wnd.SetFocus(editor.startXField)
		editor.startXField.SelectAll()
	})
	screen.Type("7")
	screen.Do(func() {
		g := editor.Gradient()
		g.StartPt.X = 0.5
		editor.SetGradient(g)
	})
	c.Equal("7", disabledFieldText(screen, editor.startXField))
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestEditorHintsAreNotTabStops verifies that the dimmed text the color and gradient editors put after a field to say
// what may be typed into it is said as that field's description, and is neither an element of its own nor a tab stop.
func TestEditorHintsAreNotTabStops(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	var colors *ColorEditor
	var gradients *GradientEditor
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 900, Height: 1200},
		StartupFinishedCallback(func() {
			colors = NewColorEditor(Red)
			gradients = NewGradientEditor(&Gradient{
				Kind:   gradienttype.Radial,
				Radius: StartEnd{Start: 1, End: 32},
				Stops:  NewEvenlySpacedGradientStopsForColors(Black, White),
			})
			wnd = axNewTestWindow(t, "editors", geom.NewRect(10, 10, 800, 1100), staticTextColumn(colors, gradients))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()
	screen.EnableAccessibility()
	SetFocusForReading(true)
	screen.Sync()
	tree := screen.AccessibilityTree(wnd)
	c.NotNil(tree)
	if tree == nil {
		return
	}

	hints := map[string]bool{"0-255 or 0-100%": true, "0-359": true, "0-100%": true, "px": true}
	found := 0
	var tabStops []string
	screen.Do(func() {
		var walk func(p *Panel)
		walk = func(p *Panel) {
			if label, ok := p.Self.(*Label); ok && hints[label.String()] {
				found++
				if p.Focusable() {
					tabStops = append(tabStops, label.String())
				}
			}
			for _, child := range p.Children() {
				walk(child)
			}
		}
		walk(wnd.Content())
	})
	c.Equal(8, found, "the editors should hold seven hints and a unit")
	c.Equal(0, len(tabStops), "none of which may be a tab stop, but these are: %v", tabStops)
	described := 0
	tree.Walk(func(n *accessibility.Node) bool {
		if hints[n.Name] {
			described++
		}
		return true
	})
	c.Equal(0, described, "nor described as an element of its own")
	if node := disabledNode(c, screen, colors.hueField, "the hue field"); node != nil {
		c.Equal("0-359", node.Description, "each is said with the field it follows")
		c.Equal("Hue", node.Name)
	}
	if node := disabledNode(c, screen, gradients.startRadiusField, "the start radius field"); node != nil {
		c.Equal("Pixels", node.Description)
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestAxInContentExcludesOpenMenus verifies that a panel inside an open menu, which is the content of its popup menu
// window's root, is not in any window's content for the purposes of taking the focus as static text, while a panel in
// the owner's content is.
func TestAxInContentExcludesOpenMenus(t *testing.T) {
	c := check.New(t)
	owner := &Window{}
	owner.root = newRootPanel(owner)
	content := NewPanel()
	owner.root.setContent(content)
	inContent := NewPanel()
	content.AddChild(inContent)
	c.True(inContent.axInContent(false) == owner)
	mp := newTestMenuPanel(0, 0, 100, 100)
	popup := newTestPopupWindow(owner, mp)
	inMenu := NewPanel()
	mp.AddChild(inMenu)
	c.True(popup.root.contentPanel == mp.AsPanel(), "the menu is the popup's content")
	c.True(inMenu.axInContent(false) == nil, "a panel in a menu is in no window's content")
	c.True(inMenu.axInContent(true) == nil)
}
