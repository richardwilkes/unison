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

	// Spoken from the user interface thread, which is where most announcements are actually made: a callback that has
	// just finished something says so from wherever it is running. The call above went through the task queue, since it
	// was made from the test's own goroutine, while this one is carried out on the spot — the two halves of a promise
	// that it is safe to call from anywhere.
	screen.Do(func() { unison.AnnounceForAccessibility("on the user interface thread") })
	announcements = screen.Announcements()
	c.Equal(1, len(announcements))
	if len(announcements) == 1 {
		c.Equal("on the user interface thread", announcements[0])
	}

	// Nothing is spoken once support has been turned off again, whichever thread asks.
	unison.SetAccessibilityEnabled(false)
	screen.Sync()
	unison.AnnounceForAccessibility("dropped")
	screen.Do(func() { unison.AnnounceForAccessibility("also dropped") })
	c.Equal(0, len(screen.Announcements()))
	unison.SetAccessibilityEnabled(true)
	screen.Sync()
}

// axHasEvent reports whether the events hold one of the given kind naming the given node.
func axHasEvent(events []accessibility.Event, kind accessibility.EventKind, node accessibility.NodeID) bool {
	for _, event := range events {
		if event.Kind == kind && event.Node == node {
			return true
		}
	}
	return false
}

// axFocusablePanel returns a plain panel that can take the keyboard focus and says what it is. It draws nothing, so it
// looks exactly the same whether or not it holds the focus, which is what makes it the right thing to move the focus
// between here: a widget that repaints its focus border — a Field does — would mark the window for redraw on its own
// and hide whether the focus move itself did.
func axFocusablePanel(name string) *unison.Panel {
	panel := unison.NewPanel()
	panel.SetFocusable(true)
	panel.Accessibility.Name = name
	panel.Accessibility.Role = role.Button
	return panel
}

// TestAccessibilityFocusMoveIsPublishedWithoutARedraw verifies that moving the keyboard focus is always reported. A
// description is published after a window has been drawn, and nothing obliges a panel to draw itself any differently
// for holding the focus, so a move between two panels that do not repaint for it would otherwise sit unreported until
// something unrelated happened to redraw the window — which may be never.
func TestAccessibilityFocusMoveIsPublishedWithoutARedraw(t *testing.T) {
	c := check.New(t)
	var first, second *unison.Panel
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			first = axFocusablePanel("First")
			second = axFocusablePanel("Second")
			wnd = newHeadlessWindow(t, "focus moves", geom.NewRect(20, 20, 240, 120), axColumn(first, second))
		}))
	c.NotNil(wnd)

	screen.AccessibilityTree(wnd)
	firstNode := screen.AccessibilityNodeFor(first)
	secondNode := screen.AccessibilityNodeFor(second)
	c.True(firstNode != nil && secondNode != nil)
	if firstNode == nil || secondNode == nil {
		return
	}
	screen.Do(func() { wnd.SetFocus(first) })
	screen.AccessibilityEvents(wnd)

	// Nothing here asks for a tree: what is being checked is that the move published one by itself.
	screen.Do(func() { wnd.SetFocus(second) })
	events := screen.AccessibilityEvents(wnd)
	c.True(axHasEvent(events, accessibility.FocusChanged, secondNode.ID),
		"moving the focus between two panels that do not repaint should have been reported: %v", events)

	screen.Do(func() { wnd.SetFocus(nil) })
	events = screen.AccessibilityEvents(wnd)
	c.True(axHasEvent(events, accessibility.FocusChanged, 0), "the focus going nowhere should have been reported: %v",
		events)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestAccessibilityWindowActivationIsPublishedWithoutARedraw verifies the same thing for the window itself. A window
// whose content holds nothing that can take the focus repaints nothing when it becomes, or stops being, the active one,
// and an assistive technology has to be told which window the person is in.
func TestAccessibilityWindowActivationIsPublishedWithoutARedraw(t *testing.T) {
	c := check.New(t)
	var first, second *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 400},
		unison.StartupFinishedCallback(func() {
			first = newHeadlessWindow(t, "first", geom.NewRect(10, 10, 200, 100), unison.NewPanel())
			second = newHeadlessWindow(t, "second", geom.NewRect(260, 10, 200, 100), unison.NewPanel())
		}))
	c.NotNil(first)
	c.NotNil(second)

	screen.Do(func() { first.ToFront() })
	firstTree := screen.AccessibilityTree(first)
	secondTree := screen.AccessibilityTree(second)
	c.True(firstTree != nil && secondTree != nil)
	if firstTree == nil || secondTree == nil {
		return
	}
	c.True(firstTree.Node(firstTree.Root).Focused, "the window that was brought to the front is the active one")
	screen.AccessibilityEvents(first)
	screen.AccessibilityEvents(second)

	screen.Do(func() { second.ToFront() })
	firstEvents := screen.AccessibilityEvents(first)
	secondEvents := screen.AccessibilityEvents(second)
	c.True(axHasEvent(firstEvents, accessibility.WindowDeactivated, firstTree.Root),
		"the window that lost the focus should have said so: %v", firstEvents)
	c.True(axHasEvent(secondEvents, accessibility.WindowActivated, secondTree.Root),
		"the window that took the focus should have said so: %v", secondEvents)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestAccessibilityHiddenWindowLeavesTheDescription verifies that a window taken off the screen is withdrawn from what
