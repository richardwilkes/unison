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
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/behavior"
	checkenum "github.com/richardwilkes/unison/enums/check"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/enums/role"
)

// These tests ask each widget what it tells an assistive technology about itself, and then ask it to do the things it
// says it can. Everything goes through the same path a screen reader's would: the window is described, the description
// is read back, and requests are handed to the node ids it named. A session owns most of the package's mutable globals
// while it runs, so none of these may call t.Parallel.

// axTestDrawable returns a drawable for the tests that need a widget to be holding one.
func axTestDrawable() unison.Drawable {
	return &unison.DrawableSVG{
		SVG:  unison.TrashSVG,
		Size: geom.NewSize(16, 16),
	}
}

// axMustNode returns node, ending the test at once when it is nil.
//
// Every lookup of a node is followed by assertions that read it, so a lookup that comes back nil — which is exactly the
// regression those assertions exist to catch — would dereference a nil pointer instead of failing. A panic on the test
// goroutine takes the whole test binary down with it, so every test that had not yet run reports nothing at all and the
// failures that were found are buried under a stack trace. Failing here reports the one thing that went wrong and lets
// the rest of the run finish. Helper() puts the calling line in the report, so a message is only worth passing when the
// line alone would not say which of several lookups came up empty.
func axMustNode(c check.Checker, node *accessibility.Node, msgAndArgs ...any) *accessibility.Node {
	c.Helper()
	if node == nil {
		c.Fatal(append([]any{"nothing describes this, so the assertions that follow it cannot be made:"},
			msgAndArgs...)...)
	}
	return node
}

// axRootChildCount returns how many children the root of the window's description has, which grows when an in-window
// menu opens.
func axRootChildCount(tree *accessibility.Tree) int {
	if tree == nil {
		return 0
	}
	root := tree.Node(tree.Root)
	if root == nil {
		return 0
	}
	return len(root.Children)
}

// TestButtonAccessibility verifies what a button says about itself: its title is its name, an icon button falls back to
// its tooltip, a sticky button in a group latches like a radio button and says so, one that springs back does not,
// however it is grouped, and pressing it clicks it.
func TestButtonAccessibility(t *testing.T) {
	c := check.New(t)
	var plain, named, icon, first, second, loose, lone *unison.Button
	var clicks, selections int
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 500},
		unison.StartupFinishedCallback(func() {
			plain = unison.NewButton()
			plain.ClickAnimationTime = 0
			plain.SetTitle("Press Me")
			plain.ClickCallback = func() { clicks++ }

			named = unison.NewButton()
			named.SetTitle("Untranslated")
			named.Accessibility.Name = "Better Name"

			icon = unison.NewSVGButton(unison.TrashSVG)
			icon.Tooltip = unison.NewTooltipWithText("Delete")

			first = unison.NewButton()
			first.SetTitle("One")
			first.Sticky = true
			second = unison.NewButton()
			second.SetTitle("Two")
			second.Sticky = true
			second.ClickCallback = func() { selections++ }
			unison.NewGroup(first, second).Select(first)

			// Grouped but not sticky: DefaultDraw draws it exactly like any other unpressed button, whether or not the
			// group has it selected, so it must not be announced as a toggle button that is on.
			loose = unison.NewButton()
			loose.SetTitle("Loose")
			unison.NewGroup(loose).Select(loose)

			// Sticky but in no group at all: nothing latches it, since DefaultDraw keeps a sticky button drawn
			// pressed only while its group has it selected and it has no group to be selected by.
			lone = unison.NewButton()
			lone.SetTitle("Lone")
			lone.Sticky = true

			wnd = newHeadlessWindow(t, "buttons", geom.NewRect(10, 10, 400, 400),
				axColumn(plain, named, icon, first, second, loose, lone))
		}))
	c.NotNil(wnd)

	screen.AccessibilityTree(wnd)
	plainNode := axMustNode(c, screen.AccessibilityNodeFor(plain))
	c.Equal(role.Button, plainNode.Role)
	c.Equal("Press Me", plainNode.Name, "a button's title is its name")
	c.False(plainNode.Pressed)
	c.True(plainNode.Actions.Has(accessibility.Press))

	namedNode := axMustNode(c, screen.AccessibilityNodeFor(named))
	c.Equal("Better Name", namedNode.Name, "an explicitly set name overrides the title")

	iconNode := axMustNode(c, screen.AccessibilityNodeFor(icon))
	c.Equal("Delete", iconNode.Name, "an icon button has only its tooltip to name it")
	c.Equal("", iconNode.Description, "the tooltip became the name, so it must not also be the description")

	firstNode := axMustNode(c, screen.AccessibilityNodeFor(first))
	secondNode := axMustNode(c, screen.AccessibilityNodeFor(second))
	c.Equal(role.RadioButton, firstNode.Role,
		"a sticky button that latches within a group is a radio button in everything but appearance")
	c.True(firstNode.HasCheck)
	c.Equal(checkenum.On, firstNode.Checked, "the selected button of the group is the checked one")
	c.Equal(checkenum.Off, secondNode.Checked)
	c.True(secondNode.Actions.Has(accessibility.Select), "the selection can be moved to it")

	looseNode := axMustNode(c, screen.AccessibilityNodeFor(loose))
	c.Equal(role.Button, looseNode.Role, "a grouped button that is not sticky springs back, so it is a plain button")
	c.False(looseNode.HasCheck, "a button drawn unpressed must not be announced as something with a state")
	c.False(looseNode.Actions.Has(accessibility.Select))

	loneNode := axMustNode(c, screen.AccessibilityNodeFor(lone))
	c.Equal(role.Button, loneNode.Role, "a sticky button with no group latches nothing, so it is a plain button")
	c.False(loneNode.HasCheck)
	c.False(loneNode.Actions.Has(accessibility.Select), "there is no group for it to be the selection of")
	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   loneNode.ID,
		Action: accessibility.Select,
	}), "selecting a button that belongs to no group cannot be carried out")

	// Selecting the other button of the group moves the check to it, which is the whole of what an assistive
	// technology can do to a latch and what it could not do at all while the pair were reported as toggle buttons.
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   secondNode.ID,
		Action: accessibility.Select,
	}))
	screen.AccessibilityTree(wnd)
	firstNode = axMustNode(c, screen.AccessibilityNodeFor(first))
	secondNode = axMustNode(c, screen.AccessibilityNodeFor(second))
	c.Equal(checkenum.Off, firstNode.Checked, "the button that was the selection gave it up")
	c.Equal(checkenum.On, secondNode.Checked)
	var selected int
	screen.Do(func() { selected = selections })
	c.Equal(1, selected,
		"a click on the same button tells the application, so a selection made by an assistive technology has to too")

	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   plainNode.ID,
		Action: accessibility.Press,
	}))
	var count int
	screen.Do(func() { count = clicks })
	c.Equal(1, count, "pressing the button should have clicked it exactly once")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestCheckBoxAccessibility verifies that all three states of a check box are reported, that both pressing and toggling
