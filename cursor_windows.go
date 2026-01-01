// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
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

type nativeCursor struct {
	cursor w32.HCURSOR
	system bool
}

func newCursor(img *image.NRGBA, xhot, yhot int) *Cursor {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	var bi w32.BITMAPV5HEADER
	bi.BV5Size = uint32(unsafe.Sizeof(bi))
	bi.BV5Width = int32(w)
	bi.BV5Height = int32(-h)
	bi.BV5Planes = 1
	bi.BV5BitCount = 32
	bi.BV5Compression = w32.BI_BITFIELDS
	bi.BV5RedMask = 0x00ff0000
	bi.BV5GreenMask = 0x0000ff00
	bi.BV5BlueMask = 0x000000ff
	bi.BV5AlphaMask = 0xff000000

	dc := w32.GetDC(0)
	if dc == 0 {
		return nil
	}
	defer w32.ReleaseDC(0, dc)

	var ppvBits *byte
	color := w32.CreateDIBSection(dc, &bi, w32.DIB_RGB_COLORS, &ppvBits, 0, 0)
	if color == 0 {
		return nil
	}
	defer w32.DeleteObject(w32.HGDIOBJ(color))

	mask := w32.CreateBitmap(int32(w), int32(h), 1, 1, nil)
	if mask == 0 {
		return nil
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
	handle := w32.CreateIconIndirect(&w32.ICONINFO{
		Icon:     iconInt32,
		XHotspot: uint32(xhot),
		YHotspot: uint32(yhot),
		Mask:     mask,
		Color:    color,
	})
	if handle == 0 {
		return nil
	}
	c := &Cursor{
		cursor: nativeCursor{
			cursor: w32.HCURSOR(handle),
			system: false,
		},
	}
	cursorList = append(cursorList, c)
	return c
}

func (c *Cursor) destroy() {
	if !c.cursor.system && c.cursor.cursor != 0 {
		w32.DestroyIcon(w32.HICON(c.cursor.cursor))
		c.cursor.cursor = 0
		c.cursor.system = false
	}
}

// ArrowCursor returns the standard arrow cursor.
func ArrowCursor() *Cursor {
	if arrowCursor == nil {
		arrowCursor = loadStdCursor(w32.OCR_NORMAL)
	}
	return arrowCursor
}

// PointingCursor returns the standard pointing cursor.
func PointingCursor() *Cursor {
	if pointingCursor == nil {
		pointingCursor = loadStdCursor(w32.OCR_HAND)
	}
	return pointingCursor
}

// TextCursor returns the standard text cursor.
func TextCursor() *Cursor {
	if textCursor == nil {
		textCursor = loadStdCursor(w32.OCR_IBEAM)
	}
	return textCursor
}

func loadStdCursor(id int) *Cursor {
	return &Cursor{
		cursor: nativeCursor{
			cursor: w32.HCURSOR(w32.LoadImageW(0, w32.MakeIntResourceW(id), w32.IMAGE_CURSOR, 0, 0,
				w32.LR_DEFAULT_SIZE|w32.LR_SHARED)),
			system: true,
		},
	}
}