// an assistive technology holds rather than left standing as the last thing said about it. AT-SPI has only what the
// application publishes, so a window hidden after it was described would otherwise go on being listed as showing.
//
// This is the one thing a headless session deliberately does not do the same way on every platform: it withdraws, as
// Linux does, while macOS and Windows keep everything they built, since there the system lists the application's
// windows itself. What is asserted below is therefore what Linux does with a hidden window, not a rule that holds
// everywhere. See Window.apiAccessibilityWindowHidden.
func TestAccessibilityHiddenWindowLeavesTheDescription(t *testing.T) {
	c := check.New(t)
	var wnd *unison.Window
	var panel *unison.Panel
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			panel = axFocusablePanel("Content")
			wnd = newHeadlessWindow(t, "hidden", geom.NewRect(20, 20, 240, 120), axColumn(panel))
		}))
	c.NotNil(wnd)

	c.True(screen.AccessibilityTree(wnd) != nil)
	c.True(screen.AccessibilityNodeFor(panel) != nil)

	// Asked for through the panel rather than through AccessibilityTree, which would describe the window again and put
	// back the very thing being checked for.
	screen.Do(func() { wnd.Hide() })
	c.True(screen.AccessibilityNodeFor(panel) == nil, "a window that has been hidden should no longer be described")

	screen.Do(func() { wnd.Show() })
	c.True(screen.AccessibilityNodeFor(panel) != nil, "showing the window again should describe it afresh")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestAccessibilityDisabledFocusFallsBack verifies what is published when the panel holding the keyboard focus is
// disabled. The window does not move the focus — SetEnabled does not, and a key press is merely not delivered — so the
// panel really does hold it, but a node that says it is focused while saying it cannot take the focus is a pair no
// assistive technology can make sense of. The focus is reported on the nearest thing above it that can be used instead.
func TestAccessibilityDisabledFocusFallsBack(t *testing.T) {
	c := check.New(t)
	var control, group *unison.Panel
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			control = axFocusablePanel("Control")
			group = axColumn(control)
			group.Accessibility.Name = "Group"
			wnd = newHeadlessWindow(t, "disabled focus", geom.NewRect(20, 20, 240, 120), group)
		}))
	c.NotNil(wnd)

	screen.Do(func() { wnd.SetFocus(control) })
	tree := screen.AccessibilityTree(wnd)
	c.True(tree != nil)
	controlNode := screen.AccessibilityNodeFor(control)
	c.True(controlNode != nil)
	if tree == nil || controlNode == nil {
		return
	}
	c.Equal(controlNode.ID, tree.Focus, "the panel holding the focus is where it is reported while it can be used")

	screen.Do(func() { control.SetEnabled(false) })
	tree = screen.AccessibilityTree(wnd)
	controlNode = screen.AccessibilityNodeFor(control)
	groupNode := screen.AccessibilityNodeFor(group)
	c.True(controlNode != nil && groupNode != nil)
	if tree == nil || controlNode == nil || groupNode == nil {
		return
	}
	var stillFocused bool
	screen.Do(func() { stillFocused = control.Is(wnd.CurrentFocus()) })
	c.True(stillFocused, "the window keeps the focus where it was, which is what makes this worth describing")
	c.True(controlNode.Disabled)
	c.False(controlNode.Focused, "a disabled node must not report that it holds the focus")
	c.Equal(groupNode.ID, tree.Focus, "the focus falls back to the nearest ancestor that can be used")
	c.True(groupNode.Focused)
	c.False(groupNode.Focusable, "which is not to say a person could tab to it")
	c.False(groupNode.Ignored, "and it must be a node an assistive technology is actually shown")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestAccessibilityUndescribedFocusFallsBack verifies the same fallback for a focus panel that is not in the tree at
