// Copyright (c) 2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

// Package f32 provides float32 slice kernels whose results do not depend on the platform or the build.
//
// Each kernel dispatches through a package-level function variable: the portable form lives beside the variable, under
// a Generic name and with no build tag, and a goexperiment.simd build's init (sum_simd.go) repoints the variable at an
// archsimd kernel on qualifying hardware. Declaring the defaults here, untagged, keeps the portable forms referenced on
// every build — they are the reference twin the equivalence suites fuzz against and the dispatch default everywhere no
// vector lane applies.
//
// Every substitute is bit-identical to the portable form (locked by TestSumSIMDMatchesScalar, with the one exception
// noted below), so the choice of lane changes throughput only, never the value a caller sees. That is why the kernels
// here fix their reduction order in the exported contract rather than leaving it to the implementation: a portable form
// free to sum sequentially and a vector form free to sum four lanes at a time would return different bits for the same
// input, which is exactly what callers that compare, hash or persist these results cannot have.
//
// The one thing outside that guarantee is the payload of a NaN result. IEEE 754 leaves payload propagation to the
// implementation, and every layer below these kernels takes it up differently: an add with two NaN operands returns
// the *first* operand's payload on both arm64 and x86, but Go is free to commute a float add, so which operand is
// first is the compiler's choice rather than the source's — sumGeneric's loop was observed on arm64 to select the
// other one than the vector lane does, i.e. to have compiled to values[i]+acc — and the default NaN the hardware
// manufactures for an invalid operation such as +Inf + -Inf is not even the same value across arches (0x7FC00000 on
// arm64, 0xFFC00000 on x86). So these kernels promise NaN-ness, which the equivalence suite checks, and not a NaN's
// bits, which nothing written in Go can promise.
package f32

// The dispatch variables. Each defaults to the portable form declared beside it; sum_simd.go's init is the only other
// assignment.
var sumFn = sumGeneric

// Sum returns the sum of values; an empty slice sums to 0. NaN and infinity propagate exactly as they do through plain
// float32 addition — no operand is special-cased, so a NaN anywhere yields NaN and opposing infinities yield NaN. Every
// accumulator starts at +0, which is the identity of float32 addition, so a slice of nothing but negative zeros sums to
// +0. The bits of a NaN result are the one part of the result this package does not pin down; see the package comment.
//
// The association is fixed, and is part of the contract rather than an implementation detail: over the largest prefix
// whose length is a multiple of four, values[0], values[4], ... accumulate into acc0, values[1], values[5], ... into
// acc1, and likewise into acc2 and acc3; those four accumulators combine as ((acc0+acc1)+acc2)+acc3; and the trailing
// len(values)%4 elements are then added on sequentially, in order.
//
// Because float32 addition is not associative, that is not the value a plain sequential loop returns. The two normally
// agree to within a few ULPs of the result (they differ only in where the rounding falls), and can diverge much further
// on input that cancels catastrophically or that mixes wildly different magnitudes. The fixed association is what buys
// that difference: four accumulators are exactly what a 4-wide vector accumulator holds, one lane each, so the portable
// kernel and the simd kernel return bit-identical results on every platform, every CPU and every build (NaN payloads
// excepted, as above).
func Sum(values []float32) float32 { return sumFn(values) }

// sumGeneric is the portable form of Sum: four scalar accumulators fed the 4-strided association Sum documents,
// combined as ((acc0+acc1)+acc2)+acc3, then the sub-quad tail added on sequentially. The accumulators are float32
// variables, so every partial sum is rounded to float32 precisely where the vector kernel's lanes round it, and Go
// neither reassociates nor fuses these adds (fusion is permitted only for a multiply followed by an add).
func sumGeneric(values []float32) float32 {
	n := len(values)
	var acc0, acc1, acc2, acc3 float32
	i := 0
	for ; i+4 <= n; i += 4 {
		acc0 += values[i]
		acc1 += values[i+1]
		acc2 += values[i+2]
		acc3 += values[i+3]
	}
	total := ((acc0 + acc1) + acc2) + acc3
	for ; i < n; i++ {
		total += values[i]
	}
	return total
}
