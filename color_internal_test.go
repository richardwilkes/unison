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

	"github.com/richardwilkes/toolbox/v2/check"
)

// TestNormalizeHue verifies the documented [0,360) contract, in particular for exact negative multiples of 360, which
// previously normalized to 360 instead of 0.
func TestNormalizeHue(t *testing.T) {
	c := check.New(t)

	c.Equal(float32(0), normalizeHue(0))
	c.Equal(float32(0), normalizeHue(360))
	c.Equal(float32(0), normalizeHue(720))
	c.Equal(float32(0), normalizeHue(-360))
	c.Equal(float32(0), normalizeHue(-720))
	c.Equal(float32(45), normalizeHue(45))
	c.Equal(float32(45), normalizeHue(405))
	c.Equal(float32(315), normalizeHue(-45))
	c.Equal(float32(359.5), normalizeHue(-0.5))

	for _, hue := range []float64{-1e9, -360.000001, -360, -0.000001, 0, 359.999, 360, 360.000001, 1e9} {
		h := normalizeHue(hue)
		c.True(h >= 0 && h < 360, "normalizeHue(%v) = %v, want a value in [0,360)", hue, h)
	}
}
