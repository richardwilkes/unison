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
// GDI expects (B in the low byte), with the green and alpha bytes staying in place. Padding pixels beyond each row's
// width never reach the output, and nothing is written past the frame in a larger destination.
func TestRepackRGBAToBGRASwizzle(t *testing.T) {
	c := check.New(t)
	for _, tc := range []struct {
		name                  string
		pix                   []uint32
		dst                   []uint32
		want                  []uint32
		width, height, stride int
	}{
		{
			name: "stride padding is dropped",
			pix: []uint32{
				0x44332211, 0x88776655, 0xDEADBEEF,
				0x00000000, 0xFF00FF00, 0xDEADBEEF,
			},
			width:  2,
			height: 2,
			stride: 3,
			dst:    make([]uint32, 4),
			want:   []uint32{0x44112233, 0x88556677, 0x00000000, 0xFF00FF00},
		},
		{
			name:   "every word of the frame is overwritten and nothing past it is touched",
			pix:    []uint32{0x44332211, 0x88776655, 0x00000000, 0xFF00FF00},
			width:  2,
			height: 2,
			stride: 2,
			dst:    []uint32{0xDEADBEEF, 0xDEADBEEF, 0xDEADBEEF, 0xDEADBEEF, 0xDEADBEEF, 0xDEADBEEF},
			want:   []uint32{0x44112233, 0x88556677, 0x00000000, 0xFF00FF00, 0xDEADBEEF, 0xDEADBEEF},
		},
	} {
		RepackRGBAToBGRA(tc.pix, tc.width, tc.height, tc.stride, tc.dst)
		c.Equal(tc.want, tc.dst, tc.name)
	}
}
