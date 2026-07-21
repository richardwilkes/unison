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
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
)

// waitForTaskQueueLength polls the task queue until it holds at least min tasks, then waits out the grace period so any
// stragglers can land, and returns the final live queue length.
func waitForTaskQueueLength(t *testing.T, minimum int, grace time.Duration) int {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		length, head := taskQueueState()
		if length-head >= minimum {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %d task(s) to be queued", minimum)
		}
		time.Sleep(time.Millisecond)
	}
	time.Sleep(grace)
	length, head := taskQueueState()
	return length - head
}

// TestIndeterminateProgressBarSchedulesSingleRedrawTask verifies that overlapping draws of an indeterminate progress
// bar coalesce into a single pending redraw task rather than each draw spawning its own redraw-timer chain. This test
// shares the global task queue and therefore must not call t.Parallel.
func TestIndeterminateProgressBarSchedulesSingleRedrawTask(t *testing.T) {
	c := check.New(t)
	resetTaskQueue()
	withRecoveryCallback(t, func(err error) { c.NoError(err) })

	p := NewProgressBar(0)
	p.TickSpeed = time.Millisecond
	p.SetFrameRect(geom.NewRect(0, 0, 100, 8))
	cv, _ := newPixmapCanvas(100, 8)

	// Simulate multiple externally triggered draws before the first tick fires.
	for range 3 {
		p.DefaultDraw(cv, p.ContentRect(false))
	}
	c.True(p.redrawPending)

	// Only one redraw task should ever arrive, no matter how many draws occurred.
	c.Equal(1, waitForTaskQueueLength(t, 1, 50*time.Millisecond))

	// Running the task clears the pending flag so the next draw can schedule the next tick of the animation.
	processNextTask()
	c.False(p.redrawPending)
	length, head := taskQueueState()
	c.Equal(0, length-head)

	p.DefaultDraw(cv, p.ContentRect(false))
	c.True(p.redrawPending)
	c.Equal(1, waitForTaskQueueLength(t, 1, 50*time.Millisecond))

	resetTaskQueue()
}

// TestDeterminateProgressBarSchedulesNoRedrawTask verifies that a determinate progress bar does not schedule animation
// redraws at all. This test shares the global task queue and therefore must not call t.Parallel.
func TestDeterminateProgressBarSchedulesNoRedrawTask(t *testing.T) {
	c := check.New(t)
	resetTaskQueue()
	withRecoveryCallback(t, func(err error) { c.NoError(err) })

	p := NewProgressBar(10)
	p.TickSpeed = time.Millisecond
	p.SetCurrent(5)
	p.SetFrameRect(geom.NewRect(0, 0, 100, 8))
	cv, _ := newPixmapCanvas(100, 8)
	p.DefaultDraw(cv, p.ContentRect(false))
	c.False(p.redrawPending)

	time.Sleep(50 * time.Millisecond)
	length, head := taskQueueState()
	c.Equal(0, length-head)
}