// it move it on to the next one, and that a box with an icon in place of a title is named by its tooltip rather than
// being announced as an unlabeled check box.
func TestCheckBoxAccessibility(t *testing.T) {
	c := check.New(t)
	var box, icon *unison.CheckBox
	var clicks int
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			box = unison.NewCheckBox()
			box.ClickAnimationTime = 0
			box.SetTitle("Enabled")
			box.ClickCallback = func() { clicks++ }
			icon = unison.NewCheckBox()
			icon.Drawable = axTestDrawable()
			icon.Tooltip = unison.NewTooltipWithText("Include deleted")
			wnd = newHeadlessWindow(t, "check box", geom.NewRect(10, 10, 240, 120), axColumn(box, icon))
		}))
	c.NotNil(wnd)

	screen.AccessibilityTree(wnd)
	node := axMustNode(c, screen.AccessibilityNodeFor(box))
	c.Equal(role.CheckBox, node.Role)
	c.Equal("Enabled", node.Name)
	c.True(node.HasCheck)
	c.Equal(checkenum.Off, node.Checked)
	c.True(node.Actions.Has(accessibility.Toggle))

	screen.Do(func() { box.State = checkenum.Mixed })
	screen.AccessibilityTree(wnd)
	node = axMustNode(c, screen.AccessibilityNodeFor(box))
	c.Equal(checkenum.Mixed, node.Checked, "a tri-state box reports its mixed state")

	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.Toggle,
	}))
	var state checkenum.Enum
	var count int
	screen.Do(func() {
		state = box.State
		count = clicks
	})
	c.Equal(checkenum.On, state, "toggling from mixed should have turned the box on")
	c.Equal(1, count)
	screen.AccessibilityTree(wnd)
	node = axMustNode(c, screen.AccessibilityNodeFor(box))
	c.Equal(checkenum.On, node.Checked)

	iconNode := screen.AccessibilityNodeFor(icon)
	c.True(iconNode != nil)
	if iconNode != nil {
		c.Equal("Include deleted", iconNode.Name, "a check box with no title has only its tooltip to name it")
		c.Equal("", iconNode.Description, "the tooltip became the name, so it must not also be the description")
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestRadioButtonAccessibility verifies that a radio button is checked exactly when it is the one selected within its
// group, and that pressing another one moves the selection.
func TestRadioButtonAccessibility(t *testing.T) {
	c := check.New(t)
	var first, second *unison.RadioButton
	var wnd *unison.Window
	var clicks, selections int
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			first = unison.NewRadioButton()
			first.ClickAnimationTime = 0
			first.SetTitle("First")
			first.ClickCallback = func() { selections++ }
			second = unison.NewRadioButton()
			second.ClickAnimationTime = 0
			second.SetTitle("Second")
			second.ClickCallback = func() { clicks++ }
			unison.NewGroup(first, second).Select(first)
			wnd = newHeadlessWindow(t, "radio", geom.NewRect(10, 10, 240, 120), axColumn(first, second))
		}))
	c.NotNil(wnd)

	screen.AccessibilityTree(wnd)
	firstNode := axMustNode(c, screen.AccessibilityNodeFor(first))
	secondNode := axMustNode(c, screen.AccessibilityNodeFor(second))
	c.Equal(role.RadioButton, firstNode.Role)
	c.Equal("First", firstNode.Name)
	c.True(firstNode.HasCheck)
	c.Equal(checkenum.On, firstNode.Checked, "the selected button in the group is the checked one")
	c.Equal(checkenum.Off, secondNode.Checked)

	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   secondNode.ID,
		Action: accessibility.Press,
	}))
	screen.AccessibilityTree(wnd)
	firstNode = axMustNode(c, screen.AccessibilityNodeFor(first))
	secondNode = axMustNode(c, screen.AccessibilityNodeFor(second))
	c.Equal(checkenum.Off, firstNode.Checked, "the selection should have moved")
	c.Equal(checkenum.On, secondNode.Checked)
	var count int
	screen.Do(func() { count = clicks })
	c.Equal(1, count, "pressing the button runs its callback, exactly as a click on it does")

	// Selecting is the other way an assistive technology moves the dot, and it has to tell the application as much as
	// pressing does: a person has made the same change either way, and an application that is never told goes on acting
	// on a value that no longer matches what is on the screen.
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   firstNode.ID,
		Action: accessibility.Select,
	}))
	screen.AccessibilityTree(wnd)
	firstNode = axMustNode(c, screen.AccessibilityNodeFor(first))
	secondNode = axMustNode(c, screen.AccessibilityNodeFor(second))
	c.Equal(checkenum.On, firstNode.Checked, "the selection should have moved back")
	c.Equal(checkenum.Off, secondNode.Checked)
	screen.Do(func() { count = selections })
	c.Equal(1, count, "selecting the button should have run its callback")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestLabelAccessibility verifies the shapes a label can take: text, a drawable with alternative text, a drawable
