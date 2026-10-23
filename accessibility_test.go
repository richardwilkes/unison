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
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/enums/role"
)

// These tests cover where a window says the keyboard focus is while a menu is open, what an assistive technology may
// ask of the node it is reported on, and the two promises that go with talking to an assistive technology at all: that
// nothing is spoken once none is being served, and that a widget describing its own children lists each of them once. A
// session owns most of the package's mutable globals while it runs, so none of these may call t.Parallel.

// axFirstWithRole returns the first node in the tree with the given role, in the order an assistive technology walks
// it, or nil if there is none.
func axFirstWithRole(tree *accessibility.Tree, r role.Enum) *accessibility.Node {
	var found *accessibility.Node
	tree.Walk(func(n *accessibility.Node) bool {
		if n.Role == r {
			found = n
			return false
		}
		return true
	})
	return found
}

// axMenuWindow is a window holding a text field, with a menu bar carrying one menu of two items. It is what the menu
// focus tests here drive: something that can hold the keyboard focus, and a menu that takes the keys away from it.
type axMenuWindow struct {
	screen *unison.HeadlessScreen
	wnd    *unison.Window
	field  *unison.Field
}

// newAXMenuWindow starts a session showing an axMenuWindow with the field holding the keyboard focus.
func newAXMenuWindow(t *testing.T, title string) *axMenuWindow {
	t.Helper()
	const (
		menuID = unison.UserBaseID + iota
		cutID
		copyID
	)
	out := &axMenuWindow{}
	out.screen = startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			out.field = unison.NewField()
			out.field.Accessibility.Role = role.TextField
			out.wnd = newHeadlessWindow(t, title, geom.NewRect(10, 10, 400, 300), axColumn(out.field))
			if out.wnd == nil {
				return
			}
			unison.DefaultMenuFactory().BarForWindow(out.wnd, func(bar unison.Menu) {
				f := bar.Factory()
				edit := f.NewMenu(menuID, "Edit", nil)
				edit.InsertItem(-1, f.NewItem(cutID, "Cut", unison.KeyBinding{}, nil, nil))
				edit.InsertItem(-1, f.NewItem(copyID, "Copy", unison.KeyBinding{}, nil, nil))
				bar.InsertMenu(-1, edit)
			})
			out.wnd.ToFront()
		}))
	out.screen.Sync()
	if out.field != nil {
		out.screen.Do(func() { out.field.RequestFocus() })
	}
	return out
}

// openMenu clicks the one title on the menu bar, which opens its menu and leaves the pointer over the title.
func (a *axMenuWindow) openMenu(t *testing.T) *accessibility.Tree {
	t.Helper()
	tree := a.screen.AccessibilityTree(a.wnd)
	if tree == nil {
		t.Fatal("no tree was built for the window")
	}
	title := axNamed(tree, "Edit")
	if title == nil {
		t.Fatal("the menu bar should hold an Edit title")
	}
	a.screen.Click(axScreenPoint(a.screen, a.wnd, title))
	return a.screen.AccessibilityTree(a.wnd)
}

// pointAway moves the pointer to the bottom right corner of the window, which is inside the window and outside both the
// menu bar and anything hanging from it, so the menu stays open with nothing in it highlighted.
func (a *axMenuWindow) pointAway() *accessibility.Tree {
	var pt geom.Point
	a.screen.Do(func() {
		r := a.wnd.ContentRect()
		pt = geom.NewPoint(r.Right()-2, r.Bottom()-2)
	})
	a.screen.MouseMove(pt, mod.None)
	return a.screen.AccessibilityTree(a.wnd)
}

