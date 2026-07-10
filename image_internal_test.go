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
	"testing"

	"github.com/richardwilkes/canvas/imagecore"
	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
)

// TestImageBackedByRaster guards the invariant DrawImageNine relies on: an Image's underlying image is always a raster
// *imagecore.Image. If that ever changes, asRaster(img.image) would stop being a plain type assertion and start forcing
// a GPU->CPU readback (or fail) — the regression fixed in canvas.go's DrawImageNine.
func TestImageBackedByRaster(t *testing.T) {
	c := check.New(t)

	const w, h = 4, 3
	img, err := NewImageFromPixels(w, h, make([]byte, w*h*4), geom.NewPoint(1, 1))
	c.NoError(err)
	c.NotNil(img)

	raster, ok := img.image.(*imagecore.Image)
	c.True(ok, "img.image should always be a raster *imagecore.Image")

	// asRaster on the base image must be a pure type assertion: it returns the identical pointer, never allocating a
	// new CPU image via MakeNonTextureImage.
	c.Equal(raster, asRaster(img.image))
}
