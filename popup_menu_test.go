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
	"runtime"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/accessibility"
	checkenum "github.com/richardwilkes/unison/enums/check"
	"github.com/richardwilkes/unison/enums/role"
)

func newTestPopup(items ...string) *unison.PopupMenu[string] {
	p := unison.NewPopupMenu[string]()
	p.AddItem(items...)
	return p
}

func TestPopupAddItemsAndCount(t *testing.T) {
	c := check.New(t)
	p := newTestPopup("a", "b")
	p.AddSeparator()
	p.AddDisabledItem("c")
	c.Equal(4, p.ItemCount())

	item, ok := p.ItemAt(0)
	c.True(ok)
	c.Equal("a", item)
	c.True(p.ItemEnabledAt(1))
	c.False(p.ItemEnabledAt(3)) // disabled item

	// A separator slot reports no item.
	_, ok = p.ItemAt(2)
	c.False(ok)
}

func TestPopupIndexOfItem(t *testing.T) {
	c := check.New(t)
	p := newTestPopup("a", "b", "c")
	c.Equal(1, p.IndexOfItem("b"))
	c.Equal(-1, p.IndexOfItem("missing"))
}

func TestPopupSelectByValue(t *testing.T) {
	c := check.New(t)
	p := newTestPopup("a", "b", "c")
	p.Select("b")
	c.Equal(1, p.SelectedIndex())
	item, ok := p.Selected()
	c.True(ok)
	c.Equal("b", item)
	c.Equal("b", p.Text())
}

func TestPopupSelectIndexReplaces(t *testing.T) {
	c := check.New(t)
	p := newTestPopup("a", "b", "c")
	p.SelectIndex(0)
	c.Equal([]int{0}, p.SelectedIndexes())
	p.SelectIndex(2)
	c.Equal([]int{2}, p.SelectedIndexes())
}

func TestPopupSelectMultipleShowsMultiple(t *testing.T) {
	c := check.New(t)
	p := newTestPopup("a", "b", "c")
	p.SelectIndex(0, 2)
	c.Equal([]int{0, 2}, p.SelectedIndexes())
	c.Equal("Multiple", p.Text())
}

func TestPopupSelectIgnoresSeparator(t *testing.T) {
	c := check.New(t)
	p := newTestPopup("a")
	p.AddSeparator()
	p.AddItem("c")
	// Selecting the separator index has no effect.
	p.SelectIndex(1)
	c.Equal(-1, p.SelectedIndex())
	c.Equal("", p.Text())
}

func TestPopupSelectionChangedCallback(t *testing.T) {
	c := check.New(t)
	p := newTestPopup("a", "b", "c")
	calls := 0
	p.SelectionChangedCallback = func(_ *unison.PopupMenu[string]) { calls++ }
	p.SelectIndex(1)
	c.Equal(1, calls)
	// Selecting the same index again does not fire the callback.
	p.SelectIndex(1)
	c.Equal(1, calls)
	p.SelectIndex(2)
	c.Equal(2, calls)
}

func TestPopupRemoveItemAtShiftsSelection(t *testing.T) {
	c := check.New(t)
	p := newTestPopup("a", "b", "c", "d")
	p.SelectIndex(3)  // select "d"
	p.RemoveItemAt(1) // remove "b"
	c.Equal(3, p.ItemCount())
	// "d" slid down from index 3 to index 2 and stays selected.
	c.Equal([]int{2}, p.SelectedIndexes())
	item, ok := p.Selected()
	c.True(ok)
	c.Equal("d", item)
}

func TestPopupRemoveItemByValue(t *testing.T) {
	c := check.New(t)
	p := newTestPopup("a", "b", "c")
	p.SelectIndex(2)
	p.RemoveItem("a")
	c.Equal(2, p.ItemCount())
	c.Equal("c", p.Text())
}

func TestPopupRemoveMissingItemLeavesSelectionAlone(t *testing.T) {
	c := check.New(t)
	p := newTestPopup("a", "b", "c")
	p.SelectIndex(2)
	p.RemoveItem("missing")
	c.Equal(3, p.ItemCount())
	c.Equal([]int{2}, p.SelectedIndexes())
	item, ok := p.Selected()
	c.True(ok)
	c.Equal("c", item)
}

func TestPopupSetItemEnabledAndReplace(t *testing.T) {
	c := check.New(t)
	p := newTestPopup("a", "b")
	p.SetItemEnabledAt(0, false)
	c.False(p.ItemEnabledAt(0))
	p.SetItemAt(1, "B", true)
	item, ok := p.ItemAt(1)
	c.True(ok)
	c.Equal("B", item)
}

