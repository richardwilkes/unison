// Copyright (c) 2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

//go:build goexperiment.simd

package f32

import "simd/archsimd"

// simdSumSupported reports whether the CPU can run the simd float32 kernels. The gate is AVX2 only, following the rule
// "gate on what the kernels actually execute": the reduction body compiles to a 128-bit load and VADDPS, which are AVX
// and below, and the only thing in the kernel that needs AVX2 is the broadcast priming the accumulator (archsimd
// emulates BroadcastFloat32x4 there and marks it AVX2). No FMA appears anywhere — a lanewise add is not a multiply-add
// — so requiring FMA would be inaccurate and would disable the path, and skip its equivalence tests, under Rosetta 2,
// which offers AVX2 without FMA. Unqualified CPUs keep the portable dispatch, which returns the same bits regardless.
func simdSumSupported() bool { return archsimd.X86.AVX2() }

// Per-kernel dispatch preference: whether the simd kernel is at least as fast as this build's default lane. On amd64
// the only alternative is the portable form in sum.go, and the two run the identical dependency chain — four float32
// accumulators over the same elements — with the vector lane spending one VADDPS and one load per quad where the scalar
// lane spends four of each, so it should win outright on issue count alone.
//
// That is an argument, not a measurement. This landed with benchstat numbers from arm64 hardware only (see
// sum_simd_arm64.go); a real amd64 benchstat run — BenchmarkSum with and without GOEXPERIMENT=simd, n>=10 — should
// confirm the constant before it is treated as settled, the way the canvas kernels' preference tables were.
const preferSIMDSum = true