// TestAccessibilityOpenMenuWithNothingHighlightedFocusesTheMenu covers the state the pointer alone produces: a menu
// that is open with nothing in it highlighted, which is what moving the pointer off an open menu without closing it
// leaves behind, since menuItem.mouseExit clears the highlight and nothing restores a last-highlighted item. Every key
// still goes to that menu, so the focus must be reported there rather than on the control that holds the keyboard
// focus underneath — which would have a screen reader announce the text field while the arrow and Return keys it
// offers there actually drove the menu, and would flip the focus back and forth every time the pointer crossed the
// menu.
func TestAccessibilityOpenMenuWithNothingHighlightedFocusesTheMenu(t *testing.T) {
	c := check.New(t)
	a := newAXMenuWindow(t, "menu with nothing highlighted")
	c.NotNil(a.wnd)
	if a.wnd == nil {
		return
	}
	c.True(a.screen.AccessibilityTree(a.wnd) != nil)
	fieldNode := a.screen.AccessibilityNodeFor(a.field)
	c.True(fieldNode != nil)
	if fieldNode == nil {
		return
	}
	c.True(fieldNode.Focused, "with no menu open the field holds the focus")

	tree := a.openMenu(t)
	menuNode := axFirstWithRole(tree, role.Menu)
	c.True(menuNode != nil)
	if menuNode == nil {
		return
	}
	c.Equal("Edit", menuNode.Name)
	item := axNamed(tree, "Cut")
	c.True(item != nil)
	if item == nil {
		return
	}

	// The pointer is still over the title that opened the menu, so that is what is highlighted and what the focus is
	// reported on. Moving it away leaves nothing highlighted anywhere.
	tree = a.pointAway()
	c.Equal(menuNode.ID, tree.Focus, "the open menu itself holds the focus while nothing in it is highlighted")
	c.True(tree.Node(menuNode.ID).Focused)
	c.False(tree.Node(fieldNode.ID).Focused, "the control underneath must not be reported as focused")
	c.False(tree.Node(menuNode.ID).Ignored, "the node the focus is reported on must be one that can be reached")
	var focused []string
	tree.Walk(func(n *accessibility.Node) bool {
		if n.Focused && n.ID != tree.Root {
			focused = append(focused, n.Name)
		}
		return true
	})
	c.Equal(1, len(focused), "exactly one node in the window may report being focused: %v", focused)
	var stillFocused bool
	a.screen.Do(func() { stillFocused = a.field.Is(a.wnd.CurrentFocus()) })
	c.True(stillFocused, "the keyboard focus itself does not move while a menu is open")

	// Moving the pointer in and out of the menu moves the focus between the item and the menu, and never back onto the
	// field the keyboard focus is really on.
	a.screen.MouseMove(axScreenPoint(a.screen, a.wnd, item), mod.None)
	tree = a.screen.AccessibilityTree(a.wnd)
	c.Equal(item.ID, tree.Focus, "the item the pointer is over is what the person is choosing from")
	a.screen.Do(func() { stillFocused = a.field.Is(a.wnd.CurrentFocus()) })
	c.True(stillFocused, "the keyboard focus stays on the field while an item is highlighted")
	tree = a.pointAway()
	c.Equal(menuNode.ID, tree.Focus, "the menu takes the focus back rather than the field")
	c.False(tree.Node(fieldNode.ID).Focused)
	c.Equal(0, len(a.screen.Errors()), "nothing should have panicked: %v", a.screen.Errors())
}

