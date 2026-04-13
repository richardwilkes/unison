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
	"log/slog"

	"github.com/richardwilkes/unison/internal/x11"
)

// Default size of a cursor should be content scale * 16

type apiNativeCursor struct {
	cursor x11.CursorID
	system bool
}

func apiNewCursor(img *image.NRGBA, xhot, yhot int) *Cursor {
	pm := x11Conn.CreatePixMap(x11.DrawableID(x11Conn.RootWindow()), 32, uint16(img.Rect.Dx()), uint16(img.Rect.Dy()))
	if pm == 0 {
		return &Cursor{}
	}
	defer x11Conn.FreePixMap(pm)
	pix := x11.DrawableID(pm)
	gc := x11Conn.CreateGC(pix, 0, nil)
	if gc == 0 {
		return &Cursor{}
	}
	defer x11Conn.FreeGC(gc)
	x11Conn.PutImage(pix, gc, 0, 0, img)
	formats := x11Conn.ExtRender.QueryPictFormats()
	var format x11.PictFormat
	// Look for ARGB32 format, which is a standard format that is always present.
	for i := range formats.Formats {
		f := &formats.Formats[i]
		if f.Type == x11.PictTypeDirect &&
			f.Depth == 32 &&
			f.Direct.RedShift == 16 &&
			f.Direct.RedMask == 0xff &&
			f.Direct.GreenShift == 8 &&
			f.Direct.GreenMask == 0xff &&
			f.Direct.BlueShift == 0 &&
			f.Direct.BlueMask == 0xff &&
			f.Direct.AlphaShift == 24 &&
			f.Direct.AlphaMask == 0xff {
			format = f.ID
			break
		}
	}
	if format == 0 {
		slog.Error("unable to find the ARGB32 format")
		return &Cursor{}
	}
	picture := x11Conn.ExtRender.CreatePicture(pix, format, 0, nil)
	if picture == 0 {
		return &Cursor{}
	}
	defer x11Conn.ExtRender.FreePicture(picture)
	return &Cursor{
		cursor: apiNativeCursor{
			cursor: x11Conn.ExtRender.CreateCursor(picture, uint16(xhot), uint16(yhot)),
		},
	}
}

func (c *Cursor) apiDestroy() {
	if c.cursor.cursor != 0 {
		if !c.cursor.system {
			x11Conn.FreeCursor(c.cursor.cursor)
		}
		c.cursor.cursor = 0
	}
}

func apiArrowCursor() *Cursor {
	return x11LoadSystemCursor(68) // LeftPtr
}

func apiPointingCursor() *Cursor {
	return x11LoadSystemCursor(60) // Hand2
}

func apiTextCursor() *Cursor {
	return x11LoadSystemCursor(152) // Xterm
}

func x11LoadSystemCursor(id uint16) *Cursor {
	var cursorID x11.CursorID
	if fontID := x11Conn.OpenFont("cursor"); fontID != 0 {
		defer x11Conn.CloseFont(fontID)
		cursorID = x11Conn.CreateGlyphCursor(fontID, fontID, id, id+1, 0, 0, 0, 0xffff, 0xffff, 0xffff)
	}
	return &Cursor{
		cursor: apiNativeCursor{
			cursor: cursorID,
			system: true,
		},
	}
}
