// Copyright (c) 2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

//go:build goexperiment.simd && (arm64 || amd64)

package pixconv

import (
	"math/rand/v2"
	"reflect"
	"testing"
)

// pixconvSIMDLen draws a pixel count for the randomized subtests: mostly runs short enough that the four-pixel vector
// body and its tail split every possible way (including runs with no vector body at all), and one in four long enough
// that the vector body carries the bulk of the work, as it does on a real frame.
func pixconvSIMDLen(rng *rand.Rand) int {
	if rng.IntN(4) == 0 {
		return rng.IntN(300)
	}
	return rng.IntN(19)
}

// pixconvSIMDWords returns n random device words.
func pixconvSIMDWords(rng *rand.Rand, n int) []uint32 {
	w := make([]uint32, n)
	for i := range w {
		w[i] = rng.Uint32()
	}
	return w
}

// pixconvSIMDQuads returns n random straight-alpha r,g,b,a quads, with alpha biased toward the 0 and 255 endpoints so
// the fully-transparent and fully-opaque pixels that dominate real icons and cursors are not 1-in-256 rare here.
func pixconvSIMDQuads(rng *rand.Rand, n int) []byte {
	q := make([]byte, n*4)
	for i := range q {
		q[i] = byte(rng.Uint32())
	}
	for i := 0; i < len(q); i += 4 {
		switch rng.IntN(4) {
		case 0:
			q[i+3] = 0
		case 1:
			q[i+3] = 255
		}
	}
	return q
}

// TestPixconvSIMDMatchesScalar drives every simd converter and its portable twin over identical inputs and requires
// bitwise identity. The randomized subtests cover every chunk/tail split; the exhaustive premultiply subtest that
// follows enumerates the complete (channel value, alpha) domain in each color byte position, which proves the vector
// divide identity right outright rather than sampling for it.
func TestPixconvSIMDMatchesScalar(t *testing.T) {
	if !simdPixconvSupported() {
		t.Skip("CPU lacks the features the simd pixel converters require; dispatch stays on the portable forms")
	}
	rng := rand.New(rand.NewPCG(41, 42)) //nolint:gosec // Deterministic seed for reproducible tests

	t.Run("swizzleRB", func(t *testing.T) {
		for range 4096 {
			n := pixconvSIMDLen(rng)
			src := pixconvSIMDWords(rng, n)
			want := make([]uint32, n)
			got := make([]uint32, n)
			swizzleRBGeneric(want, src)
			swizzleRBSIMD(got, src)
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("swizzleRB n=%d word %d src %08x = %08x, want %08x", n, i, src[i], got[i], want[i])
				}
			}
		}
	})

	t.Run("swizzleRBBytes", func(t *testing.T) {
		for range 4096 {
			n := pixconvSIMDLen(rng)
			src := pixconvSIMDQuads(rng, n)
			want := make([]byte, n*4)
			got := make([]byte, n*4)
			swizzleRBBytesGeneric(want, src)
			swizzleRBBytesSIMD(got, src)
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("swizzleRBBytes n=%d byte %d = %02x, want %02x", n, i, got[i], want[i])
				}
			}
		}
	})

	t.Run("rgbaToBGRABytes", func(t *testing.T) {
		for range 4096 {
			n := pixconvSIMDLen(rng)
			src := pixconvSIMDWords(rng, n)
			want := make([]byte, n*4)
			got := make([]byte, n*4)
			rgbaToBGRABytesGeneric(want, src)
			rgbaToBGRABytesSIMD(got, src)
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("rgbaToBGRABytes n=%d byte %d src %08x = %02x, want %02x", n, i, src[i/4], got[i], want[i])
				}
			}
		}
	})

	t.Run("rgbaToARGBBytes", func(t *testing.T) {
		for range 4096 {
			n := pixconvSIMDLen(rng)
			src := pixconvSIMDWords(rng, n)
			want := make([]byte, n*4)
			got := make([]byte, n*4)
			rgbaToARGBBytesGeneric(want, src)
			rgbaToARGBBytesSIMD(got, src)
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("rgbaToARGBBytes n=%d byte %d src %08x = %02x, want %02x", n, i, src[i/4], got[i], want[i])
				}
			}
		}
	})

	t.Run("premulBGRA", func(t *testing.T) {
		for range 4096 {
			n := pixconvSIMDLen(rng)
			src := pixconvSIMDQuads(rng, n)
			want := make([]byte, n*4)
			got := make([]byte, n*4)
			premulBGRAGeneric(want, src)
			premulBGRASIMD(got, src)
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("premulBGRA n=%d pixel %d channel %d src %v = %02x, want %02x", n, i/4, i%4,
						src[i&^3:i&^3+4], got[i], want[i])
				}
			}
		}
	})

	t.Run("premulARGB", func(t *testing.T) {
		for range 4096 {
			n := pixconvSIMDLen(rng)
			src := pixconvSIMDQuads(rng, n)
			want := make([]byte, n*4)
			got := make([]byte, n*4)
			premulARGBGeneric(want, src)
			premulARGBSIMD(got, src)
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("premulARGB n=%d pixel %d channel %d src %v = %02x, want %02x", n, i/4, i%4,
						src[i&^3:i&^3+4], got[i], want[i])
				}
			}
		}
	})

	t.Run("premulExhaustive", func(t *testing.T) {
		// Every (channel value, alpha) pair in each of the three color byte positions: 256-pixel rows sweep the channel
		// value and the loops walk the alpha and the position. The two channels not sweeping hold fixed but distinct
		// values, so all four lanes of a vector chunk carry different products at every step, and the row length is an
		// exact multiple of the chunk, so the tail path is left to the randomized subtests above.
		const others = 0x5B
		src := make([]byte, 256*4)
		want := make([]byte, 256*4)
		got := make([]byte, 256*4)
		for pos := range 3 {
			for a := range 256 {
				for i := range 256 {
					src[i*4] = others
					src[i*4+1] = others ^ 0x3C
					src[i*4+2] = others ^ 0x7E
					src[i*4+pos] = byte(i)
					src[i*4+3] = byte(a)
				}
				premulBGRAGeneric(want, src)
				premulBGRASIMD(got, src)
				for i := range want {
					if got[i] != want[i] {
						t.Fatalf("premulBGRA pos=%d alpha=%d pixel %d channel %d: %02x, want %02x", pos, a, i/4, i%4,
							got[i], want[i])
					}
				}
				premulARGBGeneric(want, src)
				premulARGBSIMD(got, src)
				for i := range want {
					if got[i] != want[i] {
						t.Fatalf("premulARGB pos=%d alpha=%d pixel %d channel %d: %02x, want %02x", pos, a, i/4, i%4,
							got[i], want[i])
					}
				}
			}
		}
	})
}