// TestAccessibilityHighlightedMenuItemOffersFocus covers the pair the highlighted item of an open menu is published
// with. It is marked focusable so that a client which checks whether a node can take the focus before trusting that it
// has it is not handed a contradiction, and every adapter refuses to pass a focus request on to a node that does not
// offer the focus action — so the action has to be there too, and asking for it has to move the menu's highlight, which
// is what having the focus in a menu means.
func TestAccessibilityHighlightedMenuItemOffersFocus(t *testing.T) {
	c := check.New(t)
	a := newAXMenuWindow(t, "menu item focus")
	c.NotNil(a.wnd)
	if a.wnd == nil {
		return
	}
	tree := a.openMenu(t)
	cut := axNamed(tree, "Cut")
	copyItem := axNamed(tree, "Copy")
	c.True(cut != nil && copyItem != nil)
	if cut == nil || copyItem == nil {
		return
	}
	a.screen.MouseMove(axScreenPoint(a.screen, a.wnd, cut), mod.None)
	tree = a.screen.AccessibilityTree(a.wnd)
	c.Equal(cut.ID, tree.Focus)
	highlighted := tree.Node(cut.ID)
	c.True(highlighted.Focusable, "the item the focus is reported on says it can take the focus")
	c.True(highlighted.Actions.Has(accessibility.Focus), "and offers the action that moves it there")

	// So does every other item of the open menu, which is what makes moving the highlight something an assistive
	// technology can do at all: each adapter refuses to pass a focus request on to a node that does not offer the
	// action, so an item that advertised it only while already highlighted could be re-focused where it is and never
	// moved to. See axSnapshot.markOpenMenuItems.
	unhighlighted := tree.Node(copyItem.ID)
	c.True(unhighlighted.Focusable, "an item the highlight is not on is still one it can be moved to")
	c.True(unhighlighted.Actions.Has(accessibility.Focus))
	c.False(unhighlighted.Focused, "though only one item of a menu ever reports that it holds the focus")

	// The action the node advertises is carried out rather than refused: the item's panel cannot hold the keyboard
	// focus, so nothing but the menu item itself could act on it.
	c.True(a.screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   cut.ID,
		Action: accessibility.Focus,
	}), "a focus request on the item the focus is already on is carried out")
	tree = a.screen.AccessibilityTree(a.wnd)
	c.Equal(cut.ID, tree.Focus)

	// Asking for the focus on another item moves the highlight there, which is what an arrow key would have done, and
	// the next description reports the focus on it. The request goes through the same gate an adapter applies — the
	// session refuses an action the node does not advertise — so this only reaches the item because the item offers it.
	c.True(a.screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   copyItem.ID,
		Action: accessibility.Focus,
	}))
	tree = a.screen.AccessibilityTree(a.wnd)
	c.Equal(copyItem.ID, tree.Focus, "the highlight, and so the focus, moved to the item that was asked for")
	c.False(tree.Node(cut.ID).Focused, "the item the highlight left must no longer claim the focus")
	c.True(tree.Node(copyItem.ID).Actions.Has(accessibility.Focus))
	c.Equal(0, len(a.screen.Errors()), "nothing should have panicked: %v", a.screen.Errors())
}

