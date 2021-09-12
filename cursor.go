// Copyright ©2021 by Richard A. Wilkes. All rights reserved.
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
	arrowCursor                    *Cursor
	arrowsHorizontalCursor         *Cursor
	arrowsHorizontalVerticalCursor *Cursor
	arrowsLeftDiagonalCursor       *Cursor
	arrowsRightDiagonalCursor      *Cursor
	arrowsVerticalCursor           *Cursor
	closedHandCursor               *Cursor
	openHandCursor                 *Cursor
	textCursor                     *Cursor
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

// ArrowsHorizontalCursor returns the standard horizontal arrows cursor, pointing left and right.
func ArrowsHorizontalCursor() *Cursor {
	return retrieveCursor(ArrowsHorizontalImage(), &arrowsHorizontalCursor)
}

// ArrowsHorizontalVerticalCursor returns the standard horizontal and vertical arrows cursor, pointing left, right, up
// and down.
func ArrowsHorizontalVerticalCursor() *Cursor {
	return retrieveCursor(ArrowsHorizontalVerticalImage(), &arrowsHorizontalVerticalCursor)
}

// ArrowsLeftDiagonalCursor returns the standard left diagonal arrows cursor, pointing northwest and southeast.
func ArrowsLeftDiagonalCursor() *Cursor {
	return retrieveCursor(ArrowsLeftDiagonalImage(), &arrowsLeftDiagonalCursor)
}

// ArrowsRightDiagonalCursor returns the standard right diagonal arrows cursor, pointing northeast and southwest.
func ArrowsRightDiagonalCursor() *Cursor {
	return retrieveCursor(ArrowsRightDiagonalImage(), &arrowsRightDiagonalCursor)
}

// ArrowsVerticalCursor returns the standard vertical arrows cursor, pointing up and down.
func ArrowsVerticalCursor() *Cursor {
	return retrieveCursor(ArrowsVerticalImage(), &arrowsVerticalCursor)
}

// ClosedHandCursor returns the standard closed hand cursor.
func ClosedHandCursor() *Cursor {
	return retrieveCursor(ClosedHandImage(), &closedHandCursor)
}

// OpenHandCursor returns the standard open hand cursor.
func OpenHandCursor() *Cursor {
	return retrieveCursor(OpenHandImage(), &openHandCursor)
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