// all. A panel that hides itself with role.None still takes the focus and still receives keys, and a tree that says the
// focus went nowhere tells an assistive technology the person is not anywhere.
func TestAccessibilityUndescribedFocusFallsBack(t *testing.T) {
	c := check.New(t)
	var control, group *unison.Panel
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			control = axFocusablePanel("Control")
			control.Accessibility.Role = role.None
			// Deliberately anonymous, so that the node the focus falls back to is one the tree would otherwise have
			// told an assistive technology to look past.
			group = axColumn(control)
			wnd = newHeadlessWindow(t, "undescribed focus", geom.NewRect(20, 20, 240, 120), group)
		}))
	c.NotNil(wnd)

	screen.Do(func() { wnd.SetFocus(control) })
	tree := screen.AccessibilityTree(wnd)
	c.True(tree != nil)
	groupNode := screen.AccessibilityNodeFor(group)
	c.True(groupNode != nil)
	if tree == nil || groupNode == nil {
		return
	}
	c.True(screen.AccessibilityNodeFor(control) == nil, "the panel holding the focus is not described at all")
	c.Equal(groupNode.ID, tree.Focus, "so the focus is reported on the nearest ancestor that is")
	c.True(groupNode.Focused)
	c.False(groupNode.Ignored, "and that ancestor must be a node an assistive technology is shown")
	c.True(tree.Focus != tree.Root, "and never on the window, which reports whether the window is active")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestAccessibilityFocusableAnonymousPanelIsNotScaffolding verifies that a panel which can be focused or clicked is
// reported however anonymous it is. A custom drawing widget that sets neither a role nor a name is exactly the element
// an assistive technology has to be able to reach, and a node it is told to look past is one it is not shown at all —
// which would leave the focus landing on something it does not have.
func TestAccessibilityFocusableAnonymousPanelIsNotScaffolding(t *testing.T) {
	c := check.New(t)
	var canvas, clickable, plain *unison.Panel
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			canvas = unison.NewPanel()
			canvas.SetFocusable(true)
			clickable = unison.NewPanel()
			clickable.MouseDownCallback = func(_ geom.Point, _, _ int, _ mod.Modifiers) bool { return true }
			clickable.MouseUpCallback = func(_ geom.Point, _ int, _ mod.Modifiers) bool { return true }
			plain = unison.NewPanel()
			wnd = newHeadlessWindow(t, "anonymous", geom.NewRect(20, 20, 240, 160),
				axColumn(canvas, clickable, plain))
		}))
	c.NotNil(wnd)

	screen.Do(func() { wnd.SetFocus(canvas) })
	tree := screen.AccessibilityTree(wnd)
	c.True(tree != nil)
	canvasNode := screen.AccessibilityNodeFor(canvas)
	clickableNode := screen.AccessibilityNodeFor(clickable)
	plainNode := screen.AccessibilityNodeFor(plain)
	c.True(canvasNode != nil && clickableNode != nil && plainNode != nil)
	if tree == nil || canvasNode == nil || clickableNode == nil || plainNode == nil {
		return
	}
	c.False(canvasNode.Ignored, "a panel that can take the focus is part of what the window holds")
	c.False(clickableNode.Ignored, "and so is one that can be pressed")
	c.True(plainNode.Ignored, "while one that can do neither, and says nothing, is scaffolding")
	c.Equal(canvasNode.ID, tree.Focus)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestAccessibilityCallbackKeepsWhatItSaidAboutIgnored verifies that a callback has the last word on whether a node is
// skipped. Naming an otherwise anonymous group is one of the things callbacks are for, and the name is what the
// scaffolding heuristic reads, so a callback that names a group and asks for it to be skipped anyway must not have its
// choice reversed by the re-reading its own name provoked.
func TestAccessibilityCallbackKeepsWhatItSaidAboutIgnored(t *testing.T) {
	c := check.New(t)
	var named, hidden, plain *unison.Panel
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			named = unison.NewPanel()
			named.Accessibility.Callback = func(n *accessibility.Node) { n.Name = "Toolbar" }
			hidden = unison.NewPanel()
			hidden.Accessibility.Callback = func(n *accessibility.Node) {
				n.Name = "Spacer"
				n.Ignored = true
			}
			plain = unison.NewPanel()
			plain.Accessibility.Name = "Shown"
			plain.Accessibility.Callback = func(n *accessibility.Node) { n.Ignored = true }
			wnd = newHeadlessWindow(t, "callbacks", geom.NewRect(20, 20, 240, 160), axColumn(named, hidden, plain))
		}))
	c.NotNil(wnd)

	c.True(screen.AccessibilityTree(wnd) != nil)
	namedNode := screen.AccessibilityNodeFor(named)
	hiddenNode := screen.AccessibilityNodeFor(hidden)
	plainNode := screen.AccessibilityNodeFor(plain)
	c.True(namedNode != nil && hiddenNode != nil && plainNode != nil)
	if namedNode == nil || hiddenNode == nil || plainNode == nil {
		return
	}
	c.Equal("Toolbar", namedNode.Name)
	c.False(namedNode.Ignored, "a group the callback named is no longer anonymous, so it is no longer scaffolding")
	c.Equal("Spacer", hiddenNode.Name)
	c.True(hiddenNode.Ignored, "a callback that names a group and asks for it to be skipped must be obeyed")
	c.True(plainNode.Ignored, "as must one that asks for a node nothing else would have skipped")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestAccessibilityPressHonorsADeclinedPress verifies that a panel which declines the press does not receive the
