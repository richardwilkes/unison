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
	"github.com/richardwilkes/unison/enums/mod"
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

// dropdownClick clicks the dropdown button of a field.
func dropdownClick(c check.Checker, screen *unison.HeadlessScreen, field *unison.Field) {
	c.Helper()
	var button *unison.Panel
	screen.Do(func() {
		if children := field.Children(); len(children) != 0 {
			button = children[0]
		}
	})
	if button == nil {
		c.Fatal("the field has no dropdown button")
	}
	screen.Click(screen.PanelCenter(button))
}

// dropdownChoose chooses the item with the given name from the menu open in the window, as an assistive technology
// would.
func dropdownChoose(c check.Checker, screen *unison.HeadlessScreen, wnd *unison.Window, name string) {
	c.Helper()
	item := axMustNode(c, axNamedWithRole(screen.AccessibilityTree(wnd), name, role.MenuItem), name)
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{Node: item.ID, Action: accessibility.Press}),
		"choosing %q", name)
}

// dropdownMenuCount returns how many menus are open in the window.
func dropdownMenuCount(screen *unison.HeadlessScreen, wnd *unison.Window) int {
	return len(axNodesWithRole(screen.AccessibilityTree(wnd), role.Menu))
}

// TestFieldDropdownAsksForOptionsOnlyWhenOpening verifies that describing a field with a dropdown does not ask for its
// options, and that each opening asks once and shows them as they are then, in order.
func TestFieldDropdownAsksForOptionsOnlyWhenOpening(t *testing.T) {
	c := check.New(t)
	asked := 0
	options := []string{comboOptionB, comboOptionA, comboOptionC}
	var field *unison.Field
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			field = unison.NewField()
			field.InstallDropdown(func() []string {
				asked++
				return options
			})
			wnd = newHeadlessWindow(t, "dropdown options", geom.NewRect(10, 10, 300, 150), axColumn(field))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() { wnd.ToFront() }))
	askedCount := func() int {
		var count int
		screen.Do(func() { count = asked })
		return count
	}

	screen.AccessibilityTree(wnd)
	node := axMustNode(c, screen.AccessibilityNodeFor(field))
	c.Equal(role.ComboBox, node.Role)
	c.True(node.Expandable)
	c.True(node.Actions.Has(accessibility.Expand))
	c.True(node.Actions.Has(accessibility.Collapse))
	c.Equal(0, askedCount(), "describing the field must not ask for its options")

	dropdownClick(c, screen, field)
	c.Equal(1, askedCount())
	c.Equal([]string{comboOptionB, comboOptionA, comboOptionC}, axMenuItemNames(screen.AccessibilityTree(wnd)))
	screen.KeyPress(unison.KeyEscape, mod.None)
	c.Equal(0, dropdownMenuCount(screen, wnd))

	screen.Do(func() { options = []string{comboOptionC, "Birch"} })
	dropdownClick(c, screen, field)
	c.Equal(2, askedCount())
	c.Equal([]string{comboOptionC, "Birch"}, axMenuItemNames(screen.AccessibilityTree(wnd)),
		"the menu shows the options as they are when it opens")
	screen.KeyPress(unison.KeyEscape, mod.None)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestFieldDropdownChecksTheTextExactly verifies that only the option equal to the field's text, case included, is
