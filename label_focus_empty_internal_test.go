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
	"github.com/richardwilkes/unison/enums/role"
)

// TestEmptyHeadingNeverTakesFocus verifies that a label with nothing to announce does not become a place a screen
// reader can land merely because the application marked it up as a heading, and that one which draws no text but was
// given a name of its own does. The first is spacing, and says nothing when it is jumped to, so Orca's structural
// navigation must not be able to stop on it; the second is announced by its name, so a jump to it has somewhere to
// land and must not be left in the list of headings with no way to reach it. See Panel.axTakesFocus, which is what
// decides both. A session owns most of the package's mutable globals while it runs, so this may not call t.Parallel.
func TestEmptyHeadingNeverTakesFocus(t *testing.T) {
	c := check.New(t)
	var empty, named, titled *Label
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			empty = NewLabel()
			empty.Accessibility.Role = role.Heading
			empty.Accessibility.Level = 1
			// Draws nothing at all, but the application said what it is, so a screen reader has something to say on
			// landing there.
			named = NewLabel()
			named.Accessibility.Role = role.Heading
			named.Accessibility.Level = 1
			named.Accessibility.Name = "Printers"
			titled = axNewHeadingLabel("Network", 1)
			column := NewPanel()
			column.SetLayout(&FlexLayout{Columns: 1})
			column.AddChild(empty)
			column.AddChild(named)
			column.AddChild(titled)
			wnd = axNewTestWindow(t, "empty heading focus", geom.NewRect(10, 10, 500, 300), column)
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

	var emptyFocusable, namedFocusable, titledFocusable bool
	screen.Do(func() {
		emptyFocusable = empty.AsPanel().Focusable()
		namedFocusable = named.AsPanel().Focusable()
		titledFocusable = titled.AsPanel().Focusable()
	})
	c.False(emptyFocusable, "a heading with nothing to announce is not somewhere to land")
	c.True(namedFocusable, "one that draws nothing but was named is announced by that name, so the jump can land")
	c.True(titledFocusable, "while the heading beside it, which has text, still is")

	c.NotNil(screen.AccessibilityTree(wnd))
	node := screen.AccessibilityNodeFor(empty)
	c.NotNil(node)
	if node != nil {
		c.False(node.Focusable, "the node says as much")
		c.False(node.Actions.Has(accessibility.Focus))
	}
	namedNode := screen.AccessibilityNodeFor(named)
	c.NotNil(namedNode)
	if namedNode != nil {
		c.Equal(role.Heading, namedNode.Role, "the role the application asked for is left alone")
		c.Equal("Printers", namedNode.Name)
		c.False(namedNode.Ignored, "a heading with a name is worth announcing")
		c.True(namedNode.Focusable, "so the node offers the focus Orca lands with")
		c.True(namedNode.Actions.Has(accessibility.Focus))
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestHeadingWithoutALabelTakesFocus verifies that what decides whether a heading is somewhere a screen reader's
// structural navigation can land is what the published node will announce, not what kind of widget the heading was
// built out of. An application marks up a heading by setting Role and Level on any panel at all — see
// AccessibilityInfo.Level — so a grouping Panel carrying a name, and a Label that draws only a drawable but takes its
// name from its tooltip (see axDescribeStaticContent), are both headings Orca jumps to and both have to be able to
// take the focus it lands with. A panel with nothing to announce still cannot. A session owns most of the package's
// mutable globals while it runs, so this may not call t.Parallel.
func TestHeadingWithoutALabelTakesFocus(t *testing.T) {
	c := check.New(t)
	var group, anonymous *Panel
	var icon *Label
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			// Not a Label at all: a container the application marked up as the heading of the group it holds.
			group = NewPanel()
			group.SetLayout(&FlexLayout{Columns: 1})
			group.Accessibility.Role = role.Heading
			group.Accessibility.Level = 1
			group.Accessibility.Name = "Network"
			inner := NewLabel()
			inner.SetTitle("Inside the group")
			group.AddChild(inner)

			// Marked up as a heading, but says nothing at all when it is landed on.
			anonymous = NewPanel()
			anonymous.SetLayout(&FlexLayout{Columns: 1})
			anonymous.Accessibility.Role = role.Heading
			anonymous.Accessibility.Level = 1

			// Draws no text, so its name comes from its tooltip, which is written onto the node before the role the
			// application asked for is looked at.
			icon = NewLabel()
			icon.Drawable = &axLabelTestDrawable{size: geom.NewSize(16, 16)}
			icon.Tooltip = NewTooltipWithText("Printers")
			icon.Accessibility.Role = role.Heading
			icon.Accessibility.Level = 2

			column := NewPanel()
			column.SetLayout(&FlexLayout{Columns: 1})
			column.AddChild(group)
			column.AddChild(anonymous)
			column.AddChild(icon)
			wnd = axNewTestWindow(t, "non-label heading focus", geom.NewRect(10, 10, 500, 300), column)
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

	var groupFocusable, anonymousFocusable, iconFocusable bool
	screen.Do(func() {
		groupFocusable = group.Focusable()
		anonymousFocusable = anonymous.Focusable()
		iconFocusable = icon.AsPanel().Focusable()
	})
	c.True(groupFocusable, "a heading that is not a Label still has to take the focus Orca lands with")
	c.False(anonymousFocusable, "one with nothing to announce is still not somewhere to land")
	c.True(iconFocusable, "and one named only by its tooltip is announced by that name, so the jump can land")

	c.NotNil(screen.AccessibilityTree(wnd))
	groupNode := screen.AccessibilityNodeFor(group)
	c.NotNil(groupNode)
	if groupNode != nil {
		c.Equal(role.Heading, groupNode.Role)
		c.Equal("Network", groupNode.Name)
		c.True(groupNode.Focusable, "the node offers the focus")
		c.True(groupNode.Actions.Has(accessibility.Focus))
	}
	iconNode := screen.AccessibilityNodeFor(icon)
	c.NotNil(iconNode)
	if iconNode != nil {
		c.Equal(role.Heading, iconNode.Role)
		c.Equal("Printers", iconNode.Name, "the tooltip is what names a heading that draws no text")
		c.True(iconNode.Focusable, "so it offers the focus too")
		c.True(iconNode.Actions.Has(accessibility.Focus))
	}

	// A screen reader asking for the focus on the heading it jumped to gets it, which is the whole point of the flag.
	if groupNode != nil {
		c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
			Node:   groupNode.ID,
			Action: accessibility.Focus,
		}))
		var focus *Panel
		screen.Do(func() { focus = wnd.CurrentFocus() })
		c.True(focus.Is(group), "and the focus lands on it")
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
