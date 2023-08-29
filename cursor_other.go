// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

//go:build !darwin

package unison

import (
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/richardwilkes/toolbox/errs"
)

// NewCursor creates a new custom cursor from an image.
func NewCursor(img *Image, hotSpot Point) *Cursor {
	nrgba, err := img.ToNRGBA()
	if err != nil {
		errs.Log(err)
		return ArrowCursor()
	}
	return glfw.CreateCursor(nrgba, int(hotSpot.X), int(hotSpot.Y))
}
