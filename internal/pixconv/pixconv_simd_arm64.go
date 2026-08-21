// Copyright (c) 2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

//go:build goexperiment.simd

package pixconv

import "simd/archsimd"

// simdPixconvSupported reports whether the CPU can run the simd pixel converters. Everything they compile to on arm64
// (the 128-bit loads and stores, AND/ORR, MUL on 16-bit lanes, USHL, and the free register reinterpretations the
// Reshape calls are) is baseline NEON, present on every arm64 CPU Go supports.
func simdPixconvSupported() bool { return true }

// Per-kernel dispatch preference: whether the simd converter is at least as fast as this build's default lane. None of
// these conversions has a hand-written NEON lane to lose to — the alternative is the portable per-byte form in
// pixconv.go, which every one of them beats: -50% (SwizzleRB) to -70% (SwizzleRBBytes) on a full 1920x1080 frame,
// measured on an M4 Max (darwin/arm64, benchstat n=6, 2026-08-21) with the benchmarks in pixconv_bench_test.go.
const (
	preferSIMDSwizzleRB       = true
	preferSIMDSwizzleRBBytes  = true
	preferSIMDRGBAToBGRABytes = true
	preferSIMDRGBAToARGBBytes = true
	preferSIMDPremulBGRA      = true
	preferSIMDPremulARGB      = true
)

// The lane shifts go through a hoisted count because arm64 has no shift-by-immediate in this API:
// ShiftAllLeft/ShiftAllRight lower to a VDUP of the (negated) count plus VUSHL, and the compiler re-materializes that
// VDUP on every iteration instead of lifting it out of the loop — three instructions per shift in bodies that have
// half a dozen of them. Broadcasting the count once and issuing the bare VUSHL is worth a large multiple on kernels
// this shift-heavy. It is also exactly the instruction ShiftAllRight would have emitted, so the results are unchanged:
// a negative count is a logical right shift for an unsigned vector, and every count these kernels use (8, 16, 24) is
// inside the lane width.
type (
	shiftCount16 = archsimd.Int16x8
	shiftCount32 = archsimd.Int32x4
)

func rightShift16(n uint8) shiftCount16 { return archsimd.BroadcastInt16x8(-int16(n)) }
func leftShift16(n uint8) shiftCount16  { return archsimd.BroadcastInt16x8(int16(n)) }
func rightShift32(n uint8) shiftCount32 { return archsimd.BroadcastInt32x4(-int32(n)) }
func leftShift32(n uint8) shiftCount32  { return archsimd.BroadcastInt32x4(int32(n)) }

func shift16(x archsimd.Uint16x8, by shiftCount16) archsimd.Uint16x8 { return x.Shift(by) }
func shift32(x archsimd.Uint32x4, by shiftCount32) archsimd.Uint32x4 { return x.Shift(by) }
