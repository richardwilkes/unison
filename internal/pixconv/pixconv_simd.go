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

import "simd/archsimd"

// The goexperiment.simd pixel converters, written against archsimd's 128-bit types for the same reason the canvas
// raster kernels are: it is the widest shape arm64 and amd64 share, so both vector lanes compute the same expressions
// in the same order, and 128 bits is exactly four pixels, which is the unit every conversion here works in. Each kernel
// takes four pixels per iteration and hands the sub-quad remainder to its Generic twin rather than duplicating the
// arithmetic; the equivalence suite (TestPixconvSIMDMatchesScalar) drives every chunk/tail split.
//
// Three pieces of machinery recur below:
//
//   - The little-endian reshape. ReshapeToUint32s and ReshapeToUint16s reinterpret a register's bytes without moving
//     any, so on the two little-endian architectures archsimd supports, byte lane 4i+j of a 16-byte load is bit 8j of
//     word i. That is what lets the byte-quad kernels do their work in 32-bit lanes and store the result straight back
//     through the byte view, and it is why the r,g,b,a -> b,g,r,a byte swap and the word-level red/blue exchange
//     w32.RepackRGBAToBGRA performs are the same expression here (swapRB32). The Generic twins deliberately are not
//     written this way — they name byte positions, not memory positions, so they survive a big-endian port; these do
//     not have to, since they only ever build for arm64 and amd64.
//   - The 8888 byte split, the vector spelling of the portable per-channel form. Viewing a Uint32x4 of four pixels as a
//     Uint16x8 puts pixel i's bytes 0 and 1 in lane 2i and its bytes 2 and 3 in lane 2i+1. Masking with 0x00FF then
//     leaves channels 0 and 2 (one per lane, 8 bits of headroom each) and shifting right by 8 leaves channels 1 and 3,
//     so a pair of vectors carries all four channels of four pixels with every byte in its own 16-bit lane.
//     Recombining the halves with lo | hi<<8 is exact whenever both are bytes, which the bound below guarantees. The
//     pixel's alpha byte is replicated into both of its 16-bit lanes by shifting the word down 24 and multiplying by
//     0x00010001, which cannot overflow a 32-bit lane for a byte and is the cheapest way to reach both halves at once.
//   - The floor divide by 255. The straight-alpha loops these kernels replace compute c*a/255 with Go's truncating
//     integer division, which has no vector counterpart, so the lanes evaluate the identity q = (t + (t>>8) + 1) >> 8
//     for t = c*a instead (div255v). It is exact for every t a byte product can be. Write t = 255q + r with
//     0 <= r <= 254; since t <= 255*255 = 65025, q <= 255. Lower bound: t>>8 >= floor(255q/256) = q-1 for q >= 1 (and 0
//     for q = 0), so t + (t>>8) + 1 >= 255q + (q-1) + 1 = 256q. Upper bound: t>>8 <= floor((255q+254)/256) <= q for
//     every q <= 255, so t + (t>>8) + 1 <= 255q + 254 + q + 1 = 256q + 255 < 256(q+1). Both together put the shift on
//     exactly q. The largest intermediate the identity forms is 65025 + 254 + 1 = 65280, which still fits a 16-bit
//     lane — that bound is what lets the entire divide run inside the byte split, and it is also why every product
//     here (at most 65025) and every quotient (at most 255) stays in its own lane and never carries into a neighboring
//     channel.
//
// The lane shifts go through the shift16/shift32 helpers with counts hoisted out of the loop (rightShift16 and
// friends), which is a codegen accommodation only — see pixconv_simd_arm64.go. The values they compute are the ones
// the straight ShiftAllLeft/ShiftAllRight calls compute.

// init swaps the dispatch variables to the simd kernels the per-arch preference constants elect. The gate is the
// feature check only: these are all-integer kernels with no floating point anywhere, so the floor they need is well
// below the one a shader pipeline would ask for (see simdPixconvSupported).
func init() {
	if !simdPixconvSupported() {
		return
	}
	if preferSIMDSwizzleRB {
		swizzleRBFn = swizzleRBSIMD
	}
	if preferSIMDSwizzleRBBytes {
		swizzleRBBytesFn = swizzleRBBytesSIMD
	}
	if preferSIMDRGBAToBGRABytes {
		rgbaToBGRABytesFn = rgbaToBGRABytesSIMD
	}
	if preferSIMDRGBAToARGBBytes {
		rgbaToARGBBytesFn = rgbaToARGBBytesSIMD
	}
	if preferSIMDPremulBGRA {
		premulBGRAFn = premulBGRASIMD
	}
	if preferSIMDPremulARGB {
		premulARGBFn = premulARGBSIMD
	}
}

