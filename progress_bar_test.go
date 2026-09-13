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
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison"
)

// TestNewProgressBarClampsNegativeMaximum verifies that NewProgressBar clamps a negative maximum to zero, matching
// SetMaximum. A negative maximum would draw as an indeterminate meter without its animation ever being scheduled,
// leaving it frozen.
func TestNewProgressBarClampsNegativeMaximum(t *testing.T) {
	c := check.New(t)
	c.Equal(float32(0), unison.NewProgressBar(-5).Maximum())
	c.Equal(float32(0), unison.NewProgressBar(0).Maximum())
	c.Equal(float32(10), unison.NewProgressBar(10).Maximum())
	p := unison.NewProgressBar(10)
	p.SetMaximum(-1)
	c.Equal(float32(0), p.Maximum())
}

// TestProgressBarAccessibilityIndeterminateReportsNoNumber verifies that a bar with no maximum reports no number at
// all. It used to claim one with its minimum, maximum and value all zero, which hands Windows a range value pattern
// whose maximum equals its minimum — something the UI Automation contract forbids — and leaves macOS answering with a
// value of zero that never moves.
func TestProgressBarAccessibilityIndeterminateReportsNoNumber(t *testing.T) {
	c := check.New(t)
	var determinate, indeterminate *unison.ProgressBar
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			determinate = unison.NewProgressBar(200)
			determinate.SetCurrent(50)

			indeterminate = unison.NewProgressBar(0)
			// The animation reschedules itself after every draw, so it is slowed to something no test will wait for
			// rather than being left to keep the session busy.
			indeterminate.TickSpeed = time.Hour

			wnd = newHeadlessWindow(t, "indeterminate", geom.NewRect(10, 10, 300, 200),
				axColumn(determinate, indeterminate))
		}))
	c.NotNil(wnd)

	screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(determinate)
	c.True(node != nil)
	if node != nil {
		c.True(node.HasNumber, "a bar with a maximum reports how far along it is")
		c.Equal(float64(50), node.Number)
		c.Equal(float64(200), node.Max)
		c.True(node.ReadOnly)
		c.False(node.Busy)
	}

	busy := screen.AccessibilityNodeFor(indeterminate)
	c.True(busy != nil)
	if busy != nil {
		c.True(busy.Busy, "a bar with no maximum knows only that something is happening")
		c.False(busy.HasNumber, "there is no telling how far along it is, so it must not claim a position")
		c.Equal(float64(0), busy.Max)
		c.True(busy.ReadOnly)
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
