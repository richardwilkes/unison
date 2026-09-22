// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/enums/role"
)

// These tests cover the keyboard focus a heading takes for an assistive technology's sake — see Panel.axTakesFocus and
// axHeadingsTakeFocus: that it takes it only while one is being served and only where a screen reader asks a heading
// for the focus, that a plain label never takes it, and that a heading is not what a window with nothing focused hands
// the focus to when there is a real tab stop in it. Both answers are pinned rather than left to the platform the test
// happens to run on. A session owns most of the package's mutable globals while it runs, so none of these may call
// t.Parallel.

// axNewHeadingLabel returns a label marked up as a heading at the given depth, which is all an application does to
// make one.
func axNewHeadingLabel(title string, level int) *Label {
	l := NewLabel()
	l.SetTitle(title)
	l.Accessibility.Role = role.Heading
	l.Accessibility.Level = level
	return l
}

// TestHeadingTakesFocusOnlyWhereScreenReadersGrabIt verifies that a heading becomes focusable exactly while an
// assistive technology is being served on a platform whose screen reader lands on a heading by asking for the focus
// there, that the plain label beside it never does, and that a screen reader asking for the focus gets it.
func TestHeadingTakesFocusOnlyWhereScreenReadersGrabIt(t *testing.T) {
	c := check.New(t)
	var heading, plain *Label
	var field *Field
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			heading = axNewHeadingLabel("Settings", 1)
			plain = NewLabel()
			plain.SetTitle("Some text")
			field = NewField()
			column := NewPanel()
			column.SetLayout(&FlexLayout{Columns: 1})
			column.AddChild(heading)
			column.AddChild(plain)
			column.AddChild(field)
			wnd = axNewTestWindow(t, "heading focus", geom.NewRect(10, 10, 500, 500), column)
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()

	focusable := func(p Paneler) bool {
		var f bool
		screen.Do(func() { f = p.AsPanel().Focusable() })
		return f
	}
	focus := func() *Panel {
		var p *Panel
		screen.Do(func() { p = wnd.CurrentFocus() })
		return p
	}

	saved := axHeadingsTakeFocus
	// Written on the UI thread, which is the only thread that reads it, since focus and snapshots both live there.
	defer screen.Do(func() { axHeadingsTakeFocus = saved })
	screen.Do(func() { axHeadingsTakeFocus = true })

	c.False(focusable(heading), "with no assistive technology, a heading is not focusable")
	c.False(focusable(plain), "and a plain label never is")

	screen.EnableAccessibility()
	c.True(IsAccessibilityActive())
	c.True(focusable(heading), "while one is being served, a heading takes the focus so the jump to it can land")
	c.False(focusable(plain), "a plain label is read where it is and is never jumped to")
	tree := screen.AccessibilityTree(wnd)
	c.NotNil(tree)
	node := screen.AccessibilityNodeFor(heading)
	plainNode := screen.AccessibilityNodeFor(plain)
	c.NotNil(node)
	c.NotNil(plainNode)
	if tree == nil || node == nil || plainNode == nil {
		return
	}
	c.True(node.Focusable, "the node says it can take the focus")
	c.True(node.Actions.Has(accessibility.Focus), "and offers to")
	c.False(plainNode.Focusable)
	c.False(plainNode.Actions.Has(accessibility.Focus))

	screen.Do(func() { wnd.SetFocus(field) })
	screen.KeyPress(KeyTab, mod.None)
	c.True(focus().Is(heading), "a heading that can be landed on is a tab stop, so Tab wraps around onto it")

	screen.Do(func() { wnd.SetFocus(nil) })
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{Node: node.ID, Action: accessibility.Focus}))
	c.True(focus().Is(heading), "a screen reader asking for the focus on the heading gets it")
	tree = screen.AccessibilityTree(wnd)
	c.NotNil(tree)
	if tree != nil {
		c.Equal(node.ID, tree.Focus, "and the description reports the focus there")
	}

	// The focus is left on the heading, so that the Tab below has to move it: were the heading still a tab stop, the
	// move would wrap around onto it again and the field would never be reached.
	screen.Do(func() {
		wnd.SetFocus(heading)
		axHeadingsTakeFocus = false
	})
	c.True(focus().Is(heading), "the heading is where the focus was left")
	c.False(focusable(heading), "where a screen reader moves a cursor of its own, a heading is never a place to land")
	screen.KeyPress(KeyTab, mod.None)
	c.True(focus().Is(field), "so Tab passes it by and lands on the field, the only tab stop there is")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestHeadingDoesNotSeedTheFocus verifies that a window with nothing focused starts the person off at a real tab stop
