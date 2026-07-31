// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package w32

// This file deliberately has no _windows suffix so that the pixel repacking logic can be tested on any platform.

// RepackRGBAToBGRA repacks premultiplied RGBA pixel words as the BGRA words GDI expects, dropping any stride padding
// in the process (rowPixels is the source row stride, in pixels). The result is written into buf, which is grown if
// its capacity is insufficient, and returned, so that callers can reuse a single scratch buffer across frames instead
// of allocating a full-frame buffer for every present.
func RepackRGBAToBGRA(pix []uint32, width, height, rowPixels int, buf []uint32) []uint32 {
	size := width * height
	if cap(buf) < size {
		buf = make([]uint32, size)
	} else {
		buf = buf[:size]
	}
	di := 0
	for y := range height {
		row := pix[y*rowPixels : y*rowPixels+width]
		for _, v := range row {
			buf[di] = (v & 0xFF00FF00) | ((v & 0xFF) << 16) | ((v >> 16) & 0xFF)
			di++
		}
	}
	return buf
}