// swapRB32 is the lanewise red/blue exchange, (v & 0xFF00FF00) | ((v & 0xFF) << 16) | ((v >> 16) & 0xFF) — the word
// expression w32.RepackRGBAToBGRA uses, which on a little-endian host is also the b,g,r,a byte reorder. keepMask must
// be 0xFF00FF00 and byteMask 0x000000FF; both are broadcast by the caller so they are hoisted out of its loop.
func swapRB32(v, keepMask, byteMask archsimd.Uint32x4, l16, r16 shiftCount32) archsimd.Uint32x4 {
	return v.And(keepMask).Or(shift32(v.And(byteMask), l16)).Or(shift32(v, r16).And(byteMask))
}

// rotl8v rotates each 32-bit lane left by 8, (v>>24) | (v<<8). Applied to a premultiplied R|G<<8|B<<16|A<<24 word on a
// little-endian host it yields the a,r,g,b byte sequence an MSBFirst X server's ZPixmap data is in.
func rotl8v(v archsimd.Uint32x4, l8w, r24w shiftCount32) archsimd.Uint32x4 {
	return shift32(v, l8w).Or(shift32(v, r24w))
}

// div255v is the lanewise floor divide by 255: (t + (t>>8) + 1) >> 8, exact for every t <= 65025 and never leaving its
// 16-bit lane, both proved in the file header. one must be a broadcast 1.
func div255v(t, one archsimd.Uint16x8, r8 shiftCount16) archsimd.Uint16x8 {
	return shift16(t.Add(shift16(t, r8)).Add(one), r8)
}

// swizzleRBSIMD swaps the red and blue bytes of a run of 8888 words. Four words per iteration, the sub-quad tail on the
// portable form. Purely a permutation of bytes, so there is no arithmetic bound to argue.
func swizzleRBSIMD(dst, src []uint32) {
	n := len(src)
	keepMask := archsimd.BroadcastUint32x4(0xFF00FF00)
	byteMask := archsimd.BroadcastUint32x4(0x000000FF)
	l16, r16 := leftShift32(16), rightShift32(16)
	i := 0
	for ; i+4 <= n; i += 4 {
		swapRB32(archsimd.LoadUint32x4(src[i:]), keepMask, byteMask, l16, r16).Store(dst[i:])
	}
	if i < n {
		swizzleRBGeneric(dst[i:], src[i:n])
	}
}

// swizzleRBBytesSIMD swaps the red and blue bytes of a run of 4-byte pixel quads. Sixteen bytes — four pixels — per
// iteration, the sub-chunk tail on the portable form. The quads are loaded as bytes and viewed as words purely so the
// swap can be one word expression instead of four byte moves; see the file header on why that view is exact here.
func swizzleRBBytesSIMD(dst, src []byte) {
	n := len(src)
	keepMask := archsimd.BroadcastUint32x4(0xFF00FF00)
	byteMask := archsimd.BroadcastUint32x4(0x000000FF)
	l16, r16 := leftShift32(16), rightShift32(16)
	i := 0
	for ; i+16 <= n; i += 16 {
		v := archsimd.LoadUint8x16(src[i:]).ReshapeToUint32s()
		swapRB32(v, keepMask, byteMask, l16, r16).ReshapeToUint8s().Store(dst[i:])
	}
	if i < n {
		swizzleRBBytesGeneric(dst[i:], src[i:n])
	}
}

// rgbaToBGRABytesSIMD unpacks premultiplied 8888 words into the b,g,r,a wire bytes an LSBFirst X server wants. Four
// words — sixteen destination bytes — per iteration, the sub-quad tail on the portable form. On a little-endian host
// that byte sequence is the red/blue-swapped word, so the unpack is swapRB32 plus a store through the byte view; the
// portable twin, which cannot make that assumption, spells the four bytes out.
func rgbaToBGRABytesSIMD(dst []byte, src []uint32) {
	n := len(src)
	keepMask := archsimd.BroadcastUint32x4(0xFF00FF00)
	byteMask := archsimd.BroadcastUint32x4(0x000000FF)
	l16, r16 := leftShift32(16), rightShift32(16)
	i := 0
	for ; i+4 <= n; i += 4 {
		v := archsimd.LoadUint32x4(src[i:])
		swapRB32(v, keepMask, byteMask, l16, r16).ReshapeToUint8s().Store(dst[i*4:])
	}
	if i < n {
		rgbaToBGRABytesGeneric(dst[i*4:], src[i:n])
	}
}