// described only by its tooltip, and nothing worth reporting at all. A tag takes the same shapes, so it is checked
// alongside: neither may leave a nameless image or an empty piece of static text in the tree, which is what a screen
// reader would otherwise stop on and have nothing to say about.
func TestLabelAccessibility(t *testing.T) {
	c := check.New(t)
	var text, image, unnamedImage, tippedImage, empty *unison.Label
	var tag, drawableTag, tippedTag *unison.Tag
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 400},
		unison.StartupFinishedCallback(func() {
			text = unison.NewLabel()
			text.SetTitle("Some text")

			image = unison.NewLabel()
			image.Drawable = axTestDrawable()
			image.Accessibility.Name = "A trash can"

			unnamedImage = unison.NewLabel()
			unnamedImage.Drawable = axTestDrawable()

			tippedImage = unison.NewLabel()
			tippedImage.Drawable = axTestDrawable()
			tippedImage.Tooltip = unison.NewTooltipWithText("Discard")

			empty = unison.NewLabel()

			tag = unison.NewTag()
			tag.SetTitle("New")

			drawableTag = unison.NewTag()
			drawableTag.Drawable = axTestDrawable()

			tippedTag = unison.NewTag()
			tippedTag.Drawable = axTestDrawable()
			tippedTag.Tooltip = unison.NewTooltipWithText("Deprecated")

			wnd = newHeadlessWindow(t, "labels", geom.NewRect(10, 10, 300, 300),
				axColumn(text, image, unnamedImage, tippedImage, empty, tag, drawableTag, tippedTag))
		}))
	c.NotNil(wnd)

	screen.AccessibilityTree(wnd)
	textNode := axMustNode(c, screen.AccessibilityNodeFor(text))
	c.Equal(role.Label, textNode.Role)
	c.Equal("Some text", textNode.Name)
	c.False(textNode.Ignored)

	imageNode := axMustNode(c, screen.AccessibilityNodeFor(image))
	c.Equal(role.Image, imageNode.Role, "a label holding only a drawable is an image")
	c.Equal("A trash can", imageNode.Name)
	c.False(imageNode.Ignored)

	unnamedNode := axMustNode(c, screen.AccessibilityNodeFor(unnamedImage))
	c.Equal(role.Image, unnamedNode.Role)
	c.Equal("", unnamedNode.Name, "there is nothing in a drawable that says what it shows")
	c.True(unnamedNode.Ignored, "an image nothing has described is skipped, exactly as a drawable panel's is")

	tippedNode := axMustNode(c, screen.AccessibilityNodeFor(tippedImage))
	c.Equal(role.Image, tippedNode.Role)
	c.Equal("Discard", tippedNode.Name, "a tooltip is the only thing describing an otherwise nameless image")
	c.False(tippedNode.Ignored, "an image its tooltip describes is worth announcing")
	c.Equal("", tippedNode.Description, "the tooltip became the name, so it must not also be the description")

	emptyNode := axMustNode(c, screen.AccessibilityNodeFor(empty))
	c.True(emptyNode.Ignored, "a label with nothing in it is not worth announcing")

	tagNode := axMustNode(c, screen.AccessibilityNodeFor(tag))
	c.Equal(role.Label, tagNode.Role, "a tag is the text inside its bubble")
	c.Equal("New", tagNode.Name)

	drawableTagNode := axMustNode(c, screen.AccessibilityNodeFor(drawableTag))
	c.Equal(role.Image, drawableTagNode.Role, "a tag holding only a drawable is an image, not empty static text")
	c.True(drawableTagNode.Ignored, "an image nothing has described is skipped")

	tippedTagNode := axMustNode(c, screen.AccessibilityNodeFor(tippedTag))
	c.Equal(role.Image, tippedTagNode.Role)
	c.Equal("Deprecated", tippedTagNode.Name, "a tag's tooltip names the drawable it holds")
	c.False(tippedTagNode.Ignored)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestLinkAccessibility verifies that a link says it is one, says where it leads, and follows itself when pressed.
func TestLinkAccessibility(t *testing.T) {
	c := check.New(t)
	const target = "https://example.com/docs"
	var link, tipped *unison.Label
	var followed string
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			link = unison.NewLink("Documentation", "", target, nil,
				func(_ unison.Paneler, where string) { followed = where })
			tipped = unison.NewLink("Guide", "Opens the guide", target, nil, nil)
			wnd = newHeadlessWindow(t, "link", geom.NewRect(10, 10, 300, 120), axColumn(link, tipped))
		}))
	c.NotNil(wnd)

	screen.AccessibilityTree(wnd)
	node := axMustNode(c, screen.AccessibilityNodeFor(link))
	c.Equal(role.Link, node.Role)
	c.Equal("Documentation", node.Name)
	c.Equal(target, node.Description, "where the link leads is worth hearing")
	c.True(node.Actions.Has(accessibility.Press))
	// A tooltip is words someone chose for this link, so it must win over the URL rather than be hidden by it.
	tippedNode := axMustNode(c, screen.AccessibilityNodeFor(tipped))
	c.Equal("Opens the guide", tippedNode.Description, "a tooltip should not be displaced by the target")

	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.Press,
	}))
	var went string
	screen.Do(func() { went = followed })
	c.Equal(target, went, "pressing the link should have followed it")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestDrawablePanelAndSeparatorAccessibility verifies that a drawable panel is an image only when something has said
