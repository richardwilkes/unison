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
	"math"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xmath"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/accessibility"
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

// newTestVerticalSlider creates a vertical slider whose usable track spans exactly 100 pixels, from y=7 to y=107, given
// the default EdgeThickness (1) and MarkerSize (12) producing a 7-pixel inset at each end.
func newTestVerticalSlider(minimum, maximum, value float32) *unison.Slider {
	s := unison.NewSlider(minimum, maximum, value)
	s.SetFrameRect(geom.NewRect(0, 0, 20, 114))
	return s
}

// TestSliderVerticalArrowKeysFollowTheDrawnAxis verifies that the up and down arrows move a vertical slider's thumb the
// way they point. A vertical slider draws its maximum at the bottom, and dragging downwards raises its value, so the
// down arrow has to raise it too; it used to lower it, which moved the thumb visibly upwards.
func TestSliderVerticalArrowKeysFollowTheDrawnAxis(t *testing.T) {
	c := check.New(t)
	s := newTestVerticalSlider(0, 100, 50)

	// The bottom of the track is the maximum, which is the direction the arrow keys have to agree with.
	s.DefaultMouseDrag(geom.NewPoint(10, 107), 1, mod.None)
	c.Equal(float32(100), s.Value(), "dragging to the bottom of a vertical slider raises it to its maximum")
	s.DefaultMouseDrag(geom.NewPoint(10, 7), 1, mod.None)
	c.Equal(float32(0), s.Value())

	s.SetValue(50)
	c.True(s.DefaultKeyDown(unison.KeyDown, mod.None, false))
	c.Equal(float32(51), s.Value(), "the down arrow moves the thumb down, which raises a vertical slider")
	c.True(s.DefaultKeyDown(unison.KeyUp, mod.None, false))
	c.True(s.DefaultKeyDown(unison.KeyUp, mod.None, false))
	c.Equal(float32(49), s.Value(), "the up arrow moves it back up, which lowers it")

	// Left and right keep their meaning whichever way round the slider is laid out.
	c.True(s.DefaultKeyDown(unison.KeyRight, mod.None, false))
	c.Equal(float32(50), s.Value())
	c.True(s.DefaultKeyDown(unison.KeyLeft, mod.None, false))
	c.Equal(float32(49), s.Value())

	// The ends of the range are still the ends of the range.
	c.True(s.DefaultKeyDown(unison.KeyEnd, mod.None, false))
	c.Equal(float32(100), s.Value())
	c.True(s.DefaultKeyDown(unison.KeyHome, mod.None, false))
	c.Equal(float32(0), s.Value())
}

// TestSliderHorizontalArrowKeys verifies that a horizontal slider keeps the arrow directions every platform gives one:
// up and right raise the value, down and left lower it.
func TestSliderHorizontalArrowKeys(t *testing.T) {
	c := check.New(t)
	s := newTestSlider(0, 100, 50)
	c.True(s.DefaultKeyDown(unison.KeyUp, mod.None, false))
	c.Equal(float32(51), s.Value(), "the up arrow raises a horizontal slider")
	c.True(s.DefaultKeyDown(unison.KeyDown, mod.None, false))
	c.Equal(float32(50), s.Value(), "the down arrow lowers it")
	c.True(s.DefaultKeyDown(unison.KeyRight, mod.None, false))
	c.Equal(float32(51), s.Value())
	c.True(s.DefaultKeyDown(unison.KeyLeft, mod.None, false))
	c.Equal(float32(50), s.Value())
}

// TestSliderAccessibilitySetValueRefusesNonNumbers verifies that a value that is not a number is refused rather than
// becoming the slider's value. None of SetValue's range clamps is true for a NaN, so one used to settle in for good,
// leaving every later comparison against it different and the marker drawn at a position that is nowhere.
func TestSliderAccessibilitySetValueRefusesNonNumbers(t *testing.T) {
	c := check.New(t)
	s := newTestSlider(0, 100, 25)
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		c.False(s.PerformAccessibilityAction(accessibility.ActionRequest{
			Action: accessibility.SetValue,
			Number: value,
		}), "%v must be refused", value)
		c.Equal(float32(25), s.Value(), "the value must be left as it was")
	}
	c.True(s.PerformAccessibilityAction(accessibility.ActionRequest{
		Action: accessibility.SetValue,
		Number: 60,
	}))
	c.Equal(float32(60), s.Value(), "an ordinary number still goes through")
}

// TestSliderDrawsFocusedWithoutSelectionInk verifies that a slider built from a theme that predates SelectionInk draws
// when it is focused rather than losing the draw to a nil ink.
func TestSliderDrawsFocusedWithoutSelectionInk(t *testing.T) {
	c := check.New(t)
	var slider *unison.Slider
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			slider = unison.NewSlider(0, 100, 50)
			slider.SelectionInk = nil
			wnd = newHeadlessWindow(t, "slider ink", geom.NewRect(10, 10, 300, 150), axColumn(slider))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() { wnd.ToFront() }))
	c.True(screen.Do(func() { slider.RequestFocus() }))
	screen.Sync()
	var focused bool
	screen.Do(func() { focused = slider.Focused() })
	c.True(focused, "the slider should be holding the focus")
	c.Equal(0, len(screen.Errors()), "drawing a focused slider must not need SelectionInk: %v", screen.Errors())
}
