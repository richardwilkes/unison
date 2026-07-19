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
