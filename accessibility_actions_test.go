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
	"sync"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/enums/role"
)

// These tests cover the gates every request from an assistive technology passes through before it reaches a widget:
// what a disabled control refuses, what a window blocked by a modal refuses, and what a request is allowed to report it
// achieved. A session owns most of the package's mutable globals while it runs, so none of these may call t.Parallel.

// axClickable returns a panel that records the clicks it is given, which is what an assistive technology's Press
// synthesizes, along with a counter of how many times it has been pressed and of how many requests its
// Accessibility.ActionCallback was consulted for.
func axClickable(presses, consulted *int) *unison.Panel {
	panel := unison.NewPanel()
	panel.SetSizer(func(_ geom.Size) (minSize, prefSize, maxSize geom.Size) {
		size := geom.NewSize(80, 24)
		return size, size, size
	})
	panel.SetFocusable(true)
	panel.Accessibility.Name = "Clickable"
	panel.Accessibility.Role = role.Button
	panel.Accessibility.ActionCallback = func(_ accessibility.ActionRequest) bool {
		*consulted++
		return false
	}
	panel.MouseDownCallback = func(_ geom.Point, _, _ int, _ mod.Modifiers) bool { return true }
	panel.MouseUpCallback = func(_ geom.Point, _ int, _ mod.Modifiers) bool {
		*presses++
		return true
	}
	return panel
}

// TestAccessibilityDisabledPanelRefusesActions verifies that a disabled control is as untouchable through an assistive
// technology as it is with the mouse and the keyboard: it advertises nothing that would act on it, and everything but
// scrolling it into view is refused before any of the panel's callbacks can run.
func TestAccessibilityDisabledPanelRefusesActions(t *testing.T) {
	c := check.New(t)
	var presses, consulted int
	var panel *unison.Panel
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			panel = axClickable(&presses, &consulted)
			wnd = newHeadlessWindow(t, "disabled", geom.NewRect(20, 20, 240, 120), axColumn(panel))
		}))
	c.NotNil(wnd)

	// While it is enabled it behaves as any other clickable panel does, so that what follows is about the enabled state
	// and nothing else.
	screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(panel)
	c.True(node != nil)
	if node == nil {
		return
	}
	c.False(node.Disabled)
	c.True(node.Actions.Has(accessibility.Press))
	c.True(node.Actions.Has(accessibility.Focus))
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.Press,
	}))
	var count int
	screen.Do(func() { count = presses })
	c.Equal(1, count, "pressing an enabled panel should have clicked it")

	// The press moved the focus onto the panel, as a person's click would have. It is taken away again so that the
	// focus a refused request must not produce cannot be one the panel already had.
	screen.Do(func() {
		panel.SetEnabled(false)
		wnd.SetFocus(nil)
	})
	screen.AccessibilityTree(wnd)
	node = screen.AccessibilityNodeFor(panel)
	c.True(node != nil)
	if node == nil {
		return
	}
	c.True(node.Disabled, "a panel that has been disabled says so")
	c.False(node.Actions.Has(accessibility.Press), "a disabled panel must not offer to be pressed")
	c.False(node.Actions.Has(accessibility.Focus), "a disabled panel cannot take the focus")
	c.True(node.Actions.Has(accessibility.ScrollIntoView),
		"scrolling a disabled panel into view acts on its ancestors, so it remains available")

	var consultedBefore int
	screen.Do(func() { consultedBefore = consulted })
	for _, action := range []accessibility.Action{
		accessibility.Press, accessibility.Focus, accessibility.Toggle,
		accessibility.SetValue,
	} {
		c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
			Node:   node.ID,
			Action: action,
		}), "a disabled panel must refuse %v", action)
	}
	var consultedAfter int
	screen.Do(func() {
		count = presses
		consultedAfter = consulted
	})
	c.Equal(1, count, "a disabled panel must not have been clicked")
	c.Equal(consultedBefore, consultedAfter,
		"the panel's own action callback must not even be consulted while it is disabled")
	var focused bool
	screen.Do(func() { focused = panel.Is(wnd.CurrentFocus()) })
	c.False(focused, "a disabled panel must not have taken the focus")

	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.ScrollIntoView,
	}), "a disabled panel can still be scrolled into view")

	// Enabling it again puts everything back, so the refusal is of the state rather than of the panel.
	screen.Do(func() { panel.SetEnabled(true) })
	screen.AccessibilityTree(wnd)
	node = screen.AccessibilityNodeFor(panel)
	c.True(node != nil)
	if node != nil {
		c.True(node.Actions.Has(accessibility.Press))
		c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
			Node:   node.ID,
			Action: accessibility.Press,
		}))
	}
	screen.Do(func() { count = presses })
	c.Equal(2, count, "an enabled panel should be pressable again")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestAccessibilityFocusActionReportsWhatHappened verifies that a request to move the focus is reported as carried out
