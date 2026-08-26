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
	"image"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/unison/internal/pixconv"
	"github.com/richardwilkes/unison/internal/w32"
)

type nativeCursorHandle = *w32Cursor

// w32Cursor retains the cursor's source so that a correctly sized native cursor can be produced for whatever monitor
// DPI the cursor is currently being displayed on. Windows does not automatically rescale custom cursors as they move
// between monitors with differing DPI, so we lazily build and cache one HCURSOR per backing scale, re-rasterizing
// vector sources at that exact scale and resampling image sources.
type w32Cursor struct {
	src     *cursorSource
	byScale map[float32]w32.HCURSOR
}

func nativeNewCursor(src *cursorSource) *Cursor {
	c := &Cursor{
		cursor: &w32Cursor{
			src:     src,
			byScale: make(map[float32]w32.HCURSOR),
		},
	}
	cursorList = append(cursorList, c)
	return c
}

// handle returns the native cursor sized for the given backing scale, creating and caching it on first use.
func (c *w32Cursor) handle(scale float32) w32.HCURSOR {
	if scale <= 0 {
		scale = 1
	}
	if h, ok := c.byScale[scale]; ok {
		return h
	}
	img, err := c.src.render(scale)
	if err != nil {
		// Don't cache the failure, so a later attempt at this scale can succeed.
		errs.Log(err)
		return 0
	}
	width := img.Rect.Dx()
	height := img.Rect.Dy()
	hot := c.src.hotSpot.Mul(scale)
	hotX := min(max(int(hot.X), 0), width-1)
	hotY := min(max(int(hot.Y), 0), height-1)
	h := w32.HCURSOR(w32CreateIconFromImage(img, hotX, hotY, false))
	c.byScale[scale] = h
	return h
}

func (c *Cursor) nativeDestroy() {
	if c.cursor != nil {
		for scale, h := range c.cursor.byScale {
			if h != 0 {
				w32.DestroyIcon(w32.HICON(h))
			}
			delete(c.cursor.byScale, scale)
		}
		// Zero the native cursor, as the other platforms do, so use after destroy cannot silently recreate an HCURSOR.
		c.cursor = nil
	}
}

func w32CreateIconFromImage(img *image.NRGBA, hotX, hotY int, icon bool) w32.HICON {
	dc := w32.GetDC(0)
	if dc == 0 {
		return 0
	}
	defer w32.ReleaseDC(0, dc)

	var ppvBits *byte
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	color := w32.CreateDIBSection(dc, &w32.BITMAPV5HEADER{
		BV5Width:       int32(w),
		BV5Height:      int32(-h),
		BV5Planes:      1,
		BV5BitCount:    32,
		BV5Compression: w32.BI_BITFIELDS,
		BV5RedMask:     0x00ff0000,
		BV5GreenMask:   0x0000ff00,
		BV5BlueMask:    0x000000ff,
		BV5AlphaMask:   0xff000000,
	}, w32.DIB_RGB_COLORS, &ppvBits, 0, 0)
	if color == 0 {
		return 0
	}
	defer w32.DeleteObject(w32.HGDIOBJ(color))

	mask := w32.CreateBitmap(int32(w), int32(h), 1, 1, nil)
	if mask == 0 {
		return 0
	}
	defer w32.DeleteObject(w32.HGDIOBJ(mask))

	// The DIB wants BGRA and holds exactly w*h*4 bytes with no stride padding, so the red/blue exchange runs a row at a
	// time through PixOffset. The source is whatever image the caller handed to NewCursor or SetTitleIcons, which may
	// be a sub-image with a non-zero origin and a stride wider than its rows; a single pass over img.Pix would then
	// copy that padding and run past the end of the DIB. The DIB's pixel store is written in place, never replaced.
	rowBytes := w * 4
	target := unsafe.Slice(ppvBits, h*rowBytes)
	for y := range h {
		si := img.PixOffset(bounds.Min.X, bounds.Min.Y+y)
		di := y * rowBytes
		pixconv.SwizzleRBBytes(target[di:di+rowBytes], img.Pix[si:si+rowBytes])
	}

	var iconInt32 int32
	if icon {
		iconInt32 = 1
	}
	return w32.CreateIconIndirect(&w32.ICONINFO{
		Icon:     iconInt32,
		XHotspot: uint32(hotX),
		YHotspot: uint32(hotY),
		Mask:     mask,
		Color:    color,
	})
}
