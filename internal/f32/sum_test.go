// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package f32

import (
	"math"
	"math/rand/v2"
	"strconv"
	"testing"
)

// f32Eps is the float32 unit roundoff, 2^-23: the gap between 1 and the next representable float32. Used only for the
// error bound in TestSumTracksSequentialSum.
const f32Eps = 1.1920928955078125e-07

// refSum is an independent statement of the association Sum documents, deliberately written in a different shape than
// either kernel — one element at a time, selecting its accumulator by index mod four, rather than an unrolled quad body
// — so that this file locks the *contract* rather than whatever sum.go currently happens to compute. Everything below
// compares against it bit for bit; if a kernel is ever rewritten, this is the statement it has to keep agreeing with.
func refSum(values []float32) float32 {
	var lanes [4]float32
	full := len(values) - len(values)%4
	for i := range full {
		lanes[i%4] += values[i]
	}
	total := ((lanes[0] + lanes[1]) + lanes[2]) + lanes[3]
	for _, v := range values[full:] {
		total += v
	}
	return total
}

// seqSum is the plain left-to-right sum Sum deliberately does not compute, used to document the relationship between
// the two rather than to check Sum against it.
func seqSum(values []float32) float32 {
	var total float32
	for _, v := range values {
		total += v
	}
	return total
}

// checkSum requires Sum to reproduce refSum's bits exactly. Bit comparison rather than == is what makes the NaN and
// signed-zero cases meaningful: NaN != NaN, and -0 == +0, so equality would pass on results the contract forbids.
//
// Bits are the right comparison for NaN results here only because every case below keeps a single NaN payload in play:
// an add with one NaN operand returns that operand's payload whatever the operand order, so nothing in this file
// depends on the order the compiler picks. A case mixing two distinct payloads — an input NaN alongside the default
// NaN an invalid operation manufactures, say — would be comparing something neither kernel promises (see the package
// comment) and belongs in the simd suite's nan subtest, which checks NaN-ness instead.
func checkSum(t *testing.T, name string, values []float32) {
	t.Helper()
	got, want := Sum(values), refSum(values)
	if math.Float32bits(got) != math.Float32bits(want) {
		t.Fatalf("%s (n=%d): Sum = %v (%08x), want %v (%08x)", name, len(values), got, math.Float32bits(got), want,
			math.Float32bits(want))
	}
}

// TestSumLengths walks every length from 0 through 17, which covers the empty slice, every length shorter than one
// vector chunk, both boundaries around the first and second chunk, and every len%4 tail residue several times over.
// The patterns are chosen so the association is observable at these lengths: a pattern whose partial sums are all
// exact would agree with any grouping and would prove nothing.
func TestSumLengths(t *testing.T) {
	for name, gen := range map[string]func(i int) float32{
		// Magnitudes spanning ~2^30, so which elements land in which accumulator decides what gets rounded away.
		"spread": func(i int) float32 { return float32(math.Ldexp(1, (i%8)*4-16)) },
		// Alternating signs across the four lanes, so lanes cancel against each other at combine time.
		"alternating": func(i int) float32 { return float32(1+i%5) * float32(1-2*(i&1)) },
		// A large leading value the small ones cannot individually move, the classic case for regrouping to matter.
		"big-then-small": func(i int) float32 {
			if i == 0 {
				return 1 << 23
			}
			return 0.5
		},
		// Denormals, where every partial sum is exact but the lane assignment still has to match.
		"denormal": func(i int) float32 { return float32(i+1) * math.SmallestNonzeroFloat32 },
	} {
		for n := range 18 {
			values := make([]float32, n)
			for i := range values {
				values[i] = gen(i)
			}
			checkSum(t, name, values)
		}
	}
}