// TestPopupSetItemAtUpdatesEnabledForUnchangedItem verifies that SetItemAt applies a new enabled state even when the
// item value itself is unchanged. The update used to be skipped entirely whenever the value matched, silently ignoring
// the enabled argument.
func TestPopupSetItemAtUpdatesEnabledForUnchangedItem(t *testing.T) {
	c := check.New(t)
	p := newTestPopup("a", "b")
	c.True(p.ItemEnabledAt(1))

	p.SetItemAt(1, "b", false)
	c.False(p.ItemEnabledAt(1))
	item, ok := p.ItemAt(1)
	c.True(ok)
	c.Equal("b", item)

	p.SetItemAt(1, "b", true)
	c.True(p.ItemEnabledAt(1))

	// Replacing a separator with an item continues to work, including when it starts out disabled.
	p.AddSeparator()
	p.SetItemAt(2, "c", false)
	item, ok = p.ItemAt(2)
	c.True(ok)
	c.Equal("c", item)
	c.False(p.ItemEnabledAt(2))
}

func TestPopupRemoveAllItems(t *testing.T) {
	c := check.New(t)
	p := newTestPopup("a", "b", "c")
	p.SelectIndex(1)
	p.RemoveAllItems()
	c.Equal(0, p.ItemCount())
	c.Equal(-1, p.SelectedIndex())
}

// TestPopupMenuAccessibilityReportsWhetherItIsOpen verifies that a popup menu says whether its choices are showing and
// can be asked to put them away again. It always claimed to be collapsed, whatever was on the screen, so UI
// Automation's ExpandCollapseState and AT-SPI's STATE_EXPANDED never moved and there was no collapse to ask for.
//
// What this covers is the in-window menu, and only that. A headless session substitutes an in-window menu factory for
// whatever the platform would otherwise hand out, so every test here — and every other one that opens a menu from a
// widget — exercises that path whatever it is running on. On macOS an application that has not asked for in-window
// menus gets a native menu instead, and none of what is asserted below holds for one: axMenuIsOpen can only answer for
// a menu that is part of the window's panel tree, so such a popup describes itself as collapsed however many choices
// are on the screen, Expanded never becomes true, and Collapse reports that it was carried out while taking nothing
// down. That is a real gap rather than a testing artifact, though a narrow one: the platform describes its own menus to
// VoiceOver, so what is lost is the tie between the popup and the choices it opened rather than the choices themselves.
// The assertion below is what makes the substitution visible — the factory in use says its menu bar is per-window,
// which the macOS one never does.
func TestPopupMenuAccessibilityReportsWhetherItIsOpen(t *testing.T) {
	c := check.New(t)
	var popup *unison.PopupMenu[string]
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			popup = unison.NewPopupMenu[string]()
			popup.AddItem("Light", "Regular", "Bold")
			popup.Select("Regular")
			wnd = newHeadlessWindow(t, "popup state", geom.NewRect(10, 10, 300, 150), axColumn(popup))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() { wnd.ToFront() }))

	if runtime.GOOS == "darwin" {
		var perWindowBar bool
		screen.Do(func() { perWindowBar = popup.MenuFactory.BarIsPerWindow() })
		c.True(perWindowBar, "the session substituted its own in-window menu factory: the *macMenu the real one hands "+
			"out on macOS is never described as expanded, since axMenuIsOpen can only answer for an in-window menu, "+
			"and its Collapse reports success while taking nothing down — none of which anything here can reach")
	}

	screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(popup)
	c.True(node != nil)
	if node == nil {
		return
	}
	c.True(node.Expandable)
	c.False(node.Expanded, "nothing is showing yet")
	c.True(node.Actions.Has(accessibility.Expand))
	c.True(node.Actions.Has(accessibility.Collapse))

	before := axRootChildCount(screen.AccessibilityTree(wnd))
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.Expand,
	}))
	c.Equal(before+1, axRootChildCount(screen.AccessibilityTree(wnd)),
		"expanding the popup menu should have opened its menu within the window")
	node = screen.AccessibilityNodeFor(popup)
	c.True(node != nil)
	if node == nil {
		return
	}
	c.True(node.Expanded, "the popup must say its choices are showing while they are")

	// Asking again for what is already there changes nothing, rather than tearing the menu down and building it back.
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.Expand,
	}))
	c.Equal(before+1, axRootChildCount(screen.AccessibilityTree(wnd)))

	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.Collapse,
	}))
	c.Equal(before, axRootChildCount(screen.AccessibilityTree(wnd)),
		"collapsing the popup menu should have taken its menu away")
	node = screen.AccessibilityNodeFor(popup)
	c.True(node != nil)
	if node != nil {
		c.False(node.Expanded)
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestPopupMenuAccessibilityMarksEveryChoice verifies that every choice in an opened popup menu is described as one of
// a set of check items, with the selection checked and the rest explicitly not. Only the selected item used to be given
// a check state at all, and an item that has never been given one is described as a plain command, so a screen-reader
// user heard "checked" for the choice the popup was showing and nothing whatsoever for its alternatives — no "not
// checked", and nothing saying that the items were one set of exclusive choices.
func TestPopupMenuAccessibilityMarksEveryChoice(t *testing.T) {
	c := check.New(t)
	var popup *unison.PopupMenu[string]
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			popup = unison.NewPopupMenu[string]()
			popup.AddItem("Light", "Regular")
			popup.AddSeparator()
			popup.AddItem("Bold")
			popup.Select("Regular")
			wnd = newHeadlessWindow(t, "popup checks", geom.NewRect(10, 10, 300, 150), axColumn(popup))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() { wnd.ToFront() }))

	screen.AccessibilityTree(wnd)
	node := axMustNode(c, screen.AccessibilityNodeFor(popup))
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.Expand,
	}))

	items := axNodesWithRole(screen.AccessibilityTree(wnd), role.MenuItem)
	c.Equal(3, len(items), "each choice is a menu item, and the separator between them is not one")
	checked := 0
	for _, item := range items {
		c.True(item.HasCheck, "%q has to be described as one of a set of choices rather than as a plain command",
			item.Name)
		switch item.Checked {
		case checkenum.On:
			checked++
			c.Equal("Regular", item.Name, "the checked choice is the one the popup is showing")
		default:
			c.Equal(checkenum.Off, item.Checked, "%q is a choice that has not been made rather than a half-made one",
				item.Name)
		}
	}
	c.Equal(1, checked, "exactly one of the choices is checked")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestPopupMenuAccessibilityExpandChecksAgainWhenItRuns verifies that a popup disabled between the moment an assistive
