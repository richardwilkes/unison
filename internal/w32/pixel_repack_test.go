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

// TestRepackRGBAToBGRASwizzle verifies the channel reordering: RGBA words (R in the low byte) become the BGRA words
// GDI expects (B in the low byte), with the green and alpha bytes staying in place, and that padding pixels beyond
// each row's width (the row stride here is 3 pixels for a 2-pixel-wide image) never reach the output.
func TestRepackRGBAToBGRASwizzle(t *testing.T) {
	c := check.New(t)
	pix := []uint32{
		0x44332211, 0x88776655, 0xDEADBEEF,
		0x00000000, 0xFF00FF00, 0xDEADBEEF,
	}
	dst := make([]uint32, 4)
	RepackRGBAToBGRA(pix, 2, 2, 3, dst)
	c.Equal([]uint32{0x44112233, 0x88556677, 0x00000000, 0xFF00FF00}, dst)
}

// TestRepackRGBAToBGRAWritesInPlace verifies that the destination is filled in place, since it addresses the
// presentation bitmap's own pixel store, and that nothing is written past the frame.
func TestRepackRGBAToBGRAWritesInPlace(t *testing.T) {
	c := check.New(t)
	pix := []uint32{0x44332211, 0x88776655, 0x00000000, 0xFF00FF00}
	dst := make([]uint32, 6)
	RepackRGBAToBGRA(pix, 2, 2, 2, dst)
	c.Equal([]uint32{0x44112233, 0x88556677, 0x00000000, 0xFF00FF00, 0, 0}, dst)

	// Presenting again at the same size overwrites the same words rather than needing a fresh destination.
	RepackRGBAToBGRA([]uint32{1, 2, 3, 4}, 2, 2, 2, dst)
	c.Equal([]uint32{0x10000, 0x20000, 0x30000, 0x40000, 0, 0}, dst)
}
