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

// simdSumSupported reports whether the CPU can run the simd float32 kernels. Everything they compile to on arm64 — the
// 128-bit loads and stores, VFADD, and the DUP that primes the accumulator — is baseline NEON, present on every arm64
// CPU Go supports, so there is nothing to probe for.
func simdSumSupported() bool { return true }

// Per-kernel dispatch preference: whether the simd kernel is at least as fast as this build's default lane. Unlike the
// canvas span kernels there is no hand-written NEON lane here to lose to — the alternative is the portable form in
// sum.go — and the vector lane beats it at every benchmarked length, including the short one where the call overhead
// has the most to hide behind. Measured on an Apple M4 Max (darwin/arm64, go1.27.0, benchstat n=10, 2026-08-21),
// BenchmarkSum with GOEXPERIMENT=simd against the same benchmark without it: n=8 -34%, n=64 -39%, n=1024 -52%,
// geomean -42%.
const preferSIMDSum = true
