// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package mac

import (
	"testing"

	"github.com/richardwilkes/toolbox/v2/geom"
)

func TestScreenFunctions(t *testing.T) {
	mainID := MainDisplayID()
	if mainID == 0 {
		t.Skip("no main display (headless environment)")
	}
	screen := ScreenForDisplayID(mainID)
	if screen == 0 {
		t.Fatal("no NSScreen found for the main display")
	}
	frame := screen.Frame()
	bounds := DisplayBounds(mainID)
	if frame.Width != bounds.Width || frame.Height != bounds.Height {
		t.Errorf("screen frame %v does not match display bounds %v", frame, bounds)
	}
	visible := screen.VisibleFrame()
	if visible.Width <= 0 || visible.Width > frame.Width || visible.Height <= 0 || visible.Height > frame.Height {
		t.Errorf("visible frame %v is not contained within frame %v", visible, frame)
	}
	// Backing scale factors are always >= 1 (1x or 2x on real hardware).
	backing := screen.ConvertRectToBacking(geom.NewRect(0, 0, 100, 50))
	if backing.Width < 100 || backing.Height < 50 {
		t.Errorf("backing rect %v is smaller than the logical rect", backing)
	}
	if ScreenForDisplayID(0xFFFFFFFE) != 0 {
		t.Error("found a screen for a bogus display ID")
	}
}
