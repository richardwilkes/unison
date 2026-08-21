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

// The golden vectors. Every exported conversion is checked against hand-computed output on every build, so the suite
// pins the bytes the window system receives regardless of which lane the dispatch picked. Seven pixels is deliberate:
// it is one full four-pixel vector chunk plus a three-pixel tail, and the prefix sweep each test runs covers the empty
// row and every shorter split as well.
var (
	// goldenWords carries the endpoints (all-zero, all-ones), a word with four distinguishable bytes, and a few
	// arbitrary values, as R|G<<8|B<<16|A<<24 device words.
	goldenWords = []uint32{0x00000000, 0xFFFFFFFF, 0x11223344, 0xDEADBEEF, 0x00FF00FF, 0x12345678, 0x80402010}

	// goldenSwizzleRB is goldenWords with the red and blue bytes exchanged: 0x11223344 has red 0x44 and blue 0x22, so
	// it becomes 0x11443322, and the two palindromic words are unchanged by the swap.
	goldenSwizzleRB = []uint32{0x00000000, 0xFFFFFFFF, 0x11443322, 0xDEEFBEAD, 0x00FF00FF, 0x12785634, 0x80102040}

	// goldenToBGRA is goldenWords unpacked as byte(v>>16), byte(v>>8), byte(v), byte(v>>24) — LSBFirst ZPixmap order.
	goldenToBGRA = []byte{
		0x00, 0x00, 0x00, 0x00,
		0xFF, 0xFF, 0xFF, 0xFF,
		0x22, 0x33, 0x44, 0x11,
		0xAD, 0xBE, 0xEF, 0xDE,
		0xFF, 0x00, 0xFF, 0x00,
		0x34, 0x56, 0x78, 0x12,
		0x40, 0x20, 0x10, 0x80,
	}

	// goldenToARGB is goldenWords unpacked as byte(v>>24), byte(v), byte(v>>8), byte(v>>16) — MSBFirst ZPixmap order.
	goldenToARGB = []byte{
		0x00, 0x00, 0x00, 0x00,
		0xFF, 0xFF, 0xFF, 0xFF,
		0x11, 0x44, 0x33, 0x22,
		0xDE, 0xEF, 0xBE, 0xAD,
		0x00, 0xFF, 0x00, 0xFF,
		0x12, 0x78, 0x56, 0x34,
		0x80, 0x10, 0x20, 0x40,
	}

	// goldenQuads are r,g,b,a byte quads: both endpoints, a quad whose four bytes are all distinct, and quads that
	// leave one channel at an extreme.
	goldenQuads = []byte{
		0x00, 0x00, 0x00, 0x00,
		0xFF, 0xFF, 0xFF, 0xFF,
		0x44, 0x33, 0x22, 0x11,
		0x01, 0x02, 0x03, 0x04,
		0x10, 0x20, 0x30, 0x40,
		0xFF, 0x00, 0x00, 0xFF,
		0x00, 0x7F, 0xFF, 0x80,
	}

	// goldenSwizzleQuads is goldenQuads with each quad's first and third byte exchanged.
	goldenSwizzleQuads = []byte{
		0x00, 0x00, 0x00, 0x00,
		0xFF, 0xFF, 0xFF, 0xFF,
		0x22, 0x33, 0x44, 0x11,
		0x03, 0x02, 0x01, 0x04,
		0x30, 0x20, 0x10, 0x40,
		0x00, 0x00, 0xFF, 0xFF,
		0xFF, 0x7F, 0x00, 0x80,
	}

	// premulQuads are straight-alpha r,g,b,a quads chosen to sweep alpha: 0 (everything vanishes), 255 (nothing
	// changes but the order), 1 and 254 (the ends of the interior), and mid values whose products land either side of a
	// multiple of 255 so the truncating divide is actually exercised.
	premulQuads = []byte{
		10, 20, 30, 0,
		10, 20, 30, 255,
		255, 255, 255, 128,
		1, 128, 254, 128,
		200, 100, 50, 77,
		255, 0, 255, 1,
		3, 3, 3, 254,
	}

	// goldenPremulBGRA is premulQuads as premultiplied b,g,r,a. The entries that pin the truncation: 254*128/255 is
	// 127.49, which floors to 127; 1*128/255 is 0.50, which floors to 0; 200*77/255 is 60.39, which floors to 60; and
	// 3*254/255 is 2.99, which floors to 2 where rounding would have given 3.
	goldenPremulBGRA = []byte{
		0, 0, 0, 0,
		30, 20, 10, 255,
		128, 128, 128, 128,
		127, 64, 0, 128,
		15, 30, 60, 77,
		1, 0, 1, 1,
		2, 2, 2, 254,
	}

	// goldenPremulARGB is premulQuads as premultiplied a,r,g,b — goldenPremulBGRA's channels in the opposite order.
	goldenPremulARGB = []byte{
		0, 0, 0, 0,
		255, 10, 20, 30,
		128, 128, 128, 128,
		128, 0, 64, 127,
		77, 60, 30, 15,
		1, 1, 0, 1,
		254, 2, 2, 2,
	}
)