// only when the node it named actually took it. Window.SetFocus redirects a target that cannot hold the focus to its
// first focusable child, or clears the focus entirely, either of which would leave Tree.Focus naming something other
// than what was asked for.
func TestAccessibilityFocusActionReportsWhatHappened(t *testing.T) {
	c := check.New(t)
	var group *unison.Panel
	var field *unison.Field
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			field = unison.NewField()
			field.Accessibility.Role = role.TextField
			group = axColumn(field)
			group.Accessibility.Name = "Group"
			wnd = newHeadlessWindow(t, "focus", geom.NewRect(20, 20, 240, 120), group)
		}))
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	c.True(tree != nil)
	groupNode := axNamed(tree, "Group")
	c.True(groupNode != nil)
	if groupNode == nil {
		return
	}
	c.False(groupNode.Actions.Has(accessibility.Focus), "a panel that cannot hold the focus does not offer to take it")
	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   groupNode.ID,
		Action: accessibility.Focus,
	}), "a panel that cannot hold the focus must not report that it took it")
	var holder *unison.Panel
	screen.Do(func() { holder = wnd.CurrentFocus() })
	c.True(holder == nil || !holder.Is(field.AsPanel()),
		"the focus must not have been handed to a child the request did not name")

	fieldNode := screen.AccessibilityNodeFor(field)
	c.True(fieldNode != nil)
	if fieldNode == nil {
		return
	}
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   fieldNode.ID,
		Action: accessibility.Focus,
	}), "a focusable panel takes the focus and says so")
	var focused bool
	screen.Do(func() { focused = field.Is(wnd.CurrentFocus()) })
	c.True(focused)
	tree = screen.AccessibilityTree(wnd)
	c.Equal(fieldNode.ID, tree.Focus, "the tree should name the panel the request moved the focus to")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestAccessibilityOpenMenuLeavesOneFocusedNode verifies that while a menu is open the item it is pointing at is the
// only focused node in the window. The keyboard focus stays where it was, since the menu handles keys ahead of it, but
// a tree with two focused nodes is one an assistive technology cannot make sense of: AT-SPI maps Node.Focused straight
// onto ATSPI_STATE_FOCUSED, so Orca would find two focused objects in the same window.
func TestAccessibilityOpenMenuLeavesOneFocusedNode(t *testing.T) {
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
			field.Accessibility.Role = role.TextField
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
	screen.Do(func() { field.RequestFocus() })

	tree := screen.AccessibilityTree(wnd)
	fieldNode := screen.AccessibilityNodeFor(field)
	c.True(fieldNode != nil)
	if fieldNode == nil {
		return
	}
	c.True(fieldNode.Focused, "with no menu open the field holds the focus")
	c.Equal(fieldNode.ID, tree.Focus)

	title := axNamed(tree, "Edit")
	c.True(title != nil)
	if title == nil {
		return
	}
	screen.Click(axScreenPoint(screen, wnd, title))
	tree = screen.AccessibilityTree(wnd)
	item := axNamed(tree, "Cut")
	c.True(item != nil)
	if item == nil {
		return
	}
	screen.MouseMove(axScreenPoint(screen, wnd, item), mod.None)
	tree = screen.AccessibilityTree(wnd)
	c.Equal(item.ID, tree.Focus, "the item the menu is pointing at is the focused one")
	var focused []string
	tree.Walk(func(n *accessibility.Node) bool {
		if n.Focused && n.ID != tree.Root {
			focused = append(focused, n.Name)
		}
		return true
	})
	c.Equal(1, len(focused), "exactly one node in the window may report being focused: %v", focused)
	var stillFocused bool
	screen.Do(func() { stillFocused = field.Is(wnd.CurrentFocus()) })
	c.True(stillFocused, "the keyboard focus itself does not move while a menu is open")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestAccessibilityDisabledMenuItemRefusesPress verifies that the gating covers what a widget says about itself rather
// than only what a panel's enabled state says: a menu item whose validator refuses it is described as disabled even
// though the panel drawing it is perfectly enabled, so it must neither offer to be pressed nor run its handler.
func TestAccessibilityDisabledMenuItemRefusesPress(t *testing.T) {
	c := check.New(t)
	const (
		menuID = unison.UserBaseID + iota
		neverID
	)
	var wnd *unison.Window
	activated := 0
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			wnd = newHeadlessWindow(t, "disabled item", geom.NewRect(10, 10, 400, 300), unison.NewPanel())
			if wnd == nil {
				return
			}
			unison.DefaultMenuFactory().BarForWindow(wnd, func(bar unison.Menu) {
				f := bar.Factory()
				edit := f.NewMenu(menuID, "Edit", nil)
				edit.InsertItem(-1, f.NewItem(neverID, "Never", unison.KeyBinding{},
					func(_ unison.MenuItem) bool { return false },
					func(_ unison.MenuItem) { activated++ }))
				bar.InsertMenu(-1, edit)
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
	item := axNamed(tree, "Never")
	c.True(item != nil)
	if item == nil {
		return
	}
	c.True(item.Disabled, "an item whose validator refuses it is disabled")
	c.False(item.Actions.Has(accessibility.Press), "a disabled item must not offer to be pressed")
	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   item.ID,
		Action: accessibility.Press,
	}), "a disabled item must refuse to be pressed")
	screen.Sync()
	var count int
	screen.Do(func() { count = activated })
	c.Equal(0, count, "the item's handler must not have run")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestAccessibilityEnabledFromAnotherGoroutine verifies the promise AccessibilityEnabled makes about where it may be
// called from. It is the race detector that does the checking here: with the flag that permits support held in a plain
// bool, this reads it while SetAccessibilityEnabled writes it on the UI thread.
func TestAccessibilityEnabledFromAnotherGoroutine(t *testing.T) {
	c := check.New(t)
	screen := startHeadless(t, unison.HeadlessConfig{Width: 200, Height: 100})
	var wg sync.WaitGroup
	stop := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				unison.AccessibilityEnabled()
			}
		}
	}()
	for range 50 {
		unison.SetAccessibilityEnabled(false)
		unison.SetAccessibilityEnabled(true)
	}
	screen.Sync()
	close(stop)
	wg.Wait()
	c.True(unison.AccessibilityEnabled(), "support was last turned back on")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestAccessibilityDisabledWidgetsRefuseActions verifies that the central gate covers the widgets an assistive
