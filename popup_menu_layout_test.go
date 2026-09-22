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
)

// TestPopupMenuItemChangesAreLaidOutAgain verifies that a popup which gains a wider item is measured again rather than
// keeping the frame it had. A PopupMenu has no Layout of its own — it sizes itself through a sizer — so marking only
// the popup had Panel.ValidateLayout clear the flag with nothing having re-measured it, and the panel above, which is
// what asks the popup for its preferred size, was never told anything had changed. The popup then kept its old width
// until something unrelated invalidated its parent.
func TestPopupMenuItemChangesAreLaidOutAgain(t *testing.T) {
	c := check.New(t)
	var popup *unison.PopupMenu[string]
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 300},
		unison.StartupFinishedCallback(func() {
			popup = unison.NewPopupMenu[string]()
			popup.AddItem("Tiny")
			popup.SelectIndex(0)
			wnd = newHeadlessWindow(t, "popup layout", geom.NewRect(10, 10, 500, 200), axColumn(popup))
		}))
	c.NotNil(wnd)

	var narrow float32
	screen.Do(func() { narrow = popup.FrameRect().Width })
	c.True(narrow > 0, "the popup should have been laid out, got a width of %v", narrow)

	// The widest item decides the preferred size, so a wider one has to widen the popup.
	screen.Do(func() { popup.AddItem("A choice whose text is a great deal wider than the first one") })
	var wide float32
	screen.Do(func() { wide = popup.FrameRect().Width })
	c.True(wide > narrow, "adding a wider item should have widened the popup, got %v then %v", narrow, wide)

	// Losing the widest item narrows it again, which is the same promise the other way around.
	screen.Do(func() {
		popup.RemoveAllItems()
		popup.AddItem("Tiny")
		popup.SelectIndex(0)
	})
	var again float32
	screen.Do(func() { again = popup.FrameRect().Width })
	c.Equal(narrow, again, "losing the widest item should have brought the popup back to the width it started at")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
