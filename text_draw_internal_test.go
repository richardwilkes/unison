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

	"github.com/richardwilkes/canvas/canvas"
	"github.com/richardwilkes/canvas/raster"
	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xmath"
)

// newPixmapCanvas returns a CPU-backed Canvas suitable for headless draw tests, plus the pixmap it renders into.
func newPixmapCanvas(width, height int32) (*Canvas, *raster.Pixmap) {
	pix := raster.NewPixmap(width, height)
	return &Canvas{canvas: canvas.NewForPixmap(pix)}, pix
}

// inkRowBounds returns the topmost and bottommost rows containing any non-transparent pixel within the column range
// [minX, maxX), or (-1, -1) if the region is empty.
func inkRowBounds(pix *raster.Pixmap, minX, maxX int32) (top, bottom int32) {
	top = -1
	bottom = -1
	for y := int32(0); y < pix.Height; y++ {
		for x := minX; x < maxX; x++ {
			if pix.Pix[int(y)*int(pix.RowPixels)+int(x)]>>24 != 0 {
				if top == -1 {
					top = y
				}
				bottom = y
				break
			}
		}
	}
	return top, bottom
}

// nearlyEqual checks that two float32 values are equal within a tiny tolerance, to allow for the reordering of
// floating-point operations in the metrics computations.
func nearlyEqual(c check.Checker, expected, actual float32) {
	c.True(xmath.Abs(expected-actual) < 0.001, "expected %v, got %v", expected, actual)
}

func TestTextDrawBaselineOffsetAppliedOnce(t *testing.T) {
	c := check.New(t)
	const size = int32(64)
	baseline := geom.NewPoint(4, 32)

	ref, refPix := newPixmapCanvas(size, size)
	NewText("H", &TextDecoration{Font: SystemFont, OnBackgroundInk: Black}).Draw(ref, baseline)
	refTop, _ := inkRowBounds(refPix, 0, size)
	c.NotEqual(int32(-1), refTop)

	offset, offsetPix := newPixmapCanvas(size, size)
	NewText("H", &TextDecoration{Font: SystemFont, OnBackgroundInk: Black, BaselineOffset: 8}).Draw(offset, baseline)
	offsetTop, _ := inkRowBounds(offsetPix, 0, size)
	c.NotEqual(int32(-1), offsetTop)

	// A BaselineOffset of 8 must shift the text down by exactly 8 pixels, not 16.
	c.Equal(refTop+8, offsetTop)
}

func TestTextDrawSecondRunUnaffectedByFirstRunBaselineOffset(t *testing.T) {
	c := check.New(t)
	const width = int32(96)
	const height = int32(64)
	baseline := geom.NewPoint(4, 32)

	ref, refPix := newPixmapCanvas(width, height)
	NewText("H", &TextDecoration{Font: SystemFont, OnBackgroundInk: Black}).Draw(ref, baseline)
	refTop, _ := inkRowBounds(refPix, 0, width)
	c.NotEqual(int32(-1), refTop)

	txt := NewText("H", &TextDecoration{Font: SystemFont, OnBackgroundInk: Black, BaselineOffset: -8})
	txt.AddRunes([]rune("H"), &TextDecoration{Font: SystemFont, OnBackgroundInk: Black})
	multi, multiPix := newPixmapCanvas(width, height)
	txt.Draw(multi, baseline)
	split := int32(xmath.Ceil(baseline.X + txt.widths[0]))

	// The first run must be shifted up by exactly its own offset of 8 pixels, not 16.
	firstTop, _ := inkRowBounds(multiPix, 0, split)
	c.Equal(refTop-8, firstTop)

	// The second run has no offset, so it must sit on the baseline rather than inherit the first run's offset.
	secondTop, _ := inkRowBounds(multiPix, split, width)
	c.Equal(refTop, secondTop)
}

func TestTextMetricsWithBaselineOffsets(t *testing.T) {
	c := check.New(t)
	lineHeight := SystemFont.LineHeight()
	fontBaseline := SystemFont.Baseline()
	plainDec := &TextDecoration{Font: SystemFont, OnBackgroundInk: Black}
	supDec := &TextDecoration{Font: SystemFont, OnBackgroundInk: Black, BaselineOffset: -8}
	subDec := &TextDecoration{Font: SystemFont, OnBackgroundInk: Black, BaselineOffset: 8}

	plain := NewText("H", plainDec)
	nearlyEqual(c, lineHeight, plain.Height())
	nearlyEqual(c, fontBaseline, plain.Baseline())

	// A superscript reserves its excursion above the line, moving the baseline down within the extents.
	sup := NewText("H", supDec)
	nearlyEqual(c, lineHeight+8, sup.Height())
	nearlyEqual(c, fontBaseline+8, sup.Baseline())

	// A subscript reserves its excursion below the line, leaving the baseline alone.
	sub := NewText("H", subDec)
	nearlyEqual(c, lineHeight+8, sub.Height())
	nearlyEqual(c, fontBaseline, sub.Baseline())

	// Superscript and subscript runs in the same text must reserve both excursions, not just the larger one.
	mixed := NewText("H", supDec)
	mixed.AddString("H", subDec)
	nearlyEqual(c, lineHeight+16, mixed.Height())
	nearlyEqual(c, fontBaseline+8, mixed.Baseline())

	// An empty text reserves the same space the creation decoration would need with content.
	empty := NewText("", supDec)
	nearlyEqual(c, lineHeight+8, empty.Height())
	nearlyEqual(c, fontBaseline+8, empty.Baseline())

	// An empty slice keeps the metrics of the text it came from.
	nearlyEqual(c, plain.Height(), plain.Slice(0, 0).Height())
}

func TestTextDrawStaysWithinExtents(t *testing.T) {
	c := check.New(t)
	const width = int32(96)
	const height = int32(64)

	txt := NewText("H", &TextDecoration{Font: SystemFont, OnBackgroundInk: Black, BaselineOffset: -8})
	txt.AddString("H", &TextDecoration{Font: SystemFont, OnBackgroundInk: Black, BaselineOffset: 8})
	boxTop := float32(8)
	cv, pix := newPixmapCanvas(width, height)
	txt.Draw(cv, geom.NewPoint(4, boxTop+txt.Baseline()))

	inkTop, inkBottom := inkRowBounds(pix, 0, width)
	c.NotEqual(int32(-1), inkTop)
	c.True(inkTop >= int32(xmath.Floor(boxTop)), "ink top %d above box top %v", inkTop, boxTop)
	c.True(inkBottom <= int32(xmath.Ceil(boxTop+txt.Height())), "ink bottom %d below box bottom %v", inkBottom,
		boxTop+txt.Height())
}