// what it shows — its tooltip counts, which is how markdown's alt text arrives — that a panel an application has given
// a role of its own is left where an assistive technology can reach it whatever it has to say for itself, and that a
// separator says it is one and which way it runs.
func TestDrawablePanelAndSeparatorAccessibility(t *testing.T) {
	c := check.New(t)
	var named, unnamed, tipped, acting *unison.DrawablePanel
	var separator, vertical *unison.Separator
	var wnd *unison.Window
	pressed := 0
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			named = unison.NewDrawablePanel()
			named.Drawable = axTestDrawable()
			named.Accessibility.Name = "Diagram"

			unnamed = unison.NewDrawablePanel()
			unnamed.Drawable = axTestDrawable()

			tipped = unison.NewDrawablePanel()
			tipped.Drawable = axTestDrawable()
			tipped.Tooltip = unison.NewTooltipWithText("A photograph of a cat")

			// A drawable an application has made into a control of its own: it says what it is, takes the focus and
			// answers a click, so it is a live control however little it has to say about what it shows.
			acting = unison.NewDrawablePanel()
			acting.Drawable = axTestDrawable()
			acting.Accessibility.Role = role.Button
			acting.SetFocusable(true)
			acting.MouseDownCallback = func(_ geom.Point, _, _ int, _ mod.Modifiers) bool { return true }
			acting.MouseUpCallback = func(_ geom.Point, _ int, _ mod.Modifiers) bool {
				pressed++
				return true
			}

			separator = unison.NewSeparator()
			vertical = unison.NewSeparator()
			vertical.Vertical = true

			wnd = newHeadlessWindow(t, "drawables", geom.NewRect(10, 10, 300, 200),
				axColumn(named, unnamed, tipped, acting, separator, vertical))
		}))
	c.NotNil(wnd)

	screen.AccessibilityTree(wnd)
	namedNode := axMustNode(c, screen.AccessibilityNodeFor(named))
	c.Equal(role.Image, namedNode.Role)
	c.Equal("Diagram", namedNode.Name)
	c.False(namedNode.Ignored)

	unnamedNode := axMustNode(c, screen.AccessibilityNodeFor(unnamed))
	c.Equal(role.Image, unnamedNode.Role)
	c.True(unnamedNode.Ignored, "an image nothing has described is skipped")

	// The description a node takes from its tooltip is filled in after the widget has spoken, so an image left ignored
	// here would never be reached to hear it. The tooltip has to become the name instead.
	tippedNode := axMustNode(c, screen.AccessibilityNodeFor(tipped))
	c.Equal(role.Image, tippedNode.Role)
	c.Equal("A photograph of a cat", tippedNode.Name, "a tooltip is what describes an otherwise nameless image")
	c.False(tippedNode.Ignored, "an image its tooltip describes must not be hidden")
	c.Equal("", tippedNode.Description, "the tooltip became the name, so it must not also be the description")

	actingNode := screen.AccessibilityNodeFor(acting)
	c.True(actingNode != nil)
	if actingNode == nil {
		return
	}
	c.Equal(role.Button, actingNode.Role, "an explicitly set role is left alone")
	c.False(actingNode.Ignored, "a live, focusable, pressable control must not be spliced out of the tree")
	c.True(actingNode.Actions.Has(accessibility.Press))
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   actingNode.ID,
		Action: accessibility.Press,
	}))
	var count int
	screen.Do(func() { count = pressed })
	c.Equal(1, count, "pressing it should have reached the callbacks a click would have")

	separatorNode := screen.AccessibilityNodeFor(separator)
	c.True(separatorNode != nil)
	if separatorNode != nil {
		c.Equal(role.Separator, separatorNode.Role)
		c.Equal(accessibility.OrientationHorizontal, separatorNode.Orientation,
			"a separator drawn across the window divides what is above from what is below")
	}
	verticalNode := screen.AccessibilityNodeFor(vertical)
	c.True(verticalNode != nil)
	if verticalNode != nil {
		c.Equal(accessibility.OrientationVertical, verticalNode.Orientation, "a vertical separator says so")
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestProgressBarAccessibility verifies that a progress bar reports how far along it is, takes its name from the label
// beside it, and says it is busy rather than lying about a position it does not have.
func TestProgressBarAccessibility(t *testing.T) {
	c := check.New(t)
	var determinate, indeterminate *unison.ProgressBar
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			label := unison.NewLabel()
			label.SetTitle("Copying:")

			determinate = unison.NewProgressBar(200)
			determinate.SetCurrent(50)

			indeterminate = unison.NewProgressBar(0)
			// The animation reschedules itself after every draw, so it is slowed to something no test will wait for
			// rather than being left to keep the session busy.
			indeterminate.TickSpeed = time.Hour

			wnd = newHeadlessWindow(t, "progress", geom.NewRect(10, 10, 300, 200),
				axColumn(label, determinate, indeterminate))
		}))
	c.NotNil(wnd)

	screen.AccessibilityTree(wnd)
	node := axMustNode(c, screen.AccessibilityNodeFor(determinate))
	c.Equal(role.ProgressBar, node.Role)
	c.Equal("Copying", node.Name, "the label before it names it, minus the colon")
	c.True(node.HasNumber)
	c.Equal(float64(50), node.Number)
	c.Equal(float64(0), node.Min)
	c.Equal(float64(200), node.Max)
	c.True(node.ReadOnly)
	c.False(node.Busy)

	busyNode := axMustNode(c, screen.AccessibilityNodeFor(indeterminate))
	c.True(busyNode.Busy, "a bar with no maximum knows only that something is happening")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestSliderAccessibility verifies that a slider reports its range and position, that an assistive technology can step
