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
	"embed"
	"sync"

	"github.com/richardwilkes/toolbox/log/jot"
)

var (
	//go:embed resources/images
	imageFS   embed.FS
	imageLock sync.Mutex
	imageMap  = make(map[string]*Image)
)

// ArrowsHorizontalImage returns the standard horizontal arrows, pointing left and right.
func ArrowsHorizontalImage() *Image {
	return getImage("arrows_horizontal")
}

// ArrowsHorizontalVerticalImage returns the standard horizontal and vertical arrows, pointing left, right, up and down.
func ArrowsHorizontalVerticalImage() *Image {
	return getImage("arrows_horizontal_vertical")
}

// ArrowsLeftDiagonalImage returns the standard left diagonal arrows, pointing northwest and southeast.
func ArrowsLeftDiagonalImage() *Image {
	return getImage("arrows_left_diagonal")
}

// ArrowsRightDiagonalImage returns the standard right diagonal arrows, pointing northeast and southwest.
func ArrowsRightDiagonalImage() *Image {
	return getImage("arrows_right_diagonal")
}

// ArrowsVerticalImage returns the standard vertical arrows, pointing up and down.
func ArrowsVerticalImage() *Image {
	return getImage("arrows_vertical")
}

// ClosedHandImage returns the standard closed hand image.
func ClosedHandImage() *Image {
	return getImage("closed_hand")
}

// ErrorImage returns the standard error alert image.
func ErrorImage() *Image {
	return getImage("error")
}

// OpenHandImage returns the standard open hand image.
func OpenHandImage() *Image {
	return getImage("open_hand")
}

// QuestionImage returns the standard question alert image.
func QuestionImage() *Image {
	return getImage("question")
}

func getImage(name string) *Image {
	imageLock.Lock()
	defer imageLock.Unlock()
	img, exists := imageMap[name]
	if !exists {
		var err error
		var data []byte
		data, err = imageFS.ReadFile("resources/images/" + name + ".png")
		jot.FatalIfErr(err)
		img, err = NewImageFromBytes(data, 0.5)
		jot.FatalIfErr(err)
		imageMap[name] = img
	}
	return img
}
