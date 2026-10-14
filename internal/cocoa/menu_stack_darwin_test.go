// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package cocoa

import (
	"testing"

	"github.com/ebitengine/purego/objc"
	"github.com/richardwilkes/toolbox/v2/check"
)

// TestMenuStackMonitor proves the monitor an open menu stack installs: a press the application dispatches is reported
// with the window it was made in, or 0 for one made in none of the application's windows, a removal asked for from
// within the function being called takes effect once it returns, and one it replaces from there stays installed.
func TestMenuStackMonitor(t *testing.T) {
	c := check.New(t)
	runOnMain(func() {
		defer RemoveMenuStackMonitor()
		var reported []Window
		InstallMenuStackMonitor(func(w Window) { reported = append(reported, w) })
		c.True(menuStackLocalMonitor != 0)
		c.True(menuStackGlobalMonitor != 0)
		c.True(menuStackObserver != 0)
		w, _, cleanup := newTestWindowAndView(t)
		defer cleanup()
		WithPool(func() {
			sharedApp().Send(Sel("sendEvent:"), synthMouseEvent(nsEventTypeLeftMouseDown, NSPoint{X: 5, Y: 5}, 0, w))
			sharedApp().Send(Sel("sendEvent:"), synthMouseEvent(nsEventTypeRightMouseDown, NSPoint{X: 5, Y: 5}, 0, 0))
		})
		c.Equal([]Window{w, 0}, reported, "a dispatched press is reported with its window, or 0 for none")

		reported = nil
		InstallMenuStackMonitor(func(w Window) {
			reported = append(reported, w)
			RemoveMenuStackMonitor()
			c.True(menuStackLocalMonitor != 0, "the removal waits for the function to return")
		})
		menuStackMonitorFired(3)
		c.Equal([]Window{3}, reported)
		c.Equal(objc.ID(0), menuStackLocalMonitor, "and is carried out once it has")
		c.Equal(objc.ID(0), menuStackGlobalMonitor)
		menuStackMonitorFired(4)
		c.Equal([]Window{3}, reported, "nothing is reported once removed")

		InstallMenuStackMonitor(func(w Window) {
			reported = append(reported, w)
			RemoveMenuStackMonitor()
			InstallMenuStackMonitor(func(w Window) { reported = append(reported, w+100) })
		})
		menuStackMonitorFired(5)
		c.True(menuStackLocalMonitor != 0, "a replacement installed from within the function stays")
		menuStackMonitorFired(6)
		c.Equal([]Window{3, 5, 106}, reported)
		RemoveMenuStackMonitor()
		c.Equal(objc.ID(0), menuStackLocalMonitor)
		RemoveMenuStackMonitor()
	})
}
