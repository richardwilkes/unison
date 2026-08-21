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
	"unsafe"

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

// The two slice conversions, toCanvasPoints and toCanvasColors, hand the canvas module a view of the caller's backing
// array instead of a copy. Two things make that sound, and both must keep holding:
//
//  1. The element types are byte-for-byte identical. The zero-width arrays and typed blank declarations below pin
//     that: a size, offset, or field-type divergence in either module makes an array length go negative or an
//     assignment illegal, so the package stops compiling rather than silently corrupting memory. Sizes and offsets
//     alone are not enough, since a float32 and a uint32 are both 4 bytes at the same offset, hence the field-type
//     pins as well.
//
//  2. Nothing on the canvas side retains a view past the call or writes through it. Audited against canvas v0.3.0,
//     which is every path a converted slice can reach:
//
//     - Canvas.DrawPoints (canvas.go) → canvas/canvas.go:645, which only reads the points (geom.Rect.SetBounds, and
//     only when the paint carries a mask or image filter) before handing them to the top device. All three Device
//     implementations read only and keep nothing: canvas/device.go:316 → canvas/draw.go:940 maps the points into a
//     32-point stack buffer per batch, gpu/gl/device.go:303 reads coordinates per point into borrowed paths, and
//     pdf/device.go:265 emits coordinates into its content stream.
//
//     - Path.Poly (path.go) → path/shapes.go:248 AddPoly, which appends the point values into the path's own
//     storage.
//
//     - The gradient shader constructors (shader.go) → shaders/{linear,radial,sweep,conical}.go, each of which
//     immediately converts the colors with shaders/gradient.go:34 colors4f into a freshly allocated slice, and
//     shaders/gradient.go:69 initGradientBase then copies those float colors into its own storage. The byte-color
//     slice is only read, never stored.
//
// This audit must be repeated whenever the canvas dependency version changes: a callee that starts retaining or
// mutating its input would turn these views into aliasing bugs that no compile-time pin can catch.
var (
	_ [unsafe.Sizeof(geom.Point{}) - unsafe.Sizeof(canvasgeom.Point{})]byte
	_ [unsafe.Sizeof(canvasgeom.Point{}) - unsafe.Sizeof(geom.Point{})]byte
	_ [unsafe.Offsetof(geom.Point{}.X) - unsafe.Offsetof(canvasgeom.Point{}.X)]byte
	_ [unsafe.Offsetof(canvasgeom.Point{}.X) - unsafe.Offsetof(geom.Point{}.X)]byte
	_ [unsafe.Offsetof(geom.Point{}.Y) - unsafe.Offsetof(canvasgeom.Point{}.Y)]byte
	_ [unsafe.Offsetof(canvasgeom.Point{}.Y) - unsafe.Offsetof(geom.Point{}.Y)]byte
	// Assignment without a conversion compiles only while the field's type is exactly float32.
	_ float32 = geom.Point{}.X
	_ float32 = geom.Point{}.Y
	_ float32 = canvasgeom.Point{}.X
	_ float32 = canvasgeom.Point{}.Y

	_ [unsafe.Sizeof(Color(0)) - unsafe.Sizeof(colorcore.Color(0))]byte
	_ [unsafe.Sizeof(colorcore.Color(0)) - unsafe.Sizeof(Color(0))]byte
	// Instantiation compiles only while the type's underlying type is exactly uint32.
	_ = pinUint32[Color]
	_ = pinUint32[colorcore.Color]
)

// pinUint32 exists only to be instantiated: its constraint accepts a type whose underlying type is exactly uint32 and
// nothing else.
func pinUint32[T ~uint32]() {}

// toCanvasPoints returns pts viewed as canvas points, without copying. The result aliases pts, so it may only be
// passed to canvas entry points that neither retain nor mutate it — see the aliasing contract and audit above.
func toCanvasPoints(pts []geom.Point) []canvasgeom.Point {
	if len(pts) == 0 {
		return nil
	}
	return unsafe.Slice((*canvasgeom.Point)(unsafe.Pointer(&pts[0])), len(pts))
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

// toCanvasColors returns colors viewed as canvas colors, without copying. The result aliases colors, so it may only be
// passed to canvas entry points that neither retain nor mutate it — see the aliasing contract and audit above.
func toCanvasColors(colors []Color) []colorcore.Color {
	if len(colors) == 0 {
		return nil
	}
	return unsafe.Slice((*colorcore.Color)(unsafe.Pointer(&colors[0])), len(colors))
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
