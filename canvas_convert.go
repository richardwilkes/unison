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
	"github.com/richardwilkes/canvas/colorcore"
	canvasgeom "github.com/richardwilkes/canvas/geom"
	"github.com/richardwilkes/toolbox/v2/geom"
)

// The canvas module ships its own geometry package that is independent of toolbox's geom. The two are not structurally
// compatible (canvas uses left/top/right/bottom rectangles and a 3x3 matrix with unexported fields, while toolbox uses
// x/y/width/height rectangles and a 6-element affine matrix), so every geometry value crossing the canvas↔toolbox geom
// boundary is converted here.

func toCanvasPoint(p geom.Point) canvasgeom.Point { return canvasgeom.Point{X: p.X, Y: p.Y} }

func fromCanvasPoint(p canvasgeom.Point) geom.Point { return geom.NewPoint(p.X, p.Y) }

func toCanvasPoints(pts []geom.Point) []canvasgeom.Point {
	out := make([]canvasgeom.Point, len(pts))
	for i, p := range pts {
		out[i] = canvasgeom.Point{X: p.X, Y: p.Y}
	}
	return out
}

func toCanvasRect(r geom.Rect) canvasgeom.Rect {
	return canvasgeom.Rect{Left: r.X, Top: r.Y, Right: r.Right(), Bottom: r.Bottom()}
}

func fromCanvasRect(r canvasgeom.Rect) geom.Rect {
	return geom.NewRect(r.Left, r.Top, r.Right-r.Left, r.Bottom-r.Top)
}

func toCanvasRectPtr(r *geom.Rect) *canvasgeom.Rect {
	if r == nil {
		return nil
	}
	sr := toCanvasRect(*r)
	return &sr
}

func toCanvasIRect(r geom.Rect) canvasgeom.IRect {
	r = r.Align()
	return canvasgeom.IRect{Left: int32(r.X), Top: int32(r.Y), Right: int32(r.Right()), Bottom: int32(r.Bottom())}
}

func toCanvasMatrix(m geom.Matrix) canvasgeom.Matrix {
	return canvasgeom.MatrixFrom9([9]float32{m.ScaleX, m.SkewX, m.TransX, m.SkewY, m.ScaleY, m.TransY, 0, 0, 1})
}

func toCanvasMatrixPtr(m geom.Matrix) *canvasgeom.Matrix {
	sm := toCanvasMatrix(m)
	return &sm
}

func toCanvasColors(colors []Color) []colorcore.Color {
	out := make([]colorcore.Color, len(colors))
	for i, c := range colors {
		out[i] = colorcore.Color(c)
	}
	return out
}

func fromCanvasMatrix(m canvasgeom.Matrix) geom.Matrix {
	return geom.Matrix{
		ScaleX: m.Get(0),
		SkewX:  m.Get(1),
		TransX: m.Get(2),
		SkewY:  m.Get(3),
		ScaleY: m.Get(4),
		TransY: m.Get(5),
	}
}
