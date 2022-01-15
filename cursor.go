// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
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
	"runtime"

	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/richardwilkes/toolbox"
	"github.com/richardwilkes/toolbox/log/jot"
	"github.com/richardwilkes/toolbox/xmath/geom32"
	"golang.org/x/image/draw"
)

var (
	arrowCursor               *Cursor
	moveCursor                *Cursor
	resizeHorizontalCursor    *Cursor
	resizeLeftDiagonalCursor  *Cursor
	resizeRightDiagonalCursor *Cursor
	resizeVerticalCursor      *Cursor
	textCursor                *Cursor
)

// Cursor provides a graphical cursor for the mouse location.
type Cursor = glfw.Cursor

// ArrowCursor returns the standard arrow cursor.
func ArrowCursor() *Cursor {
	if arrowCursor == nil {
		arrowCursor = glfw.CreateStandardCursor(glfw.ArrowCursor)
	}
	return arrowCursor
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

// TextCursor returns the standard text cursor.
func TextCursor() *Cursor {
	if textCursor == nil {
		textCursor = glfw.CreateStandardCursor(glfw.IBeamCursor)
	}
	return textCursor
}

func retrieveCursor(img *Image, cursor **Cursor) *Cursor {
	if *cursor == nil {
		size := img.LogicalSize()
		*cursor = NewCursor(img, geom32.Point{X: size.Width / 2, Y: size.Height / 2})
	}
	return *cursor
}

// NewCursor creates a new custom cursor from an image.
func NewCursor(img *Image, hotSpot geom32.Point) *Cursor {
	nrgba, err := img.ToNRGBA()
	if err != nil {
		jot.Warn(err)
		return ArrowCursor()
	}
	if runtime.GOOS == toolbox.MacOS {
		// glfw doesn't take the high resolution cursors properly, so scale them down, if needed
		logicalSize := img.LogicalSize()
		size := img.Size()
		if logicalSize != size {
			dstRect := image.Rect(0, 0, int(logicalSize.Width), int(logicalSize.Height))
			dst := image.NewNRGBA(dstRect)
			draw.CatmullRom.Scale(dst, dstRect, nrgba, image.Rect(0, 0, int(size.Width), int(size.Height)),
				draw.Over, nil)
			nrgba = dst
		}
	}
	return glfw.CreateCursor(nrgba, int(hotSpot.X), int(hotSpot.Y))
}
