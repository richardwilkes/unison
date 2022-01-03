// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package demo

import (
	_ "embed" // Used to embed the images

	"github.com/richardwilkes/unison"
)

var (
	//go:embed resources/home.png
	homePngBytes []byte
	homeImage    *unison.Image
)

// HomeImage returns a stylized image of a home, suitable for an icon.
func HomeImage() (*unison.Image, error) {
	if homeImage == nil {
		var err error
		if homeImage, err = unison.NewImageFromBytes(homePngBytes, 0.5); err != nil {
			return nil, err
		}
	}
	return homeImage, nil
}

var (
	//go:embed resources/classic-apple-logo.png
	classicAppleLogoPngBytes []byte
	classicAppleLogoImage    *unison.Image
)

// ClassicAppleLogoImage returns an image of the classic rainbow-colored Apple logo.
func ClassicAppleLogoImage() (*unison.Image, error) {
	if classicAppleLogoImage == nil {
		var err error
		if classicAppleLogoImage, err = unison.NewImageFromBytes(classicAppleLogoPngBytes, 0.5); err != nil {
			return nil, err
		}
	}
	return classicAppleLogoImage, nil
}

var (
	//go:embed resources/mountains.jpg
	mountainsJpgBytes []byte
	mountainsImage    *unison.Image
)

// MountainsImage returns an image of some mountains.
func MountainsImage() (*unison.Image, error) {
	if mountainsImage == nil {
		var err error
		if mountainsImage, err = unison.NewImageFromBytes(mountainsJpgBytes, 0.5); err != nil {
			return nil, err
		}
	}
	return mountainsImage, nil
}
