// Copyright (c) 2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package f32

import (
	"strconv"
	"testing"
)

// sumSink keeps the benchmarked result live.
var sumSink float32

// BenchmarkSum measures the exported entry point, so it reports whichever lane this build's dispatch elected. That is
// also how the per-arch preference constants are meant to be checked: run it twice, once plain and once with
// GOEXPERIMENT=simd, and benchstat the two — the portable form is what the simd kernel has to beat, and the two runs
// differ in nothing else.
//
// The three lengths bracket where the choice of lane matters: n=8 is two vector chunks, short enough that the call and
// the loop setup dominate; n=64 is a length the vector body can amortize; n=1024 is long enough to run against memory
// bandwidth rather than issue width.
func BenchmarkSum(b *testing.B) {
	for _, n := range []int{8, 64, 1024} {
		b.Run("n="+strconv.Itoa(n), func(b *testing.B) {
			values := make([]float32, n)
			for i := range values {
				// A spread of magnitudes and signs, so the adds are inexact and no value is special-cased anywhere.
				values[i] = float32(1+i%13) * 0.375 * float32(1-2*(i&1))
			}
			b.SetBytes(int64(4 * n))
			for b.Loop() {
				sumSink = Sum(values)
			}
		})
	}
}
