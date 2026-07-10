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
	"slices"
	"testing"
)

func TestDisplayFunctions(t *testing.T) {
	mainID := MainDisplayID()
	if mainID == 0 {
		t.Skip("no main display (headless environment)")
	}
	if list := ActiveDisplayList(); !slices.Contains(list, mainID) {
		// CGGetActiveDisplayList excludes sleeping displays, so a locked session with the displays asleep legitimately
		// returns an empty list even though CGMainDisplayID still reports a display (verified empirically 2026-07-10).
		if len(list) == 0 && DisplayIsAsleep(mainID) {
			t.Skip("displays are asleep (locked session) — active display list is legitimately empty")
		}
		t.Errorf("ActiveDisplayList %v does not contain the main display %d", list, mainID)
	}
	bounds := DisplayBounds(mainID)
	if bounds.Width <= 0 || bounds.Height <= 0 {
		t.Errorf("main display bounds %v are empty", bounds)
	}
	// The main display's bounds are anchored at the global origin.
	if bounds.X != 0 || bounds.Y != 0 {
		t.Errorf("main display bounds %v are not at the origin", bounds)
	}
	if sizeMM := DisplayScreenSize(mainID); sizeMM.Width < 0 || sizeMM.Height < 0 {
		t.Errorf("main display physical size %v is negative", sizeMM)
	}
	_ = DisplayIsAsleep(mainID) // Value depends on the environment; just exercise the call path.
}
