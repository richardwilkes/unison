// Copyright (c) 2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

// Package pixconv holds the pixel-format conversions the platform presentation paths run over whole frames: the
// red/blue channel swaps, the device-word-to-wire-byte unpacks and the straight-to-premultiplied conversions that sit
// between the renderer's R|G<<8|B<<16|A<<24 words and what GDI, the X11 ZPixmap wire format and the shell's
// drag/cursor bitmaps require.
//
// Every conversion dispatches through a package-level function variable, the same shape the canvas raster kernels use:
// the portable form lives here under a Generic name with no build tag, and a goexperiment.simd build's init
// (pixconv_simd.go) repoints the variable at an archsimd kernel on qualifying hardware. Every substitute is
// bit-identical to the portable form — locked by TestPixconvSIMDMatchesScalar, whose premultiply subtests enumerate the
// complete (channel value, alpha) domains rather than sampling them — so the choice of lane changes throughput only,
// never the bytes handed to the window system. The Generic forms are also the sub-chunk tail of every vector kernel,
// which calls them rather than duplicating their arithmetic.
//
// The portable kernels are written with explicit per-byte reads and shifts rather than through encoding/binary or an
// unsafe reinterpretation, so they compute the same bytes on a big-endian target as on a little-endian one; only the
// vector twins, which exist solely for arm64 and amd64, lean on little-endian lane layout.
//
// Two invariants hold across the whole package. None of these kernels allocates, reslices or replaces dst, because dst
// routinely addresses memory this process does not own — a DIB section's pixel store, an X11 request buffer — where a
// replacement would go unnoticed and leave the window showing a stale frame. And a destination shorter than the
// conversion needs is a caller bug: it panics on the bounds check rather than being silently clamped to the shorter
// run, which would ship a torn frame instead of a stack trace.
package pixconv

// The dispatch signatures. Four shapes cover the six conversions, which is why several kernels share a type.
type (
	// swizzleWordsFn exchanges the red and blue bytes of a run of 8888 words.
	swizzleWordsFn func(dst, src []uint32)
	// swizzleBytesFn is swizzleWordsFn over 4-byte pixel quads instead of words.
	swizzleBytesFn func(dst, src []byte)
	// unpackWordsFn writes each 8888 word out as the four bytes a wire format orders them in.
	unpackWordsFn func(dst []byte, src []uint32)
	// premulBytesFn premultiplies a run of straight-alpha RGBA byte quads, reordering the channels as it goes.
	premulBytesFn func(dst, src []byte)
)

// The dispatch variables. Each defaults to the portable form declared below; pixconv_simd.go's init is the only other
// assignment. Declaring the defaults here, untagged, keeps the portable forms referenced on every build — they are the
// reference twin the equivalence suite fuzzes against and the dispatch default everywhere no vector lane applies.
var (
	swizzleRBFn       swizzleWordsFn = swizzleRBGeneric
	swizzleRBBytesFn  swizzleBytesFn = swizzleRBBytesGeneric
	rgbaToBGRABytesFn unpackWordsFn  = rgbaToBGRABytesGeneric
	rgbaToARGBBytesFn unpackWordsFn  = rgbaToARGBBytesGeneric
	premulBGRAFn      premulBytesFn  = premulBGRAGeneric
	premulARGBFn      premulBytesFn  = premulARGBGeneric
)

// SwizzleRB writes each of src's 8888 words to dst with its red and blue bytes exchanged and its green and alpha bytes
// left alone, the RGBA<->BGRA conversion GDI's presentation bitmaps need in both directions (the swap is its own
// inverse). Premultiplication state is irrelevant to it: it only moves bytes.
//
// len(dst) must be at least len(src); a shorter dst panics on the bounds check rather than converting a prefix. dst may
// be exactly src, which converts in place; any other overlap between the two is undefined, since the kernels make no
// attempt to order their loads and stores for it. dst is never reallocated or resliced, so it may address a pixel store
// this process does not own.
func SwizzleRB(dst, src []uint32) { swizzleRBFn(dst, src) }

// SwizzleRBBytes is SwizzleRB over 4-byte pixel quads: each r,g,b,a quad of src becomes a b,g,r,a quad of dst. It is
// the form the cursor and icon bitmap builders want, where the pixels arrive as an image.NRGBA byte slice rather than
// as device words.
//
// len(src) must be a multiple of 4 and len(dst) at least len(src); violating either panics on the bounds check. The
// aliasing and no-realloc rules are SwizzleRB's.
func SwizzleRBBytes(dst, src []byte) { swizzleRBBytesFn(dst, src) }

// RGBAToBGRABytes unpacks premultiplied 8888 words (R|G<<8|B<<16|A<<24) into the b,g,r,a byte sequence an LSBFirst X
// server's 32bpp ZPixmap data is in. ZPixmap pixel data is not byte-swapped by the server, so the byte order is the
// server's advertised image-byte-order and has to be produced here.
//
// len(dst) must be at least 4*len(src); a shorter dst panics on the bounds check. dst and src must not overlap. dst is
// never reallocated or resliced.
func RGBAToBGRABytes(dst []byte, src []uint32) { rgbaToBGRABytesFn(dst, src) }

