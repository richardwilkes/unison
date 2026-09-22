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
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/role"
)

// These tests cover AccessibilityBuilder.FocusChild: a panel that holds the keyboard focus reporting it on one of the
// virtual children it described instead of on itself, which is how a list or a table says a screen reader should be on
// its current row. What is checked here is the rule rather than either widget, so the widget below is the smallest
// thing that can exercise it. A session owns most of the package's mutable globals while it runs, so none of these may
// call t.Parallel.

// axRowPanel is a stand-in for a list: a focusable panel that describes a row per entry as a virtual child and hands
// the focus it holds to one of them. Every field is read and written on the UI thread, from the description it drives
// or from a HeadlessScreen.Do.
type axRowPanel struct {
	// rowIDs holds the node id of each row, as of the most recent description.
	rowIDs []accessibility.NodeID
	rows   []string
	// Embedded after the slices rather than first, which is what keeps the fieldalignment linter quiet: the Panel ends
	// in plain data, so placing it first would put the slices' pointers beyond a stretch of it that holds none.
	unison.Panel
	// current is the row the focus is handed to, or -1 to hand it to nothing.
	current int
	// second is a second row to hand the focus to within the same description, or -1 to hand it on only once.
	second int
	// target, when non-zero, is handed to FocusChild in place of a row of this panel's own, which is how the tests
	// point at a node that is not beneath this one.
	target accessibility.NodeID
	// disabledRows describes every row as disabled, which is what a disabled row of a real list would be.
	disabledRows bool
	// delegated and delegatedSecond are what FocusChild answered for each of the calls above.
	delegated       bool
	delegatedSecond bool
}

// newAXRowPanel creates a focusable panel holding the named rows, with the first of them the current one.
func newAXRowPanel(rows ...string) *axRowPanel {
	p := &axRowPanel{rows: rows, second: -1}
	p.Self = p
	p.SetFocusable(true)
	p.Accessibility.Name = "Rows"
	p.Accessibility.Role = role.List
	return p
}

// ProvideAccessibility describes a row per entry and then hands the focus on, recording what it was told.
func (p *axRowPanel) ProvideAccessibility(b *unison.AccessibilityBuilder) {
	p.rowIDs = p.rowIDs[:0]
	for i, text := range p.rows {
		id := b.AddVirtualChild(i, func(n *accessibility.Node) {
			n.Name = text
			n.Role = role.ListItem
			n.Bounds = geom.NewRect(0, float32(i)*20, 100, 20)
			n.Focusable = true
			n.Selectable = true
			n.Selected = i == p.current
			n.Disabled = p.disabledRows
			n.Actions = n.Actions.With(accessibility.Focus)
		})
		p.rowIDs = append(p.rowIDs, id)
	}
	p.delegated = b.FocusChild(p.focusTarget(p.current))
	p.delegatedSecond = false
	if p.second >= 0 {
		p.delegatedSecond = b.FocusChild(p.focusTarget(p.second))
	}
}

// focusTarget returns the node id to hand to FocusChild for a row index: the explicit target when one was set, the
// row's own id when the index names one, and zero otherwise.
func (p *axRowPanel) focusTarget(row int) accessibility.NodeID {
	if p.target != 0 {
		return p.target
	}
	if row < 0 || row >= len(p.rowIDs) {
		return 0
	}
	return p.rowIDs[row]
}

// axRowWindow is a window holding an axRowPanel, another panel that can take the focus away from it, and a menu bar
// with one menu, which is what the displacement test opens.
type axRowWindow struct {
	screen *unison.HeadlessScreen
	wnd    *unison.Window
	rows   *axRowPanel
	other  *unison.Panel
}