// TestSumSpecialValues drives the float32 classes the arithmetic treats specially — NaN, both infinities, signed zero,
// denormals, mixed signs and catastrophic cancellation — through every lane position and both sides of the chunk/tail
// split, and requires Sum to reproduce the reference association exactly. The answers the contract fixes outright are
// also checked directly, so broken propagation or association could not hide behind a matching reference.
func TestSumSpecialValues(t *testing.T) {
	var (
		inf  = float32(math.Inf(1))
		ninf = float32(math.Inf(-1))
		nan  = float32(math.NaN())
		nz   = float32(math.Copysign(0, -1))
		den  = float32(math.SmallestNonzeroFloat32)
		big  = float32(3.4e38)
	)
	for name, values := range map[string][]float32{
		"nan-lane0":         {nan, 1, 2, 3},
		"nan-lane3":         {1, 2, 3, nan},
		"nan-in-tail":       {1, 2, 3, 4, 5, nan},
		"nan-second-chunk":  {1, 2, 3, 4, nan, 6, 7, 8, 9},
		"inf-plus-finite":   {inf, 1, 2, 3, 4, 5, 6, 7},
		"neg-inf":           {1, ninf, 2, 3},
		"opposing-inf":      {inf, ninf, 1, 2},
		"opposing-inf-lane": {inf, 1, 2, 3, ninf, 4, 5, 6},
		"overflow-to-inf":   {big, big, big, big, big, big, big, big},
		"neg-zeros":         {nz, nz, nz, nz, nz, nz},
		"zero-mix":          {0, nz, nz, 0, nz, 0, nz},
		"denormals":         {den, den, den, den, den, den, den},
		"denormal-cancel":   {den, -den, den, -den, den},
		"underflow":         {1, den, -1, den, den},
		"mixed-signs":       {1, -2, 3, -4, 5, -6, 7, -8, 9},
		"cancellation":      {1e30, 1, -1e30, 1, 1e30, -1, -1e30, -1},
		"cancel-tail":       {1e20, -1e20, 1, 1, 1e-20},
		"absorb":            {1 << 24, 1, 1, 1, 1, 1, 1, 1, 1},
		"absorb-reversed":   {1, 1, 1, 1, 1, 1, 1, 1, 1 << 24},
		"tiny-and-huge":     {3.4e38, 1e-38, -3.4e38, 1e-38, 1e-38},
	} {
		checkSum(t, name, values)
	}

	// The answers the contract fixes outright, checked against the contract rather than against the reference. +0 is
	// the identity every accumulator starts from, which is why an input of nothing but negative zeros sums to +0. For
	// 2^24 then seven 1s, acc0 rounds 2^24+1 away while acc1-acc3 each hold an exact 2, so the fixed association gives
	// 2^24+6 where a sequential loop rounds every add away and lands on 2^24.
	isNaN := func(got float32) bool { return math.IsNaN(float64(got)) }
	for name, tc := range map[string]struct {
		check  func(got float32) bool
		values []float32
	}{
		"nan propagates":        {check: isNaN, values: []float32{1, 2, nan, 4, 5}},
		"opposing infs are nan": {check: isNaN, values: []float32{inf, 1, ninf, 2}},
		"nan beats inf":         {check: isNaN, values: []float32{inf, nan, 1, 2, 3}},
		"inf propagates": {
			check:  func(got float32) bool { return math.IsInf(float64(got), 1) },
			values: []float32{inf, 1, 2, 3, 4},
		},
		"neg inf propagates": {
			check:  func(got float32) bool { return math.IsInf(float64(got), -1) },
			values: []float32{1, 2, 3, ninf, 4},
		},
		"neg zeros sum to +0": {
			check:  func(got float32) bool { return math.Float32bits(got) == 0 },
			values: []float32{nz, nz, nz, nz, nz},
		},
		"association differs from sequential": {
			check:  func(got float32) bool { return math.Float32bits(got) == math.Float32bits(1<<24+6) },
			values: []float32{1 << 24, 1, 1, 1, 1, 1, 1, 1},
		},
	} {
		if got := Sum(tc.values); !tc.check(got) {
			t.Fatalf("%s: Sum = %v (%08x)", name, got, math.Float32bits(got))
		}
	}
}

// TestSumRandom fuzzes Sum against the reference association over random lengths and random finite values, including
// lengths well past the point where the accumulators have drifted apart from each other.
func TestSumRandom(t *testing.T) {
	rng := rand.New(rand.NewPCG(11, 12)) //nolint:gosec // Deterministic seed for reproducible tests
	for range 4096 {
		n := rng.IntN(300)
		values := make([]float32, n)
		for i := range values {
			// A wide exponent spread rather than a uniform [0,1): rounding has to actually be lost somewhere for the
			// association to be observable.
			values[i] = float32(math.Ldexp(rng.Float64()*2-1, rng.IntN(48)-24))
		}
		checkSum(t, "random", values)
	}
}

// TestSumTracksSequentialSum documents how far Sum's fixed association drifts from a sequential loop on well-behaved
// input: for values that are all the same sign, where nothing cancels, both orderings are accurate to the standard
// n*eps relative bound, so they cannot differ from each other by more than twice it. The check is a relative tolerance,
// not a bit comparison — the two results are expected to differ, just not by much — and it is the one place in this
// file where being off by a ULP is not a failure.
func TestSumTracksSequentialSum(t *testing.T) {
	rng := rand.New(rand.NewPCG(13, 14)) //nolint:gosec // Deterministic seed for reproducible tests
	for _, n := range []int{7, 64, 1000, 4096} {
		values := make([]float32, n)
		for i := range values {
			values[i] = rng.Float32() + 0.5
		}
		got, want := float64(Sum(values)), float64(seqSum(values))
		tolerance := 2 * float64(n) * f32Eps
		if rel := math.Abs(got-want) / math.Abs(want); rel > tolerance {
			t.Fatalf("n=%d: Sum = %v, sequential = %v, relative difference %g exceeds %g", n, got, want, rel, tolerance)
		}
	}
}

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
