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
	"github.com/go-gl/glfw/v3.3/glfw"
)

var (
	arrowCursor               *Cursor
	pointingCursor            *Cursor
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

// PointingCursor returns the standard pointing cursor.
func PointingCursor() *Cursor {
	if pointingCursor == nil {
		pointingCursor = glfw.CreateStandardCursor(glfw.HandCursor)
	}
	return pointingCursor
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
		*cursor = NewCursor(img, Point{X: size.Width / 2, Y: size.Height / 2})
	}
	return *cursor
}
