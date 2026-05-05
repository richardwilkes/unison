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
	openHandCursor            *Cursor
	closedHandCursor          *Cursor
	resizeHorizontalCursor    *Cursor
	resizeLeftDiagonalCursor  *Cursor
	resizeRightDiagonalCursor *Cursor
	resizeVerticalCursor      *Cursor
	textCursor                *Cursor
)

var cursorList []*Cursor

// Cursor provides a graphical cursor for the mouse location.
type Cursor struct {
	cursor apiNativeCursor
}

// DefaultCursorSize returns the default size for cursors.
func DefaultCursorSize() geom.Size {
	return geom.NewSize(24, 24)
}

// ArrowCursor returns the standard arrow cursor.
func ArrowCursor() *Cursor {
	return retrieveCursorWithHotSpot(CursorArrowSVG, &arrowCursor, geom.NewPoint(8, 4))
}

// PointingCursor returns the standard pointing cursor.
func PointingCursor() *Cursor {
	return retrieveCursorWithHotSpot(CursorHandPointingSVG, &pointingCursor, geom.NewPoint(9, 5))
}

// OpenHandCursor returns the standard open hand cursor.
func OpenHandCursor() *Cursor {
	return retrieveCursor(CursorHandOpenSVG, &openHandCursor)
}

// ClosedHandCursor returns the standard closed hand cursor.
func ClosedHandCursor() *Cursor {
	return retrieveCursor(CursorHandClosedSVG, &closedHandCursor)
}

// TextCursor returns the standard text cursor.
func TextCursor() *Cursor {
	return retrieveCursor(CursorTextSVG, &textCursor)
}

// MoveCursor returns the standard move cursor.
func MoveCursor() *Cursor {
	return retrieveCursor(CursorMoveSVG, &moveCursor)
}

// ResizeHorizontalCursor returns the standard horizontal resize cursor.
func ResizeHorizontalCursor() *Cursor {
	return retrieveCursor(CursorResizeHorizontalSVG, &resizeHorizontalCursor)
}

// ResizeLeftDiagonalCursor returns the standard left diagonal resize cursor.
func ResizeLeftDiagonalCursor() *Cursor {
	return retrieveCursor(CursorResizeLeftDiagonalSVG, &resizeLeftDiagonalCursor)
}

// ResizeRightDiagonalCursor returns the standard right diagonal resize cursor.
func ResizeRightDiagonalCursor() *Cursor {
	return retrieveCursor(CursorResizeRightDiagonalSVG, &resizeRightDiagonalCursor)
}

// ResizeVerticalCursor returns the standard vertical resize cursor.
func ResizeVerticalCursor() *Cursor {
	return retrieveCursor(CursorResizeVerticalSVG, &resizeVerticalCursor)
}

func retrieveCursor(svg *SVG, cursor **Cursor) *Cursor {
	return retrieveCursorWithHotSpot(svg, cursor, geom.PointFromSize(DefaultCursorSize().Div(2)))
}

func retrieveCursorWithHotSpot(svg *SVG, cursor **Cursor, hotSpot geom.Point) *Cursor {
	if *cursor == nil {
		*cursor = NewCursorFromSVG(svg, hotSpot, DefaultCursorSize())
	}
	return *cursor
}

// NewCursorFromSVG creates a new custom cursor from a SVG.
func NewCursorFromSVG(svg *SVG, hotSpot geom.Point, size geom.Size) *Cursor {
	img, err := NewImageFromDrawing(int(size.Width), int(size.Height), 144, func(gc *Canvas) {
		svg.DrawInRectPreservingAspectRatio(gc, geom.NewRect(0, 0, size.Width, size.Height), nil, nil)
	})
	if err != nil {
		errs.Log(err)
		return nil
	}
	return NewCursor(img, hotSpot)
}

// NewCursor creates a new custom cursor from an image.
func NewCursor(img *Image, hotSpot geom.Point) *Cursor {
	logicalSize := img.LogicalSize()
	hotSpot.X = min(max(hotSpot.X, 0), logicalSize.Width-1)
	hotSpot.Y = min(max(hotSpot.Y, 0), logicalSize.Height-1)
	nrgba, err := img.ToNRGBA()
	if err != nil {
		errs.Log(err)
		return nil
	}
	return apiNewCursor(nrgba, hotSpot, logicalSize)
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