// newAXRowWindow starts a session showing an axRowWindow, with nothing holding the keyboard focus yet.
func newAXRowWindow(t *testing.T, title string) *axRowWindow {
	t.Helper()
	const (
		menuID = unison.UserBaseID + iota
		cutID
	)
	out := &axRowWindow{}
	out.screen = startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			out.rows = newAXRowPanel("Alpha", "Beta", "Gamma")
			out.other = axFocusablePanel("Other")
			out.wnd = newHeadlessWindow(t, title, geom.NewRect(10, 10, 400, 300), axColumn(out.rows, out.other))
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
	return out
}

// focusRows moves the keyboard focus onto the row panel.
func (a *axRowWindow) focusRows() {
	a.screen.Do(func() { a.wnd.SetFocus(a.rows) })
}

// rowNode returns the node describing a row in the given tree, or nil if the row was not described into it.
func (a *axRowWindow) rowNode(tree *accessibility.Tree, row int) *accessibility.Node {
	var id accessibility.NodeID
	a.screen.Do(func() {
		if row >= 0 && row < len(a.rows.rowIDs) {
			id = a.rows.rowIDs[row]
		}
	})
	if id == 0 {
		return nil
	}
	return tree.Node(id)
}

// results returns what FocusChild answered during the most recent description.
func (a *axRowWindow) results() (first, second bool) {
	a.screen.Do(func() {
		first = a.rows.delegated
		second = a.rows.delegatedSecond
	})
	return first, second
}

// TestAccessibilityFocusChildReportsTheFocusOnTheRow covers the delegation itself: the row the panel handed the focus
// to is what Tree.Focus names and the only node reporting it, the panel's own node has stopped, and the move is
// reported to an assistive technology as a focus change naming the row — which is the whole point, since a screen
// reader follows focus events and hears nothing from a selection change on a container.
func TestAccessibilityFocusChildReportsTheFocusOnTheRow(t *testing.T) {
	c := check.New(t)
	a := newAXRowWindow(t, "focus child")
	c.NotNil(a.wnd)
	if a.wnd == nil {
		return
	}
	a.screen.Do(func() { a.wnd.SetFocus(a.other) })
	c.True(a.screen.AccessibilityTree(a.wnd) != nil)
	a.screen.AccessibilityEvents(a.wnd)

	a.focusRows()
	tree := a.screen.AccessibilityTree(a.wnd)
	c.True(tree != nil)
	if tree == nil {
		return
	}
	first, _ := a.results()
	c.True(first, "the panel holds the focus, so handing it to a row must be accepted")
	row := a.rowNode(tree, 0)
	c.True(row != nil)
	if row == nil {
		return
	}
	c.Equal(row.ID, tree.Focus, "the tree must report the focus on the row")
	c.True(row.Focused)
	c.False(row.Ignored, "the node the focus is reported on must be reachable")
	c.Equal("Alpha", row.Name)
	panelNode := a.screen.AccessibilityNodeFor(a.rows)
	c.True(panelNode != nil)
	if panelNode == nil {
		return
	}
	c.False(panelNode.Focused, "exactly one node may report the focus, and it is the row")
	focused := 0
	tree.Walk(func(n *accessibility.Node) bool {
		if n.Focused && n.ID != tree.Root {
			focused++
		}
		return true
	})
	c.Equal(1, focused, "exactly one node other than the root may report the focus")
	events := a.screen.AccessibilityEvents(a.wnd)
	c.True(axHasEvent(events, accessibility.FocusChanged, row.ID),
		"the focus move must be reported as landing on the row: %v", events)

	a.screen.Do(func() { a.rows.current = 2 })
	tree = a.screen.AccessibilityTree(a.wnd)
	c.True(tree != nil)
	if tree == nil {
		return
	}
	moved := a.rowNode(tree, 2)
	c.True(moved != nil)
	if moved == nil {
		return
	}
	c.Equal(moved.ID, tree.Focus, "moving to another row moves the reported focus with it")
	c.True(moved.Focused)
	previous := a.rowNode(tree, 0)
	c.True(previous != nil && !previous.Focused, "the row that was left must stop reporting the focus")
	c.Equal(0, len(a.screen.Errors()), "nothing should have panicked: %v", a.screen.Errors())
}