// TestAccessibilityMenuItemFocusIsWithheldWhereItCannotBeUsed covers the other half of what an open menu publishes: the
// places the focus must not be offered. A menu that is not open holds nothing a person is choosing from — the pointer
// merely passing over a title on the bar highlights it — a separator is the line drawn between the things that can be
// chosen rather than one of them, and a disabled item is one every request is refused for, so offering to move the
// focus onto any of them would advertise something an assistive technology would then be refused, or would have it
// announce that the person had moved onto a divider. Neither the separator nor the disabled item offers Press, and both
// refuse one: the item's handler must not run even though only its validator, not its panel, disables it.
func TestAccessibilityMenuItemFocusIsWithheldWhereItCannotBeUsed(t *testing.T) {
	c := check.New(t)
	const (
		menuID = unison.UserBaseID + iota
		cutID
		pasteID
	)
	var wnd *unison.Window
	activated := 0
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			wnd = newHeadlessWindow(t, "menu focus", geom.NewRect(10, 10, 400, 300), unison.NewPanel())
			if wnd == nil {
				return
			}
			unison.DefaultMenuFactory().BarForWindow(wnd, func(bar unison.Menu) {
				f := bar.Factory()
				edit := f.NewMenu(menuID, "Edit", nil)
				edit.InsertItem(-1, f.NewItem(cutID, "Cut", unison.KeyBinding{}, nil, nil))
				edit.InsertSeparator(-1, false)
				edit.InsertItem(-1, f.NewItem(pasteID, "Paste", unison.KeyBinding{},
					func(_ unison.MenuItem) bool { return false },
					func(_ unison.MenuItem) { activated++ }))
				bar.InsertMenu(-1, edit)
			})
			wnd.ToFront()
		}))
	c.NotNil(wnd)
	if wnd == nil {
		return
	}
	screen.Sync()

	tree := screen.AccessibilityTree(wnd)
	title := axNamedWithRole(tree, "Edit", role.MenuItem)
	c.True(title != nil, "the bar carries the one menu that was added to it")
	if title == nil {
		return
	}
	c.False(title.Focusable, "a bar with nothing open below it holds nothing a person is choosing from")
	c.False(title.Actions.Has(accessibility.Focus))

	screen.Click(axScreenPoint(screen, wnd, title))
	tree = screen.AccessibilityTree(wnd)

	// The branch that is open is where the person is, so the title the menu hangs from is part of it.
	title = axNamedWithRole(tree, "Edit", role.MenuItem)
	c.True(title != nil && title.Actions.Has(accessibility.Focus),
		"the title an open menu hangs from is something the highlight can be moved back to")

	cut := axNamed(tree, "Cut")
	c.True(cut != nil)
	if cut == nil {
		return
	}
	c.True(cut.Actions.Has(accessibility.Focus), "an item that can be chosen can be moved onto")
	c.True(cut.Actions.Has(accessibility.Press))

	separators := axNodesWithRole(tree, role.Separator)
	c.Equal(1, len(separators), "the separator in the menu is described as one")
	if len(separators) == 1 {
		c.False(separators[0].Focusable, "a divider is not something a person moves onto")
		c.False(separators[0].Actions.Has(accessibility.Focus))
		c.False(separators[0].Actions.Has(accessibility.Press), "there is nothing in a separator to press")
		c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
			Node:   separators[0].ID,
			Action: accessibility.Press,
		}), "a press that did nothing must not be reported as carried out")
	}

	paste := axNamed(tree, "Paste")
	c.True(paste != nil)
	if paste == nil {
		return
	}
	c.True(paste.Disabled, "its validator turned it down")
	c.False(paste.Focusable, "an item that cannot be chosen must not say the focus can be put on it")
	c.False(paste.Actions.Has(accessibility.Focus), "the narrowing of a disabled node's actions takes it away")
	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   paste.ID,
		Action: accessibility.Focus,
	}), "and the request is refused rather than moving the highlight onto something that cannot be used")
	c.False(paste.Actions.Has(accessibility.Press), "a disabled item must not offer to be pressed")
	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   paste.ID,
		Action: accessibility.Press,
	}), "a disabled item must refuse to be pressed")
	screen.Sync()
	var count int
	screen.Do(func() { count = activated })
	c.Equal(0, count, "the item's handler must not have run")

	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   cut.ID,
		Action: accessibility.Focus,
	}), "the item that can be chosen takes the highlight")
	c.Equal(cut.ID, screen.AccessibilityTree(wnd).Focus)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestAccessibilityAnnouncementAfterSupportIsRefusedSaysNothing covers the promise AnnounceForAccessibility makes: it
// does nothing when no assistive technology is being served. An announcement made from another goroutine is carried out
// on the UI thread, so one made a moment before SetAccessibilityEnabled(false) arrives after everything serving the
// announcement has been torn down.
func TestAccessibilityAnnouncementAfterSupportIsRefusedSaysNothing(t *testing.T) {
	c := check.New(t)
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			wnd = newHeadlessWindow(t, "announce", geom.NewRect(20, 20, 240, 120), unison.NewPanel())
		}))
	c.NotNil(wnd)
	// The refusal is a package-wide flag that outlives the session, so it is lifted however this test ends. Registered
	// after the session's own cleanup, which means it runs before it, while there is still a UI thread to run on.
	t.Cleanup(func() {
		unison.SetAccessibilityEnabled(true)
		screen.Sync()
	})
	screen.EnableAccessibility()
	c.True(unison.IsAccessibilityActive())

	unison.AnnounceForAccessibility("saved")
	screen.Sync()
	c.Equal([]string{"saved"}, screen.Announcements(), "an announcement made while support is served is spoken")

	// The UI thread is parked so that the announcement is queued behind a task that refuses support, which is the race
	// this is about: the announcement is made while support is still being served and reaches the UI thread after it is
	// gone. The ordering is fixed by the channels rather than by anything about the clock.
	parked := make(chan struct{})
	release := make(chan struct{})
	c.True(screen.Post(func() {
		close(parked)
		<-release
		unison.SetAccessibilityEnabled(false)
	}))
	<-parked
	unison.AnnounceForAccessibility("too late")
	close(release)
	screen.Sync()
	c.False(unison.IsAccessibilityActive())
	c.Equal(0, len(screen.Announcements()), "nothing may be spoken once support has been refused")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// axTwiceDescribedHeading is a piece of static text that asks for its children to be described twice, which a widget
