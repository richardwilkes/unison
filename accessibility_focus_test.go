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
// announce that the person had moved onto a divider.
func TestAccessibilityMenuItemFocusIsWithheldWhereItCannotBeUsed(t *testing.T) {
	c := check.New(t)
	const (
		menuID = unison.UserBaseID + iota
		cutID
		pasteID
	)
	var wnd *unison.Window
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
					func(_ unison.MenuItem) bool { return false }, nil))
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

	separators := axNodesWithRole(tree, role.Separator)
	c.Equal(1, len(separators), "the separator in the menu is described as one")
	if len(separators) == 1 {
		c.False(separators[0].Focusable, "a divider is not something a person moves onto")
		c.False(separators[0].Actions.Has(accessibility.Focus))
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
