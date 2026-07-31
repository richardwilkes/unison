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
	c.Equal([]uint32{0x44112233, 0x88556677, 0x00000000, 0xFF00FF00}, RepackRGBAToBGRA(pix, 2, 2, 3, nil))
}

// TestRepackRGBAToBGRABufferReuse verifies the scratch buffer contract: a buffer with sufficient capacity is reused
// in place (no per-frame full-frame allocation) and one that is too small is replaced by a grown allocation of the
// exact pixel count.
func TestRepackRGBAToBGRABufferReuse(t *testing.T) {
	c := check.New(t)
	pix := []uint32{0x44332211, 0x88776655, 0x00000000, 0xFF00FF00}

	scratch := make([]uint32, 16)
	out := RepackRGBAToBGRA(pix, 2, 2, 2, scratch)
	c.Equal(4, len(out))
	c.True(&out[0] == &scratch[0])
	c.Equal([]uint32{0x44112233, 0x88556677, 0x00000000, 0xFF00FF00}, out)

	// A zero-length slice over the same backing array still gets reused, as happens frame after frame once a window
	// has presented at a given size.
	again := RepackRGBAToBGRA(pix, 2, 2, 2, out[:0])
	c.True(&again[0] == &out[0])
	c.Equal(out, again)

	// Too little capacity forces a new allocation of the right size, as happens when a window grows.
	grown := RepackRGBAToBGRA(pix, 2, 2, 2, make([]uint32, 1))
	c.Equal(4, len(grown))
	c.Equal([]uint32{0x44112233, 0x88556677, 0x00000000, 0xFF00FF00}, grown)
}
