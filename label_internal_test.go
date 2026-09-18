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
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/side"
)

// axLabelTestDrawable stands in for an icon: it takes up room and paints nothing, so what is left on the canvas after a
// label holding one has been drawn is the label's text alone.
type axLabelTestDrawable struct {
	size geom.Size
}

func (d *axLabelTestDrawable) LogicalSize() geom.Size {
	return d.size
}

func (d *axLabelTestDrawable) DrawInRect(_ *Canvas, _ geom.Rect, _ *SamplingOptions, _ *Paint) {
}

// axInkBounds returns the smallest rectangle of pixel indexes holding every non-transparent pixel of a pixmap, or a
// rectangle with a negative origin when nothing was drawn at all.
func axInkBounds(pix *raster.Pixmap) (minX, minY, maxX, maxY int32) {
	minX, minY, maxX, maxY = -1, -1, -1, -1
	for y := int32(0); y < pix.Height; y++ {
		for x := int32(0); x < pix.Width; x++ {
			if pix.Pix[int(y)*int(pix.RowPixels)+int(x)]>>24 == 0 {
				continue
			}
			if minX == -1 || x < minX {
				minX = x
			}
			if minY == -1 || y < minY {
				minY = y
			}
			maxX = max(maxX, x)
			maxY = max(maxY, y)
		}
	}
	return minX, minY, maxX, maxY
}

// TestLabelTextOriginMatchesDrawLabel checks that where a label says its text is, for an assistive technology, is where
// the label actually drew it: the text is drawn once by DrawLabel and once on its own at Label.axTextOrigin, and the
// two canvases must hold the same pixels. Both go through labelPlacement, so what this pins is that the origin the
// accessibility side hands out is the origin the drawing used, including the flooring the alignments do and the room a
// drawable takes on each of the four sides. A document composed of labels reports each run of its text at that origin,
// and an origin off by the height of a line would put a screen reader's highlight, and a person's reading caret, on the
// wrong words.
func TestLabelTextOriginMatchesDrawLabel(t *testing.T) {
	c := check.New(t)
	const extent = int32(140)
	for _, one := range []struct {
		name     string
		hAlign   align.Enum
		vAlign   align.Enum
		side     side.Enum
		drawable geom.Size
	}{
		{name: "start/start", hAlign: align.Start, vAlign: align.Start},
		{name: "middle/middle", hAlign: align.Middle, vAlign: align.Middle},
		{name: "end/end", hAlign: align.End, vAlign: align.End},
		{name: "fill/fill", hAlign: align.Fill, vAlign: align.Fill},
		{
			name: "drawable on the left", hAlign: align.Middle, vAlign: align.Middle, side: side.Left,
			drawable: geom.NewSize(24, 24),
		},
		{
			name: "drawable on the right", hAlign: align.Start, vAlign: align.Middle, side: side.Right,
			drawable: geom.NewSize(24, 24),
		},
		{
			name: "drawable on top", hAlign: align.Middle, vAlign: align.Start, side: side.Top,
			drawable: geom.NewSize(30, 18),
		},
		{
			name: "drawable below", hAlign: align.End, vAlign: align.End, side: side.Bottom,
			drawable: geom.NewSize(30, 18),
		},
		{
			name: "a drawable narrower than the text", hAlign: align.Middle, vAlign: align.Middle, side: side.Top,
			drawable: geom.NewSize(4, 4),
		},
	} {
		label := NewLabel()
		label.HAlign = one.hAlign
		label.VAlign = one.vAlign
		label.Side = one.side
		label.SetTitle("Hg jy")
		if one.drawable.Width > 0 {
			label.Drawable = &axLabelTestDrawable{size: one.drawable}
		}
		label.SetFrameRect(geom.NewRect(0, 0, float32(extent), float32(extent)))
		rect := label.ContentRect(false)

		drawn, drawnPix := newPixmapCanvas(extent, extent)
		DrawLabel(drawn, rect, label.HAlign, label.VAlign, label.Font, label.Text, label.OnBackgroundInk, nil,
			label.Drawable, label.Side, label.Gap, false)

		reported, reportedPix := newPixmapCanvas(extent, extent)
		origin := label.axTextOrigin()
		label.Text.Draw(reported, geom.NewPoint(origin.X, origin.Y+label.Text.Baseline()))

		minX, minY, maxX, maxY := axInkBounds(drawnPix)
		c.True(minX >= 0, "%s: the label drew nothing to compare", one.name)
		rMinX, rMinY, rMaxX, rMaxY := axInkBounds(reportedPix)
		c.Equal([]int32{minX, minY, maxX, maxY}, []int32{rMinX, rMinY, rMaxX, rMaxY},
			"%s: the text was drawn somewhere other than where axTextOrigin says", one.name)
		c.Equal(drawnPix.Pix, reportedPix.Pix, "%s: the two renderings must be identical", one.name)
	}
}

