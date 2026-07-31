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
	"github.com/richardwilkes/unison/enums/filtermode"
)

// TestCanvasDrawImageNilGuards verifies that every image drawing entry point tolerates a nil image, since
// DrawImageRectInRect always has and the others previously dereferenced the image before delegating to it.
func TestCanvasDrawImageNilGuards(t *testing.T) {
	c := check.New(t)
	cv, _ := newPixmapCanvas(8, 8)
	c.NotPanics(func() { cv.DrawImage(nil, geom.NewPoint(1, 1), nil, nil) })
	c.NotPanics(func() { cv.DrawImageInRect(nil, geom.NewRect(0, 0, 4, 4), nil, nil) })
	c.NotPanics(func() { cv.DrawImageRectInRect(nil, geom.NewRect(0, 0, 2, 2), geom.NewRect(0, 0, 4, 4), nil, nil) })
	c.NotPanics(func() {
		cv.DrawImageNine(nil, geom.NewRect(1, 1, 2, 2), geom.NewRect(0, 0, 8, 8), filtermode.Linear, nil)
	})
}
