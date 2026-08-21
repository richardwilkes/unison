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

// simdPixconvSupported reports whether the CPU can run the simd pixel converters. The requirement is AVX2 only, on the
// "gate on what the kernels actually execute" rule: these are all-integer kernels, and everything they compile to is
// SSE2 or below (VMOVDQU, VPAND/VPOR, VPMULLW, VPSRLD/VPSLLD/VPSRLW/VPSLLW with immediate counts) except the
// broadcasts, which need AVX2. Requiring FMA would be inaccurate — no lane here touches a float — and would disable
// the path, and skip its equivalence tests, under Rosetta 2, which offers AVX2 without FMA. Unqualified CPUs keep the
// portable dispatch.
func simdPixconvSupported() bool { return archsimd.X86.AVX2() }

// Per-kernel dispatch preference: whether the simd converter is at least as fast as this build's default lane. On
// amd64 every one of them is, by a wide margin: against the portable per-byte code on a Xeon W-2191B (darwin/amd64,
// go1.27.0, benchstat n=10, 2026-08-21, via simd-bench.sh) the vector forms ran -33% (SwizzleRB) to -64%
// (SwizzleRBBytes), geomean -55% — in line with these kernels' own arm64 numbers (-50% to -70%, see
// pixconv_simd_arm64.go) and with the near-identical canvas raster kernels (-21% to -78% on the same Xeon).
const (
	preferSIMDSwizzleRB       = true
	preferSIMDSwizzleRBBytes  = true
	preferSIMDRGBAToBGRABytes = true
	preferSIMDRGBAToARGBBytes = true
	preferSIMDPremulBGRA      = true
	preferSIMDPremulARGB      = true
)

// The lane shifts go through the same hoisted-count helpers arm64 needs (see pixconv_simd_arm64.go for why it needs
// them). On amd64 there is nothing to hoist — VPSRLW/VPSLLD take the count as an immediate — so the count is just the
// signed amount and the direction test folds away wherever the caller's count is the loop-invariant constant these
// kernels always pass.
type (
	shiftCount16 = int
	shiftCount32 = int
)

func rightShift16(n uint8) shiftCount16 { return -int(n) }
func leftShift16(n uint8) shiftCount16  { return int(n) }
func rightShift32(n uint8) shiftCount32 { return -int(n) }
func leftShift32(n uint8) shiftCount32  { return int(n) }

func shift16(x archsimd.Uint16x8, by shiftCount16) archsimd.Uint16x8 {
	if by < 0 {
		return x.ShiftAllRight(uint64(-by))
	}
	return x.ShiftAllLeft(uint64(by))
}

func shift32(x archsimd.Uint32x4, by shiftCount32) archsimd.Uint32x4 {
	if by < 0 {
		return x.ShiftAllRight(uint64(-by))
	}
	return x.ShiftAllLeft(uint64(by))
}
