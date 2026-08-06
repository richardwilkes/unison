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
	"github.com/richardwilkes/unison/enums/gradienttype"
	"github.com/richardwilkes/unison/enums/tilemode"
)

// svgTestPixel is an exact, unpremultiplied pixel value read back from a rasterized SVG.
type svgTestPixel struct {
	r, g, b, a uint8
}

var (
	svgTestBlack = svgTestPixel{a: 255}
	svgTestWhite = svgTestPixel{r: 255, g: 255, b: 255, a: 255}
	svgTestRed   = svgTestPixel{r: 255, a: 255}
	svgTestGreen = svgTestPixel{g: 128, a: 255}
)

// svgTestReplacements maps the ink-substitution convention's source colors onto two colors that share no channel value
// with either the originals or with each other, so an anti-aliased blend of the replacements can never accidentally
// reproduce a pure black or pure white pixel.
var svgTestReplacements = map[Color]Ink{Black: Red, White: Green}

// svgRasterizePixels rasterizes the SVG into a square image of the given size entirely on the CPU and returns the set
// of exact pixel values it contains. Working with the set of values present, rather than counts or positions, keeps the
// assertions insensitive to anti-aliasing.
func svgRasterizePixels(c check.Checker, svg *SVG, size int, replacements map[Color]Ink) map[svgTestPixel]bool {
	img, err := NewImageFromDrawing(size, size, 72, func(canvas *Canvas) {
		svg.DrawInRectWithReplacementInks(canvas, geom.NewRect(0, 0, float32(size), float32(size)), nil, replacements)
	})
	c.NoError(err)
	c.NotNil(img)
	defer img.Dispose()
	nrgba, err := img.ToNRGBA()
	c.NoError(err)
	present := make(map[svgTestPixel]bool)
	for y := range size {
		row := nrgba.Pix[y*nrgba.Stride : y*nrgba.Stride+size*4]
		for x := range size {
			present[svgTestPixel{r: row[x*4], g: row[x*4+1], b: row[x*4+2], a: row[x*4+3]}] = true
		}
	}
	return present
}

// svgReplacementTestContent is a single path carrying a black fill and a white stroke, so that the independent lookup
// of the two inks can be observed on one path. The stroke is wide enough that both it and the remaining interior fill
// contain fully covered pixels at the size used below.
const svgReplacementTestContent = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64">` +
	`<path fill="#000" stroke="#fff" stroke-width="16" d="M16 16h32v32H16z"/></svg>`

func TestSVGReplaceInk(t *testing.T) {
	c := check.New(t)

	c.Nil(svgReplaceInk(nil, svgTestReplacements), "a nil ink has nothing to replace")
	c.Equal(Ink(Black), svgReplaceInk(Black, nil), "a nil map replaces nothing")
	c.Equal(Ink(Black), svgReplaceInk(Black, map[Color]Ink{}), "an empty map replaces nothing")
	c.Equal(Ink(Red), svgReplaceInk(Black, svgTestReplacements), "a matching color is replaced")
	c.Equal(Ink(Green), svgReplaceInk(White, svgTestReplacements), "each color is looked up independently")
	c.Equal(Ink(Blue), svgReplaceInk(Blue, svgTestReplacements), "an unlisted color is left alone")

	// Inks that aren't plain colors, such as gradients, can't be map keys and must pass through untouched.
	gradient := &Gradient{
		Stops:    NewEvenlySpacedGradientStopsForColors(Black, White),
		EndPt:    geom.NewPoint(1, 1),
		Kind:     gradienttype.Linear,
		TileMode: tilemode.Clamp,
	}
	c.Equal(Ink(gradient), svgReplaceInk(gradient, svgTestReplacements), "a gradient is not a plain color")
}

func TestSVGDrawInRectWithReplacementInks(t *testing.T) {
	c := check.New(t)
	svg, err := NewSVGFromContentString(svgReplacementTestContent)
	c.NoError(err)

	const size = 64

	// Control: with no replacements, the source colors must appear untouched.
	present := svgRasterizePixels(c, svg, size, nil)
	c.True(present[svgTestBlack], "the unreplaced fill must produce pure black pixels")
	c.True(present[svgTestWhite], "the unreplaced stroke must produce pure white pixels")

	// With replacements, both inks are substituted and neither original survives. Anti-aliased edges blend the
	// replacement colors, so they can never reproduce the originals either.
	present = svgRasterizePixels(c, svg, size, svgTestReplacements)
	c.True(present[svgTestRed], "the black fill must be replaced by red")
	c.True(present[svgTestGreen], "the white stroke must be replaced by green")
	c.False(present[svgTestBlack], "no pure black pixel may survive the replacement")
	c.False(present[svgTestWhite], "no pure white pixel may survive the replacement")
}

// TestSVGCursorAssetsFollowInkConvention verifies that every cursor asset is drawn purely with pure black and pure
// white, the two colors the cursor foreground and background substitution relies on. Anything painted with some other
// color, or with an implicit fill that the convention doesn't cover, would leave pure black or pure white pixels behind
// after substituting both, or would fail to produce the substituted colors at all.
func TestSVGCursorAssetsFollowInkConvention(t *testing.T) {
	for name, svg := range map[string]*SVG{
		"arrow":                 CursorArrowSVG,
		"hand_closed":           CursorHandClosedSVG,
		"hand_open":             CursorHandOpenSVG,
		"hand_pointing":         CursorHandPointingSVG,
		"move":                  CursorMoveSVG,
		"resize_horizontal":     CursorResizeHorizontalSVG,
		"resize_left_diagonal":  CursorResizeLeftDiagonalSVG,
		"resize_right_diagonal": CursorResizeRightDiagonalSVG,
		"resize_vertical":       CursorResizeVerticalSVG,
		"text":                  CursorTextSVG,
	} {
		t.Run(name, func(t *testing.T) {
			c := check.New(t)

			// Rasterize at the assets' native viewBox size so that even the thinnest stroke has fully covered pixels.
			const size = 256

			present := svgRasterizePixels(c, svg, size, nil)
			c.True(present[svgTestBlack], "the foreground must be drawn with pure black")
			c.True(present[svgTestWhite], "the background must be drawn with pure white")

			present = svgRasterizePixels(c, svg, size, svgTestReplacements)
			c.True(present[svgTestRed], "the foreground must pick up the black replacement")
			c.True(present[svgTestGreen], "the background must pick up the white replacement")
			c.False(present[svgTestBlack], "no pure black pixel may survive the replacement")
			c.False(present[svgTestWhite], "no pure white pixel may survive the replacement")
		})
	}
}
