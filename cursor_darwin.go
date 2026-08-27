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

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/unison/internal/cocoa"
)

type nativeCursorHandle = cocoa.Cursor

// nativeNewCursor hands the cursor's pixels to AppKit, which does the per-display work itself: an NSCursor's image has
// a point size and one bitmap representation per scale it was rasterized at, and AppKit draws whichever representation
// best matches the display the cursor appears on, so nothing needs to be re-rasterized as the cursor crosses displays.
func nativeNewCursor(src *cursorSource) *Cursor {
	var reps []*image.NRGBA
	if src.svg == nil {
		// An image-backed cursor has no more detail than the pixels it was handed, so it becomes a single
		// representation, preserving the image's own pixel density.
		reps = []*image.NRGBA{src.img}
	} else {
		// macOS backing scale factors are only ever 1 or 2. Rasterizing a true 1x alongside the 2x is sharper on a
		// non-retina display than letting AppKit downsample the 2x representation for it.
		for _, scale := range []float32{1, 2} {
			img, err := src.render(scale)
			if err != nil {
				errs.Log(err)
				return nil
			}
			reps = append(reps, img)
		}
	}
	nsCursor := cocoa.NewCursor(reps, src.hotSpot, src.size)
	if nsCursor == 0 {
		return nil
	}
	c := &Cursor{
		cursor: nsCursor,
	}
	cursorList = append(cursorList, c)
	return c
}

func (c *Cursor) nativeDestroy() {
	if c.cursor != 0 {
		c.cursor.Release()
		c.cursor = 0
	}
}