// rather than at the heading beside it, whichever end the move of the focus starts from, and that a window holding
// nothing else still starts them at the heading, since a screen reader has to begin somewhere inside the content.
func TestHeadingDoesNotSeedTheFocus(t *testing.T) {
	c := check.New(t)
	var heading, trailing, lone *Label
	var field *Field
	var wnd, headingOnly *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			heading = axNewHeadingLabel("Network", 2)
			field = NewField()
			// A heading at each end, so that neither the forward seeding nor the backward one can reach a real tab
			// stop by simply taking the panel at its own end of the order.
			trailing = axNewHeadingLabel("Advanced", 2)
			column := NewPanel()
			column.SetLayout(&FlexLayout{Columns: 1})
			column.AddChild(heading)
			column.AddChild(field)
			column.AddChild(trailing)
			wnd = axNewTestWindow(t, "heading seeding", geom.NewRect(10, 10, 500, 300), column)

			lone = axNewHeadingLabel("Alone", 1)
			headingOnly = axNewTestWindow(t, "heading only", geom.NewRect(60, 60, 300, 200), lone)
		}))
	c.NotNil(wnd)
	c.NotNil(headingOnly)
	screen.Sync()

	saved := axHeadingsTakeFocus
	defer screen.Do(func() { axHeadingsTakeFocus = saved })
	screen.Do(func() { axHeadingsTakeFocus = true })
	screen.EnableAccessibility()

	var seeded, seededBackward, seededAlone, seededAloneBackward *Panel
	screen.Do(func() {
		wnd.SetFocus(nil)
		wnd.FocusNext()
		seeded = wnd.CurrentFocus()
		wnd.SetFocus(nil)
		wnd.FocusPrevious()
		seededBackward = wnd.CurrentFocus()
		headingOnly.SetFocus(nil)
		headingOnly.FocusNext()
		seededAlone = headingOnly.CurrentFocus()
		headingOnly.SetFocus(nil)
		headingOnly.FocusPrevious()
		seededAloneBackward = headingOnly.CurrentFocus()
	})
	c.True(seeded.Is(field), "the person is put in the first real tab stop, not on the heading above it")
	c.True(seededBackward.Is(field), "and moving the other way passes over the trailing heading just the same")
	c.True(seededAlone.Is(lone), "a window holding only a heading still starts a screen reader inside its content")
	c.True(seededAloneBackward.Is(lone), "from either end")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestDocumentSeedsTheFocus verifies the rest of the seeding rule FocusNext documents. A document can take the focus
