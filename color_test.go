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

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/unison"
)

func TestOKLCH(t *testing.T) {
	chk := check.New(t)
	chk.Equal(unison.White, unison.OKLCH(1, 0, 0, 1))
	l, c, h := unison.White.OKLCH()
	chk.Equal(float32(1), l)
	chk.Equal(float32(0), c)
	chk.Equal(float32(0), h)

	chk.Equal(unison.Black, unison.OKLCH(0, 0, 0, 1))
	l, c, h = unison.Black.OKLCH()
	chk.Equal(float32(0), l)
	chk.Equal(float32(0), c)
	chk.Equal(float32(0), h)

	lchGray := unison.RGB(0x11, 0x11, 0x11)
	chk.Equal(lchGray, unison.OKLCH(0.17763777, 0, 0, 1))
	l, c, h = lchGray.OKLCH()
	chk.Equal(float32(0.17763777), l)
	chk.Equal(float32(0), c)
	chk.Equal(float32(0), h)

	chk.Equal(unison.Red, unison.OKLCH(0.6279554, 0.2576833, 29.233885, 1))
	l, c, h = unison.Red.OKLCH()
	chk.Equal(float32(0.6279554), l)
	chk.Equal(float32(0.2576833), c)
	chk.Equal(float32(29.233885), h)

	chk.Equal(unison.Green, unison.OKLCH(0.51975185, 0.17685826, 142.4953389, 1))
	l, c, h = unison.Green.OKLCH()
	chk.Equal(float32(0.51975185), l)
	chk.Equal(float32(0.17685826), c)
	chk.Equal(float32(142.4953389), h)

	chk.Equal(unison.Blue, unison.OKLCH(0.45201373, 0.31321436, 264.0520206, 1))
	l, c, h = unison.Blue.OKLCH()
	chk.Equal(float32(0.45201373), l)
	chk.Equal(float32(0.31321436), c)
	chk.Equal(float32(264.052), h)
}
