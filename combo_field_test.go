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
	checkenum "github.com/richardwilkes/unison/enums/check"
	"github.com/richardwilkes/unison/enums/role"
)

// The options the combo field tests added below choose from, named once so that the same words are not repeated
// through every one of them.
const (
	comboOptionA = "Larch"
	comboOptionB = "Rowan"
	comboOptionC = "Alder"
)

// TestComboFieldAccessibilityReportsWhetherItIsOpen verifies that a combo field says whether its dropdown is showing
// and can be asked to put it away again. It always claimed to be collapsed, whatever was on the screen, so UI
// Automation's ExpandCollapseState and AT-SPI's STATE_EXPANDED never moved and there was no collapse to ask for.
func TestComboFieldAccessibilityReportsWhetherItIsOpen(t *testing.T) {
	c := check.New(t)
	first := "One"
	second := "Two"
	var combo *unison.Field
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			combo = unison.NewComboField([]*string{&first, &second}, &first, nil)
			wnd = newHeadlessWindow(t, "combo state", geom.NewRect(10, 10, 300, 150), axColumn(combo))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() { wnd.ToFront() }))

	screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(combo)
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
		"expanding the combo field should have opened its menu within the window")
	node = screen.AccessibilityNodeFor(combo)
	c.True(node != nil)
	if node == nil {
		return
	}
	c.True(node.Expanded, "the combo field must say its choices are showing while they are")

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
		"collapsing the combo field should have taken its menu away")
	node = screen.AccessibilityNodeFor(combo)
	c.True(node != nil)
	if node != nil {
		c.False(node.Expanded)
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestComboFieldAccessibilityMarksEveryChoiceAndPointsAtItsMenu verifies two things about an opened combo field: every
// option is described as one of a set of check items, with the field's current value checked and the rest explicitly
// not, and the field points at the menu panel showing them.
//
// The options used to be given no check state at all, so nothing in the dropdown said which of them the field was
// showing — an item that has never been given a state is described as a plain command. And the field never filled in
// Controls, which PopupMenu does for the menu it opens, so an assistive technology could tie a popup to its choices but
// not a combo field to its own, despite the two being written to describe themselves alike.
func TestComboFieldAccessibilityMarksEveryChoiceAndPointsAtItsMenu(t *testing.T) {
	c := check.New(t)
	first := comboOptionA
	second := comboOptionB
	third := comboOptionC
	var combo *unison.Field
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			combo = unison.NewComboField([]*string{&first, &second, &third}, &second, nil)
			wnd = newHeadlessWindow(t, "combo checks", geom.NewRect(10, 10, 300, 150), axColumn(combo))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() { wnd.ToFront() }))

	screen.AccessibilityTree(wnd)
	node := axMustNode(c, screen.AccessibilityNodeFor(combo))
	c.Equal(0, len(node.Controls), "nothing is showing yet, so there is nothing to point at")
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.Expand,
	}))

	tree := screen.AccessibilityTree(wnd)
	items := axNodesWithRole(tree, role.MenuItem)
	c.Equal(3, len(items), "one menu item per option")
	checked := 0
	for _, item := range items {
		c.True(item.HasCheck, "%q has to be described as one of a set of choices rather than as a plain command",
			item.Name)
		switch item.Checked {
		case checkenum.On:
			checked++
			c.Equal(comboOptionB, item.Name, "the checked choice is the value the field is holding")
		default:
			c.Equal(checkenum.Off, item.Checked, "%q is a choice that has not been made rather than a half-made one",
				item.Name)
		}
	}
	c.Equal(1, checked, "exactly one of the choices is checked")

	node = axMustNode(c, screen.AccessibilityNodeFor(combo))
	c.Equal(1, len(node.Controls), "the field points at the menu its choices are showing in")
	if len(node.Controls) == 1 {
		menuNode := axMustNode(c, tree.Node(node.Controls[0]))
		c.Equal(role.Menu, menuNode.Role, "what it points at is the menu panel itself")
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestComboFieldAccessibilityExpandChecksAgainWhenItRuns verifies that a combo field disabled between the moment an
// assistive technology asked it to expand and the moment the queued click came to run does not open its menu. The
// dispatcher gates every request on Panel.Enabled, but the work happens a task later, and a click could not open the
// dropdown of a disabled field: Window.mouseDown passes straight over a panel that is not enabled.
func TestComboFieldAccessibilityExpandChecksAgainWhenItRuns(t *testing.T) {
	c := check.New(t)
	first := comboOptionA
	second := comboOptionB
	var combo *unison.Field
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			combo = unison.NewComboField([]*string{&first, &second}, &first, nil)
			wnd = newHeadlessWindow(t, "combo disabled", geom.NewRect(10, 10, 300, 150), axColumn(combo))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() { wnd.ToFront() }))

	before := axRootChildCount(screen.AccessibilityTree(wnd))
	// Both happen within one turn of the UI thread, so the field is disabled by the time the queued click runs. The
	// callback is asked directly because a request routed through the window would have been refused outright, which is
	// the very check this one has to survive to reach.
	c.True(screen.Do(func() {
		c.True(combo.Accessibility.ActionCallback(accessibility.ActionRequest{Action: accessibility.Expand}))
		combo.SetEnabled(false)
	}))
	c.Equal(before, axRootChildCount(screen.AccessibilityTree(wnd)),
		"a combo field disabled before the queued click ran must not have opened its menu")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestComboFieldAccessibilityRefusesWhenItsWindowIsNotActive verifies that a combo field in a background window refuses