// padByte is written into the slack beyond a destination's converted run, so a kernel that walked past its end or
// resliced dst to its own full length is caught rather than merely producing the right prefix.
const padByte = 0xA5

// checkWords fails the test at the first differing word.
func checkWords(t *testing.T, what string, got, want []uint32) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: length %d, want %d", what, len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s: word %d = %08x, want %08x", what, i, got[i], want[i])
		}
	}
}

// checkBytes fails the test at the first differing byte, reporting the pixel and channel it belongs to.
func checkBytes(t *testing.T, what string, got, want []byte) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: length %d, want %d", what, len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s: byte %d (pixel %d channel %d) = %02x, want %02x", what, i, i/4, i%4, got[i], want[i])
		}
	}
}

// paddedWords returns a destination of n convertible words followed by pad words of a recognizable filler.
func paddedWords(n, pad int) []uint32 {
	dst := make([]uint32, n+pad)
	for i := n; i < len(dst); i++ {
		dst[i] = padByte
	}
	return dst
}

// paddedBytes returns a destination of n convertible bytes followed by pad bytes of a recognizable filler.
func paddedBytes(n, pad int) []byte {
	dst := make([]byte, n+pad)
	for i := n; i < len(dst); i++ {
		dst[i] = padByte
	}
	return dst
}

// checkWordPad fails if a kernel wrote into the slack past the run it was given.
func checkWordPad(t *testing.T, what string, dst []uint32, n int) {
	t.Helper()
	for i := n; i < len(dst); i++ {
		if dst[i] != padByte {
			t.Fatalf("%s: wrote %08x into padding word %d", what, dst[i], i)
		}
	}
}

// checkBytePad fails if a kernel wrote into the slack past the run it was given.
func checkBytePad(t *testing.T, what string, dst []byte, n int) {
	t.Helper()
	for i := n; i < len(dst); i++ {
		if dst[i] != padByte {
			t.Fatalf("%s: wrote %02x into padding byte %d", what, dst[i], i)
		}
	}
}

// TestSwizzleRBGolden checks SwizzleRB against hand-computed words at every prefix length, on a padded destination, and
// with dst aliased exactly onto src.
func TestSwizzleRBGolden(t *testing.T) {
	for n := range len(goldenWords) + 1 {
		dst := paddedWords(n, 3)
		SwizzleRB(dst, goldenWords[:n])
		checkWords(t, "SwizzleRB", dst[:n], goldenSwizzleRB[:n])
		checkWordPad(t, "SwizzleRB", dst, n)
	}
	inPlace := append([]uint32(nil), goldenWords...)
	SwizzleRB(inPlace, inPlace)
	checkWords(t, "SwizzleRB in place", inPlace, goldenSwizzleRB)
	// The swap is an involution, so a second pass must restore the original words exactly.
	SwizzleRB(inPlace, inPlace)
	checkWords(t, "SwizzleRB in place twice", inPlace, goldenWords)
}

// TestSwizzleRBBytesGolden checks SwizzleRBBytes against hand-computed quads at every prefix length, on a padded
// destination, and with dst aliased exactly onto src.
func TestSwizzleRBBytesGolden(t *testing.T) {
	for n := 0; n <= len(goldenQuads); n += 4 {
		dst := paddedBytes(n, 7)
		SwizzleRBBytes(dst, goldenQuads[:n])
		checkBytes(t, "SwizzleRBBytes", dst[:n], goldenSwizzleQuads[:n])
		checkBytePad(t, "SwizzleRBBytes", dst, n)
	}
	inPlace := append([]byte(nil), goldenQuads...)
	SwizzleRBBytes(inPlace, inPlace)
	checkBytes(t, "SwizzleRBBytes in place", inPlace, goldenSwizzleQuads)
	SwizzleRBBytes(inPlace, inPlace)
	checkBytes(t, "SwizzleRBBytes in place twice", inPlace, goldenQuads)
}

// TestRGBAToBGRABytesGolden checks RGBAToBGRABytes against the hand-computed LSBFirst wire bytes at every prefix
// length and on a padded destination.
func TestRGBAToBGRABytesGolden(t *testing.T) {
	for n := range len(goldenWords) + 1 {
		dst := paddedBytes(n*4, 7)
		RGBAToBGRABytes(dst, goldenWords[:n])
		checkBytes(t, "RGBAToBGRABytes", dst[:n*4], goldenToBGRA[:n*4])
		checkBytePad(t, "RGBAToBGRABytes", dst, n*4)
	}
}

