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
	"strconv"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
)

// TestHueDegreesStaysInRange verifies that the hue shown alongside the 0-359 label never falls outside that range.
// Rounding the raw hue used to yield 360 for colors just shy of a full turn on the wheel.
func TestHueDegreesStaysInRange(t *testing.T) {
	c := check.New(t)

	c.Equal(0, hueDegrees(RGB(255, 0, 0)))
	c.Equal(120, hueDegrees(RGB(0, 255, 0)))
	c.Equal(180, hueDegrees(RGB(0, 255, 255)))
	c.Equal(240, hueDegrees(RGB(0, 0, 255)))

	// RGB(255, 0, 1) has a hue of ~359.76 degrees, which rounds to 360 -- out of range, and disagreeing with the
	// slider, which clamps to its 359 maximum.
	c.Equal(359, hueDegrees(RGB(255, 0, 1)))

	// No hue at all may land outside the range, including the ones nearest the wrap point.
	for i := range 3601 {
		degrees := hueDegrees(HSB(float32(i)/3600, 1, 1))
		c.True(degrees >= 0 && degrees <= 359, "hue %d/3600 produced %d degrees", i, degrees)
	}
}

// TestColorEditorHueFieldMatchesSlider verifies that the hue field and hue slider agree on the value they present for a
// color at the top of the hue wheel, where the field used to read 360 while the slider read 359.
func TestColorEditorHueFieldMatchesSlider(t *testing.T) {
	c := check.New(t)
	e := NewColorEditor(RGB(255, 0, 1))
	c.Equal("359", e.hueField.Text())
	c.Equal(float32(359), e.hueSlider.Value())

	// The same must hold after the editor is handed a new color.
	e.SetColor(RGB(255, 0, 2))
	c.Equal("359", e.hueField.Text())
	e.SetColor(RGB(0, 0, 255))
	c.Equal("240", e.hueField.Text())

	// Sweeping the reds nearest the wrap point must never push the field out of range.
	for blue := range 16 {
		e.SetColor(RGB(255, 0, blue))
		degrees, err := strconv.Atoi(e.hueField.Text())
		c.NoError(err)
		c.True(degrees >= 0 && degrees <= 359, "hue field %q out of range for RGB(255, 0, %d)", e.hueField.Text(), blue)
	}
}