// built out of parts that each describe what they hold really can do. See AccessibilityBuilder.DescribeChildren.
type axTwiceDescribedHeading struct {
	unison.Panel
}

// newAXTwiceDescribedHeading creates a heading laid out in a single column.
func newAXTwiceDescribedHeading() *axTwiceDescribedHeading {
	h := &axTwiceDescribedHeading{}
	h.Self = h
	h.SetLayout(&unison.FlexLayout{Columns: 1})
	h.Accessibility.Role = role.Heading
	return h
}

// ProvideAccessibility asks twice, which must describe the children exactly once.
func (h *axTwiceDescribedHeading) ProvideAccessibility(b *unison.AccessibilityBuilder) {
	b.DescribeChildren()
	b.DescribeChildren()
}

// TestAccessibilityDescribeChildrenIsIdempotent covers what a second call to AccessibilityBuilder.DescribeChildren
// within one description must not do: append every child's id to the parent a second time. A parent listing each of its
// children twice makes every index an assistive technology counts out of that list wrong, along with the position in
// set each child reports, which is the failure AddVirtualChildOf refuses a repeated key for.
func TestAccessibilityDescribeChildrenIsIdempotent(t *testing.T) {
	c := check.New(t)
	var heading *axTwiceDescribedHeading
	var button *unison.Button
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			heading = newAXTwiceDescribedHeading()
			label := unison.NewLabel()
			label.SetTitle("Section")
			button = unison.NewButton()
			button.SetTitle("Reachable")
			heading.AddChild(label)
			heading.AddChild(button)
			wnd = newHeadlessWindow(t, "described twice", geom.NewRect(10, 10, 400, 300), axColumn(heading))
		}))
	c.NotNil(wnd)
	if wnd == nil {
		return
	}
	tree := screen.AccessibilityTree(wnd)
	c.True(tree != nil)
	if tree == nil {
		return
	}
	node := screen.AccessibilityNodeFor(heading)
	c.True(node != nil)
	if node == nil {
		return
	}
	c.Equal(role.Heading, node.Role)
	c.Equal(2, len(node.Children), "each child must be listed exactly once: %v", node.Children)
	seen := make(map[accessibility.NodeID]int)
	for _, id := range node.Children {
		seen[id]++
	}
	for id, count := range seen {
		c.Equal(1, count, "child %d is listed %d times", id, count)
	}
	buttonNode := screen.AccessibilityNodeFor(button)
	c.True(buttonNode != nil)
	if buttonNode == nil {
		return
	}
	pos, size := tree.PositionInSet(buttonNode.ID)
	c.Equal(1, pos)
	c.Equal(1, size, "the button is the only one of its role among the heading's children")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

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

// axRowWindow is a window holding an axRowPanel and another panel that can take the focus away from it.
type axRowWindow struct {
	screen *unison.HeadlessScreen
	wnd    *unison.Window
	rows   *axRowPanel
	other  *unison.Panel
}

