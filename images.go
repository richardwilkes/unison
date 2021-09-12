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
	_ "embed" // Needed for image embedding

	"github.com/richardwilkes/toolbox/log/jot"
)

var (
	//go:embed resources/images/arrows_horizontal.png
	arrowsHorizontalData []byte
	arrowsHorizontalImg  *Image

	//go:embed resources/images/arrows_horizontal_vertical.png
	arrowsHorizontalVerticalData []byte
	arrowsHorizontalVerticalImg  *Image

	//go:embed resources/images/arrows_left_diagonal.png
	arrowsLeftDiagonalData []byte
	arrowsLeftDiagonalImg  *Image

	//go:embed resources/images/arrows_right_diagonal.png
	arrowsRightDiagonalData []byte
	arrowsRightDiagonalImg  *Image

	//go:embed resources/images/arrows_vertical.png
	arrowsVerticalData []byte
	arrowsVerticalImg  *Image

	//go:embed resources/images/closed_hand.png
	closedHandData []byte
	closedHandImg  *Image

	//go:embed resources/images/error.png
	errorData []byte
	errorImg  *Image

	//go:embed resources/images/open_hand.png
	openHandData []byte
	openHandImg  *Image

	//go:embed resources/images/question.png
	questionData []byte
	questionImg  *Image
)

// ArrowsHorizontalImage returns the standard horizontal arrows, pointing left and right.
func ArrowsHorizontalImage() *Image {
	return retrieveImage(arrowsHorizontalData, &arrowsHorizontalImg)
}

// ArrowsHorizontalVerticalImage returns the standard horizontal and vertical arrows, pointing left, right, up and down.
func ArrowsHorizontalVerticalImage() *Image {
	return retrieveImage(arrowsHorizontalVerticalData, &arrowsHorizontalVerticalImg)
}

// ArrowsLeftDiagonalImage returns the standard left diagonal arrows, pointing northwest and southeast.
func ArrowsLeftDiagonalImage() *Image {
	return retrieveImage(arrowsLeftDiagonalData, &arrowsLeftDiagonalImg)
}

// ArrowsRightDiagonalImage returns the standard right diagonal arrows, pointing northeast and southwest.
func ArrowsRightDiagonalImage() *Image {
	return retrieveImage(arrowsRightDiagonalData, &arrowsRightDiagonalImg)
}

// ArrowsVerticalImage returns the standard vertical arrows, pointing up and down.
func ArrowsVerticalImage() *Image {
	return retrieveImage(arrowsVerticalData, &arrowsVerticalImg)
}

// ClosedHandImage returns the standard closed hand image.
func ClosedHandImage() *Image {
	return retrieveImage(closedHandData, &closedHandImg)
}

// ErrorImage returns the standard error alert image.
func ErrorImage() *Image {
	return retrieveImage(errorData, &errorImg)
}

// OpenHandImage returns the standard open hand image.
func OpenHandImage() *Image {
	return retrieveImage(openHandData, &openHandImg)
}

// QuestionImage returns the standard question alert image.
func QuestionImage() *Image {
	return retrieveImage(questionData, &questionImg)
}

func retrieveImage(imgData []byte, img **Image) *Image {
	if *img == nil {
		var err error
		*img, err = NewImageFromBytes(imgData, 0.5)
		jot.FatalIfErr(err)
	}
	return *img
}