// TestAccessibilityFocusChildRefusedWithoutTheFocus covers the first thing the delegation has to be true about: a panel
// that does not hold the keyboard focus has no focus to hand on, and a tree that said otherwise would point an
// assistive technology at a row of a control the person is not in.
func TestAccessibilityFocusChildRefusedWithoutTheFocus(t *testing.T) {
	c := check.New(t)
	a := newAXRowWindow(t, "focus child without focus")
	c.NotNil(a.wnd)
	if a.wnd == nil {
		return
	}
	a.screen.Do(func() { a.wnd.SetFocus(a.other) })
	tree := a.screen.AccessibilityTree(a.wnd)
	c.True(tree != nil)
	if tree == nil {
		return
	}
	first, _ := a.results()
	c.False(first, "a panel that does not hold the focus may not hand it on")
	row := a.rowNode(tree, 0)
	c.True(row != nil && !row.Focused, "no row may report a focus the panel does not have")
	otherNode := a.screen.AccessibilityNodeFor(a.other)
	c.True(otherNode != nil)
	if otherNode == nil {
		return
	}
	c.Equal(otherNode.ID, tree.Focus, "the focus stays where it actually is")
	c.Equal(0, len(a.screen.Errors()), "nothing should have panicked: %v", a.screen.Errors())
}

// TestAccessibilityFocusChildRefusedWhenDisabled covers the two disabled cases, both of which would offer an assistive
// technology a place the person cannot be: a disabled panel — which the window may still leave the keyboard focus on —
// and a row that is itself disabled, which every request about would be refused.
func TestAccessibilityFocusChildRefusedWhenDisabled(t *testing.T) {
	c := check.New(t)
	a := newAXRowWindow(t, "focus child disabled")
	c.NotNil(a.wnd)
	if a.wnd == nil {
		return
	}
	a.focusRows()
	a.screen.Do(func() { a.rows.disabledRows = true })
	tree := a.screen.AccessibilityTree(a.wnd)
	c.True(tree != nil)
	if tree == nil {
		return
	}
	first, _ := a.results()
	c.False(first, "a disabled row is no use as the place the person is said to be")
	row := a.rowNode(tree, 0)
	c.True(row != nil && !row.Focused)
	panelNode := a.screen.AccessibilityNodeFor(a.rows)
	c.True(panelNode != nil)
	if panelNode == nil {
		return
	}
	c.True(panelNode.Focused, "the refusal must leave the panel reporting the focus it really holds")
	c.Equal(panelNode.ID, tree.Focus)

	a.screen.Do(func() {
		a.rows.disabledRows = false
		a.rows.SetEnabled(false)
	})
	tree = a.screen.AccessibilityTree(a.wnd)
	c.True(tree != nil)
	if tree == nil {
		return
	}
	first, _ = a.results()
	c.False(first, "a disabled panel may not hand on the focus the window left it holding")
	row = a.rowNode(tree, 0)
	c.True(row != nil && !row.Focused)
	panelNode = a.screen.AccessibilityNodeFor(a.rows)
	c.True(panelNode != nil)
	if panelNode == nil {
		return
	}
	c.True(panelNode.Disabled)
	c.False(panelNode.Focused, "a disabled node is published without the focus")
	c.True(tree.Focus != 0 && tree.Focus != row.ID && tree.Focus != panelNode.ID,
		"the focus must fall back to an ancestor that can be shown")
	c.Equal(0, len(a.screen.Errors()), "nothing should have panicked: %v", a.screen.Errors())
}