// it in either direction or set it outright, and that it now answers the keyboard.
func TestSliderAccessibility(t *testing.T) {
	c := check.New(t)
	var slider, fine *unison.Slider
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 300},
		unison.StartupFinishedCallback(func() {
			label := unison.NewLabel()
			label.SetTitle("Red:")
			slider = unison.NewSlider(0, 255, 100)
			fine = unison.NewSlider(0, 1, 0.5)
			wnd = newHeadlessWindow(t, "slider", geom.NewRect(10, 10, 400, 200), axColumn(label, slider, fine))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() { wnd.ToFront() }))

	screen.AccessibilityTree(wnd)
	node := axMustNode(c, screen.AccessibilityNodeFor(slider))
	c.Equal(role.Slider, node.Role)
	c.Equal("Red", node.Name, "the label before it names it")
	c.True(node.HasNumber)
	c.Equal(float64(100), node.Number)
	c.Equal(float64(0), node.Min)
	c.Equal(float64(255), node.Max)
	c.Equal(float64(1), node.Step, "a range of whole numbers steps by one of them")
	c.Equal(accessibility.OrientationHorizontal, node.Orientation)
	c.True(node.Focusable, "a slider can now be reached from the keyboard")
	c.True(node.Actions.Has(accessibility.Increment))
	c.True(node.Actions.Has(accessibility.Decrement))
	c.True(node.Actions.Has(accessibility.SetValue))
	c.False(node.Actions.Has(accessibility.Press), "pressing a slider would throw its value to the middle")

	fineNode := axMustNode(c, screen.AccessibilityNodeFor(fine))
	c.Equal(float64(float32(0.05)), fineNode.Step, "a range smaller than twenty units steps by a twentieth of it")

	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.Increment,
	}))
	var value float32
	screen.Do(func() { value = slider.Value() })
	c.Equal(float32(101), value, "incrementing should have moved the value by one step")

	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.Decrement,
	}))
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.SetValue,
		Number: 42,
	}))
	screen.Do(func() { value = slider.Value() })
	c.Equal(float32(42), value)

	// The keyboard behavior the slider gained: focus it, then walk it with the arrow keys and the ends.
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.Focus,
	}))
	var focused bool
	screen.Do(func() { focused = slider.Focused() })
	c.True(focused, "the slider should be holding the focus")

	screen.KeyPress(unison.KeyRight, mod.None)
	screen.Do(func() { value = slider.Value() })
	c.Equal(float32(43), value, "the right arrow raises the value")
	screen.KeyPress(unison.KeyLeft, mod.None)
	screen.KeyPress(unison.KeyLeft, mod.None)
	screen.Do(func() { value = slider.Value() })
	c.Equal(float32(41), value, "the left arrow lowers it")
	screen.KeyPress(unison.KeyUp, mod.None)
	screen.Do(func() { value = slider.Value() })
	c.Equal(float32(42), value, "the up arrow raises it")
	screen.KeyPress(unison.KeyDown, mod.None)
	screen.Do(func() { value = slider.Value() })
	c.Equal(float32(41), value, "the down arrow lowers it")
	screen.KeyPress(unison.KeyHome, mod.None)
	screen.Do(func() { value = slider.Value() })
	c.Equal(float32(0), value, "Home goes to the start of the range")
	screen.KeyPress(unison.KeyEnd, mod.None)
	screen.Do(func() { value = slider.Value() })
	c.Equal(float32(255), value, "End goes to the end of it")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestScrollPanelAccessibility verifies that a scroll panel says it is a scrollable view, that its bars are reported
// only when there is something to scroll, that stepping a bar scrolls the content, and that what has been scrolled out
// of sight is reported as offscreen.
func TestScrollPanelAccessibility(t *testing.T) {
	c := check.New(t)
	var scroller, quiet *unison.ScrollPanel
	var firstRow, lastRow *unison.Label
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			rows := make([]unison.Paneler, 40)
			labels := make([]*unison.Label, len(rows))
			for i := range rows {
				labels[i] = unison.NewLabel()
				labels[i].SetTitle("Row")
				rows[i] = labels[i]
			}
			firstRow = labels[0]
			lastRow = labels[len(labels)-1]
			scroller = unison.NewScrollPanel()
			scroller.SetContent(axColumn(rows...), behavior.Unmodified, behavior.Unmodified)
			// The view port is deliberately far shorter than the rows within it, so that most of them are out of sight.
			scroller.SetLayoutData(&unison.FlexLayoutData{
				SizeHint: geom.NewSize(200, 100),
				HAlign:   align.Fill,
				VAlign:   align.Start,
			})

			quiet = unison.NewScrollPanel()
			small := unison.NewLabel()
			small.SetTitle("Nothing to scroll")
			quiet.SetContent(small, behavior.Unmodified, behavior.Unmodified)
			quiet.SetLayoutData(&unison.FlexLayoutData{
				SizeHint: geom.NewSize(200, 100),
				HAlign:   align.Fill,
				VAlign:   align.Start,
			})

			wnd = newHeadlessWindow(t, "scrolling", geom.NewRect(10, 10, 300, 300), axColumn(scroller, quiet))
		}))
	c.NotNil(wnd)

	screen.AccessibilityTree(wnd)
	node := axMustNode(c, screen.AccessibilityNodeFor(scroller))
	c.Equal(role.ScrollArea, node.Role)

	var bar, quietBar *unison.ScrollBar
	screen.Do(func() {
		bar = scroller.Bar(false)
		quietBar = quiet.Bar(false)
	})
	barNode := axMustNode(c, screen.AccessibilityNodeFor(bar))
	c.Equal(role.ScrollBar, barNode.Role)
	c.Equal(accessibility.OrientationVertical, barNode.Orientation)
	c.True(barNode.HasNumber)
	c.Equal(float64(0), barNode.Number)
	c.True(barNode.Max > 0, "there is more content than the view can show")
	c.False(barNode.Ignored)
	c.True(barNode.Actions.Has(accessibility.Increment))
	c.False(barNode.Actions.Has(accessibility.Press), "pressing a scroll bar would jump to the middle")

	quietBarNode := axMustNode(c, screen.AccessibilityNodeFor(quietBar))
	c.True(quietBarNode.Ignored, "a scroll bar with nothing to scroll has nothing to say")

	lastNode := axMustNode(c, screen.AccessibilityNodeFor(lastRow))
	c.True(lastNode.Offscreen, "a row below the view port has been scrolled out of sight")
	firstNode := axMustNode(c, screen.AccessibilityNodeFor(firstRow))
	c.False(firstNode.Offscreen)

	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   barNode.ID,
		Action: accessibility.Increment,
	}))
	var position float32
	screen.Do(func() { _, position = scroller.Position() })
	c.True(position > 0, "incrementing the bar should have scrolled the content")

	// Setting the bar's value outright scrolls straight there, which is how VoiceOver reveals what lies past the edge.
	c.True(barNode.Actions.Has(accessibility.SetValue))
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   barNode.ID,
		Action: accessibility.SetValue,
		Number: 40,
	}))
	screen.Do(func() { _, position = scroller.Position() })
	c.Equal(float32(40), position, "setting the bar's value should have scrolled the content there")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestPopupMenuAccessibility verifies that a popup menu reports the choice it is showing and says it can be expanded.