// TestPixconvSIMDWiring locks that the goexperiment.simd build's init actually repointed every dispatch variable whose
// per-arch preference constant elects the simd kernel, so a refactor cannot silently fall back to the portable forms.
func TestPixconvSIMDWiring(t *testing.T) {
	if !simdPixconvSupported() {
		t.Skip("CPU lacks the features the simd pixel converters require; dispatch stays on the portable forms")
	}
	for _, tc := range []struct {
		wired  any
		simd   any
		name   string
		prefer bool
	}{
		{swizzleRBFn, swizzleWordsFn(swizzleRBSIMD), "SwizzleRB", preferSIMDSwizzleRB},
		{swizzleRBBytesFn, swizzleBytesFn(swizzleRBBytesSIMD), "SwizzleRBBytes", preferSIMDSwizzleRBBytes},
		{rgbaToBGRABytesFn, unpackWordsFn(rgbaToBGRABytesSIMD), "RGBAToBGRABytes", preferSIMDRGBAToBGRABytes},
		{rgbaToARGBBytesFn, unpackWordsFn(rgbaToARGBBytesSIMD), "RGBAToARGBBytes", preferSIMDRGBAToARGBBytes},
		{premulBGRAFn, premulBytesFn(premulBGRASIMD), "PremulBGRA", preferSIMDPremulBGRA},
		{premulARGBFn, premulBytesFn(premulARGBSIMD), "PremulARGB", preferSIMDPremulARGB},
	} {
		if tc.prefer && reflect.ValueOf(tc.wired).Pointer() != reflect.ValueOf(tc.simd).Pointer() {
			t.Fatalf("%s: dispatch fn is not the simd kernel", tc.name)
		}
	}
}