// TestLabelTextLineDescribesWhatWasDrawn covers the line a label contributes to a document's text: its runes, one
// advance per rune boundary, and bounds that sit where the text was drawn. It is what a list's bullet and an alert's
// title are folded into a document's stream as.
func TestLabelTextLineDescribesWhatWasDrawn(t *testing.T) {
	c := check.New(t)
	label := NewLabel()
	label.SetTitle("Hi there")
	label.SetFrameRect(geom.NewRect(0, 0, 200, 40))
	runes, decorations, line := label.axTextLine()
	c.Equal([]rune("Hi there"), runes)
	c.Equal(len(runes), len(decorations), "every rune must say what it was drawn with")
	for i, decoration := range decorations {
		c.True(decoration != nil, "rune %d has no decoration", i)
	}
	c.Equal(0, line.Start)
	c.Equal(len(runes), line.End)
	c.Equal(len(runes)+1, len(line.Advances), "the advances are the rune boundaries, so there is one more of them")
	c.Equal(float32(0), line.Advances[0])
	for i := 1; i < len(line.Advances); i++ {
		c.True(line.Advances[i] >= line.Advances[i-1], "the advances must not go backwards at %d", i)
	}
	nearlyEqual(c, label.Text.Width(), line.Advances[len(line.Advances)-1])
	origin := label.axTextOrigin()
	c.Equal(origin, line.Bounds.Point, "the line sits where the text was drawn")
	nearlyEqual(c, label.Text.Width(), line.Bounds.Width)
	nearlyEqual(c, label.Text.Height(), line.Bounds.Height)

	// A label holding no text still occupies a line, since it is still as tall as one and a caret can sit in it.
	empty := NewLabel()
	empty.SetTitle("")
	empty.SetFrameRect(geom.NewRect(0, 0, 200, 40))
	runes, decorations, line = empty.axTextLine()
	c.Equal(0, len(runes))
	c.Equal(0, len(decorations))
	c.Equal([]float32{0}, line.Advances, "one advance, which is where a caret in empty text sits")
	c.Equal(0, line.End)
	nearlyEqual(c, empty.Text.Height(), line.Bounds.Height)

	// That holds for a label showing nothing but an icon as well: what the drawable decides is how big the label is, not
	// how tall the line of text within it is, and a line with no height is not one a caret could sit in or a rectangle an
	// assistive technology could be told about.
	iconOnly := NewLabel()
	iconOnly.SetTitle("")
	iconOnly.Drawable = &axLabelTestDrawable{size: geom.NewSize(24, 18)}
	iconOnly.SetFrameRect(geom.NewRect(0, 0, 200, 40))
	runes, decorations, line = iconOnly.axTextLine()
	c.Equal(0, len(runes))
	c.Equal(0, len(decorations))
	c.Equal([]float32{0}, line.Advances, "one advance, which is where a caret in empty text sits")
	c.Equal(0, line.End)
	c.True(line.Bounds.Height > 0, "an icon-only label's line is as tall as a line of its font")
	nearlyEqual(c, iconOnly.Text.Height(), line.Bounds.Height)

	// The size the label lays out at is untouched by that: an icon-only label is exactly as big as its icon, which is
	// what every widget built on LabelContentSizes measures itself by.
	size, txtSize := LabelContentSizes(iconOnly.Text, iconOnly.Drawable, iconOnly.Font, iconOnly.Side, iconOnly.Gap)
	c.Equal(geom.NewSize(24, 18), size, "the label is still exactly as big as the icon it shows")
	c.Equal(float32(0), txtSize.Width, "and its text is still as wide as the nothing it holds")
	nearlyEqual(c, iconOnly.Text.Height(), txtSize.Height)
}