// release. Window.mouseDown remembers which panel to deliver the release to only when the press was accepted, so a
// panel that returns false — letting the press go to its parent — never sees the matching release from a real click.
func TestAccessibilityPressHonorsADeclinedPress(t *testing.T) {
	c := check.New(t)
	var declines *unison.Panel
	var downs, ups int
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			declines = unison.NewPanel()
			declines.Accessibility.Name = "Declines"
			declines.MouseDownCallback = func(_ geom.Point, _, _ int, _ mod.Modifiers) bool {
				downs++
				return false
			}
			declines.MouseUpCallback = func(_ geom.Point, _ int, _ mod.Modifiers) bool {
				ups++
				return true
			}
			wnd = newHeadlessWindow(t, "declined press", geom.NewRect(20, 20, 240, 120), axColumn(declines))
		}))
	c.NotNil(wnd)

	screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(declines)
	c.True(node != nil)
	if node == nil {
		return
	}
	c.True(node.Actions.Has(accessibility.Press), "a panel that handles both halves of a click offers the press")
	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.Press,
	}), "a press the panel declined must not be reported as carried out")
	var pressed, released int
	screen.Do(func() { pressed, released = downs, ups })
	c.Equal(1, pressed)
	c.Equal(0, released, "no real click would have delivered a release after a press that was declined")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestAccessibilityNodeForChecksOwnership verifies that the node a panel is asked about is the panel's own. An
// identity that arrived by having another panel's AccessibilityInfo assigned onto this one describes that other panel,
// and a panel that is never described — one that hides itself with role.None — keeps such an identity, since the
// builder only replaces it while describing it.
func TestAccessibilityNodeForChecksOwnership(t *testing.T) {
	c := check.New(t)
	var described, copied *unison.Panel
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			described = axFocusablePanel("Described")
			copied = unison.NewPanel()
			wnd = newHeadlessWindow(t, "ownership", geom.NewRect(20, 20, 240, 120), axColumn(described, copied))
		}))
	c.NotNil(wnd)

	screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(described)
	c.True(node != nil)
	if node == nil {
		return
	}
	screen.Do(func() {
		// What an application might write meaning only to copy the name across, onto a panel that is then never
		// described and so never has the identity taken back off it.
		copied.Accessibility = described.Accessibility
		copied.Accessibility.Role = role.None
	})
	screen.AccessibilityTree(wnd)
	c.True(screen.AccessibilityNodeFor(copied) == nil,
		"a panel must not be answered for with the node describing another one")
	c.True(screen.AccessibilityNodeFor(described) != nil, "while the panel the identity belongs to keeps it")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestAccessibilityCallbackCannotPublishAnUnresolvedRole verifies the invariant accessibility.Node.Role states: a
