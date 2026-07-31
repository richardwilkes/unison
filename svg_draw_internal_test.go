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

	"github.com/richardwilkes/canvas/raster"
	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/enums/paintstyle"
)

// inkColBounds returns the leftmost and rightmost columns containing any non-transparent pixel within the row range
// [minY, maxY), or (-1, -1) if the region is empty.
func inkColBounds(pix *raster.Pixmap, minY, maxY int32) (left, right int32) {
	left = -1
	right = -1
	for x := int32(0); x < pix.Width; x++ {
		for y := minY; y < maxY; y++ {
			if pix.Pix[int(y)*int(pix.RowPixels)+int(x)]>>24 != 0 {
				if left == -1 {
					left = x
				}
				right = x
				break
			}
		}
	}
	return left, right
}

// checkInkBounds asserts that the ink drawn into pix exactly covers rect, allowing one pixel of slack on each edge for
// antialiasing.
func checkInkBounds(c check.Checker, pix *raster.Pixmap, rect geom.Rect) {
	top, bottom := inkRowBounds(pix, 0, pix.Width)
	left, right := inkColBounds(pix, 0, pix.Height)
	c.NotEqual(int32(-1), top, "expected some ink to be drawn")
	within1 := func(expected, actual int32, what string) {
		diff := expected - actual
		if diff < 0 {
			diff = -diff
		}
		c.True(diff <= 1, "%s: expected %d, got %d", what, expected, actual)
	}
	within1(int32(rect.Y), top, "top")
	within1(int32(rect.Bottom())-1, bottom, "bottom")
	within1(int32(rect.X), left, "left")
	within1(int32(rect.Right())-1, right, "right")
}

func TestSVGDrawInRectStretchesToExactRect(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
	}{
		{
			// A square SVG whose single path covers the entire viewBox.
			name:    "origin viewBox",
			content: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect width="10" height="10"/></svg>`,
		},
		{
			// The same shape, but with a viewBox whose origin is not (0, 0), to verify the origin translation composes
			// with the stretch correctly.
			name:    "offset viewBox",
			content: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="5 -3 10 10"><rect x="5" y="-3" width="10" height="10"/></svg>`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			svg, err := NewSVGFromContentString(tc.content)
			c.NoError(err)

			// Draw into a rect much wider than the SVG's aspect ratio. The content must be stretched non-uniformly to
			// fill exactly this rect, with no leftover centering shift pushing it outside.
			cv, pix := newPixmapCanvas(64, 32)
			rect := geom.NewRect(8, 4, 40, 16)
			svg.DrawInRect(cv, rect, nil, Black.Paint(cv, rect, paintstyle.Fill))
			checkInkBounds(c, pix, rect)
		})
	}
}

func TestSVGDrawInRectPreservingAspectRatioCenters(t *testing.T) {
	c := check.New(t)
	svg, err := NewSVGFromContentString(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect width="10" height="10"/></svg>`)
	c.NoError(err)

	// A square SVG drawn into a wide rect must be scaled uniformly to the rect height and centered horizontally.
	cv, pix := newPixmapCanvas(64, 24)
	rect := geom.NewRect(0, 0, 60, 20)
	svg.DrawInRectPreservingAspectRatio(cv, rect, nil, Black.Paint(cv, rect, paintstyle.Fill))
	checkInkBounds(c, pix, geom.NewRect(20, 0, 20, 20))
}
