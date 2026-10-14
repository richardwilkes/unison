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

// TestPopupMenuClickOnlyInTheActiveWindow verifies that Click on a popup in a background window opens nothing, since
// the menu it pops up is opened for the active window at coordinates taken from the popup's own window, and that the
// same popup opens its menu for its own window once that is the active one.
func TestPopupMenuClickOnlyInTheActiveWindow(t *testing.T) {
	c := check.New(t)
	var back, front *Window
	var popup *PopupMenu[string]
	willShow := 0
	screen := startHeadlessTest(t, HeadlessConfig{Width: 500, Height: 400},
		StartupFinishedCallback(func() {
			back = newHeadlessTestWindow(t, "back", geom.NewRect(10, 10, 200, 200))
			front = newHeadlessTestWindow(t, "front", geom.NewRect(50, 50, 300, 200))
			if back == nil || front == nil {
				return
			}
			popup = NewPopupMenu[string]()
			popup.AddItem("Light", "Regular", "Bold")
			popup.WillShowMenuCallback = func(_ *PopupMenu[string]) { willShow++ }
			back.Content().AddChild(popup)
			front.ToFront()
		}))
	c.NotNil(back)
	c.NotNil(front)
	if back == nil || front == nil {
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
		popup.Click()
		look()
	}))
	c.True(active == front, "the test needs the popup's window in the background")
	c.Equal(0, frontOpen, "the menu must not land in the active window")
	c.Equal(0, backOpen, "nor anywhere else")
	c.Equal(0, willShow, "nothing is about to be shown, so the callback is not run")

	c.True(screen.Do(func() {
		back.ToFront()
		popup.Click()
		look()
	}))
	c.Equal(1, backOpen, "the menu opens for the popup's own window once that is the active one")
	c.Equal(0, frontOpen)
	c.Equal(1, willShow)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
