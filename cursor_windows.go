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

	"github.com/richardwilkes/unison/internal/w32"
)

type apiNativeCursor struct {
	cursor w32.HCURSOR
	system bool
}

func apiNewCursor(img *image.NRGBA, xhot, yhot int) *Cursor {
	icon := createIconFromImage(img, xhot, yhot, false)
	if icon == 0 {
		return nil
	}
	c := &Cursor{
		cursor: apiNativeCursor{
			cursor: w32.HCURSOR(icon),
		},
	}
	cursorList = append(cursorList, c)
	return c
}

func (c *Cursor) apiDestroy() {
	if !c.cursor.system && c.cursor.cursor != 0 {
		w32.DestroyIcon(w32.HICON(c.cursor.cursor))
		c.cursor.cursor = 0
		c.cursor.system = false
	}
}

func apiArrowCursor() *Cursor {
	return loadStdCursor(w32.OCR_NORMAL)
}

func apiPointingCursor() *Cursor {
	return loadStdCursor(w32.OCR_HAND)
}

func apiTextCursor() *Cursor {
	return loadStdCursor(w32.OCR_IBEAM)
}

func loadStdCursor(id int) *Cursor {
	return &Cursor{
		cursor: apiNativeCursor{
			cursor: w32.HCURSOR(w32.LoadImageW(0, w32.MakeIntResourceW(id), w32.IMAGE_CURSOR, 0, 0,
				w32.LR_DEFAULT_SIZE|w32.LR_SHARED)),
			system: true,
		},
	}
}

func createIconFromImage(img *image.NRGBA, hotX, hotY int, icon bool) w32.HICON {
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

	target := unsafe.Slice(ppvBits, len(img.Pix))
	for i := 0; i < len(img.Pix)/4; i++ {
		target[4*i] = img.Pix[4*i+2]
		target[4*i+1] = img.Pix[4*i+1]
		target[4*i+2] = img.Pix[4*i+0]
		target[4*i+3] = img.Pix[4*i+3]
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