// TestAccessibilityFocusChildRefusedOutsideTheSubtree covers an id that is not this panel's to point at: one belonging
// to another panel of the window, and one belonging to no node at all, which is what a widget that kept an id from an
// earlier description would hand over.
func TestAccessibilityFocusChildRefusedOutsideTheSubtree(t *testing.T) {
	c := check.New(t)
	a := newAXRowWindow(t, "focus child outside")
	c.NotNil(a.wnd)
	if a.wnd == nil {
		return
	}
	a.focusRows()
	c.True(a.screen.AccessibilityTree(a.wnd) != nil)
	otherNode := a.screen.AccessibilityNodeFor(a.other)
	c.True(otherNode != nil)
	if otherNode == nil {
		return
	}

	a.screen.Do(func() { a.rows.target = otherNode.ID })
	tree := a.screen.AccessibilityTree(a.wnd)
	c.True(tree != nil)
	if tree == nil {
		return
	}
	first, _ := a.results()
	c.False(first, "a node that is not beneath the panel is not the panel's to hand the focus to")
	c.False(tree.Node(otherNode.ID).Focused)
	panelNode := a.screen.AccessibilityNodeFor(a.rows)
	c.True(panelNode != nil && panelNode.Focused)
	c.Equal(panelNode.ID, tree.Focus)

	a.screen.Do(func() { a.rows.target = accessibility.NodeID(1 << 40) })
	tree = a.screen.AccessibilityTree(a.wnd)
	c.True(tree != nil)
	if tree == nil {
		return
	}
	first, _ = a.results()
	c.False(first, "a node that is not in the tree cannot be handed the focus")
	panelNode = a.screen.AccessibilityNodeFor(a.rows)
	c.True(panelNode != nil && panelNode.Focused)
	c.Equal(panelNode.ID, tree.Focus)
	c.Equal(0, len(a.screen.Errors()), "nothing should have panicked: %v", a.screen.Errors())
}

// TestAccessibilityFocusChildOutlastsACallbackClaim covers what an Accessibility.Callback that sets Focused on the
// panel's own node does to a delegation made a moment earlier: nothing. The callback runs after the widget has
// described itself, and a window with two focused nodes is one no assistive technology can make sense of.
func TestAccessibilityFocusChildOutlastsACallbackClaim(t *testing.T) {
	c := check.New(t)
	a := newAXRowWindow(t, "focus child callback")
	c.NotNil(a.wnd)
	if a.wnd == nil {
		return
	}
	a.focusRows()
	a.screen.Do(func() {
		a.rows.Accessibility.Callback = func(node *accessibility.Node) { node.Focused = true }
	})
	tree := a.screen.AccessibilityTree(a.wnd)
	c.True(tree != nil)
	if tree == nil {
		return
	}
	row := a.rowNode(tree, 0)
	c.True(row != nil)
	if row == nil {
		return
	}
	c.True(row.Focused)
	c.Equal(row.ID, tree.Focus)
	panelNode := a.screen.AccessibilityNodeFor(a.rows)
	c.True(panelNode != nil)
	if panelNode == nil {
		return
	}
	c.False(panelNode.Focused, "the callback may not take back a focus the widget handed to a row")
	c.Equal(0, len(a.screen.Errors()), "nothing should have panicked: %v", a.screen.Errors())
}

// TestAccessibilityFocusChildUndoneByACallbackDisablingThePanel covers the one thing a callback can still say about a
// delegation. FocusChild refuses a disabled panel, but the callback runs afterwards and may be the only thing that
// knows the panel is disabled; a row of a control the person cannot reach is no place to report the focus, so the
// delegation is undone and the focus falls back exactly as it does for any other disabled focus panel.
func TestAccessibilityFocusChildUndoneByACallbackDisablingThePanel(t *testing.T) {
	c := check.New(t)
	a := newAXRowWindow(t, "focus child disabled by callback")
	c.NotNil(a.wnd)
	if a.wnd == nil {
		return
	}
	a.focusRows()
	a.screen.Do(func() {
		a.rows.Accessibility.Callback = func(node *accessibility.Node) { node.Disabled = true }
	})
	tree := a.screen.AccessibilityTree(a.wnd)
	c.True(tree != nil)
	if tree == nil {
		return
	}
	row := a.rowNode(tree, 0)
	c.True(row != nil)
	if row == nil {
		return
	}
	c.False(row.Focused, "a row of a disabled control may not report the focus")
	panelNode := a.screen.AccessibilityNodeFor(a.rows)
	c.True(panelNode != nil)
	if panelNode == nil {
		return
	}
	c.True(panelNode.Disabled)
	c.False(panelNode.Focused)
	c.True(tree.Focus != 0 && tree.Focus != row.ID && tree.Focus != panelNode.ID,
		"the focus must fall back to an ancestor that can be shown: %d", tree.Focus)
	c.Equal(0, len(a.screen.Errors()), "nothing should have panicked: %v", a.screen.Errors())
}