func TestPopupMenuAccessibility(t *testing.T) {
	c := check.New(t)
	var popup *unison.PopupMenu[string]
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			label := unison.NewLabel()
			label.SetTitle("Weight:")
			popup = unison.NewPopupMenu[string]()
			popup.AddItem("Light", "Regular", "Bold")
			popup.Select("Regular")
			wnd = newHeadlessWindow(t, "popup", geom.NewRect(10, 10, 300, 150), axColumn(label, popup))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() { wnd.ToFront() }))

	screen.AccessibilityTree(wnd)
	node := axMustNode(c, screen.AccessibilityNodeFor(popup))
	c.Equal(role.PopupButton, node.Role)
	c.Equal("Weight", node.Name, "the label before it names it")
	c.Equal("Regular", node.Value, "its value is the choice it is showing")
	c.True(node.Expandable)
	c.True(node.Actions.Has(accessibility.Expand))
	c.True(node.Actions.Has(accessibility.Press))

	before := axRootChildCount(screen.AccessibilityTree(wnd))
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.Expand,
	}))
	c.Equal(before+1, axRootChildCount(screen.AccessibilityTree(wnd)),
		"expanding the popup menu should have opened its menu within the window")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestComboFieldAccessibility verifies that a combo field is described as one control rather than two, and that asking
// it to expand opens its list of choices.
func TestComboFieldAccessibility(t *testing.T) {
	c := check.New(t)
	first := "One"
	second := "Two"
	var combo *unison.Field
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			combo = unison.NewComboField([]*string{&first, &second}, &first, nil)
			wnd = newHeadlessWindow(t, "combo", geom.NewRect(10, 10, 300, 150), axColumn(combo))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() { wnd.ToFront() }))

	tree := screen.AccessibilityTree(wnd)
	node := axMustNode(c, screen.AccessibilityNodeFor(combo))
	c.Equal(role.ComboBox, node.Role)
	c.True(node.Expandable)
	c.True(node.Actions.Has(accessibility.Expand))
	c.True(node.Actions.Has(accessibility.ShowContextMenu))
	c.Equal(0, len(tree.UnignoredChildren(node.ID)),
		"the dropdown button is part of the field rather than an element of its own")

	before := axRootChildCount(tree)
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.Expand,
	}))
	c.Equal(before+1, axRootChildCount(screen.AccessibilityTree(wnd)),
		"expanding the combo field should have opened its menu within the window")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestFieldAccessibilityWrappedSingleLineReportsMultiline verifies that a single-line field that wraps says its content
// is laid out over more than one line. TextInfo.Multiline is what an adapter reads to decide whether to offer line
// navigation, and such a field really does lay its content out over several — buildLines breaks the one line it holds
// to the field's width — so one that said otherwise while handing back a line for each of them was the pair an adapter
// could make no sense of.
func TestFieldAccessibilityWrappedSingleLineReportsMultiline(t *testing.T) {
	c := check.New(t)
	const sentence = "a sentence with quite enough words in it to need more than one line here"
	var wrapped, plain *unison.Field
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			// The size hint pins the width whatever the text would rather have, so the wrap is the layout's doing
			// rather than the window's size.
			wrapped = unison.NewField()
			wrapped.SetWrap(true)
			wrapped.SetText(sentence)
			wrapped.SetLayoutData(&unison.FlexLayoutData{
				SizeHint: geom.NewSize(120, 80),
				HAlign:   align.Fill,
				VAlign:   align.Fill,
			})
			plain = unison.NewField()
			plain.SetText(sentence)
			plain.SetLayoutData(&unison.FlexLayoutData{
				SizeHint: geom.NewSize(120, 80),
				HAlign:   align.Fill,
				VAlign:   align.Fill,
			})
			wnd = newHeadlessWindow(t, "wrapped field", geom.NewRect(10, 10, 300, 300), axColumn(wrapped, plain))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	// Where the lines fall is measured only for the field the person is working in, so the focus goes there.
	c.True(screen.Do(func() { wrapped.RequestFocus() }))

	screen.AccessibilityTree(wnd)
	node := axMustNode(c, screen.AccessibilityNodeFor(wrapped))
	c.Equal(role.TextField, node.Role, "a field that accepts no line feeds is a text field however it is drawn")
	c.NotNil(node.Text)
	if node.Text == nil {
		return
	}
	c.True(len(node.Text.Lines) > 1, "the text should have wrapped, got %d line(s)", len(node.Text.Lines))
	c.True(node.Text.Multiline, "a field whose content is laid out over more than one line says so")

	plainNode := axMustNode(c, screen.AccessibilityNodeFor(plain))
	c.NotNil(plainNode.Text)
	if plainNode.Text != nil {
		c.False(plainNode.Text.Multiline,
			"a single-line field that does not wrap holds the one line, however wide it is")
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestWellAccessibility verifies that a well reports the ink it is holding and runs its click callback when pressed.
func TestWellAccessibility(t *testing.T) {
	c := check.New(t)
	red := unison.RGB(255, 0, 0)
	var well *unison.Well
	var clicks int
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			label := unison.NewLabel()
			label.SetTitle("Fill:")
			well = unison.NewWell()
			well.ClickAnimationTime = 0
			well.SetInk(red)
			well.ClickCallback = func() { clicks++ }
			wnd = newHeadlessWindow(t, "well", geom.NewRect(10, 10, 300, 150), axColumn(label, well))
		}))
	c.NotNil(wnd)

	screen.AccessibilityTree(wnd)
	node := axMustNode(c, screen.AccessibilityNodeFor(well))
	c.Equal(role.ColorWell, node.Role)
	c.Equal("Fill", node.Name, "the label before it names it")
	c.Equal(red.String(), node.Value, "its value is the color it is holding")
	c.True(node.Actions.Has(accessibility.Press))

	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.Press,
	}))
	var count int
	screen.Do(func() { count = clicks })
	c.Equal(1, count, "pressing the well should have clicked it")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestCompositeEditorAccessibility verifies that the editors the toolkit assembles out of other widgets say what each
