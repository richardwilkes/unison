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
	"slices"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"golang.org/x/image/draw"
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

// Cursor provides a graphical cursor for the mouse location.
type Cursor struct {
	cursor nativeCursor
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
		size := img.LogicalSize()
		*cursor = NewCursor(img, geom.NewPoint(size.Width/2, size.Height/2))
	}
	return *cursor
}

// NewCursor creates a new custom cursor from an image.
func NewCursor(img *Image, hotSpot geom.Point) *Cursor {
	nrgba, err := img.ToNRGBA()
	if err != nil {
		errs.Log(err)
		return ArrowCursor()
	}
	// TODO: Look at which platforms need this scaling step.
	logicalSize := img.LogicalSize()
	size := img.Size()
	if logicalSize != size {
		dstRect := image.Rect(0, 0, int(logicalSize.Width), int(logicalSize.Height))
		dst := image.NewNRGBA(dstRect)
		draw.CatmullRom.Scale(dst, dstRect, nrgba, image.Rect(0, 0, int(size.Width), int(size.Height)), draw.Over, nil)
		nrgba = dst
	}
	return newCursor(nrgba, int(hotSpot.X), int(hotSpot.Y))
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
	c.destroy()
}