// to expand. The dropdown is not built in the field's own window: menu.createPopup inserts it into ActiveWindow(), so
// carrying the request out would have put this field's choices up in whatever window was frontmost, at coordinates
// translated from this one. The mouse path never could, since Window.mouseDown delivers nothing to a window that does
// not have the focus.
func TestComboFieldAccessibilityRefusesWhenItsWindowIsNotActive(t *testing.T) {
	c := check.New(t)
	first := comboOptionA
	second := comboOptionB
	var combo *unison.Field
	var background, front *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 500},
		unison.StartupFinishedCallback(func() {
			combo = unison.NewComboField([]*string{&first, &second}, &first, nil)
			background = newHeadlessWindow(t, "background", geom.NewRect(10, 10, 250, 150), axColumn(combo))
			front = newHeadlessWindow(t, "front", geom.NewRect(300, 10, 250, 150), axColumn(unison.NewButton()))
		}))
	c.NotNil(background)
	c.NotNil(front)

	c.True(screen.Do(func() { background.ToFront() }))
	screen.AccessibilityTree(background)
	node := axMustNode(c, screen.AccessibilityNodeFor(combo))

	c.True(node.Actions.Has(accessibility.Expand), "its own window is the active one to begin with")

	c.True(screen.Do(func() { front.ToFront() }))
	before := axRootChildCount(screen.AccessibilityTree(background))

	// What is refused is not advertised either: a combo box that went on offering an expansion which silently did
	// nothing would leave a screen reader saying the choices can be shown while nothing came of asking. See
	// axSnapshot.narrowMenuActions.
	backgrounded := axMustNode(c, screen.AccessibilityNodeFor(combo))
	c.True(backgrounded.Expandable, "it still holds choices; what it has lost is anywhere to show them")
	c.False(backgrounded.Actions.Has(accessibility.Expand),
		"a combo field that would refuse to expand must not offer to")
	c.True(backgrounded.Actions.Has(accessibility.Collapse),
		"collapsing takes down a menu that is showing rather than opening one, so it is still worth offering")
	c.False(backgrounded.Actions.Has(accessibility.ShowContextMenu),
		"the field's own contextual menu goes the same way, and for the same reason")

	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.Expand,
	}), "expanding a combo field in a window that is not the active one has to be refused")
	c.Equal(before, axRootChildCount(screen.AccessibilityTree(background)),
		"nothing should have been opened in the background window")

	// Bringing the window back to the front brings them back with it, since Window.gainedFocus marks it for publishing
	// just as losing the focus did.
	c.True(screen.Do(func() { background.ToFront() }))
	screen.AccessibilityTree(background)
	restored := axMustNode(c, screen.AccessibilityNodeFor(combo))
	c.True(restored.Actions.Has(accessibility.Expand), "the active window's combo field offers its choices again")
	c.True(restored.Actions.Has(accessibility.ShowContextMenu))
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.Expand,
	}), "and carries the request out")
	c.Equal(before+1, axRootChildCount(screen.AccessibilityTree(background)),
		"the choices should have been shown in the combo field's own window")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
