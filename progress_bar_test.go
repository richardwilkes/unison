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