// only for an assistive technology's sake — see Panel.axFocusable — so it is preferred over a heading but passed over
// for a real tab stop: a dialog whose first element is a Markdown explanation above its fields must still open with
// the person in the first field, where what they type goes somewhere, while a window holding nothing but the document
// hands it the focus, since that is where a screen reader has to begin. Both answers of axReadersFollowFocus are
// pinned rather than left to the platform the test happens to run on.
func TestDocumentSeedsTheFocus(t *testing.T) {
	c := check.New(t)
	var md, lone *Markdown
	var button *Button
	var wnd, docOnly *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			md = NewMarkdown(false)
			md.SetContent("# Title\n\nSome body text.\n", 400)
			button = NewButton()
			button.SetTitle("OK")
			column := NewPanel()
			column.SetLayout(&FlexLayout{Columns: 1})
			column.AddChild(md)
			column.AddChild(button)
			wnd = axNewTestWindow(t, "document seeding", geom.NewRect(10, 10, 500, 300), column)

			lone = NewMarkdown(false)
			lone.SetContent("# Alone\n\nNothing else to focus.\n", 400)
			docOnly = axNewTestWindow(t, "document only", geom.NewRect(60, 60, 300, 200), lone)
		}))
	c.NotNil(wnd)
	c.NotNil(docOnly)
	screen.Sync()

	saved := axReadersFollowFocus
	// Written on the UI thread, which is the only thread that reads it, since focus and snapshots both live there.
	defer screen.Do(func() { axReadersFollowFocus = saved })
	screen.EnableAccessibility()
	c.True(IsAccessibilityActive())

	for _, follows := range []bool{true, false} {
		var seeded, seededBackward, seededAlone *Panel
		screen.Do(func() {
			axReadersFollowFocus = follows
			wnd.SetFocus(nil)
			wnd.FocusNext()
			seeded = wnd.CurrentFocus()
			wnd.SetFocus(nil)
			wnd.FocusPrevious()
			seededBackward = wnd.CurrentFocus()
			docOnly.SetFocus(nil)
			docOnly.FocusNext()
			seededAlone = docOnly.CurrentFocus()
		})
		c.True(seeded.Is(button), "the person is put in the first real tab stop, not in the document above it: %v",
			follows)
		c.True(seededBackward.Is(button), "the last thing that can take the focus is the button either way: %v",
			follows)
		if follows {
			c.True(seededAlone.Is(lone), "a window holding nothing but a document starts a screen reader inside it")
		} else {
			c.True(seededAlone == nil, "where a document never takes the focus, there is nothing there to focus")
		}
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestSetFocusOnContainerSkipsHeading verifies that resolving a container to something that can hold the keyboard
// focus follows the same rule the seeding of an empty window does: a real tab stop is preferred over a heading, which
// can take the focus only for a screen reader's sake. Window.SetFocus on a container comes through
// Panel.FirstFocusableChild, and DockContainer.AcquireFocus takes exactly that path on every tab switch, so a
// container whose first focusable descendant is a heading would otherwise land the keyboard on the heading rather than
// on the first real tab stop of the panel that was just switched to. A container holding nothing else still lands on
// the heading, since a screen reader has to begin somewhere inside the content.
func TestSetFocusOnContainerSkipsHeading(t *testing.T) {
	c := check.New(t)
	var heading, lone *Label
	var field *Field
	var column, headingOnly *Panel
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			heading = axNewHeadingLabel("Network", 2)
			field = NewField()
			column = NewPanel()
			column.SetLayout(&FlexLayout{Columns: 1})
			column.AddChild(heading)
			column.AddChild(field)

			lone = axNewHeadingLabel("Alone", 1)
			headingOnly = NewPanel()
			headingOnly.SetLayout(&FlexLayout{Columns: 1})
			headingOnly.AddChild(lone)

			content := NewPanel()
			content.SetLayout(&FlexLayout{Columns: 1})
			content.AddChild(column)
			content.AddChild(headingOnly)
			wnd = axNewTestWindow(t, "container focus", geom.NewRect(10, 10, 500, 300), content)
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()

	saved := axHeadingsTakeFocus
	// Written on the UI thread, which is the only thread that reads it, since focus and snapshots both live there.
	defer screen.Do(func() { axHeadingsTakeFocus = saved })
	screen.Do(func() { axHeadingsTakeFocus = true })
	screen.EnableAccessibility()

	var landed, landedLast, landedAlone *Panel
	screen.Do(func() {
		wnd.SetFocus(nil)
		wnd.SetFocus(column)
		landed = wnd.CurrentFocus()
		landedLast = column.LastFocusableChild()
		wnd.SetFocus(nil)
		wnd.SetFocus(headingOnly)
		landedAlone = wnd.CurrentFocus()
	})
	c.True(landed.Is(field), "the keyboard lands on the first real tab stop, not on the heading above it")
	c.True(landedLast.Is(field), "and asking from the other end passes the heading over just the same")
	c.True(landedAlone.Is(lone), "a container holding only a heading still puts a screen reader inside its content")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
