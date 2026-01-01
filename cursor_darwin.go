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

	"github.com/richardwilkes/unison/internal/mac"
)

type nativeCursor = mac.Cursor

func newCursor(img *image.NRGBA, xhot, yhot int) *Cursor {
	nsCursor := mac.NewCursor(img, xhot, yhot)
	if nsCursor == 0 {
		return nil
	}
	c := &Cursor{cursor: nsCursor}
	cursorList = append(cursorList, c)
	return c
}

func (c *Cursor) destroy() {
	if c.cursor != 0 {
		c.cursor.Release()
		c.cursor = 0
	}
}

// ArrowCursor returns the standard arrow cursor.
func ArrowCursor() *Cursor {
	if arrowCursor == nil {
		arrowCursor = &Cursor{cursor: mac.ArrowCursor()}
		cursorList = append(cursorList, arrowCursor)
	}
	return arrowCursor
}

// PointingCursor returns the standard pointing cursor.
func PointingCursor() *Cursor {
	if pointingCursor == nil {
		pointingCursor = &Cursor{cursor: mac.PointingHandCursor()}
		cursorList = append(cursorList, pointingCursor)
	}
	return pointingCursor
}

// TextCursor returns the standard text cursor.
func TextCursor() *Cursor {
	if textCursor == nil {
		textCursor = &Cursor{cursor: mac.IBeamCursor()}
		cursorList = append(cursorList, textCursor)
	}
	return textCursor
}
