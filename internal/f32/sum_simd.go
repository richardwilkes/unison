// Copyright (c) 2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

//go:build goexperiment.simd && (arm64 || amd64)

package f32

import "simd/archsimd"

// The goexperiment.simd float32 kernels, written against archsimd's 128-bit types: that is the widest shape arm64 and
// amd64 share, so both vector lanes compute the same expressions in the same order. For Sum it is also the shape the
// contract itself is written in — the four lanes of a Float32x4 *are* the four accumulators sum.go documents, one lane
// each — so the vector form is not an approximation of the portable form that has to be argued back into agreement with
// it; the two are the same reduction spelled two ways.
//
// Every kernel here is locked against its portable twin by TestSumSIMDMatchesScalar, which fuzzes the whole float32
// value space, hostile classes included, rather than taking the argument below on faith. Its one concession is the
// payload of a NaN result, which no lane can promise (see the package comment) and which it therefore compares by
// class rather than by bits.

// init swaps the dispatch variables to the simd kernels the per-arch preference constants elect. CPUs that fail the
// gate keep the portable dispatch, as would an arch whose preference constant declined a kernel — the constants are
// per-arch precisely so a lane that loses a benchstat somewhere can be turned off there without touching this file.
func init() {
	if !simdSumSupported() {
		return
	}
	if preferSIMDSum {
		sumFn = sumSIMD
	}
}

// sumSIMD is Sum's vector lane: one Float32x4 accumulator over the largest 4-element-aligned prefix, then the same
// scalar combine and sequential tail sumGeneric performs. It matches sumGeneric by construction, not by coincidence:
//
//   - The accumulator is primed with +0 in every lane, which is what sumGeneric's four float32 accumulators start at,
//     and +0 is the identity of float32 addition for every operand including -0 (+0 + -0 is +0 in both lanes).
//   - Each iteration consumes a whole quad, so lane k receives values[i+k] for exactly the i values the scalar loop
//     feeds acc k, in the same order. A lanewise IEEE float32 add is the same operation on the same operands as the
//     scalar one — same rounding, same overflow to infinity, same denormal handling, since both lanes read the same
//     control state — and Go neither reassociates nor fuses either form.
//   - The accumulator is then stored out and combined by the identical ((a0+a1)+a2)+a3 scalar expression, and the
//     trailing len%4 elements are added on by the identical sequential loop.
//
// So the two lanes agree bit for bit on every input whose result is a number, denormals and signed zeros included. They
// agree on NaN-ness too, but not necessarily on a NaN's payload: an add of two NaNs returns one operand's payload, and
// which one is not fixed by the source, since the compiler may commute the scalar add (it does — see the package
// comment). That is a limit of the language and the hardware rather than of this kernel; no reduction written in Go can
// promise more.
//
// The single accumulator is also why the loop is not unrolled further: extra accumulators would change the association,
// which is the contract, so the only thing wider chunking could buy is a different answer.
func sumSIMD(values []float32) float32 {
	n := len(values)
	acc := archsimd.BroadcastFloat32x4(0)
	i := 0
	for ; i+4 <= n; i += 4 {
		acc = acc.Add(archsimd.LoadFloat32x4(values[i:]))
	}
	var lanes [4]float32
	acc.StoreArray(&lanes)
	total := ((lanes[0] + lanes[1]) + lanes[2]) + lanes[3]
	for ; i < n; i++ {
		total += values[i]
	}
	return total
}
