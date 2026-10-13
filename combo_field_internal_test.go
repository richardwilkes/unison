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
)

// TestFieldDropdownOpensOnlyInTheActiveWindow verifies that the dropdown of a field in a background window opens
// nothing, whether by the down arrow or a click on its button, since the menu is built in the active window at
// coordinates taken from the field's own window, and that it opens in the field's own window once that is the active
// one.
func TestFieldDropdownOpensOnlyInTheActiveWindow(t *testing.T) {
	c := check.New(t)
	var back, front *Window
	var field *Field
	asked := 0
	screen := startHeadlessTest(t, HeadlessConfig{Width: 500, Height: 400},
		StartupFinishedCallback(func() {
			back = newHeadlessTestWindow(t, "back", geom.NewRect(10, 10, 200, 200))
			front = newHeadlessTestWindow(t, "front", geom.NewRect(50, 50, 300, 200))
			if back == nil || front == nil {
				return
			}
			field = NewField()
			field.InstallDropdown(func() []string {
				asked++
				return []string{"Light", "Regular", "Bold"}
			})
			back.Content().AddChild(field)
			front.ToFront()
		}))
	c.NotNil(back)
	c.NotNil(front)
	if back == nil || front == nil {
		return
	}
	var button *Button
	c.True(screen.Do(func() {
		for _, child := range field.Children() {
			if b, ok := child.Self.(*Button); ok {
				button = b
			}
		}
	}))
	c.NotNil(button, "the test needs the dropdown button")
	if button == nil {
		return
	}
	var active *Window
	var frontOpen, backOpen int
	look := func() {
		frontOpen = len(front.root.openMenuPanels)
		backOpen = len(back.root.openMenuPanels)
	}
	c.True(screen.Do(func() {
		active = ActiveWindow()
		field.KeyDownCallback(KeyDown, 0, false)
		look()
	}))
	c.True(active == front, "the test needs the field's window in the background")
	c.Equal(0, frontOpen, "the menu must not land in the active window")
	c.Equal(0, backOpen, "nor anywhere else")
	c.True(screen.Do(func() {
		button.ClickCallback()
		look()
	}))
	c.Equal(0, frontOpen, "nor when the button is clicked")
	c.Equal(0, backOpen)
	c.Equal(0, asked, "nothing is about to be shown, so the options are not asked for")

	var openedIn *Window
	c.True(screen.Do(func() {
		back.ToFront()
		button.ClickCallback()
		look()
		if backOpen == 1 {
			openedIn = back.root.openMenuPanels[0].Window()
		}
	}))
	c.Equal(1, backOpen, "the menu opens in the field's own window once that is the active one")
	c.Equal(0, frontOpen)
	c.True(openedIn == back)
	c.Equal(1, asked)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
