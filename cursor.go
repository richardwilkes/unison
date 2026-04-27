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
	"slices"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
)

var (
	arrowCursor               *Cursor
	moveCursor                *Cursor
	pointingCursor            *Cursor
	resizeHorizontalCursor    *Cursor
	resizeLeftDiagonalCursor  *Cursor
	resizeRightDiagonalCursor *Cursor
	resizeVerticalCursor      *Cursor
	textCursor                *Cursor
)

var cursorList []*Cursor

// TODO: Cursors on Windows & Linux need to be dynamically sized based on the content scale of the window

// Cursor provides a graphical cursor for the mouse location.
type Cursor struct {
	cursor apiNativeCursor
}

// ArrowCursor returns the standard arrow cursor.
func ArrowCursor() *Cursor {
	if arrowCursor == nil {
		arrowCursor = apiArrowCursor()
	}
	return arrowCursor
}

// PointingCursor returns the standard pointing cursor.
func PointingCursor() *Cursor {
	if pointingCursor == nil {
		pointingCursor = apiPointingCursor()
	}
	return pointingCursor
}

// TextCursor returns the standard text cursor.
func TextCursor() *Cursor {
	if textCursor == nil {
		textCursor = apiTextCursor()
	}
	return textCursor
}

// MoveCursor returns the standard move cursor.
func MoveCursor() *Cursor {
	return retrieveCursor(MoveCursorImage(), &moveCursor)
}

// ResizeHorizontalCursor returns the standard horizontal resize cursor.
func ResizeHorizontalCursor() *Cursor {
	return retrieveCursor(ResizeHorizontalCursorImage(), &resizeHorizontalCursor)
}

// ResizeLeftDiagonalCursor returns the standard left diagonal resize cursor.
func ResizeLeftDiagonalCursor() *Cursor {
	return retrieveCursor(ResizeLeftDiagonalCursorImage(), &resizeLeftDiagonalCursor)
}

// ResizeRightDiagonalCursor returns the standard right diagonal resize cursor.
func ResizeRightDiagonalCursor() *Cursor {
	return retrieveCursor(ResizeRightDiagonalCursorImage(), &resizeRightDiagonalCursor)
}

// ResizeVerticalCursor returns the standard vertical resize cursor.
func ResizeVerticalCursor() *Cursor {
	return retrieveCursor(ResizeVerticalCursorImage(), &resizeVerticalCursor)
}

func retrieveCursor(img *Image, cursor **Cursor) *Cursor {
	if *cursor == nil {
		*cursor = NewCursor(img)
	}
	return *cursor
}

// NewCursor creates a new custom cursor from an image, with the hot spot at the center of the image.
func NewCursor(img *Image) *Cursor {
	size := img.LogicalSize()
	return NewCursorWithHotSpot(img, geom.NewPoint(size.Width/2, size.Height/2))
}

// NewCursorWithHotSpot creates a new custom cursor from an image with the specified hot spot.
func NewCursorWithHotSpot(img *Image, hotSpot geom.Point) *Cursor {
	logicalSize := img.LogicalSize()
	if hotSpot.X < 0 {
		hotSpot.X = 0
	} else if hotSpot.X >= logicalSize.Width {
		hotSpot.X = logicalSize.Width - 1
	}
	if hotSpot.Y < 0 {
		hotSpot.Y = 0
	} else if hotSpot.Y >= logicalSize.Height {
		hotSpot.Y = logicalSize.Height - 1
	}
	nrgba, err := img.ToNRGBA()
	if err != nil {
		errs.Log(err)
		return ArrowCursor()
	}
	return apiNewCursor(nrgba, hotSpot, img.LogicalSize())
}

// Destroy releases the resources associated with the cursor.
func (c *Cursor) Destroy() {
	if c == nil {
		return
	}
	for _, w := range windowList {
		if w.cursor == c {
			w.cursor = nil
			w.adjustToCursorChange()
		}
	}
	cursorList = slices.DeleteFunc(cursorList, func(cur *Cursor) bool { return cur == c })
	c.apiDestroy()
}
