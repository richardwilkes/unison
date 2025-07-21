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
	_ "embed"

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xos"
)

var (
	moveCursorImage *Image
	//go:embed resources/images/move.png
	moveCursorImageData []byte
)

// MoveCursorImage returns the standard move cursor image.
func MoveCursorImage() *Image {
	if moveCursorImage == nil {
		var err error
		moveCursorImage, err = NewImageFromBytes(moveCursorImageData, geom.NewPoint(0.5, 0.5))
		xos.ExitIfErr(err)
	}
	return moveCursorImage
}

var (
	resizeHorizontalCursorImage *Image
	//go:embed resources/images/resize_horizontal.png
	resizeHorizontalCursorImageData []byte
)

// ResizeHorizontalCursorImage returns the standard horizontal resize cursor image.
func ResizeHorizontalCursorImage() *Image {
	if resizeHorizontalCursorImage == nil {
		var err error
		resizeHorizontalCursorImage, err = NewImageFromBytes(resizeHorizontalCursorImageData, geom.NewPoint(0.5, 0.5))
		xos.ExitIfErr(err)
	}
	return resizeHorizontalCursorImage
}

var (
	resizeLeftDiagonalCursorImage *Image
	//go:embed resources/images/resize_left_diagonal.png
	resizeLeftDiagonalCursorImageData []byte
)

// ResizeLeftDiagonalCursorImage returns the standard left diagonal resize cursor image.
func ResizeLeftDiagonalCursorImage() *Image {
	if resizeLeftDiagonalCursorImage == nil {
		var err error
		resizeLeftDiagonalCursorImage, err = NewImageFromBytes(resizeLeftDiagonalCursorImageData,
			geom.NewPoint(0.5, 0.5))
		xos.ExitIfErr(err)
	}
	return resizeLeftDiagonalCursorImage
}

var (
	resizeRightDiagonalCursorImage *Image
	//go:embed resources/images/resize_right_diagonal.png
	resizeRightDiagonalCursorImageData []byte
)

// ResizeRightDiagonalCursorImage returns the standard right diagonal resize cursor image.
func ResizeRightDiagonalCursorImage() *Image {
	if resizeRightDiagonalCursorImage == nil {
		var err error
		resizeRightDiagonalCursorImage, err = NewImageFromBytes(resizeRightDiagonalCursorImageData,
			geom.NewPoint(0.5, 0.5))
		xos.ExitIfErr(err)
	}
	return resizeRightDiagonalCursorImage
}

var (
	resizeVerticalCursorImage *Image
	//go:embed resources/images/resize_vertical.png
	resizeVerticalCursorImageData []byte
)

// ResizeVerticalCursorImage returns the standard vertical resize cursor image.
func ResizeVerticalCursorImage() *Image {
	if resizeVerticalCursorImage == nil {
		var err error
		resizeVerticalCursorImage, err = NewImageFromBytes(resizeVerticalCursorImageData, geom.NewPoint(0.5, 0.5))
		xos.ExitIfErr(err)
	}
	return resizeVerticalCursorImage
}