// TestRGBAToARGBBytesGolden checks RGBAToARGBBytes against the hand-computed MSBFirst wire bytes at every prefix
// length and on a padded destination.
func TestRGBAToARGBBytesGolden(t *testing.T) {
	for n := range len(goldenWords) + 1 {
		dst := paddedBytes(n*4, 7)
		RGBAToARGBBytes(dst, goldenWords[:n])
		checkBytes(t, "RGBAToARGBBytes", dst[:n*4], goldenToARGB[:n*4])
		checkBytePad(t, "RGBAToARGBBytes", dst, n*4)
	}
}

// TestPremulBGRAGolden checks PremulBGRA against hand-computed premultiplied quads at every prefix length, on a padded
// destination, and with dst aliased exactly onto src.
func TestPremulBGRAGolden(t *testing.T) {
	for n := 0; n <= len(premulQuads); n += 4 {
		dst := paddedBytes(n, 7)
		PremulBGRA(dst, premulQuads[:n])
		checkBytes(t, "PremulBGRA", dst[:n], goldenPremulBGRA[:n])
		checkBytePad(t, "PremulBGRA", dst, n)
	}
	inPlace := append([]byte(nil), premulQuads...)
	PremulBGRA(inPlace, inPlace)
	checkBytes(t, "PremulBGRA in place", inPlace, goldenPremulBGRA)
}

// TestPremulARGBGolden checks PremulARGB against hand-computed premultiplied quads at every prefix length, on a padded
// destination, and with dst aliased exactly onto src.
func TestPremulARGBGolden(t *testing.T) {
	for n := 0; n <= len(premulQuads); n += 4 {
		dst := paddedBytes(n, 7)
		PremulARGB(dst, premulQuads[:n])
		checkBytes(t, "PremulARGB", dst[:n], goldenPremulARGB[:n])
		checkBytePad(t, "PremulARGB", dst, n)
	}
	inPlace := append([]byte(nil), premulQuads...)
	PremulARGB(inPlace, inPlace)
	checkBytes(t, "PremulARGB in place", inPlace, goldenPremulARGB)
}

// refDiv255 is floor(c*a/255) found by search rather than by dividing, so the reference below shares no arithmetic with
// the kernels it checks — neither the portable forms' truncating divide nor the vector forms' shift identity.
func refDiv255(c, a byte) byte {
	t := int(c) * int(a)
	q := 0
	for 255*(q+1) <= t {
		q++
	}
	return byte(q)
}

// refBytes decomposes a device word into its four channel bytes, the step every reference below starts from.
func refBytes(v uint32) (r, g, b, a byte) {
	return byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)
}

// refSwizzleRB is an independent SwizzleRB: take the word apart into channels and put it back together with red and
// blue exchanged, rather than masking and shifting the word as the kernel does.
func refSwizzleRB(src []uint32) []uint32 {
	out := make([]uint32, len(src))
	for i, v := range src {
		r, g, b, a := refBytes(v)
		out[i] = uint32(b) | uint32(g)<<8 | uint32(r)<<16 | uint32(a)<<24
	}
	return out
}

// refSwizzleRBBytes is an independent SwizzleRBBytes.
func refSwizzleRBBytes(src []byte) []byte {
	out := make([]byte, len(src))
	for i := 0; i < len(src); i += 4 {
		s, d := (*[4]byte)(src[i:]), (*[4]byte)(out[i:])
		d[0], d[1], d[2], d[3] = s[2], s[1], s[0], s[3]
	}
	return out
}

// refToBGRA is an independent RGBAToBGRABytes, built from the channel decomposition rather than from shifts.
func refToBGRA(src []uint32) []byte {
	out := make([]byte, len(src)*4)
	for i, v := range src {
		r, g, b, a := refBytes(v)
		out[i*4], out[i*4+1], out[i*4+2], out[i*4+3] = b, g, r, a
	}
	return out
}

// refToARGB is an independent RGBAToARGBBytes, built from the channel decomposition rather than from shifts.
func refToARGB(src []uint32) []byte {
	out := make([]byte, len(src)*4)
	for i, v := range src {
		r, g, b, a := refBytes(v)
		out[i*4], out[i*4+1], out[i*4+2], out[i*4+3] = a, r, g, b
	}
	return out
}

// refPremulBGRA is an independent PremulBGRA, scaling through refDiv255.
func refPremulBGRA(src []byte) []byte {
	out := make([]byte, len(src))
	for i := 0; i < len(src); i += 4 {
		s, d := (*[4]byte)(src[i:]), (*[4]byte)(out[i:])
		r, g, b, a := s[0], s[1], s[2], s[3]
		d[0], d[1], d[2], d[3] = refDiv255(b, a), refDiv255(g, a), refDiv255(r, a), a
	}
	return out
}