// newAXRowWindow starts a session showing an axRowWindow, with nothing holding the keyboard focus yet.
func newAXRowWindow(t *testing.T, title string) *axRowWindow {
	t.Helper()
	out := &axRowWindow{}
	out.screen = startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			out.rows = newAXRowPanel("Alpha", "Beta", "Gamma")
			out.other = axFocusablePanel("Other")
			out.wnd = newHeadlessWindow(t, title, geom.NewRect(10, 10, 400, 300), axColumn(out.rows, out.other))
			if out.wnd != nil {
				out.wnd.ToFront()
			}
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
// reader follows focus events and hears nothing from a selection change on a container. Before the panel holds the
// focus it has none to hand on, and a tree that said otherwise would point an assistive technology at a row of a
// control the person is not in.
func TestAccessibilityFocusChildReportsTheFocusOnTheRow(t *testing.T) {
	c := check.New(t)
	a := newAXRowWindow(t, "focus child")
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
	otherNode := axMustNode(c, a.screen.AccessibilityNodeFor(a.other))
	c.Equal(otherNode.ID, tree.Focus, "the focus stays where it actually is")
	a.screen.AccessibilityEvents(a.wnd)

	a.focusRows()
	tree = a.screen.AccessibilityTree(a.wnd)
	c.True(tree != nil)
	if tree == nil {
		return
	}
	first, _ = a.results()
	c.True(first, "the panel holds the focus, so handing it to a row must be accepted")
	row = a.rowNode(tree, 0)
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

// TestAccessibilityFocusChildRefusedWhenDisabled covers what a panel holding the focus may not hand it to. A disabled
// row, or any row of a disabled panel — which the window may still leave the keyboard focus on — would offer an
// assistive technology a place the person cannot be. An id belonging to another panel of the window, or to no node at
// all, as one kept from an earlier description does, is not this panel's to point at.
func TestAccessibilityFocusChildRefusedWhenDisabled(t *testing.T) {
	for _, one := range []struct {
		// set changes one thing about the row panel, which holds the focus; other is the node id of the other panel.
		set  func(rows *axRowPanel, other accessibility.NodeID)
		name string
		why  string
		// panelDisabled is set when the panel itself is disabled, which takes the focus past it to an ancestor.
		panelDisabled bool
		// otherFirst describes the other panel ahead of the row panel, so that its node is in the tree by the time
		// it is refused and what refuses it is that it is not beneath the row panel.
		otherFirst bool
	}{
		{
			name: "disabled row",
			why:  "a disabled row is no use as the place the person is said to be",
			set:  func(rows *axRowPanel, _ accessibility.NodeID) { rows.disabledRows = true },
		},
		{
			name:          "disabled panel",
			why:           "a disabled panel may not hand on the focus the window left it holding",
			set:           func(rows *axRowPanel, _ accessibility.NodeID) { rows.SetEnabled(false) },
			panelDisabled: true,
		},
		{
			name:       "node of another panel",
			why:        "a node that is not beneath the panel is not the panel's to hand the focus to",
			set:        func(rows *axRowPanel, other accessibility.NodeID) { rows.target = other },
			otherFirst: true,
		},
		{
			name: "node not in the tree",
			why:  "a node that is not in the tree cannot be handed the focus",
			set:  func(rows *axRowPanel, _ accessibility.NodeID) { rows.target = accessibility.NodeID(1 << 40) },
		},
	} {
		t.Run(one.name, func(t *testing.T) {
			c := check.New(t)
			a := newAXRowWindow(t, "focus child refused")
			c.NotNil(a.wnd)
			if a.wnd == nil {
				return
			}
			if one.otherFirst {
				a.screen.Do(func() {
					content := a.rows.Parent()
					content.RemoveChild(a.other)
					content.AddChildAtIndex(a.other, 0)
				})
			}
			a.focusRows()
			c.True(a.screen.AccessibilityTree(a.wnd) != nil)
			otherNode := axMustNode(c, a.screen.AccessibilityNodeFor(a.other))
			a.screen.Do(func() { one.set(a.rows, otherNode.ID) })
			tree := a.screen.AccessibilityTree(a.wnd)
			c.True(tree != nil)
			if tree == nil {
				return
			}
			first, _ := a.results()
			c.False(first, one.why)
			row := axMustNode(c, a.rowNode(tree, 0))
			c.False(row.Focused, "the row must not report a focus it was refused")
			c.False(axMustNode(c, tree.Node(otherNode.ID)).Focused)
			panelNode := axMustNode(c, a.screen.AccessibilityNodeFor(a.rows))
			if one.panelDisabled {
				c.True(panelNode.Disabled)
				c.False(panelNode.Focused, "a disabled node is published without the focus")
				c.True(tree.Focus != 0 && tree.Focus != row.ID && tree.Focus != panelNode.ID,
					"the focus must fall back to an ancestor that can be shown")
			} else {
				c.True(panelNode.Focused, "the refusal must leave the panel reporting the focus it really holds")
				c.Equal(panelNode.ID, tree.Focus)
			}
			c.Equal(0, len(a.screen.Errors()), "nothing should have panicked: %v", a.screen.Errors())
		})
	}
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
