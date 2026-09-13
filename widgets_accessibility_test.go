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
// its tooltip, a button that stays drawn in the state a click puts it in is a toggle button, one that springs back is
// not, however it is grouped, and pressing it clicks it.
func TestButtonAccessibility(t *testing.T) {
	c := check.New(t)
	var plain, named, icon, first, second, loose *unison.Button
	var clicks int
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
			unison.NewGroup(first, second).Select(first)

			// Grouped but not sticky: DefaultDraw draws it exactly like any other unpressed button, whether or not the
			// group has it selected, so it must not be announced as a toggle button that is on.
			loose = unison.NewButton()
			loose.SetTitle("Loose")
			unison.NewGroup(loose).Select(loose)

			wnd = newHeadlessWindow(t, "buttons", geom.NewRect(10, 10, 400, 400),
				axColumn(plain, named, icon, first, second, loose))
		}))
	c.NotNil(wnd)

	screen.AccessibilityTree(wnd)
	plainNode := screen.AccessibilityNodeFor(plain)
	c.True(plainNode != nil)
	c.Equal(role.Button, plainNode.Role)
	c.Equal("Press Me", plainNode.Name, "a button's title is its name")
	c.False(plainNode.Pressed)
	c.True(plainNode.Actions.Has(accessibility.Press))

	namedNode := screen.AccessibilityNodeFor(named)
	c.True(namedNode != nil)
	c.Equal("Better Name", namedNode.Name, "an explicitly set name overrides the title")

	iconNode := screen.AccessibilityNodeFor(icon)
	c.True(iconNode != nil)
	c.Equal("Delete", iconNode.Name, "an icon button has only its tooltip to name it")
	c.Equal("", iconNode.Description, "the tooltip became the name, so it must not also be the description")

	firstNode := screen.AccessibilityNodeFor(first)
	secondNode := screen.AccessibilityNodeFor(second)
	c.True(firstNode != nil)
	c.True(secondNode != nil)
	c.Equal(role.ToggleButton, firstNode.Role, "a sticky button stays in the state a click puts it in")
	c.True(firstNode.Pressed, "the selected button in the group is the pressed one")
	c.False(secondNode.Pressed)

	looseNode := screen.AccessibilityNodeFor(loose)
	c.True(looseNode != nil)
	c.Equal(role.Button, looseNode.Role, "a grouped button that is not sticky springs back, so it is a plain button")
	c.False(looseNode.Pressed, "a button drawn unpressed must not be announced as an on toggle button")

	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   plainNode.ID,
		Action: accessibility.Press,
	}))
	var count int
	screen.Do(func() { count = clicks })
	c.Equal(1, count, "pressing the button should have clicked it exactly once")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestCheckBoxAccessibility verifies that all three states of a check box are reported and that both pressing and
// toggling it move it on to the next one.
func TestCheckBoxAccessibility(t *testing.T) {
	c := check.New(t)
	var box *unison.CheckBox
	var clicks int
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			box = unison.NewCheckBox()
			box.ClickAnimationTime = 0
			box.SetTitle("Enabled")
			box.ClickCallback = func() { clicks++ }
			wnd = newHeadlessWindow(t, "check box", geom.NewRect(10, 10, 240, 120), axColumn(box))
		}))
	c.NotNil(wnd)

	screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(box)
	c.True(node != nil)
	c.Equal(role.CheckBox, node.Role)
	c.Equal("Enabled", node.Name)
	c.True(node.HasCheck)
	c.Equal(checkenum.Off, node.Checked)
	c.True(node.Actions.Has(accessibility.Toggle))

	screen.Do(func() { box.State = checkenum.Mixed })
	screen.AccessibilityTree(wnd)
	node = screen.AccessibilityNodeFor(box)
	c.True(node != nil)
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
	node = screen.AccessibilityNodeFor(box)
	c.True(node != nil)
	c.Equal(checkenum.On, node.Checked)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestRadioButtonAccessibility verifies that a radio button is checked exactly when it is the one selected within its
