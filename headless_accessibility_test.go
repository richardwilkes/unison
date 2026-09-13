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

// These tests drive the accessibility core the way a screen reader would: they let the application run, ask for the
// description of a window, and assert on what an assistive technology would have been handed. The panels here mostly
// say what they are through Panel.Accessibility directly rather than leaning on a widget's own ProvideAccessibility, so
// that what is being checked is the core rather than any one widget. A session owns most of the package's mutable
// globals while it runs, so none of these may call t.Parallel.

// axColumn returns a panel that stacks its children in a single column, for building the content of a test window.
func axColumn(children ...unison.Paneler) *unison.Panel {
	panel := unison.NewPanel()
	panel.SetLayout(&unison.FlexLayout{
		Columns:  1,
		HSpacing: unison.StdHSpacing,
		VSpacing: unison.StdVSpacing,
	})
	for _, child := range children {
		panel.AddChild(child)
	}
	return panel
}

// axSnapshotCount returns how many accessibility snapshots have been built, read on the UI thread that owns the count.
func axSnapshotCount(t *testing.T, screen *unison.HeadlessScreen) uint64 {
	t.Helper()
	var count uint64
	screen.Do(func() { count = unison.AccessibilitySnapshotCountForTest() })
	return count
}

// axNamed returns the first node in the tree whose name matches, or nil if there is none.
func axNamed(tree *accessibility.Tree, name string) *accessibility.Node {
	var found *accessibility.Node
	tree.Walk(func(n *accessibility.Node) bool {
		if n.Name == name {
			found = n
			return false
		}
		return true
	})
	return found
}

// TestAccessibilityInactiveBuildsNothing verifies the promise that an application no assistive technology is watching
// pays nothing: a session that is clicked at and typed into builds no snapshot at all, and only once a tree is actually
// asked for does any of it run.
func TestAccessibilityInactiveBuildsNothing(t *testing.T) {
	c := check.New(t)
	var button *unison.Button
	var field *unison.Field
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			button = unison.NewButton()
			button.SetTitle("Press Me")
			field = unison.NewField()
			wnd = newHeadlessWindow(t, "inactive", geom.NewRect(20, 20, 240, 120), axColumn(button, field))
		}))
	c.NotNil(wnd)

	screen.Click(screen.PanelCenter(button))
	screen.Click(screen.PanelCenter(field))
	screen.Type("hi")
	screen.Sync()
	c.False(unison.IsAccessibilityActive(), "nothing asked for accessibility support, so it must be off")
	c.Equal(uint64(0), axSnapshotCount(t, screen), "no snapshot may be built while accessibility support is off")

	tree := screen.AccessibilityTree(wnd)
	c.True(tree != nil, "asking for the tree must turn accessibility support on and build one")
	c.True(unison.IsAccessibilityActive())
	c.True(axSnapshotCount(t, screen) >= 1, "at least one snapshot must have been built")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestAccessibilityNoAccessibilityOptionRefusesActivation verifies that the startup option holds even against a direct
// request for a tree.
func TestAccessibilityNoAccessibilityOptionRefusesActivation(t *testing.T) {
	c := check.New(t)
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300}, unison.NoAccessibility(),
		unison.StartupFinishedCallback(func() {
			wnd = newHeadlessWindow(t, "refused", geom.NewRect(20, 20, 240, 120), unison.NewPanel())
		}))
	c.NotNil(wnd)

	screen.EnableAccessibility()
	c.False(unison.IsAccessibilityActive())
	c.True(screen.AccessibilityTree(wnd) == nil, "no tree may be built when accessibility support has been refused")
	c.Equal(uint64(0), axSnapshotCount(t, screen))
}

