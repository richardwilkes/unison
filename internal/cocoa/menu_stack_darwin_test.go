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
	"time"

	"github.com/ebitengine/purego/objc"
	"github.com/richardwilkes/toolbox/v2/check"
)

// TestMenuStackMonitor proves the monitor an open menu stack installs: a press the application dispatches is reported
// with the window it was made in, or 0 for one made in none of the application's windows, a removal asked for from
// within the function being called takes effect on the next pass of the main run loop, once the block that called it
// has returned, and one it replaces from there stays installed.
func TestMenuStackMonitor(t *testing.T) {
	c := check.New(t)
	runOnMain(func() {
		defer RemoveMenuStackMonitor()
		// The real monitors also report, as 0, any press in another application or change of the active Space made
		// while the run loop runs, so what is reported then is not recorded.
		var reported []Window
		recording := true
		record := func(w Window) {
			if recording {
				reported = append(reported, w)
			}
		}
		pump := func(done func() bool) {
			recording = false
			pumpMainRunLoop(done)
			recording = true
		}
		InstallMenuStackMonitor(record)
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
			record(w)
			RemoveMenuStackMonitor()
			c.True(menuStackLocalMonitor != 0, "the removal waits for the function to return")
		})
		menuStackMonitorFired(3)
		c.Equal([]Window{3}, reported)
		c.True(menuStackLocalMonitor != 0, "and for the block that called it to return")
		menuStackMonitorFired(4)
		c.Equal([]Window{3}, reported, "nothing is reported once removed")
		pump(func() bool { return menuStackLocalMonitor == 0 })
		c.Equal(objc.ID(0), menuStackLocalMonitor, "the removal is carried out on the next pass of the run loop")
		c.Equal(objc.ID(0), menuStackGlobalMonitor)

		InstallMenuStackMonitor(func(w Window) {
			record(w)
			RemoveMenuStackMonitor()
			InstallMenuStackMonitor(func(w Window) { record(w + 100) })
		})
		menuStackMonitorFired(5)
		c.True(menuStackLocalMonitor != 0, "a replacement installed from within the function stays")
		pump(func() bool { return false })
		c.True(menuStackLocalMonitor != 0, "even once the deferred removal has had its chance")
		menuStackMonitorFired(6)
		c.Equal([]Window{3, 5, 106}, reported)
		RemoveMenuStackMonitor()
		c.Equal(objc.ID(0), menuStackLocalMonitor, "a removal from outside the function is carried out at once")
		RemoveMenuStackMonitor()
	})
}

// pumpMainRunLoop runs the main run loop, in passes of up to 20ms, until done returns true or a quarter of a second
// has gone by, so that work queued for a later pass of the loop is performed: a pass may return after serving another
// source before it gets to that work. Main thread only.
func pumpMainRunLoop(done func() bool) {
	deadline := time.Now().Add(250 * time.Millisecond)
	for !done() && time.Now().Before(deadline) {
		WithPool(func() {
			date := objc.ID(Cls("NSDate")).Send(Sel("dateWithTimeIntervalSinceNow:"), 0.02)
			objc.ID(Cls("NSRunLoop")).Send(Sel("currentRunLoop")).Send(Sel("runMode:beforeDate:"), defaultRunLoopMode(),
				date)
		})
	}
}

// TestMenuStackMonitorReportsMenuBarTracking proves that a menu of the application's menu bar beginning to track is
// reported as a press in none of the application's windows, since the press that opened it is consumed by menu
// tracking before either event monitor could see it, and that a removed monitor reports nothing.
func TestMenuStackMonitorReportsMenuBarTracking(t *testing.T) {
	c := check.New(t)
	runOnMain(func() {
		defer RemoveMenuStackMonitor()
		var reported []Window
		InstallMenuStackMonitor(func(w Window) { reported = append(reported, w) })
		post := func() {
			WithPool(func() {
				objc.ID(Cls("NSNotificationCenter")).Send(Sel("defaultCenter")).Send(
					Sel("postNotificationName:object:"), NSStringFromGo("NSMenuDidBeginTrackingNotification"), 0,
				)
			})
		}
		post()
		c.Equal([]Window{0}, reported, "a menu bar menu beginning to track is reported as a press in no window")
		RemoveMenuStackMonitor()
		post()
		c.Equal([]Window{0}, reported, "nothing is reported once removed")
	})
}
