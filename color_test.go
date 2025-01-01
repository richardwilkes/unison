// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
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

	"github.com/richardwilkes/toolbox/check"
	"github.com/richardwilkes/unison"
)

func TestOKLCH(t *testing.T) {
	check.Equal(t, unison.White, unison.OKLCH(1, 0, 0, 1))
	l, c, h := unison.White.OKLCH()
	check.Equal(t, float32(1), l)
	check.Equal(t, float32(0), c)
	check.Equal(t, float32(0), h)

	check.Equal(t, unison.Black, unison.OKLCH(0, 0, 0, 1))
	l, c, h = unison.Black.OKLCH()
	check.Equal(t, float32(0), l)
	check.Equal(t, float32(0), c)
	check.Equal(t, float32(0), h)

	lchGray := unison.RGB(0x11, 0x11, 0x11)
	check.Equal(t, lchGray, unison.OKLCH(0.17763777, 0, 0, 1))
	l, c, h = lchGray.OKLCH()
	check.Equal(t, float32(0.17763777), l)
	check.Equal(t, float32(0), c)
	check.Equal(t, float32(0), h)

	check.Equal(t, unison.Red, unison.OKLCH(0.6279554, 0.2576833, 29.233885, 1))
	l, c, h = unison.Red.OKLCH()
	check.Equal(t, float32(0.6279554), l)
	check.Equal(t, float32(0.2576833), c)
	check.Equal(t, float32(29.233885), h)

	check.Equal(t, unison.Green, unison.OKLCH(0.51975185, 0.17685826, 142.4953389, 1))
	l, c, h = unison.Green.OKLCH()
	check.Equal(t, float32(0.51975185), l)
	check.Equal(t, float32(0.17685826), c)
	check.Equal(t, float32(142.4953389), h)

	check.Equal(t, unison.Blue, unison.OKLCH(0.45201373, 0.31321436, 264.0520206, 1))
	l, c, h = unison.Blue.OKLCH()
	check.Equal(t, float32(0.45201373), l)
	check.Equal(t, float32(0.31321436), c)
	check.Equal(t, float32(264.0520206), h)
}