// group, and that pressing another one moves the selection.
func TestRadioButtonAccessibility(t *testing.T) {
	c := check.New(t)
	var first, second *unison.RadioButton
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			first = unison.NewRadioButton()
			first.ClickAnimationTime = 0
			first.SetTitle("First")
			second = unison.NewRadioButton()
			second.ClickAnimationTime = 0
			second.SetTitle("Second")
			unison.NewGroup(first, second).Select(first)
			wnd = newHeadlessWindow(t, "radio", geom.NewRect(10, 10, 240, 120), axColumn(first, second))
		}))
	c.NotNil(wnd)

	screen.AccessibilityTree(wnd)
	firstNode := screen.AccessibilityNodeFor(first)
	secondNode := screen.AccessibilityNodeFor(second)
	c.True(firstNode != nil)
	c.True(secondNode != nil)
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
	firstNode = screen.AccessibilityNodeFor(first)
	secondNode = screen.AccessibilityNodeFor(second)
	c.True(firstNode != nil)
	c.True(secondNode != nil)
	c.Equal(checkenum.Off, firstNode.Checked, "the selection should have moved")
	c.Equal(checkenum.On, secondNode.Checked)
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
	textNode := screen.AccessibilityNodeFor(text)
	c.True(textNode != nil)
	c.Equal(role.Label, textNode.Role)
	c.Equal("Some text", textNode.Name)
	c.False(textNode.Ignored)

	imageNode := screen.AccessibilityNodeFor(image)
	c.True(imageNode != nil)
	c.Equal(role.Image, imageNode.Role, "a label holding only a drawable is an image")
	c.Equal("A trash can", imageNode.Name)
	c.False(imageNode.Ignored)

	unnamedNode := screen.AccessibilityNodeFor(unnamedImage)
	c.True(unnamedNode != nil)
	c.Equal(role.Image, unnamedNode.Role)
	c.Equal("", unnamedNode.Name, "there is nothing in a drawable that says what it shows")
	c.True(unnamedNode.Ignored, "an image nothing has described is skipped, exactly as a drawable panel's is")

	tippedNode := screen.AccessibilityNodeFor(tippedImage)
	c.True(tippedNode != nil)
	c.Equal(role.Image, tippedNode.Role)
	c.Equal("Discard", tippedNode.Name, "a tooltip is the only thing describing an otherwise nameless image")
	c.False(tippedNode.Ignored, "an image its tooltip describes is worth announcing")
	c.Equal("", tippedNode.Description, "the tooltip became the name, so it must not also be the description")

	emptyNode := screen.AccessibilityNodeFor(empty)
	c.True(emptyNode != nil)
	c.True(emptyNode.Ignored, "a label with nothing in it is not worth announcing")

	tagNode := screen.AccessibilityNodeFor(tag)
	c.True(tagNode != nil)
	c.Equal(role.Label, tagNode.Role, "a tag is the text inside its bubble")
	c.Equal("New", tagNode.Name)

	drawableTagNode := screen.AccessibilityNodeFor(drawableTag)
	c.True(drawableTagNode != nil)
	c.Equal(role.Image, drawableTagNode.Role, "a tag holding only a drawable is an image, not empty static text")
	c.True(drawableTagNode.Ignored, "an image nothing has described is skipped")

	tippedTagNode := screen.AccessibilityNodeFor(tippedTag)
	c.True(tippedTagNode != nil)
	c.Equal(role.Image, tippedTagNode.Role)
	c.Equal("Deprecated", tippedTagNode.Name, "a tag's tooltip names the drawable it holds")
	c.False(tippedTagNode.Ignored)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestLinkAccessibility verifies that a link says it is one, says where it leads, and follows itself when pressed.