// of their controls is, whether by pointing at the label beside it or, where there is no label, by naming it outright.
func TestCompositeEditorAccessibility(t *testing.T) {
	c := check.New(t)
	var colorEditor *unison.ColorEditor
	var gradientEditor *unison.GradientEditor
	var fontPanel *unison.FontPanel
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 900, Height: 900},
		unison.StartupFinishedCallback(func() {
			colorEditor = unison.NewColorEditor(unison.RGB(10, 20, 30))
			gradientEditor = unison.NewGradientEditor(nil)
			fontPanel = unison.NewFontPanel()
			wnd = newHeadlessWindow(t, "editors", geom.NewRect(10, 10, 800, 800),
				axColumn(colorEditor, gradientEditor, fontPanel))
		}))
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	c.NotNil(tree)
	colorNode := axMustNode(c, screen.AccessibilityNodeFor(colorEditor))
	c.Equal(role.Group, colorNode.Role)
	gradientNode := axMustNode(c, screen.AccessibilityNodeFor(gradientEditor))
	c.Equal(role.Group, gradientNode.Role)
	fontNode := axMustNode(c, screen.AccessibilityNodeFor(fontPanel))
	c.Equal(role.Group, fontNode.Role)

	// The color editor pairs a label with both a slider and a field, and the field's neighbor is the slider rather than
	// the label, so only an explicit association can name it.
	redSlider := axMustNode(c, axNamedWithRole(tree, "Red", role.Slider),
		"the red slider should have been named by the label beside it")
	c.Equal(1, len(redSlider.LabeledBy))
	// The role has to be named along with the name. The label the color editor puts beside each pair carries that very
	// text, and a search by name alone stops at the label, which says nothing at all about whether the two controls
	// beside it were named.
	c.True(axNamedWithRole(tree, "Alpha", role.Slider) != nil, "the alpha slider should have been named")
	c.True(axNamedWithRole(tree, "Alpha", role.TextField) != nil,
		"the alpha field can only be named by the association the color editor sets up for it")
	c.True(axNamedWithRole(tree, "Saturation", role.Slider) != nil, "the saturation slider should have been named")
	c.True(axNamedWithRole(tree, "Saturation", role.TextField) != nil,
		"the saturation field can only be named by the association the color editor sets up for it")

	// The gradient editor's icon buttons have no text at all, so they name themselves.
	c.True(axNamed(tree, "Add Stop") != nil)
	c.True(axNamed(tree, "Remove Stop") != nil)
	c.True(axNamed(tree, "Gradient Stops") != nil)
	c.True(axNamedWithRole(tree, "Gradient Type", role.PopupButton) != nil)
	c.True(axNamedWithRole(tree, "Tile Mode", role.PopupButton) != nil)
	c.True(axNamedWithRole(tree, "Color", role.ColorWell) != nil,
		"the gradient editor's color well should have been named by the label beside it")

	// The font panel is a row of controls with no labels at all.
	c.True(axNamed(tree, "Font Size") != nil)
	c.True(axNamedWithRole(tree, "Font Family", role.PopupButton) != nil)
	c.True(axNamedWithRole(tree, "Font Weight", role.PopupButton) != nil)
	c.True(axNamedWithRole(tree, "Font Slant", role.PopupButton) != nil)
	c.True(axNamedWithRole(tree, "Font Spacing", role.PopupButton) != nil)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// axNamedWithRole returns the first node in the tree with the given name and role, or nil if there is none.
func axNamedWithRole(tree *accessibility.Tree, name string, r role.Enum) *accessibility.Node {
	var found *accessibility.Node
	tree.Walk(func(n *accessibility.Node) bool {
		if n.Name == name && n.Role == r {
			found = n
			return false
		}
		return true
	})
	return found
}

// TestMenuItemAccessibilityExpandDefersOpeningTheSubMenu verifies that asking an item with a sub-menu to expand is
// answered at once and the sub-menu built afterwards. Opening one goes through menu.createPopup to menu.newPanel, which
// runs the menu's updater and then every one of its items' validators — application code — and Expand is classified as
// a navigation action, so on macOS it is carried out inline within the AppKit callback VoiceOver is waiting on. That is
// precisely what PopupMenu.axExpandLater defers the equivalent click to avoid, and the two paths have to agree.
func TestMenuItemAccessibilityExpandDefersOpeningTheSubMenu(t *testing.T) {
	c := check.New(t)
	const menuID = unison.UserBaseID + 300
	var wnd *unison.Window
	updates := 0
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			wnd = newHeadlessWindow(t, "sub-menu expand", geom.NewRect(10, 10, 400, 300), unison.NewPanel())
			if wnd == nil {
				return
			}
			unison.DefaultMenuFactory().BarForWindow(wnd, func(bar unison.Menu) {
				f := bar.Factory()
				edit := f.NewMenu(menuID, "Edit", nil)
				more := f.NewMenu(menuID+1, "More", func(_ unison.Menu) { updates++ })
				more.InsertItem(-1, f.NewItem(menuID+2, "Deeper", unison.KeyBinding{}, nil, nil))
				edit.InsertMenu(-1, more)
				bar.InsertMenu(-1, edit)
			})
			wnd.ToFront()
		}))
	c.NotNil(wnd)
	screen.Sync()

	tree := screen.AccessibilityTree(wnd)
	title := axMustNode(c, axNamed(tree, "Edit"), "the bar carries the one menu that was added to it")
	screen.Click(axScreenPoint(screen, wnd, title))
	tree = screen.AccessibilityTree(wnd)
	c.Equal(1, len(axNodesWithRole(tree, role.Menu)), "clicking the title should have opened the menu")
	more := axMustNode(c, axNamed(tree, "More"), "the item that opens the sub-menu")
	c.True(more.Expandable)
	c.True(more.Actions.Has(accessibility.Expand))

	// Performing the request from the UI thread is what makes the two halves of it observable: the task the request
	// queues cannot run until the closure has returned to the event loop, so what is seen inside the closure is exactly
	// what an assistive technology waiting on the callback would have been handed.
	var handled bool
	var updatesDuring, menusDuring int
	c.True(screen.Do(func() {
		handled = screen.PerformAccessibilityAction(accessibility.ActionRequest{
			Node:   more.ID,
			Action: accessibility.Expand,
		})
		updatesDuring = updates
		menusDuring = len(axNodesWithRole(screen.AccessibilityTree(wnd), role.Menu))
	}))
	c.True(handled, "the request is accepted, and what came of it shows in the next description of the item")
	c.Equal(0, updatesDuring, "the sub-menu's updater must not have run from within the callback")
	c.Equal(1, menusDuring, "nor may the sub-menu have been built there")

	tree = screen.AccessibilityTree(wnd)
	c.Equal(2, len(axNodesWithRole(tree, role.Menu)), "the queued task should have opened the sub-menu")
	c.Equal(1, updates, "and run the updater exactly once while doing it")
	expanded := axMustNode(c, tree.Node(more.ID))
	c.True(expanded.Expanded, "the item says its sub-menu is showing")
	c.True(axNamed(tree, "Deeper") != nil, "what the sub-menu holds is described")

	// Asking again for what is already there changes nothing, rather than tearing the sub-menu down and building it
	// back up.
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   more.ID,
		Action: accessibility.Expand,
	}))
	tree = screen.AccessibilityTree(wnd)
	c.Equal(2, len(axNodesWithRole(tree, role.Menu)))
	c.Equal(1, updates, "the sub-menu that was already showing was left alone")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestClickTakesFocusOnlyWhileAccessibilityIsActive covers the two halves of the promise Panel.axFocusOnClick makes.
