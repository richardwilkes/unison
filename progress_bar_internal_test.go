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

	"github.com/richardwilkes/canvas/raster"
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

// hasPixelInColumns reports whether any pixel in columns [minX, maxX) of the pixmap exactly matches the given value.
func hasPixelInColumns(pix *raster.Pixmap, value uint32, minX, maxX int32) bool {
	for y := int32(0); y < pix.Height; y++ {
		row := int(y) * int(pix.RowPixels)
		for x := minX; x < maxX; x++ {
			if pix.Pix[row+int(x)] == value {
				return true
			}
		}
	}
	return false
}

// TestIndeterminateProgressBarMeterHonorsContentRectOrigin verifies that the animated indeterminate meter is positioned
// relative to the content rect rather than the panel origin. With a border installed, ContentRect(false).X is nonzero,
// and the meter previously dropped that origin, drawing shifted left by the border inset at every point of the
// traversal.
func TestIndeterminateProgressBarMeterHonorsContentRectOrigin(t *testing.T) {
	c := check.New(t)

	const inset = 20
	p := NewProgressBar(0)
	p.SetBorder(NewEmptyBorder(geom.NewUniformInsets(inset)))
	// Use plain colors and square corners so the rendered output is deterministic regardless of theme state, with the
	// meter fill as the only source of white pixels.
	p.BackgroundInk = Black
	p.EdgeInk = Black
	p.FillInk = White
	p.CornerRadius = geom.Size{}
	// Make a traversal take so long that the few milliseconds a draw call takes cannot move the meter measurably.
	p.FullTraversalSpeed = time.Hour
	p.SetFrameRect(geom.NewRect(0, 0, 100, 48))
	bounds := p.ContentRect(false)
	c.Equal(geom.NewRect(inset, inset, 100-2*inset, 48-2*inset), bounds)

	// At the start of a traversal the meter must sit at the content rect's left edge, not the panel's. The fill is
	// drawn trimmed by half a pixel on each side, so the first fully white column is one past the meter's left edge.
	p.redrawPending = true // suppress scheduling animation ticks on the shared task queue
	p.lastAnimationTime = time.Now()
	cv, pix := newPixmapCanvas(100, 48)
	p.DefaultDraw(cv, bounds)
	const white = uint32(0xFFFFFFFF)
	c.False(hasPixelInColumns(pix, white, 0, inset), "meter must not overlap the border at the start of a traversal")
	c.True(hasPixelInColumns(pix, white, inset, inset+int32(p.IndeterminateWidth)),
		"meter should be drawn at the content rect's left edge")

	// At the end of a traversal the meter must sit flush against the content rect's right edge; without the origin the
	// meter stops short by the border inset.
	p.lastAnimationTime = time.Now().Add(-p.FullTraversalSpeed)
	cv, pix = newPixmapCanvas(100, 48)
	p.DefaultDraw(cv, bounds)
	meterStart := int32(bounds.Right() - p.IndeterminateWidth)
	c.False(hasPixelInColumns(pix, white, 0, meterStart), "meter should have traversed to the content rect's right edge")
	c.True(hasPixelInColumns(pix, white, meterStart, int32(bounds.Right())),
		"meter should be drawn at the content rect's right edge")
	c.False(hasPixelInColumns(pix, white, int32(bounds.Right()), 100),
		"meter must not extend past the content rect's right edge")
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