// checked, that every option has a check state, and that the field points at its menu, which is named after it.
func TestFieldDropdownChecksTheTextExactly(t *testing.T) {
	c := check.New(t)
	var field *unison.Field
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			field = unison.NewField()
			field.SetText(comboOptionB)
			field.Accessibility.Name = "Tree"
			field.InstallDropdown(func() []string { return []string{comboOptionA, comboOptionB, "rowan", comboOptionC} })
			wnd = newHeadlessWindow(t, "dropdown checks", geom.NewRect(10, 10, 300, 150), axColumn(field))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() { wnd.ToFront() }))

	screen.AccessibilityTree(wnd)
	dropdownClick(c, screen, field)
	tree := screen.AccessibilityTree(wnd)
	items := axNodesWithRole(tree, role.MenuItem)
	c.Equal(4, len(items))
	for _, item := range items {
		c.True(item.HasCheck, "%q has a check state", item.Name)
		expected := checkenum.Off
		if item.Name == comboOptionB {
			expected = checkenum.On
		}
		c.Equal(expected, item.Checked, "%q", item.Name)
	}

	node := axMustNode(c, screen.AccessibilityNodeFor(field))
	c.Equal(1, len(node.Controls), "the field points at its menu")
	if len(node.Controls) == 1 {
		menuNode := axMustNode(c, tree.Node(node.Controls[0]))
		c.Equal(role.Menu, menuNode.Role)
		c.Equal("Tree", menuNode.Name, "the menu is named after the field")
	}
	screen.KeyPress(unison.KeyEscape, mod.None)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestFieldDropdownChoiceGoesThroughSetText verifies that a choice replaces the field's text as an edit of its own,
// seen by ModifiedCallback, and that choosing the text the field already holds changes nothing.
func TestFieldDropdownChoiceGoesThroughSetText(t *testing.T) {
	c := check.New(t)
	modified := 0
	var other, field *unison.Field
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			other = unison.NewField()
			field = unison.NewField()
			field.SetText(comboOptionA)
			field.ModifiedCallback = func(_, _ *unison.FieldState) { modified++ }
			field.InstallDropdown(func() []string { return []string{comboOptionA, comboOptionB, comboOptionC} })
			wnd = newHeadlessWindow(t, "dropdown choice", geom.NewRect(10, 10, 300, 150), axColumn(other, field))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() {
		wnd.ToFront()
		wnd.SetFocus(other)
	}))

	screen.AccessibilityTree(wnd)
	dropdownClick(c, screen, field)
	var before int64
	screen.Do(func() { before = field.CurrentUndoID() })
	dropdownChoose(c, screen, wnd, comboOptionC)
	var count int
	var text string
	var focused bool
	var after int64
	screen.Do(func() {
		count = modified
		text = field.Text()
		focused = field.Focused()
		after = field.CurrentUndoID()
	})
	c.Equal(1, count)
	c.Equal(comboOptionC, text)
	c.True(focused, "the field has the focus after a choice")
	c.NotEqual(before, after, "a choice is an undoable edit of its own")
	c.Equal(0, dropdownMenuCount(screen, wnd))

	dropdownClick(c, screen, field)
	dropdownChoose(c, screen, wnd, comboOptionC)
	screen.Do(func() { count = modified })
	c.Equal(1, count, "choosing the text the field already holds changes nothing")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestFieldDropdownOpensOnDownAndKeepsOtherKeys verifies that the down arrow opens the menu and that every other key
// still reaches the KeyDownCallback the field had before.
func TestFieldDropdownOpensOnDownAndKeepsOtherKeys(t *testing.T) {
	c := check.New(t)
	ups := 0
	var field *unison.Field
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			field = unison.NewField()
			previous := field.KeyDownCallback
			field.KeyDownCallback = func(keyCode unison.KeyCode, mods mod.Modifiers, repeat bool) bool {
				if keyCode == unison.KeyUp {
					ups++
				}
				return previous(keyCode, mods, repeat)
			}
			field.InstallDropdown(func() []string { return []string{comboOptionA, comboOptionB} })
			wnd = newHeadlessWindow(t, "dropdown keys", geom.NewRect(10, 10, 300, 150), axColumn(field))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() {
		wnd.ToFront()
		wnd.SetFocus(field)
	}))

	screen.KeyPress(unison.KeyDown, mod.None)
	c.Equal(1, dropdownMenuCount(screen, wnd), "the down arrow opens the menu")
	screen.KeyPress(unison.KeyEscape, mod.None)
	c.Equal(0, dropdownMenuCount(screen, wnd), "Escape closes it")
	screen.KeyPress(unison.KeyUp, mod.None)
	var count int
	screen.Do(func() { count = ups })
	c.Equal(1, count, "the up arrow reaches the previous callback")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestFieldDropdownAccessibilityExpandsAndCollapses verifies that the field can be expanded and collapsed by an