// TestAccessibilitySetEnabled verifies that support can be turned off and back on while the application runs: turning
// it off shuts down what is being served and refuses to build anything more, and turning it on lets the next request
// start it again.
func TestAccessibilitySetEnabled(t *testing.T) {
	c := check.New(t)
	var wnd *unison.Window
	var button *unison.Button
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			button = unison.NewButton()
			button.SetTitle("Press")
			wnd = newHeadlessWindow(t, "switch", geom.NewRect(20, 20, 240, 120), button)
		}))
	c.NotNil(wnd)
	c.True(unison.AccessibilityEnabled())
	c.True(screen.AccessibilityTree(wnd) != nil)
	c.True(unison.IsAccessibilityActive())

	unison.SetAccessibilityEnabled(false)
	screen.Sync()
	c.False(unison.AccessibilityEnabled())
	c.False(unison.IsAccessibilityActive(), "turning support off should have shut down what was being served")
	c.True(screen.AccessibilityTree(wnd) == nil, "no tree may be built while support is refused")
	built := axSnapshotCount(t, screen)
	var center geom.Point
	screen.Do(func() {
		center = button.RectToRoot(button.ContentRect(true)).Center().Add(wnd.ContentRect().Point)
	})
	screen.Click(center)
	screen.Sync()
	c.Equal(built, axSnapshotCount(t, screen), "nothing may be built for a redraw while support is refused")

	unison.SetAccessibilityEnabled(true)
	screen.Sync()
	c.True(unison.AccessibilityEnabled())
	c.False(unison.IsAccessibilityActive(), "lifting the refusal starts nothing by itself")
	c.True(screen.AccessibilityTree(wnd) != nil, "the next request should start support again")
	c.True(unison.IsAccessibilityActive())
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestAccessibilityEnvironmentOverrides verifies both directions of AccessibilityEnvKey: a true value turns support on
// during startup, without anything having asked for it, and a false value refuses it as the startup option does.
func TestAccessibilityEnvironmentOverrides(t *testing.T) {
	t.Run("forced on", func(t *testing.T) {
		c := check.New(t)
		t.Setenv(unison.AccessibilityEnvKey, "1")
		var wnd *unison.Window
		screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
			unison.StartupFinishedCallback(func() {
				wnd = newHeadlessWindow(t, "forced", geom.NewRect(20, 20, 240, 120), unison.NewPanel())
			}))
		c.NotNil(wnd)
		c.True(unison.IsAccessibilityActive(), "the environment asked for accessibility support")
		c.True(axSnapshotCount(t, screen) >= 1, "the window should have been described as it was drawn")
	})
	t.Run("forced off", func(t *testing.T) {
		c := check.New(t)
		t.Setenv(unison.AccessibilityEnvKey, "0")
		var wnd *unison.Window
		screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
			unison.StartupFinishedCallback(func() {
				wnd = newHeadlessWindow(t, "refused", geom.NewRect(20, 20, 240, 120), unison.NewPanel())
			}))
		c.NotNil(wnd)
		screen.EnableAccessibility()
		c.False(unison.IsAccessibilityActive())
		c.True(screen.AccessibilityTree(wnd) == nil)
	})
}

// TestAccessibilityTreeShape verifies how the panel hierarchy is turned into a tree: the window is the root, hidden
// panels are left out, a panel that hides itself promotes its children, scaffolding groups are marked to be looked
// past, a control takes its name from the label beside it or from the panel it points at, and a tooltip becomes a
// description.
func TestAccessibilityTreeShape(t *testing.T) {
	c := check.New(t)
	var content, hidden, promoted, group, well, tipped *unison.Panel
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 400},
		unison.StartupFinishedCallback(func() {
			nameLabel := unison.NewLabel()
			nameLabel.SetTitle("Name:")
			nameLabel.Accessibility.Role = role.Label
			field := unison.NewField()
			field.Accessibility.Role = role.TextField

			hidden = unison.NewPanel()
			hidden.Accessibility.Role = role.Button
			hidden.Accessibility.Name = "Hidden"
			hidden.Hidden = true

			promoted = unison.NewPanel()
			promoted.Accessibility.Role = role.Button
			promoted.Accessibility.Name = "Promoted"
			wrapper := unison.NewPanel()
			wrapper.Accessibility.Role = role.None
			wrapper.AddChild(promoted)

			group = unison.NewPanel()

			colorLabel := unison.NewLabel()
			colorLabel.SetTitle("Color:")
			colorLabel.Accessibility.Role = role.Label
			well = unison.NewPanel()
			well.Accessibility.Role = role.ColorWell
			well.Accessibility.LabeledBy = colorLabel

			tipped = unison.NewPanel()
			tipped.Accessibility.Role = role.Button
			tipped.Accessibility.Name = "Tipped"
			tipped.Tooltip = unison.NewTooltipWithText("Explains things")

			content = axColumn(nameLabel, field, hidden, wrapper, group, well, colorLabel, tipped)
			wnd = newHeadlessWindow(t, "shape", geom.NewRect(10, 10, 300, 320), content)
		}))
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	c.True(tree != nil)
	root := tree.Node(tree.Root)
	c.True(root != nil)
	c.Equal(role.Window, root.Role, "the root of a window's tree is the window")
	c.Equal("shape", root.Name, "the window's name is its title")
	c.False(root.Modal, "nothing is modal")

	c.True(screen.AccessibilityNodeFor(hidden) == nil, "a hidden panel is not described at all")

	contentNode := screen.AccessibilityNodeFor(content)
	c.True(contentNode != nil)
	c.Equal(role.Group, contentNode.Role, "a plain panel is a group")
	c.True(contentNode.Ignored, "a group with nothing to say about itself is scaffolding")

	promotedNode := screen.AccessibilityNodeFor(promoted)
	c.True(promotedNode != nil)
	c.Equal(contentNode.ID, promotedNode.Parent,
		"a child of a role.None panel is promoted into that panel's own parent")

	groupNode := screen.AccessibilityNodeFor(group)
	c.True(groupNode != nil)
	c.True(groupNode.Ignored)
	c.Equal(0, len(tree.UnignoredChildren(groupNode.ID)), "there is nothing beneath the empty group")

	field := axNamed(tree, "Name")
	c.True(field != nil, "the field should have taken its name from the label before it, minus the colon")
	c.Equal(role.TextField, field.Role)
	c.Equal(1, len(field.LabeledBy), "the label it took its name from is reported as its label")
	labelNode := tree.Node(field.LabeledBy[0])
	c.True(labelNode != nil)
	c.Equal("Name:", labelNode.Name, "the label's own name is its text, colon and all")
	c.Equal(role.Label, labelNode.Role)
	c.True(field.Focusable)
	c.True(field.Actions.Has(accessibility.Focus))
	c.True(field.Actions.Has(accessibility.ScrollIntoView))

	wellNode := screen.AccessibilityNodeFor(well)
	c.True(wellNode != nil)
	c.Equal("Color", wellNode.Name,
		"an explicit LabeledBy names the control, minus the colon the label beside one has taken off it")
	c.Equal(1, len(wellNode.LabeledBy))
	colorLabelNode := tree.Node(wellNode.LabeledBy[0])
	c.True(colorLabelNode != nil)
	if colorLabelNode != nil {
		c.Equal("Color:", colorLabelNode.Name, "the label's own name is its text, colon and all")
	}

	tippedNode := screen.AccessibilityNodeFor(tipped)
	c.True(tippedNode != nil)
	c.Equal("Explains things", tippedNode.Description, "the tooltip text becomes the description")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestAccessibilityCallbackDecidesScaffolding verifies that Accessibility.Callback, which runs last and may adjust
