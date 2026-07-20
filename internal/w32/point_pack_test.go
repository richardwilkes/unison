// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package w32

import (
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
)

// TestPackPointLayout verifies the Win64 by-value POINT convention: X in the low 32 bits and Y in the high 32 bits,
// with negative coordinates (which occur on multi-monitor setups) confined to their own half rather than
// sign-extending across the full register. WindowFromPoint relies on this; passing X and Y as two separate syscall
// arguments instead makes Windows see the point (X, 0).
func TestPackPointLayout(t *testing.T) {
	c := check.New(t)
	c.Equal(uintptr(0), packPoint(0, 0))
	c.Equal(uintptr(0x0000009000000010), packPoint(0x10, 0x90))
	c.Equal(uintptr(0x00000005FFFFFFFF), packPoint(-1, 5))
	c.Equal(uintptr(0xFFFFFFF800000007), packPoint(7, -8))
}

// TestPointPackingRoundTrip verifies that unpackPoint recovers exactly what packPoint encodes, since the drop-target
// callbacks decode with unpackPoint what Windows encodes with the same convention packPoint implements.
func TestPointPackingRoundTrip(t *testing.T) {
	c := check.New(t)
	for _, pt := range [][2]int32{{0, 0}, {123, 456}, {-1920, 1080}, {2560, -1440}, {-1, -1}} {
		x, y := unpackPoint(packPoint(pt[0], pt[1]))
		c.Equal(pt[0], x)
		c.Equal(pt[1], y)
	}
}
