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

	"github.com/richardwilkes/unison/internal/mac"
)

type apiNativeCursor struct {
	cursor mac.Cursor
	system bool
}

func apiNewCursor(img *image.NRGBA, xhot, yhot int) *Cursor {
	nsCursor := mac.NewCursor(img, xhot, yhot)
	if nsCursor == 0 {
		return nil
	}
	c := &Cursor{
		cursor: apiNativeCursor{
			cursor: nsCursor,
		},
	}
	cursorList = append(cursorList, c)
	return c
}

func (c *Cursor) apiDestroy() {
	if !c.cursor.system && c.cursor.cursor != 0 {
		c.cursor.cursor.Release()
		c.cursor.cursor = 0
	}
}

func apiArrowCursor() *Cursor {
	return &Cursor{
		cursor: apiNativeCursor{
			cursor: mac.ArrowCursor(),
			system: true,
		},
	}
}

func apiPointingCursor() *Cursor {
	return &Cursor{
		cursor: apiNativeCursor{
			cursor: mac.PointingHandCursor(),
			system: true,
		},
	}
}

func apiTextCursor() *Cursor {
	return &Cursor{
		cursor: apiNativeCursor{
			cursor: mac.IBeamCursor(),
			system: true,
		},
	}
}