// assistive technology, and that the accessibility callbacks it had before go on running.
func TestFieldDropdownAccessibilityExpandsAndCollapses(t *testing.T) {
	c := check.New(t)
	var reached []accessibility.Action
	var field *unison.Field
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			field = unison.NewField()
			field.Accessibility.Callback = func(node *accessibility.Node) { node.Description = "kept" }
			field.Accessibility.ActionCallback = func(req accessibility.ActionRequest) bool {
				reached = append(reached, req.Action)
				return false
			}
			field.InstallDropdown(func() []string { return []string{comboOptionA, comboOptionB} })
			wnd = newHeadlessWindow(t, "dropdown expand", geom.NewRect(10, 10, 300, 150), axColumn(field))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() { wnd.ToFront() }))

	before := axRootChildCount(screen.AccessibilityTree(wnd))
	node := axMustNode(c, screen.AccessibilityNodeFor(field))
	c.Equal("kept", node.Description, "the previous callback still runs")
	c.True(node.Expandable)
	c.False(node.Expanded)

	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{Node: node.ID, Action: accessibility.Expand}))
	c.Equal(before+1, axRootChildCount(screen.AccessibilityTree(wnd)), "expanding opens the menu")
	expanded := axMustNode(c, screen.AccessibilityNodeFor(field))
	c.True(expanded.Expanded)
	c.Equal("kept", expanded.Description)

	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{Node: node.ID, Action: accessibility.Expand}))
	c.Equal(before+1, axRootChildCount(screen.AccessibilityTree(wnd)), "expanding again changes nothing")

	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{Node: node.ID, Action: accessibility.Collapse}))
	c.Equal(before, axRootChildCount(screen.AccessibilityTree(wnd)), "collapsing closes the menu")
	c.False(axMustNode(c, screen.AccessibilityNodeFor(field)).Expanded)

	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{Node: node.ID, Action: accessibility.Focus}))
	var actions []accessibility.Action
	screen.Do(func() { actions = reached })
	c.Equal([]accessibility.Action{accessibility.Focus}, actions,
		"only what the dropdown does not handle reaches the previous action callback")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestFieldDropdownWithNothingToOfferOpensNothing verifies that a field whose options are empty still says it can
// expand, since describing it does not ask for them, but opens nothing and refuses a request to expand, which asks for
// them once, while a click still gives it the focus.
func TestFieldDropdownWithNothingToOfferOpensNothing(t *testing.T) {
	c := check.New(t)
	asked := 0
	var other, field *unison.Field
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			other = unison.NewField()
			field = unison.NewField()
			field.InstallDropdown(func() []string {
				asked++
				return nil
			})
			wnd = newHeadlessWindow(t, "dropdown empty", geom.NewRect(10, 10, 300, 150), axColumn(other, field))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() { wnd.ToFront() }))

	screen.AccessibilityTree(wnd)
	node := axMustNode(c, screen.AccessibilityNodeFor(field))
	c.True(node.Expandable)
	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{Node: node.ID, Action: accessibility.Expand}),
		"a request to expand is refused when there is nothing to show")
	var count int
	screen.Do(func() { count = asked })
	c.Equal(1, count, "the request asks for the options once")
	c.Equal(0, dropdownMenuCount(screen, wnd))
	c.False(axMustNode(c, screen.AccessibilityNodeFor(field)).Expanded)

	screen.Do(func() { wnd.SetFocus(other) })
	dropdownClick(c, screen, field)
	c.Equal(0, dropdownMenuCount(screen, wnd))
	var focused bool
	screen.Do(func() { focused = field.Focused() })
	c.True(focused, "the click still gives the field the focus")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestFieldDropdownRefusesWhileDisabled verifies that the dropdown of a disabled field neither opens nor asks for its
