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
	"github.com/richardwilkes/toolbox/v2/xmath"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/enums/mod"
)

// newTestSlider creates a horizontal slider whose usable track spans exactly 100 pixels, from x=7 to x=107, given the
// default EdgeThickness (1) and MarkerSize (12) producing a 7-pixel inset on each end.
func newTestSlider(minimum, maximum, value float32) *unison.Slider {
	s := unison.NewSlider(minimum, maximum, value)
	s.SetFrameRect(geom.NewRect(0, 0, 114, 20))
	return s
}

func dragTo(s *unison.Slider, x float32) {
	s.DefaultMouseDrag(geom.NewPoint(x, 10), 1, mod.None)
}

func TestSliderDragWithZeroMinimum(t *testing.T) {
	c := check.New(t)
	s := newTestSlider(0, 100, 0)
	dragTo(s, 7)
	c.Equal(float32(0), s.Value())
	dragTo(s, 57)
	c.Equal(float32(50), s.Value())
	dragTo(s, 107)
	c.Equal(float32(100), s.Value())
}

func TestSliderDragWithNonZeroMinimum(t *testing.T) {
	c := check.New(t)
	s := newTestSlider(50, 100, 50)
	dragTo(s, 107)
	c.Equal(float32(100), s.Value())
	dragTo(s, 57)
	c.Equal(float32(75), s.Value())
	dragTo(s, 7)
	c.Equal(float32(50), s.Value())
}

func TestSliderDragWithNegativeMinimum(t *testing.T) {
	c := check.New(t)
	s := newTestSlider(-100, 100, 0)
	dragTo(s, 7)
	c.Equal(float32(-100), s.Value())
	dragTo(s, 57)
	c.Equal(float32(0), s.Value())
	dragTo(s, 107)
	c.Equal(float32(100), s.Value())
}

func TestSliderDragClampsToTrackEnds(t *testing.T) {
	c := check.New(t)
	s := newTestSlider(50, 100, 75)
	dragTo(s, -500)
	c.Equal(float32(50), s.Value())
	dragTo(s, 500)
	c.Equal(float32(100), s.Value())
}

func TestSliderSetValueClamps(t *testing.T) {
	c := check.New(t)
	s := unison.NewSlider(50, 100, 75)
	s.SetValue(0)
	c.Equal(float32(50), s.Value())
	s.SetValue(200)
	c.Equal(float32(100), s.Value())
}

// TestSliderRangeChangeNotifiesWhenValueMoves verifies that narrowing the range reports the value it displaces.
// SetMinimum and SetMaximum used to clamp the value directly, so listeners never learned the value had changed.
func TestSliderRangeChangeNotifiesWhenValueMoves(t *testing.T) {
	c := check.New(t)

	s := unison.NewSlider(0, 100, 10)
	changes := 0
	s.ValueChangedCallback = func() { changes++ }

	// A range change that leaves the value within range notifies no one.
	s.SetMinimum(5)
	c.Equal(float32(10), s.Value())
	c.Equal(0, changes)
	s.SetMaximum(90)
	c.Equal(float32(10), s.Value())
	c.Equal(0, changes)

	// A new minimum above the value moves it, and that must be reported.
	s.SetMinimum(50)
	c.Equal(float32(50), s.Value())
	c.Equal(1, changes)

	// So must a new maximum below it.
	s.SetMaximum(60)
	c.Equal(float32(50), s.Value())
	c.Equal(1, changes)
	s.SetMinimum(0)
	s.SetMaximum(20)
	c.Equal(float32(20), s.Value())
	c.Equal(2, changes)
}

// TestSliderRangeChangeAppliesSnap verifies that a value the range change moves goes through the snap callback, just as
// it would had SetValue been called directly.
func TestSliderRangeChangeAppliesSnap(t *testing.T) {
	c := check.New(t)
	s := unison.NewSlider(0, 100, 30)
	snaps := 0
	s.ValueSnapCallback = func(value float32) float32 {
		snaps++
		return xmath.Round(value/25) * 25
	}
	changes := 0
	s.ValueChangedCallback = func() { changes++ }

	s.SetMaximum(20)
	c.True(snaps > 0, "a range change must run the value through the snap callback")
	c.Equal(float32(20), s.Value())
	c.Equal(1, changes)
}