// anything on the node, is not too late to settle whether the node is scaffolding: a group it gives the name that was
// missing is content and must be reported, which is exactly what naming an otherwise anonymous group is for, while a
// group it marks to be looked past stays that way however much the node says.
func TestAccessibilityCallbackDecidesScaffolding(t *testing.T) {
	c := check.New(t)
	var named, looked, plain *unison.Panel
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 400},
		unison.StartupFinishedCallback(func() {
			named = unison.NewPanel()
			named.Accessibility.Callback = func(n *accessibility.Node) { n.Name = "Toolbar" }

			looked = unison.NewPanel()
			looked.Accessibility.Name = "Says something"
			looked.Accessibility.Callback = func(n *accessibility.Node) { n.Ignored = true }

			plain = unison.NewPanel()

			wnd = newHeadlessWindow(t, "callbacks", geom.NewRect(10, 10, 300, 300),
				axColumn(named, looked, plain))
		}))
	c.NotNil(wnd)

	c.True(screen.AccessibilityTree(wnd) != nil)
	namedNode := screen.AccessibilityNodeFor(named)
	c.True(namedNode != nil)
	if namedNode != nil {
		c.Equal("Toolbar", namedNode.Name, "the callback runs last and may name the node")
		c.False(namedNode.Ignored, "a group the callback named has something to say and is not scaffolding")
	}
	lookedNode := screen.AccessibilityNodeFor(looked)
	c.True(lookedNode != nil)
	if lookedNode != nil {
		c.True(lookedNode.Ignored, "a callback that marks a node to be looked past has the last word")
	}
	plainNode := screen.AccessibilityNodeFor(plain)
	c.True(plainNode != nil)
	if plainNode != nil {
		c.True(plainNode.Ignored, "a group with nothing to say about itself is still scaffolding")
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestAccessibilityFocusTracking verifies that the tree says where the keyboard focus is, and that moving it is
// reported as an event.
func TestAccessibilityFocusTracking(t *testing.T) {
	c := check.New(t)
	var first, second *unison.Field
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			first = unison.NewField()
			first.Accessibility.Role = role.TextField
			second = unison.NewField()
			second.Accessibility.Role = role.TextField
			wnd = newHeadlessWindow(t, "focus", geom.NewRect(20, 20, 240, 120), axColumn(first, second))
		}))
	c.NotNil(wnd)

	screen.EnableAccessibility()
	screen.Click(screen.PanelCenter(first))
	tree := screen.AccessibilityTree(wnd)
	c.True(tree != nil)
	firstNode := screen.AccessibilityNodeFor(first)
	c.True(firstNode != nil)
	c.Equal(firstNode.ID, tree.Focus, "the tree should point at the focused panel")
	c.True(firstNode.Focused)
	c.True(tree.Node(tree.Root).Focused, "the window itself is active")

	screen.AccessibilityEvents(wnd)
	screen.Click(screen.PanelCenter(second))
	tree = screen.AccessibilityTree(wnd)
	secondNode := screen.AccessibilityNodeFor(second)
	c.True(secondNode != nil)
	c.Equal(secondNode.ID, tree.Focus)
	events := screen.AccessibilityEvents(wnd)
	found := false
	for _, event := range events {
		if event.Kind == accessibility.FocusChanged && event.Node == secondNode.ID {
			found = true
		}
	}
	c.True(found, "moving the focus should have been reported: %v", events)
}