// options, whether clicked or expanded by an assistive technology, and leaves the focus where it was.
func TestFieldDropdownRefusesWhileDisabled(t *testing.T) {
	c := check.New(t)
	asked := 0
	var other, field *unison.Field
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			other = unison.NewField()
			field = unison.NewField()
			field.InstallDropdown(func() []string {
				asked++
				return []string{comboOptionA, comboOptionB}
			})
			field.SetEnabled(false)
			wnd = newHeadlessWindow(t, "dropdown disabled", geom.NewRect(10, 10, 300, 150), axColumn(other, field))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() {
		wnd.ToFront()
		wnd.SetFocus(other)
	}))

	screen.AccessibilityTree(wnd)
	node := axMustNode(c, screen.AccessibilityNodeFor(field))
	dropdownClick(c, screen, field)
	c.Equal(0, dropdownMenuCount(screen, wnd))
	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{Node: node.ID, Action: accessibility.Expand}))
	c.Equal(0, dropdownMenuCount(screen, wnd))
	var count int
	var focused bool
	screen.Do(func() {
		count = asked
		focused = other.Focused()
	})
	c.Equal(0, count, "the options are never asked for")
	c.True(focused, "the focus stays where it was")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestFieldAccessoryInsets verifies that AccessoryInsets is zero for a plain field and, for one with a dropdown, the
// width of the button on the right.
func TestFieldAccessoryInsets(t *testing.T) {
	c := check.New(t)
	var plain, field *unison.Field
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			plain = unison.NewField()
			field = unison.NewField()
			field.InstallDropdown(func() []string { return nil })
			wnd = newHeadlessWindow(t, "accessory insets", geom.NewRect(10, 10, 300, 150), axColumn(plain, field))
		}))
	c.NotNil(wnd)

	var plainInsets, insets geom.Insets
	var buttonWidth float32
	screen.Do(func() {
		plainInsets = plain.AccessoryInsets()
		insets = field.AccessoryInsets()
		if children := field.Children(); len(children) != 0 {
			buttonWidth = children[0].FrameRect().Width
		}
	})
	c.Equal(geom.Insets{}, plainInsets)
	c.True(insets.Right > 0)
	c.Equal(geom.Insets{Right: buttonWidth}, insets)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestComboFieldChoosingReportsEachChangeOnce verifies that choosing from a combo field reports each change of value
// once, including the change between «not set» and «empty», which both show an empty field.
func TestComboFieldChoosingReportsEachChangeOnce(t *testing.T) {
	c := check.New(t)
	const (
		notSetTitle = "«not set»"
		emptyTitle  = "«empty»"
	)
	empty := ""
	first := comboOptionA
	second := comboOptionB
	var reported []*string
	var combo *unison.Field
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			combo = unison.NewComboField([]*string{nil, &empty, &first, &second}, &first,
				func(value *string) { reported = append(reported, value) })
			wnd = newHeadlessWindow(t, "combo choices", geom.NewRect(10, 10, 300, 150), axColumn(combo))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() { wnd.ToFront() }))
	screen.AccessibilityTree(wnd)
	choose := func(title string) (calls []*string, watermark, text string) {
		dropdownClick(c, screen, combo)
		dropdownChoose(c, screen, wnd, title)
		screen.Do(func() {
			calls = reported
			reported = nil
			watermark = combo.Watermark
			text = combo.Text()
		})
		return calls, watermark, text
	}

	calls, watermark, text := choose(notSetTitle)
	c.Equal(1, len(calls))
	if len(calls) == 1 {
		c.True(calls[0] == nil, "«not set» is reported as nil")
	}
	c.Equal(notSetTitle, watermark)
	c.Equal("", text)

	calls, watermark, text = choose(emptyTitle)
	c.Equal(1, len(calls), "«empty» is a change from «not set», although both show an empty field")
	if len(calls) == 1 {
		c.True(calls[0] != nil && *calls[0] == "", "«empty» is reported as an empty string")
	}
	c.Equal(emptyTitle, watermark)
	c.Equal("", text)

	calls, _, _ = choose(emptyTitle)
	c.Equal(0, len(calls), "choosing the current value reports nothing")

	calls, watermark, text = choose(comboOptionB)
	c.Equal(1, len(calls))
	if len(calls) == 1 {
		c.True(calls[0] != nil && *calls[0] == comboOptionB)
	}
	c.Equal("", watermark)
	c.Equal(comboOptionB, text)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// dropdownEmbeddingField embeds a field, as applications build field types of their own.