// refPremulARGB is an independent PremulARGB, scaling through refDiv255.
func refPremulARGB(src []byte) []byte {
	out := make([]byte, len(src))
	for i := 0; i < len(src); i += 4 {
		s, d := (*[4]byte)(src[i:]), (*[4]byte)(out[i:])
		r, g, b, a := s[0], s[1], s[2], s[3]
		d[0], d[1], d[2], d[3] = a, refDiv255(r, a), refDiv255(g, a), refDiv255(b, a)
	}
	return out
}

// TestPixconvMatchesReference drives every exported conversion over randomized rows and compares against the
// independent references above. Row lengths run 0..71 pixels so every chunk/tail split of the four-pixel vector body
// is covered, including the lengths that are not multiples of it, and the alphas are drawn to keep 0 and 255 common
// rather than 1-in-256 rare.
func TestPixconvMatchesReference(t *testing.T) {
	rng := rand.New(rand.NewPCG(0x7069_7863, 0x6F6E_7600)) //nolint:gosec // Deterministic seed for reproducible tests
	randWords := func(n int) []uint32 {
		w := make([]uint32, n)
		for i := range w {
			w[i] = rng.Uint32()
		}
		return w
	}
	randQuads := func(n int) []byte {
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
	for range 4096 {
		n := rng.IntN(72)
		words := randWords(n)
		quads := randQuads(n)

		gotWords := make([]uint32, n)
		SwizzleRB(gotWords, words)
		checkWords(t, "SwizzleRB", gotWords, refSwizzleRB(words))

		gotQuads := make([]byte, n*4)
		SwizzleRBBytes(gotQuads, quads)
		checkBytes(t, "SwizzleRBBytes", gotQuads, refSwizzleRBBytes(quads))

		gotBytes := make([]byte, n*4)
		RGBAToBGRABytes(gotBytes, words)
		checkBytes(t, "RGBAToBGRABytes", gotBytes, refToBGRA(words))

		RGBAToARGBBytes(gotBytes, words)
		checkBytes(t, "RGBAToARGBBytes", gotBytes, refToARGB(words))

		PremulBGRA(gotQuads, quads)
		checkBytes(t, "PremulBGRA", gotQuads, refPremulBGRA(quads))

		PremulARGB(gotQuads, quads)
		checkBytes(t, "PremulARGB", gotQuads, refPremulARGB(quads))

		// The same rows again with dst aliased exactly onto src, the in-place form the DIB and cursor paths use.
		inPlaceWords := append([]uint32(nil), words...)
		SwizzleRB(inPlaceWords, inPlaceWords)
		checkWords(t, "SwizzleRB aliased", inPlaceWords, refSwizzleRB(words))

		inPlaceQuads := append([]byte(nil), quads...)
		SwizzleRBBytes(inPlaceQuads, inPlaceQuads)
		checkBytes(t, "SwizzleRBBytes aliased", inPlaceQuads, refSwizzleRBBytes(quads))

		copy(inPlaceQuads, quads)
		PremulBGRA(inPlaceQuads, inPlaceQuads)
		checkBytes(t, "PremulBGRA aliased", inPlaceQuads, refPremulBGRA(quads))

		copy(inPlaceQuads, quads)
		PremulARGB(inPlaceQuads, inPlaceQuads)
		checkBytes(t, "PremulARGB aliased", inPlaceQuads, refPremulARGB(quads))
	}
}

// TestPremulExhaustive walks the complete (channel value, alpha) domain through both premultiply conversions on every
// build, in each of the three color byte positions: the domain is only 65536 pairs per position, so the portable form
// can be proved right outright rather than sampled. The row carries the sweeping channel in one position and fixed but
// distinct values in the other two, so each lane of a vector chunk carries a different product at every step.
func TestPremulExhaustive(t *testing.T) {
	const others = 0x5B
	src := make([]byte, 256*4)
	bgra := make([]byte, 256*4)
	argb := make([]byte, 256*4)
	for pos := range 3 {
		for a := range 256 {
			for i := range 256 {
				src[i*4] = others
				src[i*4+1] = others ^ 0x3C
				src[i*4+2] = others ^ 0x7E
				src[i*4+pos] = byte(i)
				src[i*4+3] = byte(a)
			}
			PremulBGRA(bgra, src)
			checkBytes(t, "PremulBGRA exhaustive", bgra, refPremulBGRA(src))
			PremulARGB(argb, src)
			checkBytes(t, "PremulARGB exhaustive", argb, refPremulARGB(src))
		}
	}
}