// technology asked it to expand and the moment the queued click came to run does not open its menu. The dispatcher
// gates every request on Panel.Enabled, but the work happens a task later, and a click could not open the menu of a
// disabled popup: Window.mouseDown passes straight over a panel that is not enabled.
func TestPopupMenuAccessibilityExpandChecksAgainWhenItRuns(t *testing.T) {
	c := check.New(t)
	var popup *unison.PopupMenu[string]
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			popup = unison.NewPopupMenu[string]()
			popup.AddItem("Light", "Regular", "Bold")
			popup.Select("Regular")
			wnd = newHeadlessWindow(t, "popup disabled", geom.NewRect(10, 10, 300, 150), axColumn(popup))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() { wnd.ToFront() }))

	before := axRootChildCount(screen.AccessibilityTree(wnd))
	// Both happen within one turn of the UI thread, so the popup is disabled by the time the queued click runs. Asking
	// the widget directly is what makes that possible: a request routed through the window would have been refused
	// outright, which is the very check this one has to survive to reach.
	c.True(screen.Do(func() {
		c.True(popup.PerformAccessibilityAction(accessibility.ActionRequest{Action: accessibility.Expand}))
		popup.SetEnabled(false)
	}))
	c.Equal(before, axRootChildCount(screen.AccessibilityTree(wnd)),
		"a popup disabled before the queued click ran must not have opened its menu")
	node := axMustNode(c, screen.AccessibilityNodeFor(popup))
	c.False(node.Expanded)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestPopupMenuAccessibilityRefusesWhenItsWindowIsNotActive verifies that a popup in a background window refuses to be