// While no assistive technology is being served, clicking a check box, a radio button, a button, a popup menu or a
// color well leaves the keyboard focus where it was, as it always has, so that a click on one of them does not pull the
// focus out of the text field a person is typing in. Once one is being served, the same click moves the focus to the
// control, and the description published for it reports the control's new state before it reports the focus, so that
// what a screen reader speaks is the control as it now is rather than a state change on an element it was not watching.
func TestClickTakesFocusOnlyWhileAccessibilityIsActive(t *testing.T) {
	c := check.New(t)
	var field *unison.Field
	var box *unison.CheckBox
	var first, second *unison.RadioButton
	var button *unison.Button
	var popup *unison.PopupMenu[string]
	var well *unison.Well
	var wellClicks int
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 400},
		unison.StartupFinishedCallback(func() {
			field = unison.NewField()
			box = unison.NewCheckBox()
			box.SetTitle("Enabled")
			first = unison.NewRadioButton()
			first.SetTitle("First")
			second = unison.NewRadioButton()
			second.SetTitle("Second")
			unison.NewGroup(first, second).Select(first)
			button = unison.NewButton()
			button.SetTitle("Bold")
			button.Sticky = true
			unison.NewGroup(button)
			popup = unison.NewPopupMenu[string]()
			popup.AddItem("Light", "Regular", "Bold")
			popup.Select("Regular")
			well = unison.NewWell()
			// Counted rather than left to the default, which opens a dialog the test would then have to drive.
			well.ClickCallback = func() { wellClicks++ }
			wnd = newHeadlessWindow(t, "click focus", geom.NewRect(10, 10, 300, 320),
				axColumn(field, box, first, second, button, popup, well))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()

	type control struct {
		panel     unison.Paneler
		landed    func() bool
		name      string
		opensMenu bool
	}
	controls := []control{
		{name: "check box", panel: box, landed: func() bool { return box.State == checkenum.On }},
		{name: "radio button", panel: second, landed: func() bool { return second.Group().Selected(second) }},
		{name: "button", panel: button, landed: func() bool { return button.Group().Selected(button) }},
		{name: "popup menu", panel: popup, opensMenu: true},
		{name: "color well", panel: well, landed: func() bool { return wellClicks > 0 }},
	}
	focusField := func() {
		screen.Do(func() { field.RequestFocus() })
	}
	focused := func() *unison.Panel {
		var p *unison.Panel
		screen.Do(func() { p = wnd.CurrentFocus() })
		return p
	}
	// click clicks the control and, for the one that opens a menu, closes the menu again so that the focus the window
	// reports is the control's own rather than the menu's.
	click := func(ctl control) {
		screen.Click(screen.PanelCenter(ctl.panel))
		if ctl.opensMenu {
			screen.KeyPress(unison.KeyEscape, mod.None)
		}
		if ctl.landed != nil {
			var landed bool
			screen.Do(func() { landed = ctl.landed() })
			c.True(landed, "the click on the %s should have landed", ctl.name)
		}
	}

	// Nothing is being served, so a click changes the control and nothing else.
	for _, ctl := range controls {
		focusField()
		click(ctl)
		c.True(focused().Is(field), "with no assistive technology, clicking the %s must leave the focus on the field",
			ctl.name)
	}

	// Now one is. The check box is the one whose events are read, since its state change is the one a person is most
	// likely to miss: the box is drawn ticked, and a screen reader that was watching the field says nothing.
	screen.EnableAccessibility()
	screen.Do(func() {
		box.State = checkenum.Off
		box.MarkForRedraw()
	})
	for _, ctl := range controls {
		focusField()
		screen.AccessibilityEvents(wnd)
		click(ctl)
		events := screen.AccessibilityEvents(wnd)
		tree := screen.AccessibilityTree(wnd)
		c.NotNil(tree)
		node := axMustNode(c, screen.AccessibilityNodeFor(ctl.panel), ctl.name)
		c.Equal(node.ID, tree.Focus, "with an assistive technology, clicking the %s must move the focus to it",
			ctl.name)
		c.True(focused().Is(ctl.panel), "and the window's own focus must agree for the %s", ctl.name)
		if ctl.panel != box {
			continue
		}
		checked, focus := -1, -1
		for i, e := range events {
			switch {
			case e.Kind == accessibility.StateChanged && e.State == accessibility.StateChecked && e.Node == node.ID:
				checked = i
			case e.Kind == accessibility.FocusChanged && e.Node == node.ID:
				focus = i
			}
		}
		c.True(checked >= 0, "the click's change to the check state should have been reported: %v", events)
		c.True(focus >= 0, "as should the focus moving to the box: %v", events)
		c.True(focus > checked, "and the state before the focus, so the box is read in its new state: %v", events)
		c.Equal(checkenum.On, node.Checked)
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
