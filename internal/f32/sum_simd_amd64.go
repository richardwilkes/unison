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
// it is not: on a Xeon W-2191B (darwin/amd64, go1.27.0, benchstat n=10, 2026-08-21, via simd-bench.sh) the vector form
// ran +2% at n=8, +6% at n=64, and level within noise (p=0.09) at n=1024. The issue-count argument this constant first
// landed on — one VADDPS and one load per quad against the scalar lane's four of each — did not survive measurement:
// the portable form's four independent accumulators already keep this silicon's ALUs saturated, and the out-of-line
// kernel call costs more than the arithmetic it saves. Both forms return identical bits, so preferring the portable
// one changes nothing but speed. Dispatch stays on sumGeneric here; TestSumSIMDWiring skips (it gates on this
// constant) while TestSumSIMDMatchesScalar keeps verifying the kernel itself.
const preferSIMDSum = false