// pressed or expanded. The menu is not built in the popup's own window: menu.createPopup inserts it into
// ActiveWindow(), so carrying the request out would have put this popup's choices up in whatever window was frontmost,
// at coordinates translated from this one. The mouse path never could, since Window.mouseDown delivers nothing to a
// window that does not have the focus.
func TestPopupMenuAccessibilityRefusesWhenItsWindowIsNotActive(t *testing.T) {
	c := check.New(t)
	var popup *unison.PopupMenu[string]
	var background, front *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 500},
		unison.StartupFinishedCallback(func() {
			popup = unison.NewPopupMenu[string]()
			popup.AddItem("Light", "Regular", "Bold")
			popup.Select("Regular")
			background = newHeadlessWindow(t, "background", geom.NewRect(10, 10, 250, 150), axColumn(popup))
			front = newHeadlessWindow(t, "front", geom.NewRect(300, 10, 250, 150), axColumn(unison.NewButton()))
		}))
	c.NotNil(background)
	c.NotNil(front)

	// The popup's own window is the active one to begin with, so what follows is about the window it is in rather than
	// about the popup.
	c.True(screen.Do(func() { background.ToFront() }))
	screen.AccessibilityTree(background)
	node := axMustNode(c, screen.AccessibilityNodeFor(popup))
	c.True(node.Actions.Has(accessibility.Expand))

	c.True(screen.Do(func() { front.ToFront() }))
	var focused bool
	screen.Do(func() { focused = background.Focused() })
	c.False(focused, "the other window should have taken the focus")

	before := axRootChildCount(screen.AccessibilityTree(background))

	// What is refused is not advertised either. A node that went on offering an expansion that silently did nothing
	// would leave a screen reader saying the popup can be opened while nothing came of asking, so the actions go with
	// the window's activation: Window.lostFocus marks the window for publishing, and the description that follows is
	// one without them. See axSnapshot.narrowMenuActions.
	backgrounded := axMustNode(c, screen.AccessibilityNodeFor(popup))
	c.True(backgrounded.Expandable, "the popup still holds choices; what it has lost is anywhere to show them")
	c.False(backgrounded.Actions.Has(accessibility.Expand),
		"a popup that would refuse to expand must not offer to")
	c.False(backgrounded.Actions.Has(accessibility.Press), "nor the press that is the same thing by another name")
	c.True(backgrounded.Actions.Has(accessibility.Collapse),
		"collapsing takes down a menu that is showing rather than opening one, so it is still worth offering")

	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.Expand,
	}), "expanding a popup in a window that is not the active one has to be refused")
	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.Press,
	}), "so does pressing it, which is the same thing by another name")
	c.Equal(before, axRootChildCount(screen.AccessibilityTree(background)),
		"nothing should have been opened in the background window")

	// Bringing the window back to the front brings them back with it, since Window.gainedFocus marks it for publishing
	// just as losing the focus did.
	c.True(screen.Do(func() { background.ToFront() }))
	screen.AccessibilityTree(background)
	restored := axMustNode(c, screen.AccessibilityNodeFor(popup))
	c.True(restored.Actions.Has(accessibility.Expand), "the active window's popup offers its choices again")
	c.True(restored.Actions.Has(accessibility.Press))
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.Expand,
	}), "and carries the request out")
	c.Equal(before+1, axRootChildCount(screen.AccessibilityTree(background)),
		"the choices should have been shown in the popup's own window")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestPopupMenuAccessibilityInATransientWindowOffersNoMenu verifies that a popup in a transient window neither offers
// nor carries out the actions that would open its choices, however firmly that window holds the focus.
//
// A transient window is one that takes input without ever becoming the active one — a menu or a tooltip window — and
// ActiveWindow() passes over it and answers with the next window that is not. menu.createPopup builds the popup inside
// the root of whatever that answer is, so this popup's choices would appear in some other window entirely, at
// coordinates translated from this one, or nowhere at all when there is no other window to put them in. That is a
// different question from the one Window.mouseDown asks when it lets a press through to a transient window, which is
// merely whether the window may take input at all. See axMayPopupMenu.
func TestPopupMenuAccessibilityInATransientWindowOffersNoMenu(t *testing.T) {
	c := check.New(t)
	var popup *unison.PopupMenu[string]
	var transient *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 500},
		unison.StartupFinishedCallback(func() {
			popup = unison.NewPopupMenu[string]()
			popup.AddItem("Light", "Regular", "Bold")
			popup.Select("Regular")
			w, err := unison.NewWindow("transient", unison.TransientWindowOption())
			if err != nil {
				t.Errorf("unable to create transient window: %v", err)
				return
			}
			content := w.Content()
			content.SetLayout(&unison.FlexLayout{Columns: 1})
			content.AddChild(axColumn(popup))
			w.SetContentRect(geom.NewRect(10, 10, 250, 150))
			w.Show()
			w.ToFront()
			transient = w
		}))
	c.NotNil(transient)
	if transient == nil {
		return
	}

	var focused bool
	var active *unison.Window
	screen.Do(func() {
		focused = transient.Focused()
		active = unison.ActiveWindow()
	})
	c.True(focused, "the window holds the focus, which is what makes this about where a menu would land")
	c.True(active != transient, "and is still not the window a menu would be built in")

	screen.AccessibilityTree(transient)
	node := axMustNode(c, screen.AccessibilityNodeFor(popup))
	c.True(node.Expandable, "the popup holds choices; what it has is nowhere to show them")
	c.False(node.Actions.Has(accessibility.Expand), "a popup that cannot open its menu must not offer to")
	c.False(node.Actions.Has(accessibility.Press), "nor the press that is the same thing by another name")

	before := axRootChildCount(screen.AccessibilityTree(transient))
	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.Expand,
	}), "expanding a popup in a transient window has to be refused")
	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.Press,
	}), "so does pressing it")
	c.Equal(before, axRootChildCount(screen.AccessibilityTree(transient)),
		"nothing should have been opened in the transient window")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
