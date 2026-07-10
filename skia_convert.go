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
	skgeom "github.com/richardwilkes/canvas/geom"
	"github.com/richardwilkes/canvas/skcolor"
	"github.com/richardwilkes/toolbox/v2/geom"
)

// The canvas module ships its own geometry package that is independent of toolbox's geom. The two are not
// structurally compatible (canvas uses left/top/right/bottom rectangles and a 3x3 matrix with unexported
// fields, while toolbox uses x/y/width/height rectangles and a 6-element affine matrix), so every geometry
// value crossing the skcapi boundary is converted here.

func toSkPoint(p geom.Point) skgeom.Point { return skgeom.Point{X: p.X, Y: p.Y} }

func fromSkPoint(p skgeom.Point) geom.Point { return geom.NewPoint(p.X, p.Y) }

func toSkPoints(pts []geom.Point) []skgeom.Point {
	out := make([]skgeom.Point, len(pts))
	for i, p := range pts {
		out[i] = skgeom.Point{X: p.X, Y: p.Y}
	}
	return out
}

func toSkRect(r geom.Rect) skgeom.Rect {
	return skgeom.Rect{Left: r.X, Top: r.Y, Right: r.Right(), Bottom: r.Bottom()}
}

func fromSkRect(r skgeom.Rect) geom.Rect {
	return geom.NewRect(r.Left, r.Top, r.Right-r.Left, r.Bottom-r.Top)
}

func toSkRectPtr(r *geom.Rect) *skgeom.Rect {
	if r == nil {
		return nil
	}
	sr := toSkRect(*r)
	return &sr
}

func toSkIRect(r geom.Rect) skgeom.IRect {
	r = r.Align()
	return skgeom.IRect{Left: int32(r.X), Top: int32(r.Y), Right: int32(r.Right()), Bottom: int32(r.Bottom())}
}

func toSkMatrix(m geom.Matrix) skgeom.Matrix {
	return skgeom.MatrixFrom9([9]float32{m.ScaleX, m.SkewX, m.TransX, m.SkewY, m.ScaleY, m.TransY, 0, 0, 1})
}

func toSkMatrixPtr(m geom.Matrix) *skgeom.Matrix {
	sm := toSkMatrix(m)
	return &sm
}

func toSkColors(colors []Color) []skcolor.Color {
	out := make([]skcolor.Color, len(colors))
	for i, c := range colors {
		out[i] = skcolor.Color(c)
	}
	return out
}

func fromSkMatrix(m skgeom.Matrix) geom.Matrix {
	return geom.Matrix{
		ScaleX: m.Get(0),
		SkewX:  m.Get(1),
		TransX: m.Get(2),
		SkewY:  m.Get(3),
		ScaleY: m.Get(4),
		TransY: m.Get(5),
	}
}
