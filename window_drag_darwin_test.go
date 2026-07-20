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

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
)

// TestMacDragImageAndFrameNilImage is the regression test for apiStartDrag panicking on a nil drag image.
// Window.StartDrag and Panel.StartDrag both document that the image may be nil, and the Linux and Windows
// implementations guard for it, but the macOS path dereferenced it unconditionally. A nil image must yield nil pixel
// data and a minimal 1x1 frame anchored at the origin so the drag proceeds without an image, matching the other
// platforms.
func TestMacDragImageAndFrameNilImage(t *testing.T) {
	c := check.New(t)
	origin := geom.NewPoint(17, 42)
	nrgba, r := macDragImageAndFrame(nil, origin)
	c.Nil(nrgba)
	c.Equal(geom.Rect{Point: origin, Size: geom.NewSize(1, 1)}, r)
}

// TestMacDragImageAndFrameWithImage verifies the normal path: a valid image produces its pixel data and a frame whose
// size is the image's logical (scaled) size, anchored at the drag origin.
func TestMacDragImageAndFrameWithImage(t *testing.T) {
	c := check.New(t)
	const w, h = 4, 2
	img, err := NewImageFromPixels(w, h, distinctPixels(w, h, 47), geom.NewPoint(2, 2))
	c.NoError(err)
	c.NotNil(img)
	origin := geom.NewPoint(5, 9)
	nrgba, r := macDragImageAndFrame(img, origin)
	c.NotNil(nrgba)
	c.Equal(w, nrgba.Rect.Dx())
	c.Equal(h, nrgba.Rect.Dy())
	c.Equal(geom.Rect{Point: origin, Size: img.LogicalSize()}, r)
}
