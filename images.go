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
	_ "embed"

	"github.com/richardwilkes/toolbox/log/jot"
)

var (
	arrowsHorizontalCursorImage *Image
	//go:embed resources/images/arrows_horizontal.png
	arrowsHorizontalCursorImageData []byte
)

// ArrowsHorizontalCursorImage returns the standard horizontal arrows cursor image, pointing left and right.
func ArrowsHorizontalCursorImage() *Image {
	if arrowsHorizontalCursorImage == nil {
		var err error
		arrowsHorizontalCursorImage, err = NewImageFromBytes(arrowsHorizontalCursorImageData, 0.5)
		jot.FatalIfErr(err)
	}
	return arrowsHorizontalCursorImage
}

var (
	arrowsHorizontalVerticalCursorImage *Image
	//go:embed resources/images/arrows_horizontal_vertical.png
	arrowsHorizontalVerticalCursorImageData []byte
)

// ArrowsHorizontalVerticalCursorImage returns the standard horizontal and vertical arrows cursor image, pointing left,
// right, up and down.
func ArrowsHorizontalVerticalCursorImage() *Image {
	if arrowsHorizontalVerticalCursorImage == nil {
		var err error
		arrowsHorizontalVerticalCursorImage, err = NewImageFromBytes(arrowsHorizontalVerticalCursorImageData, 0.5)
		jot.FatalIfErr(err)
	}
	return arrowsHorizontalVerticalCursorImage
}

var (
	arrowsLeftDiagonalCursorImage *Image
	//go:embed resources/images/arrows_left_diagonal.png
	arrowsLeftDiagonalCursorImageData []byte
)

// ArrowsLeftDiagonalCursorImage returns the standard left diagonal arrows cursor image, pointing northwest and
// southeast.
func ArrowsLeftDiagonalCursorImage() *Image {
	if arrowsLeftDiagonalCursorImage == nil {
		var err error
		arrowsLeftDiagonalCursorImage, err = NewImageFromBytes(arrowsLeftDiagonalCursorImageData, 0.5)
		jot.FatalIfErr(err)
	}
	return arrowsLeftDiagonalCursorImage
}

var (
	arrowsRightDiagonalCursorImage *Image
	//go:embed resources/images/arrows_right_diagonal.png
	arrowsRightDiagonalCursorImageData []byte
)

// ArrowsRightDiagonalCursorImage returns the standard right diagonal arrows cursor image, pointing northeast and
// southwest.
func ArrowsRightDiagonalCursorImage() *Image {
	if arrowsRightDiagonalCursorImage == nil {
		var err error
		arrowsRightDiagonalCursorImage, err = NewImageFromBytes(arrowsRightDiagonalCursorImageData, 0.5)
		jot.FatalIfErr(err)
	}
	return arrowsRightDiagonalCursorImage
}

var (
	arrowsVerticalCursorImage *Image
	//go:embed resources/images/arrows_vertical.png
	arrowsVerticalCursorImageData []byte
)

// ArrowsVerticalCursorImage returns the standard vertical arrows cursor image, pointing up and down.
func ArrowsVerticalCursorImage() *Image {
	if arrowsVerticalCursorImage == nil {
		var err error
		arrowsVerticalCursorImage, err = NewImageFromBytes(arrowsVerticalCursorImageData, 0.5)
		jot.FatalIfErr(err)
	}
	return arrowsVerticalCursorImage
}

var (
	closedHandCursorImage *Image
	//go:embed resources/images/closed_hand.png
	closedHandCursorImageData []byte
)

// ClosedHandCursorImage returns the standard closed hand cursor image.
func ClosedHandCursorImage() *Image {
	if closedHandCursorImage == nil {
		var err error
		closedHandCursorImage, err = NewImageFromBytes(closedHandCursorImageData, 0.5)
		jot.FatalIfErr(err)
	}
	return closedHandCursorImage
}

var (
	openHandCursorImage *Image
	//go:embed resources/images/open_hand.png
	openHandCursorImageData []byte
)

// OpenHandCursorImage returns the standard open hand cursor image.
func OpenHandCursorImage() *Image {
	if openHandCursorImage == nil {
		var err error
		openHandCursorImage, err = NewImageFromBytes(openHandCursorImageData, 0.5)
		jot.FatalIfErr(err)
	}
	return openHandCursorImage
}
