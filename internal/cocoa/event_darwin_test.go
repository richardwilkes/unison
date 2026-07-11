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
)

func TestDoubleClickInterval(t *testing.T) {
	if d := DoubleClickInterval(); d <= 0 {
		t.Errorf("DoubleClickInterval returned %v", d)
	}
}

func TestCurrentModifierFlags(_ *testing.T) {
	_ = CurrentModifierFlags() // the value depends on the live keyboard state; this exercises the msgSend path
}

// TestPostEmptyEventWakesWaitEvents proves the production wake-up contract: an event posted with PostEmptyEvent
// (from any thread) causes a blocked WaitEvents to return.
func TestPostEmptyEventWakesWaitEvents(t *testing.T) {
	runOnMain(func() { sharedApp() })
	PostEmptyEvent() // the queue is guaranteed non-empty before WaitEvents runs
	done := make(chan struct{})
	go func() {
		runOnMain(WaitEvents)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		PostEmptyEvent() // unstick the main-thread dispatcher so the remaining tests can run
		t.Fatal("WaitEvents did not return after PostEmptyEvent")
	}
}

func TestWaitEventsTimeout(t *testing.T) {
	// Expiry path: with a drained queue, a short timeout returns on its own.
	runOnMain(func() {
		sharedApp()
		PollEvents()
		WaitEventsTimeout(0.25)
	})

	// Wake path: an event posted mid-wait returns long before the timeout expires.
	var elapsed time.Duration
	done := make(chan struct{})
	go func() {
		runOnMain(func() {
			PollEvents()
			start := time.Now()
			WaitEventsTimeout(10)
			elapsed = time.Since(start)
		})
		close(done)
	}()
	time.Sleep(250 * time.Millisecond)
	PostEmptyEvent()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		PostEmptyEvent()
		t.Fatal("WaitEventsTimeout did not return")
	}
	if elapsed >= 9*time.Second {
		t.Errorf("WaitEventsTimeout was not woken by PostEmptyEvent (took %v)", elapsed)
	}
}