// TestAccessibilityEventsReportChanges verifies that two successive descriptions of a window are turned into the events
// an assistive technology needs to hear, and that reading them empties the list.
func TestAccessibilityEventsReportChanges(t *testing.T) {
	c := check.New(t)
	var panel *unison.Panel
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			panel = unison.NewPanel()
			panel.Accessibility.Role = role.Button
			panel.Accessibility.Name = "Before"
			wnd = newHeadlessWindow(t, "events", geom.NewRect(20, 20, 240, 120), axColumn(panel))
		}))
	c.NotNil(wnd)

	screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(panel)
	c.True(node != nil)
	screen.AccessibilityEvents(wnd)

	screen.Do(func() { panel.Accessibility.Name = "After" })
	screen.AccessibilityTree(wnd)
	events := screen.AccessibilityEvents(wnd)
	found := false
	for _, event := range events {
		if event.Kind == accessibility.NameChanged && event.Node == node.ID {
			found = true
			c.Equal("Before", event.Old)
			c.Equal("After", event.New)
		}
	}
	c.True(found, "the rename should have been reported: %v", events)
	c.Equal(0, len(screen.AccessibilityEvents(wnd)), "reading the events should have emptied the list")
}

// TestAccessibilityActions verifies the default behaviors a plain panel gets for free: it can be focused, and it can be
// pressed, which synthesizes the click it has no other way of being told about.
func TestAccessibilityActions(t *testing.T) {
	c := check.New(t)
	var field *unison.Field
	var button *unison.Button
	var clicks int
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			field = unison.NewField()
			field.Accessibility.Role = role.TextField
			button = unison.NewButton()
			button.SetTitle("Press Me")
			button.ClickCallback = func() { clicks++ }
			wnd = newHeadlessWindow(t, "actions", geom.NewRect(20, 20, 240, 120), axColumn(field, button))
		}))
	c.NotNil(wnd)

	screen.AccessibilityTree(wnd)
	fieldNode := screen.AccessibilityNodeFor(field)
	c.True(fieldNode != nil)
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   fieldNode.ID,
		Action: accessibility.Focus,
	}), "focusing a focusable panel should have been carried out")
	var focused bool
	// The window was never activated, so Focused() would answer for the window rather than for the field; what the
	// action is being asked to do is move the focus within the window.
	screen.Do(func() { focused = field.Is(wnd.CurrentFocus()) })
	c.True(focused, "the field should have taken the focus")

	buttonNode := screen.AccessibilityNodeFor(button)
	c.True(buttonNode != nil)
	c.True(buttonNode.Actions.Has(accessibility.Press), "a panel that handles both halves of a click can be pressed")
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   buttonNode.ID,
		Action: accessibility.Press,
	}))
	var count int
	screen.Do(func() { count = clicks })
	c.Equal(1, count, "pressing the button should have clicked it exactly once")

	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   fieldNode.ID,
		Action: accessibility.Increment,
	}), "an action nothing handles is refused")
	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   accessibility.NodeID(1 << 40),
		Action: accessibility.Focus,
	}), "an action naming a node that does not exist is refused")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestAccessibilityAnnouncements verifies that a request to speak something reaches the platform while an assistive
// technology is being served, and is dropped when one is not.
func TestAccessibilityAnnouncements(t *testing.T) {
	c := check.New(t)
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			newHeadlessWindow(t, "announce", geom.NewRect(20, 20, 240, 120), unison.NewPanel())
		}))

	unison.AnnounceForAccessibility("ignored")
	c.Equal(0, len(screen.Announcements()), "nothing is spoken while accessibility support is off")

	screen.EnableAccessibility()
	unison.AnnounceForAccessibility("saved")
	screen.Sync()
	announcements := screen.Announcements()
	c.Equal(1, len(announcements))
	if len(announcements) == 1 {
		c.Equal("saved", announcements[0])
	}
	c.Equal(0, len(screen.Announcements()), "reading the announcements should have emptied the list")
}