type dropdownEmbeddingField struct {
	*unison.Field
}

// TestFieldDropdownOnAnEmbeddingField verifies that a field type embedding Field is described as a combo box and loses
// Expand while its window is not the active one, as a plain field does.
func TestFieldDropdownOnAnEmbeddingField(t *testing.T) {
	c := check.New(t)
	var field *dropdownEmbeddingField
	var background, front *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 500},
		unison.StartupFinishedCallback(func() {
			field = &dropdownEmbeddingField{Field: unison.NewField()}
			field.Self = field
			field.InstallDropdown(func() []string { return []string{comboOptionA, comboOptionB} })
			background = newHeadlessWindow(t, "background", geom.NewRect(10, 10, 250, 150), axColumn(field))
			front = newHeadlessWindow(t, "front", geom.NewRect(300, 10, 250, 150), axColumn(unison.NewButton()))
		}))
	c.NotNil(background)
	c.NotNil(front)

	c.True(screen.Do(func() { background.ToFront() }))
	screen.AccessibilityTree(background)
	node := axMustNode(c, screen.AccessibilityNodeFor(field))
	c.Equal(role.ComboBox, node.Role)
	c.True(node.Actions.Has(accessibility.Expand))

	c.True(screen.Do(func() { front.ToFront() }))
	screen.AccessibilityTree(background)
	node = axMustNode(c, screen.AccessibilityNodeFor(field))
	c.True(node.Expandable)
	c.False(node.Actions.Has(accessibility.Expand), "a menu opened from a background window would land elsewhere")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestFieldDropdownChecksOnlyTheFirstDuplicate verifies that when the options repeat the field's text, only the first
// of the duplicates is checked and the menu opens on it.
func TestFieldDropdownChecksOnlyTheFirstDuplicate(t *testing.T) {
	c := check.New(t)
	var field *unison.Field
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			field = unison.NewField()
			field.SetText(comboOptionB)
			field.InstallDropdown(func() []string { return []string{comboOptionA, comboOptionB, comboOptionB} })
			wnd = newHeadlessWindow(t, "dropdown duplicates", geom.NewRect(10, 10, 300, 150), axColumn(field))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() { wnd.ToFront() }))

	screen.AccessibilityTree(wnd)
	dropdownClick(c, screen, field)
	items := axNodesWithRole(screen.AccessibilityTree(wnd), role.MenuItem)
	c.Equal(3, len(items))
	if len(items) == 3 {
		c.Equal(checkenum.Off, items[0].Checked)
		c.Equal(checkenum.On, items[1].Checked, "the first duplicate is checked")
		c.Equal(checkenum.Off, items[2].Checked, "the second duplicate is not")
	}
	screen.KeyPress(unison.KeyEscape, mod.None)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestFieldDropdownLeavesModifiedDownToTheField verifies that the down arrow with a modifier held is the field's own
