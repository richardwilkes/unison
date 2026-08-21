// Copyright (c) 2026 by Richard A. Wilkes. All rights reserved.
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
	"unsafe"

	"github.com/richardwilkes/canvas/colorcore"
	canvasgeom "github.com/richardwilkes/canvas/geom"
	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
)

// copyToCanvasPoints is the element-wise conversion toCanvasPoints replaced with an unsafe reinterpret, kept here as an
// independent statement of what the conversion must produce.
func copyToCanvasPoints(pts []geom.Point) []canvasgeom.Point {
	out := make([]canvasgeom.Point, len(pts))
	for i, p := range pts {
		out[i] = canvasgeom.Point{X: p.X, Y: p.Y}
	}
	return out
}

// copyToCanvasColors is the element-wise conversion toCanvasColors replaced with an unsafe reinterpret, kept here as an
// independent statement of what the conversion must produce.
func copyToCanvasColors(colors []Color) []colorcore.Color {
	out := make([]colorcore.Color, len(colors))
	for i, c := range colors {
		out[i] = colorcore.Color(c)
	}
	return out
}

// nextPseudoRandom advances a splitmix64 state and returns its output. It supplies the seeded test data below without
// depending on any generator whose stream could change, so the values these tests compare are fixed forever.
func nextPseudoRandom(state *uint64) uint64 {
	*state += 0x9E3779B97F4A7C15
	z := *state
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

// pseudoRandomPoints returns count deterministic points, spanning both signs and very different magnitudes in X and Y,
// so a reinterpret that swapped the fields or misjudged their size could not survive by accident.
func pseudoRandomPoints(state *uint64, count int) []geom.Point {
	pts := make([]geom.Point, count)
	for i := range pts {
		v := nextPseudoRandom(state)
		pts[i] = geom.NewPoint(float32(int32(v>>32))/1024, float32(int32(uint32(v)))/65536)
	}
	return pts
}

// TestToCanvasPointsMatchesCopy locks the reinterpreting toCanvasPoints against the element-wise copy it replaced: the
// view must present exactly the same values, for every length from one through a length that spans several cache
// lines. The empty case, where the two deliberately differ (nil rather than an empty slice), is covered separately by
// TestToCanvasSlicesOfNilAndEmpty.
func TestToCanvasPointsMatchesCopy(t *testing.T) {
	c := check.New(t)

	state := uint64(0x5eed_ca4a)
	for count := 1; count < 65; count++ {
		pts := pseudoRandomPoints(&state, count)
		c.Equal(copyToCanvasPoints(pts), toCanvasPoints(pts), "count=%d", count)
	}
}

// TestToCanvasColorsMatchesCopy locks the reinterpreting toCanvasColors against the element-wise copy it replaced.
func TestToCanvasColorsMatchesCopy(t *testing.T) {
	c := check.New(t)

	state := uint64(0xc0107_5eed)
	for count := 1; count < 65; count++ {
		colors := make([]Color, count)
		for i := range colors {
			colors[i] = Color(uint32(nextPseudoRandom(&state)))
		}
		c.Equal(copyToCanvasColors(colors), toCanvasColors(colors), "count=%d", count)
	}
}

// TestToCanvasSlicesOfNilAndEmpty verifies the empty-input guard the reinterprets need: there is no element 0 whose
// address could be taken, so both conversions must return nil rather than indexing. The canvas side treats a nil and an
// empty slice alike (Canvas.DrawPoints and Path.AddPoly return early, and validGradient rejects both), so returning nil
// instead of the empty non-nil slice the copies used to produce changes nothing for callers.
func TestToCanvasSlicesOfNilAndEmpty(t *testing.T) {
	c := check.New(t)

	c.Nil(toCanvasPoints(nil), "a nil point slice should convert to nil")
	c.Nil(toCanvasPoints([]geom.Point{}), "an empty point slice should convert to nil")
	c.Nil(toCanvasColors(nil), "a nil color slice should convert to nil")
	c.Nil(toCanvasColors([]Color{}), "an empty color slice should convert to nil")

	// A slice with the elements sliced away still has a backing array, and must be treated as empty just the same.
	state := uint64(0x0e_0e_0e)
	pts := pseudoRandomPoints(&state, 4)
	c.Nil(toCanvasPoints(pts[:0]), "a zero-length slice with a backing array should convert to nil")
	colors := []Color{Black, White}
	c.Nil(toCanvasColors(colors[:0]), "a zero-length slice with a backing array should convert to nil")
}

// TestToCanvasSlicesAliasSource documents the aliasing contract these conversions carry: the result is a view of the
// caller's backing array, not a copy, so a write through the source is visible through the view (and vice versa). Every
// canvas entry point unison hands the result to has been audited to neither retain nor mutate it — see the audit in
// canvas_convert.go, which must be repeated when the canvas dependency version changes. If that audit ever fails, this
// test is what explains why the conversion has to go back to copying.
func TestToCanvasSlicesAliasSource(t *testing.T) {
	c := check.New(t)

	state := uint64(0xa11a_5ed)
	pts := pseudoRandomPoints(&state, 8)
	view := toCanvasPoints(pts)
	c.Equal(len(pts), len(view))
	c.True(unsafe.Pointer(&view[0]) == unsafe.Pointer(&pts[0]), "the view must share the source's backing array")
	pts[3] = geom.NewPoint(12.5, -34.75)
	c.Equal(canvasgeom.Point{X: 12.5, Y: -34.75}, view[3], "a write through the source must be visible in the view")

	colors := []Color{Black, White, RGB(1, 2, 3)}
	colorView := toCanvasColors(colors)
	c.Equal(len(colors), len(colorView))
	c.True(unsafe.Pointer(&colorView[0]) == unsafe.Pointer(&colors[0]),
		"the view must share the source's backing array")
	colors[1] = RGB(4, 5, 6)
	c.Equal(colorcore.Color(RGB(4, 5, 6)), colorView[1], "a write through the source must be visible in the view")
}
