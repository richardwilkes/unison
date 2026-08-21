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

import (
	"math"
	"math/rand/v2"
	"reflect"
	"testing"
)

// simdHostileFloats seeds the fuzz with the float32 classes the kernels have to handle exactly: signed zeros, both
// infinities, denormals, the extremes where one more add overflows, the powers of two where an add starts losing its
// addend, and ordinary values whose sums are inexact. NaN is deliberately absent — it is the one class whose result the
// kernels cannot promise bit for bit, so the nan subtest below covers it on its own terms. (The idiom, and much of the
// list, comes from the canvas raster package's span kernel equivalence suite; the packages do not share test helpers.)
var simdHostileFloats = []float32{
	0, float32(math.Copysign(0, -1)), 1, -1, 0.5, -0.5, 2, 1 << 23, 1 << 24, -(1 << 24),
	float32(math.Inf(1)), float32(math.Inf(-1)),
	math.SmallestNonzeroFloat32, -math.SmallestNonzeroFloat32, 1e-38, -1e-38,
	math.MaxFloat32, -math.MaxFloat32, 3.4e38, -3.4e38, 1e-45, 0.1, 1.0 / 3.0, 2.0 / 3.0, 16777215, 16777217,
}

// simdFuzzFloat turns arbitrary random bits into a value the strict subtest can hold both kernels to bit for bit. NaN
// patterns are mapped to the infinity of the same sign, which keeps the extreme the draw was reaching for and keeps
// every NaN out of the strict comparison: an add of two NaNs returns one operand's payload and neither the hardware nor
// the compiler fixes which (see the package comment), so a slice carrying two different NaN payloads — an input one and
// the default NaN an invalid operation manufactures, which is not even the same value on arm64 and x86 — could differ
// between the lanes in its payload alone. The nan subtest covers NaN propagation deliberately rather than by accident.
func simdFuzzFloat(bits uint32) float32 {
	const expMask, mantissaMask = 0x7F800000, 0x007FFFFF
	if bits&expMask == expMask && bits&mantissaMask != 0 {
		bits &^= mantissaMask
	}
	return math.Float32frombits(bits)
}

// simdSumLen draws a length for the fuzz. Half the draws cluster within three of a multiple of four, which is where the
// vector body's chunk/tail split lives and where a mis-sized loop bound would show; a quarter are short enough that the
// body may not run at all, the empty slice included; the rest sample the whole 0..1000 range, long enough for the four
// accumulators to have drifted well apart before they are combined.
func simdSumLen(rng *rand.Rand) int {
	switch rng.IntN(4) {
	case 0:
		return rng.IntN(17)
	case 1:
		return rng.IntN(1001)
	default:
		return min(1000, max(0, 4*(1+rng.IntN(250))+rng.IntN(7)-3))
	}
}

// TestSumSIMDMatchesScalar drives sumSIMD and its portable twin over identical randomized slices and requires them to
// agree — the property the whole package rests on, since callers are promised the same answer from every build. Two
// thirds of the elements are random bit patterns (every finite magnitude, both signs) and the rest come from the
// hostile list, so infinities, signed zeros, denormals and the absorbing magnitudes land in every lane position and on
// both sides of the chunk/tail split.
//
// The two subtests split on what "agree" can mean. Everything the contract pins down is checked bit for bit; NaN
// results, whose payload no lane can promise, are checked for NaN-ness, which is what the contract does promise.
func TestSumSIMDMatchesScalar(t *testing.T) {
	if !simdSumSupported() {
		t.Skip("CPU lacks the features the simd kernels require; dispatch stays on the portable forms")
	}
	rng := rand.New(rand.NewPCG(41, 42)) //nolint:gosec // Deterministic seed for reproducible tests
	pick := func() float32 {
		if rng.IntN(3) == 0 {
			return simdHostileFloats[rng.IntN(len(simdHostileFloats))]
		}
		return simdFuzzFloat(rng.Uint32())
	}
	values := func(n int) []float32 {
		buf := make([]float32, n)
		for i := range buf {
			buf[i] = pick()
		}
		return buf
	}

	t.Run("strict", func(t *testing.T) {
		for range 8192 {
			n := simdSumLen(rng)
			buf := values(n)
			got, want := sumSIMD(buf), sumGeneric(buf)
			if math.Float32bits(got) != math.Float32bits(want) {
				t.Fatalf("n=%d: sumSIMD = %v (%08x), sumGeneric = %v (%08x); values %v", n, got,
					math.Float32bits(got), want, math.Float32bits(want), buf)
			}
		}
	})

	t.Run("nan", func(t *testing.T) {
		// A NaN anywhere makes every subsequent add in its accumulator, and then the combine, return a NaN, so both
		// kernels must report NaN no matter which lane the NaN lands in or how many there are. The payloads are drawn
		// at random precisely because the result must not depend on them.
		for range 2048 {
			n := 1 + simdSumLen(rng)
			buf := values(n)
			for range 1 + rng.IntN(3) {
				buf[rng.IntN(n)] = math.Float32frombits(0x7F800001 | rng.Uint32()&0x807FFFFF)
			}
			got, want := sumSIMD(buf), sumGeneric(buf)
			if !math.IsNaN(float64(got)) || !math.IsNaN(float64(want)) {
				t.Fatalf("n=%d: sumSIMD = %v (%08x), sumGeneric = %v (%08x); both should be NaN; values %v", n, got,
					math.Float32bits(got), want, math.Float32bits(want), buf)
			}
		}
	})
}

// TestSumSIMDWiring locks that where the simd kernels are the preferred lane, the goexperiment.simd build's init
// actually repointed the dispatch variables at them, so a refactor cannot silently leave every build on the portable
// forms — which would still be correct, and therefore silent, but would give up the whole point of the file.
func TestSumSIMDWiring(t *testing.T) {
	if !simdSumSupported() || !preferSIMDSum {
		t.Skip("the simd kernels are not the preferred lane on this hardware; dispatch keeps the portable forms")
	}
	if reflect.ValueOf(sumFn).Pointer() != reflect.ValueOf(sumSIMD).Pointer() {
		t.Fatal("sumFn: dispatch fn is not the simd kernel")
	}
}
