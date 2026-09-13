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
