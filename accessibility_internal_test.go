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

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/enums/role"
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

	// Dropping most of the rows leaves the ids of the ones that are gone to be reclaimed, which happens the next time the
	// list is described. Each row accounts for one id of its own plus one per cell.
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