// technology is most likely to reach for: a disabled button must not run its click handler, and a disabled field must
// not have its contents rewritten, neither of which a person could do with the mouse or the keyboard, since both of
// those paths are gated on Panel.Enabled.
func TestAccessibilityDisabledWidgetsRefuseActions(t *testing.T) {
	c := check.New(t)
	var button *unison.Button
	var field *unison.Field
	var clicks, modifications int
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			button = unison.NewButton()
			button.SetTitle("Save")
			button.ClickCallback = func() { clicks++ }
			button.SetEnabled(false)
			field = unison.NewField()
			field.SetText("hello")
			field.ModifiedCallback = func(_, _ *unison.FieldState) { modifications++ }
			field.SetEnabled(false)
			wnd = newHeadlessWindow(t, "disabled widgets", geom.NewRect(10, 10, 300, 200), axColumn(button, field))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() { wnd.ToFront() }))

	screen.AccessibilityTree(wnd)
	buttonNode := screen.AccessibilityNodeFor(button)
	fieldNode := screen.AccessibilityNodeFor(field)
	c.True(buttonNode != nil)
	c.True(fieldNode != nil)
	if buttonNode == nil || fieldNode == nil {
		return
	}
	c.True(buttonNode.Disabled)
	c.True(fieldNode.Disabled)
	c.False(buttonNode.Actions.Has(accessibility.Press), "a disabled button must not offer to be pressed")
	c.False(fieldNode.Actions.Has(accessibility.SetValue), "a disabled field must not offer to be rewritten")
	c.False(fieldNode.Actions.Has(accessibility.ReplaceText))
	c.False(fieldNode.Actions.Has(accessibility.SetTextSelection))

	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   buttonNode.ID,
		Action: accessibility.Press,
	}))
	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   fieldNode.ID,
		Action: accessibility.SetValue,
		Value:  "rewritten",
	}))
	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   fieldNode.ID,
		Action: accessibility.ReplaceText,
		Start:  0,
		End:    5,
		Value:  "bye",
	}))
	screen.Sync()
	var clicked, modified int
	var text string
	screen.Do(func() {
		clicked = clicks
		modified = modifications
		text = field.Text()
	})
	c.Equal(0, clicked, "a disabled button's handler must not have run")
	c.Equal(0, modified, "a disabled field must not have been modified")
	c.Equal("hello", text, "a disabled field's contents must be untouched")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