// command, extending the selection or moving the caret to the end, rather than opening the menu.
func TestFieldDropdownLeavesModifiedDownToTheField(t *testing.T) {
	c := check.New(t)
	var field *unison.Field
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			field = unison.NewField()
			field.SetText(comboOptionA)
			field.InstallDropdown(func() []string { return []string{comboOptionA, comboOptionB} })
			wnd = newHeadlessWindow(t, "dropdown modifiers", geom.NewRect(10, 10, 300, 150), axColumn(field))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() {
		wnd.ToFront()
		wnd.SetFocus(field)
		field.SetSelectionToStart()
	}))
	selection := func() (start, end int) {
		screen.Do(func() { start, end = field.Selection() })
		return start, end
	}

	screen.KeyPress(unison.KeyDown, mod.Shift)
	c.Equal(0, dropdownMenuCount(screen, wnd), "Shift+Down does not open the menu")
	start, end := selection()
	c.Equal(0, start)
	c.Equal(len(comboOptionA), end, "Shift+Down extends the selection to the end")

	screen.Do(func() { field.SetSelectionToStart() })
	screen.KeyPress(unison.KeyDown, mod.OSMenuCommand())
	c.Equal(0, dropdownMenuCount(screen, wnd), "Command+Down does not open the menu")
	start, end = selection()
	c.Equal(len(comboOptionA), start, "Command+Down moves the caret to the end")
	c.Equal(len(comboOptionA), end)

	screen.KeyPress(unison.KeyDown, mod.None)
	c.Equal(1, dropdownMenuCount(screen, wnd), "a bare Down still opens the menu")
	screen.KeyPress(unison.KeyEscape, mod.None)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestFieldDropdownWithNothingToOfferLeavesDownToTheField verifies that when the options are empty, the down arrow is
// left to the field, which moves the caret to the end, rather than being swallowed by a menu that never opens.
func TestFieldDropdownWithNothingToOfferLeavesDownToTheField(t *testing.T) {
	c := check.New(t)
	var field *unison.Field
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			field = unison.NewField()
			field.SetText(comboOptionA)
			field.InstallDropdown(func() []string { return nil })
			wnd = newHeadlessWindow(t, "dropdown empty keys", geom.NewRect(10, 10, 300, 150), axColumn(field))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() {
		wnd.ToFront()
		wnd.SetFocus(field)
		field.SetSelectionToStart()
	}))

	screen.KeyPress(unison.KeyDown, mod.None)
	c.Equal(0, dropdownMenuCount(screen, wnd))
	var start, end int
	screen.Do(func() { start, end = field.Selection() })
	c.Equal(len(comboOptionA), start, "the field moves the caret to the end")
	c.Equal(len(comboOptionA), end)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestFieldAccessoryBorderFollowsTheButton verifies that the border a dropdown installs leaves room for the button as
// it is now, so that a font set afterward, which resizes the button, moves the text out from under it.
func TestFieldAccessoryBorderFollowsTheButton(t *testing.T) {
	c := check.New(t)
	var field *unison.Field
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			field = unison.NewField()
			field.InstallDropdown(func() []string { return nil })
			wnd = newHeadlessWindow(t, "accessory border", geom.NewRect(10, 10, 300, 150), axColumn(field))
		}))
	c.NotNil(wnd)

	measure := func() (border, insets, button float32) {
		screen.Do(func() {
			border = field.Border().Insets().Right
			insets = field.AccessoryInsets().Right
			if children := field.Children(); len(children) != 0 {
				button = children[0].FrameRect().Width
			}
		})
		return border, insets, button
	}
	border, insets, button := measure()
	c.True(insets > 0)
	c.Equal(insets, button)
	c.True(border >= insets, "the border leaves room for the button")

	screen.Do(func() {
		field.Font = field.Font.Face().Font(field.Font.Size() * 2)
		field.MarkForLayoutAndRedraw()
		wnd.Content().ValidateLayout()
	})
	largerBorder, largerInsets, largerButton := measure()
	c.True(largerInsets > insets, "a larger font widens the button")
	c.Equal(largerInsets, largerButton)
	c.Equal(largerBorder-border, largerInsets-insets, "the border grows with the button")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
