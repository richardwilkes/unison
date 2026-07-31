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

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/internal/cocoa"
)

type apiNativeCursor = cocoa.Cursor

func apiNewCursor(img *image.NRGBA, hotSpot geom.Point, logicalSize geom.Size) *Cursor {
	nsCursor := cocoa.NewCursor(img, hotSpot, logicalSize)
	if nsCursor == 0 {
		return nil
	}
	c := &Cursor{
		cursor: nsCursor,
	}
	cursorList = append(cursorList, c)
	return c
}

func (c *Cursor) apiDestroy() {
	if c.cursor != 0 {
		c.cursor.Release()
		c.cursor = 0
	}
}
