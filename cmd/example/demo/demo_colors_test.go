// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package demo

import (
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/unison"
)

// TestNewDemoColorsWindowReusesExistingWindow is the regression test for the colors window singleton: colorsWindow was
// never assigned, so every invocation created another identical window. With an existing window recorded, the call must
// return it rather than creating a new one. A zero-value Window reports itself as invalid, so the ToFront on the reuse
// path is a safe no-op in this headless test. This test mutates the package-level colorsWindow and therefore must not
// call t.Parallel.
func TestNewDemoColorsWindowReusesExistingWindow(t *testing.T) {
	c := check.New(t)
	sentinel := &unison.Window{}
	colorsWindow = sentinel
	t.Cleanup(func() { colorsWindow = nil })
	wnd, err := NewDemoColorsWindow()
	c.NoError(err)
	c.True(wnd == sentinel, "an existing colors window must be returned instead of creating a new one")
}

// TestTrackColorsWindowClearsRecordOnClose verifies the wiring NewDemoColorsWindow applies to a freshly created window:
// the window is recorded as the singleton and its WillCloseCallback (invoked by Window.Dispose) clears the record so a
// subsequent call can create a replacement. This test mutates the package-level colorsWindow and therefore must not
// call t.Parallel.
func TestTrackColorsWindowClearsRecordOnClose(t *testing.T) {
	c := check.New(t)
	t.Cleanup(func() { colorsWindow = nil })
	wnd := &unison.Window{}
	trackColorsWindow(wnd)
	c.True(colorsWindow == wnd, "the created window must be recorded as the singleton")
	c.NotNil(wnd.WillCloseCallback, "closing the window must be observable so the record can be cleared")
	wnd.WillCloseCallback()
	c.Nil(colorsWindow, "closing the window must clear the singleton record")
}