// RGBAToARGBBytes is RGBAToBGRABytes for an MSBFirst server: the same premultiplied words emitted as a,r,g,b byte
// sequences. The preconditions are RGBAToBGRABytes's.
func RGBAToARGBBytes(dst []byte, src []uint32) { rgbaToARGBBytesFn(dst, src) }

// PremulBGRA converts straight-alpha r,g,b,a byte quads into premultiplied b,g,r,a byte quads — the combined
// premultiply-and-reorder the X11 icon/image upload paths and the Windows layered drag bitmap all perform. Each color
// channel c becomes c*a/255 with Go's truncating integer division, which is what the loops it replaces compute; alpha
// passes through untouched.
//
// len(src) must be a multiple of 4 and len(dst) at least len(src); violating either panics on the bounds check. dst may
// be exactly src, which converts in place, since every quad is read before it is written; any other overlap is
// undefined. dst is never reallocated or resliced.
func PremulBGRA(dst, src []byte) { premulBGRAFn(dst, src) }

// PremulARGB is PremulBGRA emitting a,r,g,b quads instead, the order an MSBFirst X server's ZPixmap data wants. The
// arithmetic, including the truncating divide, and the preconditions are PremulBGRA's.
func PremulARGB(dst, src []byte) { premulARGBFn(dst, src) }

// swizzleRBGeneric is the portable SwizzleRB. The word expression is the one w32.RepackRGBAToBGRA uses, and it is
// endian-agnostic: it names byte positions within the uint32 value, not within memory, so it produces the same words
// wherever the loads and stores put them.
func swizzleRBGeneric(dst, src []uint32) {
	for i := range src {
		v := src[i]
		dst[i] = (v & 0xFF00FF00) | ((v & 0xFF) << 16) | ((v >> 16) & 0xFF)
	}
}

// swizzleRBBytesGeneric is the portable SwizzleRBBytes. The quad is read into locals before any of it is written, which
// is what makes dst == src safe; addressing the bytes individually also keeps the kernel independent of the host's
// endianness, since no word is ever formed. Each quad is taken as a *[4]byte window rather than by four separate
// indices: the conversion carries the whole bounds check, so the trailing bytes of a src whose length is not a
// multiple of 4 still panic, but the compiler only has to prove it once per pixel instead of eight times.
func swizzleRBBytesGeneric(dst, src []byte) {
	for i := 0; i < len(src); i += 4 {
		s, d := (*[4]byte)(src[i:]), (*[4]byte)(dst[i:])
		r, g, b, a := s[0], s[1], s[2], s[3]
		d[0], d[1], d[2], d[3] = b, g, r, a
	}
}

// rgbaToBGRABytesGeneric is the portable RGBAToBGRABytes. Each byte is extracted from the word by shift rather than by
// reinterpreting the word's storage, so the wire order is the same on either endianness — which matters because the
// order is dictated by the X server, not by this host.
func rgbaToBGRABytesGeneric(dst []byte, src []uint32) {
	di := 0
	for i := range src {
		v := src[i]
		d := (*[4]byte)(dst[di:])
		d[0], d[1], d[2], d[3] = byte(v>>16), byte(v>>8), byte(v), byte(v>>24)
		di += 4
	}
}

// rgbaToARGBBytesGeneric is the portable RGBAToARGBBytes, extracting its bytes by shift for the reason
// rgbaToBGRABytesGeneric does.
func rgbaToARGBBytesGeneric(dst []byte, src []uint32) {
	di := 0
	for i := range src {
		v := src[i]
		d := (*[4]byte)(dst[di:])
		d[0], d[1], d[2], d[3] = byte(v>>24), byte(v), byte(v>>8), byte(v>>16)
		di += 4
	}
}

// premulBGRAGeneric is the portable PremulBGRA. c*a/255 is computed exactly as the loops it replaces do — a truncating
// divide of a product that cannot exceed 255*255 = 65025 — and the whole quad is read before any of it is written, so
// dst == src converts in place. An a == 255 shortcut would be dead weight: c*255/255 == c already. The quads are
// windowed as *[4]byte for the reason swizzleRBBytesGeneric's are.
func premulBGRAGeneric(dst, src []byte) {
	for i := 0; i < len(src); i += 4 {
		s, d := (*[4]byte)(src[i:]), (*[4]byte)(dst[i:])
		r, g, b, a := uint32(s[0]), uint32(s[1]), uint32(s[2]), uint32(s[3])
		d[0], d[1], d[2], d[3] = byte(b*a/255), byte(g*a/255), byte(r*a/255), byte(a)
	}
}

// premulARGBGeneric is the portable PremulARGB, premulBGRAGeneric's arithmetic written out in the opposite channel
// order.
func premulARGBGeneric(dst, src []byte) {
	for i := 0; i < len(src); i += 4 {
		s, d := (*[4]byte)(src[i:]), (*[4]byte)(dst[i:])
		r, g, b, a := uint32(s[0]), uint32(s[1]), uint32(s[2]), uint32(s[3])
		d[0], d[1], d[2], d[3] = byte(a), byte(r*a/255), byte(g*a/255), byte(b*a/255)
	}
}
