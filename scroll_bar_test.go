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
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/accessibility"
)

// TestScrollBarAccessibilitySetValueRefusesNonNumbers verifies that a value that is not a number is refused rather than
// becoming the bar's position. None of SetRange's clamps is true for a NaN, so one used to settle in for good and go on
// through ChangedCallback to the frame of whatever the bar scrolls; the AT-SPI value interface will hand over any
// double a client cares to send.
func TestScrollBarAccessibilitySetValueRefusesNonNumbers(t *testing.T) {
	c := check.New(t)
	bar := unison.NewScrollBar(false)
	changes := 0
	bar.ChangedCallback = func() { changes++ }
	bar.SetRange(10, 50, 200)
	c.Equal(float32(10), bar.Value())
	c.Equal(1, changes)

	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		c.False(bar.PerformAccessibilityAction(accessibility.ActionRequest{
			Action: accessibility.SetValue,
			Number: value,
		}), "%v must be refused", value)
		c.Equal(float32(10), bar.Value(), "the value must be left as it was")
		c.Equal(1, changes, "nothing changed, so nothing should have been reported")
	}

	c.True(bar.PerformAccessibilityAction(accessibility.ActionRequest{
		Action: accessibility.SetValue,
		Number: 30,
	}))
	c.Equal(float32(30), bar.Value(), "an ordinary number still goes through")
	c.Equal(2, changes)
}
