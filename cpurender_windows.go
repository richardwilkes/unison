// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"github.com/richardwilkes/canvas/raster"
	"github.com/richardwilkes/unison/internal/w32"
)

// apiPresentCPUPixels displays a CPU-rendered frame by copying the pixels to the window's device context.
func (w *Window) apiPresentCPUPixels(pixels *raster.Pixmap) {
	dc := w32.GetDC(w.wnd.wnd)
	if dc == 0 {
		return
	}
	defer w32.ReleaseDC(w.wnd.wnd, dc)
	// Repack the premultiplied RGBA words as the BGRA words GDI expects, dropping any stride padding in the process.
	width := int(pixels.Width)
	height := int(pixels.Height)
	buf := make([]uint32, width*height)
	di := 0
	for y := range height {
		row := pixels.Pix[y*int(pixels.RowPixels) : y*int(pixels.RowPixels)+width]
		for _, v := range row {
			buf[di] = (v & 0xFF00FF00) | ((v & 0xFF) << 16) | ((v >> 16) & 0xFF)
			di++
		}
	}
	hdr := w32.BITMAPINFOHEADER{
		BiWidth:       int32(width),
		BiHeight:      -int32(height), // Negative height makes the rows top-down, matching the pixmap's layout.
		BiPlanes:      1,
		BiBitCount:    32,
		BiCompression: w32.BI_RGB,
	}
	w32.StretchDIBits(dc, 0, 0, int32(width), int32(height), 0, 0, int32(width), int32(height), buf, &hdr,
		w32.DIB_RGB_COLORS, w32.SRCCOPY)
}