// TestAccessibilityFocusChildMovesWithinOneDescription covers a widget that hands the focus on twice while describing
// itself: the delegation moves rather than accumulating, since two focused rows would be the pair the delegation
// exists to avoid.
func TestAccessibilityFocusChildMovesWithinOneDescription(t *testing.T) {
	c := check.New(t)
	a := newAXRowWindow(t, "focus child moved")
	c.NotNil(a.wnd)
	if a.wnd == nil {
		return
	}
	a.focusRows()
	a.screen.Do(func() { a.rows.second = 1 })
	tree := a.screen.AccessibilityTree(a.wnd)
	c.True(tree != nil)
	if tree == nil {
		return
	}
	first, second := a.results()
	c.True(first)
	c.True(second, "handing the focus to another row must be accepted")
	firstRow := a.rowNode(tree, 0)
	secondRow := a.rowNode(tree, 1)
	c.True(firstRow != nil && secondRow != nil)
	if firstRow == nil || secondRow == nil {
		return
	}
	c.False(firstRow.Focused, "the row the focus was handed to first must give it up")
	c.True(secondRow.Focused)
	c.Equal(secondRow.ID, tree.Focus)
	panelNode := a.screen.AccessibilityNodeFor(a.rows)
	c.True(panelNode != nil && !panelNode.Focused)
	c.Equal(0, len(a.screen.Errors()), "nothing should have panicked: %v", a.screen.Errors())
}

// TestAccessibilityFocusChildDisplacedByAnOpenMenu covers what an open menu does to a delegated focus: it displaces it
// exactly as it displaces the focus of a panel that kept it. The keys go to the menu, so the menu is where the person
// is, and a row still claiming the focus would be the second focused object in the window that displacement exists to
// prevent.
func TestAccessibilityFocusChildDisplacedByAnOpenMenu(t *testing.T) {
	c := check.New(t)
	a := newAXRowWindow(t, "focus child menu")
	c.NotNil(a.wnd)
	if a.wnd == nil {
		return
	}
	a.focusRows()
	tree := a.screen.AccessibilityTree(a.wnd)
	c.True(tree != nil)
	if tree == nil {
		return
	}
	row := a.rowNode(tree, 0)
	c.True(row != nil && row.Focused)
	title := axNamed(tree, "Edit")
	c.True(title != nil)
	if title == nil {
		return
	}

	a.screen.Click(axScreenPoint(a.screen, a.wnd, title))
	tree = a.screen.AccessibilityTree(a.wnd)
	c.True(tree != nil)
	if tree == nil {
		return
	}
	row = a.rowNode(tree, 0)
	c.True(row != nil)
	if row == nil {
		return
	}
	c.False(row.Focused, "an open menu displaces a delegated focus as it displaces any other")
	panelNode := a.screen.AccessibilityNodeFor(a.rows)
	c.True(panelNode != nil && !panelNode.Focused)
	c.True(tree.Focus != row.ID, "the focus must be reported within the menu")
	focused := 0
	tree.Walk(func(n *accessibility.Node) bool {
		if n.Focused && n.ID != tree.Root {
			focused++
		}
		return true
	})
	c.Equal(1, focused, "exactly one node other than the root may report the focus: %d", focused)
	c.Equal(0, len(a.screen.Errors()), "nothing should have panicked: %v", a.screen.Errors())
}
