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
	"unsafe"

	"github.com/richardwilkes/canvas/raster"
	"github.com/richardwilkes/unison/internal/w32"
)

// nativePresentCPUPixels displays a CPU-rendered frame by blitting the pixels to the window's device context.
func (w *Window) nativePresentCPUPixels(pixels *raster.Pixmap) {
	dc := w32.GetDC(w.wnd.wnd)
	if dc == 0 {
		return
	}
	defer w32.ReleaseDC(w.wnd.wnd, dc)
	width := int(pixels.Width)
	height := int(pixels.Height)
	if !w.w32EnsurePresentSurface(dc, width, height) {
		return
	}
	w32.RepackRGBAToBGRA(pixels.Pix, width, height, int(pixels.RowPixels), w.wnd.presentPixels)
	w32.BitBlt(dc, 0, 0, int32(width), int32(height), w.wnd.presentDC, 0, 0, w32.SRCCOPY)
}

// w32EnsurePresentSurface makes sure a DIB section of the given size is selected into the window's memory DC, ready
// to be blitted from, recreating it whenever the size changes. Reports whether a usable surface is in place.
//
// The frame is staged in a DIB section rather than handed to StretchDIBits directly because GDI refuses a single blit
// of more than a few megabytes of caller-supplied pixels: it silently copies nothing and reports success, leaving the
// window blank. A window of any ordinary size exceeds that once the display scaling is above 100% (1164x1744 pixels,
// about 8MB, is a routine size at 200%), so the direct path fails on exactly the machines the CPU renderer exists to
// serve. A DIB section is owned by the kernel and mapped into this process, so its pixels can be written in place and
// then moved with BitBlt, which has no such limit.
func (w *Window) w32EnsurePresentSurface(dc w32.HDC, width, height int) bool {
	if w.wnd.presentBmp != 0 && w.wnd.presentWidth == width && w.wnd.presentHeight == height {
		return true
	}
	w.w32DisposePresentSurface()
	if width < 1 || height < 1 {
		return false
	}
	var bits *byte
	bmp := w32.CreateDIBSection(dc, &w32.BITMAPV5HEADER{
		BV5Width:       int32(width),
		BV5Height:      -int32(height), // Negative height makes the rows top-down, matching the pixmap's layout.
		BV5Planes:      1,
		BV5BitCount:    32,
		BV5Compression: w32.BI_BITFIELDS,
		BV5RedMask:     0x00ff0000,
		BV5GreenMask:   0x0000ff00,
		BV5BlueMask:    0x000000ff,
		BV5AlphaMask:   0xff000000,
	}, w32.DIB_RGB_COLORS, &bits, 0, 0)
	if bmp == 0 || bits == nil {
		if bmp != 0 {
			w32.DeleteObject(w32.HGDIOBJ(bmp))
		}
		return false
	}
	memDC := w32.CreateCompatibleDC(dc)
	if memDC == 0 {
		w32.DeleteObject(w32.HGDIOBJ(bmp))
		return false
	}
	w.wnd.presentBmp = bmp
	w.wnd.presentDC = memDC
	w.wnd.presentPrev = w32.SelectObject(memDC, w32.HGDIOBJ(bmp))
	w.wnd.presentPixels = unsafe.Slice((*uint32)(unsafe.Pointer(bits)), width*height)
	w.wnd.presentWidth = width
	w.wnd.presentHeight = height
	return true
}

// w32DisposePresentSurface releases the presentation surface, if one has been created.
func (w *Window) w32DisposePresentSurface() {
	if w.wnd.presentDC != 0 {
		// The displaced object has to go back in before the DC can be deleted, since a DC never owns what is selected
		// into it and deleting one with the bitmap still selected would leak the bitmap.
		w32.SelectObject(w.wnd.presentDC, w.wnd.presentPrev)
		w32.DeleteDC(w.wnd.presentDC)
		w.wnd.presentDC = 0
		w.wnd.presentPrev = 0
	}
	if w.wnd.presentBmp != 0 {
		w32.DeleteObject(w32.HGDIOBJ(w.wnd.presentBmp))
		w.wnd.presentBmp = 0
	}
	w.wnd.presentPixels = nil
	w.wnd.presentWidth = 0
	w.wnd.presentHeight = 0
}
