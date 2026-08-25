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
	"log/slog"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/unison/internal/x11"
)

type apiNativeCursor = x11.CursorID

// apiNewCursor creates the native cursor. X11 cursors are immutable, fixed-size pixmaps and this backend uses a single
// cached, global content scale rather than a per-monitor one, so one render at creation time is all X11 can consume.
// Mixed-DPI setups therefore get a single, uniform cursor scale, a pre-existing limitation of this backend.
func apiNewCursor(src *cursorSource) *Cursor {
	if x11Conn == nil {
		// There is no connection to an X server until Start has run (or once Terminate has finished), which is the
		// permanent state of a headless process such as a test run on a machine without a display, so there is
		// nothing to create a native cursor with. Callers can still reasonably expect the built-in cursors to exist
		// and to be distinct from one another, so hand back an inert cursor: a non-nil Cursor with no native cursor
		// behind it, which apiDestroy already knows there is nothing to free for. It is not recorded in cursorList,
		// since there is nothing for the teardown to release either.
		return &Cursor{}
	}
	scale := float32(1)
	if s, err := x11Conn.ContentScale(); err == nil {
		scale = s
	}
	img, err := src.render(scale)
	if err != nil {
		errs.Log(err)
		return nil
	}
	pixelWidth := img.Rect.Dx()
	pixelHeight := img.Rect.Dy()
	hotSpot := src.hotSpot.Mul(scale)
	hotSpotX := min(max(int(hotSpot.X), 0), pixelWidth-1)
	hotSpotY := min(max(int(hotSpot.Y), 0), pixelHeight-1)
	pm := x11Conn.CreatePixMap(x11.DrawableID(x11Conn.RootWindow()), 32, uint16(pixelWidth), uint16(pixelHeight))
	if pm == 0 {
		return nil
	}
	defer x11Conn.FreePixMap(pm)
	pix := x11.DrawableID(pm)
	gc := x11Conn.CreateGC(pix, 0, nil)
	if gc == 0 {
		return nil
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
		return nil
	}
	picture := x11Conn.ExtRender.CreatePicture(pix, format, 0, nil)
	if picture == 0 {
		return nil
	}
	defer x11Conn.ExtRender.FreePicture(picture)
	cursor := x11Conn.ExtRender.CreateCursor(picture, uint16(hotSpotX), uint16(hotSpotY))
	if cursor == 0 {
		return nil
	}
	c := &Cursor{cursor: cursor}
	cursorList = append(cursorList, c)
	return c
}

func (c *Cursor) apiDestroy() {
	if c.cursor != 0 {
		x11Conn.FreeCursor(c.cursor)
		c.cursor = 0
	}
}