// published tree never holds role.Auto or role.None. Panel.Accessibility.Role takes both, so a callback — which runs
// after everything else about the node has been decided — is entitled to write either, and by then neither can be
// acted on: the panel has been described and its children already point at its node as their parent. Both become
// role.Group, exactly as they do for a virtual child a widget invented.
func TestAccessibilityCallbackCannotPublishAnUnresolvedRole(t *testing.T) {
	c := check.New(t)
	var none, auto, provided *unison.Panel
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 400},
		unison.StartupFinishedCallback(func() {
			none = unison.NewPanel()
			none.Accessibility.Name = "Says none"
			none.Accessibility.Callback = func(n *accessibility.Node) { n.Role = role.None }

			auto = unison.NewPanel()
			auto.Accessibility.Name = "Says auto"
			auto.Accessibility.Role = role.Button
			auto.Accessibility.Callback = func(n *accessibility.Node) { n.Role = role.Auto }

			// A panel whose children are described, to prove the ones beneath a node the callback tried to hide are
			// still where the tree said they were rather than being promoted out from under it.
			child := unison.NewLabel()
			child.SetTitle("Inside")
			provided = axColumn(child)
			provided.Accessibility.Name = "Holds something"
			provided.Accessibility.Callback = func(n *accessibility.Node) { n.Role = role.None }

			wnd = newHeadlessWindow(t, "unresolved roles", geom.NewRect(10, 10, 300, 300),
				axColumn(none, auto, provided))
		}))
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	c.True(tree != nil)
	if tree == nil {
		return
	}
	for _, one := range []struct {
		panel *unison.Panel
		name  string
	}{{panel: none, name: "Says none"}, {panel: auto, name: "Says auto"}, {panel: provided, name: "Holds something"}} {
		node := screen.AccessibilityNodeFor(one.panel)
		c.True(node != nil, one.name)
		if node != nil {
			c.Equal(role.Group, node.Role, "%s must be published as a group", one.name)
		}
	}
	providedNode := screen.AccessibilityNodeFor(provided)
	c.True(providedNode != nil)
	if providedNode != nil {
		c.Equal(1, len(providedNode.Children), "the panel's children stay beneath it")
	}
	tree.Walk(func(n *accessibility.Node) bool {
		c.NotEqual(role.Auto, n.Role, "no node in a published tree may hold role.Auto: %s", n.Name)
		c.NotEqual(role.None, n.Role, "no node in a published tree may hold role.None: %s", n.Name)
		return true
	})
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestAccessibilityGenerationSurvivesAWithdrawal verifies that the snapshot count a tree carries only ever goes up.
// Hiding a window releases everything built for it on this backend, the count included if it were held there, and an
// adapter tells a stale tree from a current one by that number: a window shown again after being hidden would hand it
// generations it had already been given, so a tree from before the window was hidden would look like the newer one.
func TestAccessibilityGenerationSurvivesAWithdrawal(t *testing.T) {
	c := check.New(t)
	var panel *unison.Panel
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			panel = axFocusablePanel("Content")
			wnd = newHeadlessWindow(t, "generations", geom.NewRect(20, 20, 240, 120), axColumn(panel))
		}))
	c.NotNil(wnd)

	first := screen.AccessibilityTree(wnd)
	c.True(first != nil)
	if first == nil {
		return
	}
	c.True(first.Generation >= 1, "the first tree is generation one or later")
	before := screen.AccessibilityTree(wnd).Generation
	c.True(before > first.Generation, "each description is a generation later than the one before it")

	screen.Do(func() { wnd.Hide() })
	c.True(screen.AccessibilityNodeFor(panel) == nil, "hiding the window withdraws the description on this backend")
	screen.Do(func() { wnd.Show() })
	after := screen.AccessibilityTree(wnd)
	c.True(after != nil)
	if after == nil {
		return
	}
	c.True(after.Generation > before,
		"the count must go on where it left off, not restart: %d after %d", after.Generation, before)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestAccessibilityWindowTitleChangeIsPublished verifies that renaming a window is reported. The title is the
// accessible name of the window itself, and on Linux and Windows the window element's name comes straight from that
// node, so a screen reader would go on reporting the name from before a "Save As" until something unrelated happened
// to redraw the window — which may be never, since nothing about setting a title repaints anything.
func TestAccessibilityWindowTitleChangeIsPublished(t *testing.T) {
	c := check.New(t)
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			wnd = newHeadlessWindow(t, "before", geom.NewRect(20, 20, 240, 120), axColumn(axFocusablePanel("Content")))
		}))
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	c.True(tree != nil)
	if tree == nil {
		return
	}
	root := tree.Node(tree.Root)
	c.True(root != nil)
	if root == nil {
		return
	}
	c.Equal("before", root.Name)
	screen.AccessibilityEvents(wnd)

	// Set and then read back without asking for the tree in between, so that what is being checked is the publish the
	// title change itself asked for rather than one this test provoked.
	screen.Do(func() { wnd.SetTitle("after") })
	screen.Sync()
	events := screen.AccessibilityEvents(wnd)
	c.True(axHasEvent(events, accessibility.NameChanged, tree.Root),
		"the window's new name should have been reported: %v", events)
	renamed := screen.AccessibilityTree(wnd)
	c.True(renamed != nil)
	if renamed != nil {
		c.Equal("after", renamed.Node(renamed.Root).Name)
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
