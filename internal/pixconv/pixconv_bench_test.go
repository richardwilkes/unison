// Copyright (c) 2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package pixconv

import (
	"math/rand/v2"
	"testing"
)

// The benchmarks run a whole 1920x1080 frame through each conversion, which is the unit the presentation paths
// actually call them in: a per-row or per-pixel benchmark would be dominated by call overhead these kernels never pay
// in production. They go through the exported wrappers rather than a named kernel, so the same benchmark measures
// whichever lane the build wired up and a `benchstat` of a plain build against a GOEXPERIMENT=simd build is the
// portable-versus-vector comparison directly. b.SetBytes reports the source bytes converted (four per pixel either
// way), so the throughput figure is comparable across all six.
const (
	benchWidth  = 1920
	benchHeight = 1080
	benchPixels = benchWidth * benchHeight
	benchBytes  = benchPixels * 4
)

// benchWords returns a frame of pseudo-random device words, seeded so every run converts the same frame.
func benchWords() []uint32 {
	rng := rand.New(rand.NewPCG(51, 52)) //nolint:gosec // Deterministic seed for reproducible tests
	w := make([]uint32, benchPixels)
	for i := range w {
		w[i] = rng.Uint32()
	}
	return w
}

// benchQuads returns a frame of pseudo-random straight-alpha r,g,b,a quads, seeded so every run converts the same
// frame.
func benchQuads() []byte {
	rng := rand.New(rand.NewPCG(53, 54)) //nolint:gosec // Deterministic seed for reproducible tests
	q := make([]byte, benchBytes)
	for i := range q {
		q[i] = byte(rng.Uint32())
	}
	return q
}

func BenchmarkSwizzleRB(b *testing.B) {
	src := benchWords()
	dst := make([]uint32, benchPixels)
	b.SetBytes(benchBytes)
	b.ResetTimer()
	for b.Loop() {
		SwizzleRB(dst, src)
	}
}

func BenchmarkSwizzleRBBytes(b *testing.B) {
	src := benchQuads()
	dst := make([]byte, benchBytes)
	b.SetBytes(benchBytes)
	b.ResetTimer()
	for b.Loop() {
		SwizzleRBBytes(dst, src)
	}
}

func BenchmarkRGBAToBGRABytes(b *testing.B) {
	src := benchWords()
	dst := make([]byte, benchBytes)
	b.SetBytes(benchBytes)
	b.ResetTimer()
	for b.Loop() {
		RGBAToBGRABytes(dst, src)
	}
}

func BenchmarkRGBAToARGBBytes(b *testing.B) {
	src := benchWords()
	dst := make([]byte, benchBytes)
	b.SetBytes(benchBytes)
	b.ResetTimer()
	for b.Loop() {
		RGBAToARGBBytes(dst, src)
	}
}

func BenchmarkPremulBGRA(b *testing.B) {
	src := benchQuads()
	dst := make([]byte, benchBytes)
	b.SetBytes(benchBytes)
	b.ResetTimer()
	for b.Loop() {
		PremulBGRA(dst, src)
	}
}

func BenchmarkPremulARGB(b *testing.B) {
	src := benchQuads()
	dst := make([]byte, benchBytes)
	b.SetBytes(benchBytes)
	b.ResetTimer()
	for b.Loop() {
		PremulARGB(dst, src)
	}
}