func TestLinkAccessibility(t *testing.T) {
	c := check.New(t)
	const target = "https://example.com/docs"
	var link *unison.Label
	var followed string
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			link = unison.NewLink("Documentation", "", target, nil,
				func(_ unison.Paneler, where string) { followed = where })
			wnd = newHeadlessWindow(t, "link", geom.NewRect(10, 10, 300, 120), axColumn(link))
		}))
	c.NotNil(wnd)

	screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(link)
	c.True(node != nil)
	c.Equal(role.Link, node.Role)
	c.Equal("Documentation", node.Name)
	c.Equal(target, node.Description, "where the link leads is worth hearing")
	c.True(node.Actions.Has(accessibility.Press))

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
// what it shows — its tooltip counts, which is how markdown's alt text arrives — and that a separator says it is one.
func TestDrawablePanelAndSeparatorAccessibility(t *testing.T) {
	c := check.New(t)
	var named, unnamed, tipped *unison.DrawablePanel
	var separator *unison.Separator
	var wnd *unison.Window
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

			separator = unison.NewSeparator()

			wnd = newHeadlessWindow(t, "drawables", geom.NewRect(10, 10, 300, 200),
				axColumn(named, unnamed, tipped, separator))
		}))
	c.NotNil(wnd)

	screen.AccessibilityTree(wnd)
	namedNode := screen.AccessibilityNodeFor(named)
	c.True(namedNode != nil)
	c.Equal(role.Image, namedNode.Role)
	c.Equal("Diagram", namedNode.Name)
	c.False(namedNode.Ignored)

	unnamedNode := screen.AccessibilityNodeFor(unnamed)
	c.True(unnamedNode != nil)
	c.Equal(role.Image, unnamedNode.Role)
	c.True(unnamedNode.Ignored, "an image nothing has described is skipped")

	// The description a node takes from its tooltip is filled in after the widget has spoken, so an image left ignored
	// here would never be reached to hear it. The tooltip has to become the name instead.
	tippedNode := screen.AccessibilityNodeFor(tipped)
	c.True(tippedNode != nil)
	c.Equal(role.Image, tippedNode.Role)
	c.Equal("A photograph of a cat", tippedNode.Name, "a tooltip is what describes an otherwise nameless image")
	c.False(tippedNode.Ignored, "an image its tooltip describes must not be hidden")
	c.Equal("", tippedNode.Description, "the tooltip became the name, so it must not also be the description")

	separatorNode := screen.AccessibilityNodeFor(separator)
	c.True(separatorNode != nil)
	c.Equal(role.Separator, separatorNode.Role)
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
	node := screen.AccessibilityNodeFor(determinate)
	c.True(node != nil)
	c.Equal(role.ProgressBar, node.Role)
	c.Equal("Copying", node.Name, "the label before it names it, minus the colon")
	c.True(node.HasNumber)
	c.Equal(float64(50), node.Number)
	c.Equal(float64(0), node.Min)
	c.Equal(float64(200), node.Max)
	c.True(node.ReadOnly)
	c.False(node.Busy)

	busyNode := screen.AccessibilityNodeFor(indeterminate)
	c.True(busyNode != nil)
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
	node := screen.AccessibilityNodeFor(slider)
	c.True(node != nil)
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

	fineNode := screen.AccessibilityNodeFor(fine)
	c.True(fineNode != nil)
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
	node := screen.AccessibilityNodeFor(scroller)
	c.True(node != nil)
	c.Equal(role.ScrollArea, node.Role)

	var bar, quietBar *unison.ScrollBar
	screen.Do(func() {
		bar = scroller.Bar(false)
		quietBar = quiet.Bar(false)
	})
	barNode := screen.AccessibilityNodeFor(bar)
	c.True(barNode != nil)
	c.Equal(role.ScrollBar, barNode.Role)
	c.Equal(accessibility.OrientationVertical, barNode.Orientation)
	c.True(barNode.HasNumber)
	c.Equal(float64(0), barNode.Number)
	c.True(barNode.Max > 0, "there is more content than the view can show")
	c.False(barNode.Ignored)
	c.True(barNode.Actions.Has(accessibility.Increment))
	c.False(barNode.Actions.Has(accessibility.Press), "pressing a scroll bar would jump to the middle")

	quietBarNode := screen.AccessibilityNodeFor(quietBar)
	c.True(quietBarNode != nil)
	c.True(quietBarNode.Ignored, "a scroll bar with nothing to scroll has nothing to say")

	lastNode := screen.AccessibilityNodeFor(lastRow)
	c.True(lastNode != nil)
	c.True(lastNode.Offscreen, "a row below the view port has been scrolled out of sight")
	firstNode := screen.AccessibilityNodeFor(firstRow)
	c.True(firstNode != nil)
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
	node := screen.AccessibilityNodeFor(popup)
	c.True(node != nil)
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
	node := screen.AccessibilityNodeFor(combo)
	c.True(node != nil)
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
	node := screen.AccessibilityNodeFor(well)
	c.True(node != nil)
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
	c.True(tree != nil)
	colorNode := screen.AccessibilityNodeFor(colorEditor)
	c.True(colorNode != nil)
	c.Equal(role.Group, colorNode.Role)
	gradientNode := screen.AccessibilityNodeFor(gradientEditor)
	c.True(gradientNode != nil)
	c.Equal(role.Group, gradientNode.Role)
	fontNode := screen.AccessibilityNodeFor(fontPanel)
	c.True(fontNode != nil)
	c.Equal(role.Group, fontNode.Role)

	// The color editor pairs a label with both a slider and a field, and the field's neighbor is the slider rather than
	// the label, so only an explicit association can name it.
	redSlider := axNamedWithRole(tree, "Red", role.Slider)
	c.True(redSlider != nil, "the red slider should have been named by the label beside it")
	if redSlider != nil {
		c.Equal(1, len(redSlider.LabeledBy))
	}
	c.True(axNamed(tree, "Alpha") != nil)
	c.True(axNamed(tree, "Saturation") != nil)

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