// rgbaToARGBBytesSIMD unpacks premultiplied 8888 words into the a,r,g,b wire bytes an MSBFirst X server wants. Four
// words per iteration, the sub-quad tail on the portable form. Moving alpha from the top byte to the bottom and each
// color byte up one is a rotate of the whole word, so this is rotl8v plus a store through the byte view.
func rgbaToARGBBytesSIMD(dst []byte, src []uint32) {
	n := len(src)
	l8w, r24w := leftShift32(8), rightShift32(24)
	i := 0
	for ; i+4 <= n; i += 4 {
		v := archsimd.LoadUint32x4(src[i:])
		rotl8v(v, l8w, r24w).ReshapeToUint8s().Store(dst[i*4:])
	}
	if i < n {
		rgbaToARGBBytesGeneric(dst[i*4:], src[i:n])
	}
}

// premulBGRASIMD premultiplies straight-alpha r,g,b,a quads into b,g,r,a quads. Sixteen bytes — four pixels — per
// iteration, the sub-chunk tail on the portable form.
//
// The body is the 8888 byte split with div255v as the divide, both described in the file header: lo carries channels 0
// and 2 of the four pixels and hi channels 1 and 3, each byte scaled by its pixel's alpha, which is replicated into
// both of that pixel's 16-bit lanes. The alpha lane's own product is computed and then discarded — the
// rgbMask/alphaMask pair reinstates the original alpha byte — which costs less than branching around it and keeps the
// body straight-line, and the portable form has no a == 255 shortcut for this one to have to reproduce. swapRB32 then
// puts the recombined word's bytes in wire order, which it can do after the fact because the swap never touches the
// green or alpha byte the premultiply just settled.
func premulBGRASIMD(dst, src []byte) {
	n := len(src)
	byteMask := archsimd.BroadcastUint16x8(0x00FF)
	one := archsimd.BroadcastUint16x8(1)
	spread := archsimd.BroadcastUint32x4(0x00010001)
	rgbMask := archsimd.BroadcastUint32x4(0x00FFFFFF)
	alphaMask := archsimd.BroadcastUint32x4(0xFF000000)
	keepMask := archsimd.BroadcastUint32x4(0xFF00FF00)
	loByte := archsimd.BroadcastUint32x4(0x000000FF)
	r8, l8 := rightShift16(8), leftShift16(8)
	l16, r16, r24w := leftShift32(16), rightShift32(16), rightShift32(24)
	i := 0
	for ; i+16 <= n; i += 16 {
		s32 := archsimd.LoadUint8x16(src[i:]).ReshapeToUint32s()
		s16 := s32.ReshapeToUint16s()
		a := shift32(s32, r24w).Mul(spread).ReshapeToUint16s()
		lo := div255v(s16.And(byteMask).Mul(a), one, r8)
		hi := div255v(shift16(s16, r8).Mul(a), one, r8)
		p := lo.Or(shift16(hi, l8)).ReshapeToUint32s().And(rgbMask).Or(s32.And(alphaMask))
		swapRB32(p, keepMask, loByte, l16, r16).ReshapeToUint8s().Store(dst[i:])
	}
	if i < n {
		premulBGRAGeneric(dst[i:], src[i:n])
	}
}

// premulARGBSIMD premultiplies straight-alpha r,g,b,a quads into a,r,g,b quads. Sixteen bytes — four pixels — per
// iteration, the sub-chunk tail on the portable form. It is premulBGRASIMD's body with rotl8v as the reorder, which
// must run after the premultiply rather than before it, since the rotate is what moves the settled alpha byte down
// into byte 0.
func premulARGBSIMD(dst, src []byte) {
	n := len(src)
	byteMask := archsimd.BroadcastUint16x8(0x00FF)
	one := archsimd.BroadcastUint16x8(1)
	spread := archsimd.BroadcastUint32x4(0x00010001)
	rgbMask := archsimd.BroadcastUint32x4(0x00FFFFFF)
	alphaMask := archsimd.BroadcastUint32x4(0xFF000000)
	r8, l8 := rightShift16(8), leftShift16(8)
	l8w, r24w := leftShift32(8), rightShift32(24)
	i := 0
	for ; i+16 <= n; i += 16 {
		s32 := archsimd.LoadUint8x16(src[i:]).ReshapeToUint32s()
		s16 := s32.ReshapeToUint16s()
		a := shift32(s32, r24w).Mul(spread).ReshapeToUint16s()
		lo := div255v(s16.And(byteMask).Mul(a), one, r8)
		hi := div255v(shift16(s16, r8).Mul(a), one, r8)
		p := lo.Or(shift16(hi, l8)).ReshapeToUint32s().And(rgbMask).Or(s32.And(alphaMask))
		rotl8v(p, l8w, r24w).ReshapeToUint8s().Store(dst[i:])
	}
	if i < n {
		premulARGBGeneric(dst[i:], src[i:n])
	}
}
