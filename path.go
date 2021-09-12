// Copyright ©2021 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"runtime"

	"github.com/richardwilkes/toolbox/errs"
	"github.com/richardwilkes/toolbox/xmath/geom32"
	"github.com/richardwilkes/toolbox/xmath/geom32/poly32"
	"github.com/richardwilkes/unison/internal/skia"
)

// ArcSize holds the relative size of an arc.
type ArcSize byte

// Possible values for ArcSize.
const (
	SmallArcSize ArcSize = iota
	LargeArcSize
)

// Direction holds the direction of a path.
type Direction byte

// Possible values for Direction.
const (
	Clockwise Direction = iota
	CounterClockwise
)

// FillType holds the type of fill operation to perform, which affects how overlapping contours interact with each
// other.
type FillType byte

// Possible values for FillType.
const (
	Winding FillType = iota
	EvenOdd
	InverseWinding
	InverseEvenOdd
)

// PathOp holds the possible operations that can be performed on a pair of paths.
type PathOp byte

// Possible values for PathOp.
const (
	Difference PathOp = iota
	Intersect
	Union
	Xor
	ReverseDifference
)

// PathOpPair holds the combination of a Path and a PathOp.
type PathOpPair struct {
	Path *Path
	Op   PathOp
}

// Path holds geometry.
type Path struct {
	path skia.Path
}

func newPath(path skia.Path) *Path {
	p := &Path{path: path}
	runtime.SetFinalizer(p, func(obj *Path) {
		ReleaseOnUIThread(func() {
			skia.PathDelete(obj.path)
		})
	})
	return p
}

// NewPath creates a new, empty path.
func NewPath() *Path {
	return newPath(skia.PathNew())
}

// NewPathFromSVGString attempts to create a path from the given SVG string.
func NewPathFromSVGString(svg string) (*Path, error) {
	p := NewPath()
	if !skia.PathParseSVGString(p.path, svg) {
		return nil, errs.New("unable to parse SVG string into path")
	}
	return p, nil
}

// ToSVGString returns an SVG string that represents this path.
func (p *Path) ToSVGString() string {
	ss := skia.StringNewEmpty()
	defer skia.StringDelete(ss)
	skia.PathToSVGString(p.path, ss)
	return skia.StringGetString(ss)
}

// FillType returns the FillType for this path.
func (p *Path) FillType() FillType {
	return FillType(skia.PathGetFillType(p.path))
}

// SetFillType sets the FillType for this path.
func (p *Path) SetFillType(fillType FillType) {
	skia.PathSetFillType(p.path, skia.FillType(fillType))
}

// ArcTo appends an arc. rotation is in degrees.
func (p *Path) ArcTo(x, y, rx, ry, rotation float32, arcSize ArcSize, direction Direction) {
	skia.PathArcTo(p.path, x, y, rx, ry, rotation, skia.ArcSize(arcSize), skia.Direction(direction))
}

// ArcToPt appends an arc. rotation is in degrees.
func (p *Path) ArcToPt(pt, radius geom32.Point, rotation float32, arcSize ArcSize, direction Direction) {
	skia.PathArcTo(p.path, radius.X, radius.Y, pt.X, pt.Y, rotation, skia.ArcSize(arcSize), skia.Direction(direction))
}

// ArcToFromTangent appends an arc. The arc is contained by the tangent from the current point to (x1, y1) and the
// tangent from (x1, y1) to (x2, y2). The arc is part of the circle sized to radius, positioned so it touches both
// tangent lines.
func (p *Path) ArcToFromTangent(x1, y1, x2, y2, radius float32) {
	skia.PathArcToWithPoints(p.path, x1, y1, x2, y2, radius)
}

// ArcToFromTangentPt appends an arc. The arc is contained by the tangent from the current point to pt1 and the tangent
// from pt1 to pt2. The arc is part of the circle sized to radius, positioned so it touches both tangent lines.
func (p *Path) ArcToFromTangentPt(pt1, pt2 geom32.Point, radius float32) {
	skia.PathArcToWithPoints(p.path, pt1.X, pt1.Y, pt2.X, pt2.Y, radius)
}

// ArcToRelative appends an arc. The destination point is relative to the current point. rotation is in degrees.
func (p *Path) ArcToRelative(dx, dy, rx, ry, rotation float32, arcSize ArcSize, direction Direction) {
	skia.PathRArcTo(p.path, dx, dy, rx, ry, rotation, skia.ArcSize(arcSize), skia.Direction(direction))
}

// ArcToPtRelative appends an arc. The destination point is relative to the current point. rotation is in degrees.
func (p *Path) ArcToPtRelative(dPt, radius geom32.Point, rotation float32, arcSize ArcSize, direction Direction) {
	skia.PathRArcTo(p.path, dPt.X, dPt.Y, radius.X, radius.Y, rotation, skia.ArcSize(arcSize), skia.Direction(direction))
}

// ArcToOval appends an arc bounded by an oval. Both startAngle and sweepAngle are in degrees. A positive sweepAngle
// extends clockwise while a negative value extends counter-clockwise. If forceMoveTo is true, a new contour is started.
func (p *Path) ArcToOval(x, y, width, height, startAngle, sweepAngle float32, forceMoveTo bool) {
	skia.PathArcToWithOval(p.path, skia.RectToSkRect(geom32.NewRectPtr(x, y, width, height)), startAngle, sweepAngle, forceMoveTo)
}

// ArcToOvalBounds appends an arc bounded by an oval. Both startAngle and sweepAngle are in degrees. A positive
// sweepAngle extends clockwise while a negative value extends counter-clockwise. If forceMoveTo is true, a new contour
// is started.
func (p *Path) ArcToOvalBounds(bounds geom32.Rect, startAngle, sweepAngle float32, forceMoveTo bool) {
	skia.PathArcToWithOval(p.path, skia.RectToSkRect(&bounds), startAngle, sweepAngle, forceMoveTo)
}

// Bounds returns the bounding rectangle of the path. This is an approximation and may be different than the actual area
// covered when drawn.
func (p *Path) Bounds() geom32.Rect {
	return skia.PathGetBounds(p.path).ToRect()
}

// ComputeTightBounds returns the bounding rectangle of the path. This is an approximation and may be different than the
// actual area covered when drawn. When a path contains only lines, this method is functionally equivalent a call to
// Bounds(), though slower. When a path contains curves, the computed bounds includes the maximum extent of the quad,
// conic, or cubic.
func (p *Path) ComputeTightBounds() geom32.Rect {
	return skia.PathComputeTightBounds(p.path).ToRect()
}

// Circle adds a circle to the path with a clockwise direction. The circle is a complete contour, i.e. it starts with a
// MoveTo and ends with a Close operation.
func (p *Path) Circle(x, y, radius float32) {
	skia.PathAddCircle(p.path, x, y, radius, skia.Direction(Clockwise))
}

// CirclePt adds a circle to the path with a clockwise direction. The circle is a complete contour, i.e. it starts with
// a MoveTo and ends with a Close operation.
func (p *Path) CirclePt(centerPt geom32.Point, radius float32) {
	skia.PathAddCircle(p.path, centerPt.X, centerPt.Y, radius, skia.Direction(Clockwise))
}

// CircleWithDirection adds a circle to the path. The circle is a complete contour, i.e. it starts with a MoveTo and
// ends with a Close operation.
func (p *Path) CircleWithDirection(x, y, radius float32, direction Direction) {
	skia.PathAddCircle(p.path, x, y, radius, skia.Direction(direction))
}

// CirclePtWithDirection adds a circle to the path. The circle is a complete contour, i.e. it starts with a MoveTo and
// ends with a Close operation.
func (p *Path) CirclePtWithDirection(centerPt geom32.Point, radius float32, direction Direction) {
	skia.PathAddCircle(p.path, centerPt.X, centerPt.Y, radius, skia.Direction(direction))
}

// Clone this path.
func (p *Path) Clone() *Path {
	return newPath(skia.PathClone(p.path))
}

// Close the current contour.
func (p *Path) Close() {
	skia.PathClose(p.path)
}

// ConicTo appends a conic curve.
func (p *Path) ConicTo(cpx, cpy, x, y, weight float32) {
	skia.PathConicTo(p.path, cpx, cpy, x, y, weight)
}

// ConicToPt appends a conic curve.
func (p *Path) ConicToPt(ctrlPt, endPt geom32.Point, weight float32) {
	skia.PathConicTo(p.path, ctrlPt.X, ctrlPt.Y, endPt.X, endPt.Y, weight)
}

// ConicToRelative appends a conic curve. The control point and end point are relative to the current point.
func (p *Path) ConicToRelative(cpdx, cpdy, dx, dy, weight float32) {
	skia.PathRConicTo(p.path, cpdx, cpdy, dx, dy, weight)
}

// ConicToPtRelative appends a conic curve. The control point and end point are relative to the current point.
func (p *Path) ConicToPtRelative(dCtrlPt, dEndPt geom32.Point, weight float32) {
	skia.PathRConicTo(p.path, dCtrlPt.X, dCtrlPt.Y, dEndPt.X, dEndPt.Y, weight)
}

// CubicTo appends a cubic curve.
func (p *Path) CubicTo(cp1x, cp1y, cp2x, cp2y, x, y float32) {
	skia.PathCubicTo(p.path, cp1x, cp1y, cp2x, cp2y, x, y)
}

// CubicToPt appends a cubic curve.
func (p *Path) CubicToPt(ctrl1Pt, ctrl2Pt, endPt geom32.Point) {
	skia.PathCubicTo(p.path, ctrl1Pt.X, ctrl1Pt.Y, ctrl2Pt.X, ctrl2Pt.Y, endPt.X, endPt.Y)
}

// CubicToRelative appends a cubic curve. The control point and end point are relative to the current point.
func (p *Path) CubicToRelative(cp1dx, cp1dy, cp2dx, cp2dy, dx, dy float32) {
	skia.PathRCubicTo(p.path, cp1dx, cp1dy, cp2dx, cp2dy, dx, dy)
}

// CubicToPtRelative appends a cubic curve. The control point and end point are relative to the current point.
func (p *Path) CubicToPtRelative(dCtrl1Pt, dCtrl2Pt, dEndPt geom32.Point) {
	skia.PathRCubicTo(p.path, dCtrl1Pt.X, dCtrl1Pt.Y, dCtrl2Pt.X, dCtrl2Pt.Y, dEndPt.X, dEndPt.Y)
}

// LineTo appends a straight line segment.
func (p *Path) LineTo(x, y float32) {
	skia.PathLineTo(p.path, x, y)
}

// LineToPt appends a straight line segment.
func (p *Path) LineToPt(pt geom32.Point) {
	skia.PathLineTo(p.path, pt.X, pt.Y)
}

// LineToRelative appends a straight line segment. The end point is relative to the current point.
func (p *Path) LineToRelative(x, y float32) {
	skia.PathRLineTo(p.path, x, y)
}

// LineToPtRelative appends a straight line segment. The end point is relative to the current point.
func (p *Path) LineToPtRelative(pt geom32.Point) {
	skia.PathRLineTo(p.path, pt.X, pt.Y)
}

// MoveTo begins a new contour at the specified point.
func (p *Path) MoveTo(x, y float32) {
	skia.PathMoveTo(p.path, x, y)
}

// MoveToPt begins a new contour at the specified point.
func (p *Path) MoveToPt(pt geom32.Point) {
	skia.PathMoveTo(p.path, pt.X, pt.Y)
}

// MoveToRelative begins a new contour at the specified point, which is relative to the current point.
func (p *Path) MoveToRelative(x, y float32) {
	skia.PathRMoveTo(p.path, x, y)
}

// MoveToPtRelative begins a new contour at the specified point, which is relative to the current point.
func (p *Path) MoveToPtRelative(pt geom32.Point) {
	skia.PathRMoveTo(p.path, pt.X, pt.Y)
}

// Oval adds an oval to the path with a clockwise direction. The oval is a complete contour, i.e. it starts with a
// MoveTo and ends with a Close operation.
func (p *Path) Oval(x, y, width, height float32) {
	skia.PathAddOval(p.path, skia.RectToSkRect(geom32.NewRectPtr(x, y, width, height)), skia.Direction(Clockwise))
}

// OvalBounds adds an oval to the path with a clockwise direction. The oval is a complete contour, i.e. it starts with a
// MoveTo and ends with a Close operation.
func (p *Path) OvalBounds(bounds geom32.Rect) {
	skia.PathAddOval(p.path, skia.RectToSkRect(&bounds), skia.Direction(Clockwise))
}

// OvalWithDirection adds an oval to the path. The oval is a complete contour, i.e. it starts with a MoveTo and ends
// with a Close operation.
func (p *Path) OvalWithDirection(x, y, width, height float32, direction Direction) {
	skia.PathAddOval(p.path, skia.RectToSkRect(geom32.NewRectPtr(x, y, width, height)), skia.Direction(direction))
}

// OvalBoundsWithDirection adds an oval to the path. The oval is a complete contour, i.e. it starts with a MoveTo and
// ends with a Close operation.
func (p *Path) OvalBoundsWithDirection(bounds geom32.Rect, direction Direction) {
	skia.PathAddOval(p.path, skia.RectToSkRect(&bounds), skia.Direction(direction))
}

// Path appends a path. If extend is true, a line from the current point to the start of the added path is created.
func (p *Path) Path(path *Path, extend bool) {
	skia.PathAddPath(p.path, path.path, pathAddMode(extend))
}

// PathReverse appends a path in reverse order.
func (p *Path) PathReverse(path *Path) {
	skia.PathAddPathReverse(p.path, path.path)
}

// PathRotated appends a path after rotating it. If extend is true, a line from the current point to the start of the
// added path is created.
func (p *Path) PathRotated(path *Path, radians float32, extend bool) {
	skia.PathAddPathMatrix(p.path, path.path, skia.Matrix2DtoMatrix(geom32.NewRotationMatrix2D(radians)), pathAddMode(extend))
}

// PathRotatedByDegrees appends a path after rotating it. If extend is true, a line from the current point to the start
// of the added path is created.
func (p *Path) PathRotatedByDegrees(path *Path, degrees float32, extend bool) {
	skia.PathAddPathMatrix(p.path, path.path, skia.Matrix2DtoMatrix(geom32.NewRotationByDegreesMatrix2D(degrees)), pathAddMode(extend))
}

// PathScaled appends a path after scaling it on both the horizontal and vertical axis. If extend is true, a line from
// the current point to the start of the added path is created.
func (p *Path) PathScaled(path *Path, scale float32, extend bool) {
	skia.PathAddPathMatrix(p.path, path.path, skia.Matrix2DtoMatrix(geom32.NewScaleMatrix2D(scale, scale)), pathAddMode(extend))
}

// PathScaledIndependently appends a path after scaling it. If extend is true, a line from the current point to the
// start of the added path is created.
func (p *Path) PathScaledIndependently(path *Path, scale geom32.Point, extend bool) {
	skia.PathAddPathMatrix(p.path, path.path, skia.Matrix2DtoMatrix(geom32.NewScaleMatrix2D(scale.X, scale.Y)), pathAddMode(extend))
}

// PathTransformed appends a path after transforming it. If extend is true, a line from the current point to the start
// of the added path is created.
func (p *Path) PathTransformed(path *Path, matrix *geom32.Matrix2D, extend bool) {
	skia.PathAddPathMatrix(p.path, path.path, skia.Matrix2DtoMatrix(matrix), pathAddMode(extend))
}

// PathTranslated appends a path after translating it with the given offset. If extend is true, a line from the current
// point to the start of the added path is created.
func (p *Path) PathTranslated(path *Path, offsetX, offsetY float32, extend bool) {
	skia.PathAddPathOffset(p.path, path.path, offsetX, offsetY, pathAddMode(extend))
}

// PathTranslatedPt appends a path after translating it with the given offset. If extend is true, a line from the
// current point to the start of the added path is created.
func (p *Path) PathTranslatedPt(path *Path, offset geom32.Point, extend bool) {
	skia.PathAddPathOffset(p.path, path.path, offset.X, offset.Y, pathAddMode(extend))
}

// Poly appends the line segments represented by pts to the path.
func (p *Path) Poly(pts []geom32.Point, closePath bool) {
	if len(pts) > 0 {
		skia.PathAddPoly(p.path, pts, closePath)
	}
}

// Polygon appends the polygon to the path.
func (p *Path) Polygon(poly poly32.Polygon) {
	for _, contour := range poly {
		p.Poly(contour, true)
	}
}

// QuadTo appends a quadratic curve.
func (p *Path) QuadTo(cpx, cpy, x, y float32) {
	skia.PathQuadTo(p.path, cpx, cpy, x, y)
}

// QuadToPt appends a quadratic curve.
func (p *Path) QuadToPt(ctrlPt, endPt geom32.Point) {
	skia.PathQuadTo(p.path, ctrlPt.X, ctrlPt.Y, endPt.X, endPt.Y)
}

// Rect adds a rectangle to the path with a clockwise direction. The rectangle is a complete contour, i.e. it starts
// with a MoveTo and ends with a Close operation.
func (p *Path) Rect(x, y, width, height float32) {
	skia.PathAddRect(p.path, skia.RectToSkRect(geom32.NewRectPtr(x, y, width, height)), skia.Direction(Clockwise))
}

// RectBounds adds a rectangle to the path with a clockwise direction. The rectangle is a complete contour, i.e. it
// starts with a MoveTo and ends with a Close operation.
func (p *Path) RectBounds(bounds geom32.Rect) {
	skia.PathAddRect(p.path, skia.RectToSkRect(&bounds), skia.Direction(Clockwise))
}

// RectWithDirection adds a rectangle to the path. The rectangle is a complete contour, i.e. it starts with a MoveTo and
// ends with a Close operation.
func (p *Path) RectWithDirection(x, y, width, height float32, direction Direction) {
	skia.PathAddRect(p.path, skia.RectToSkRect(geom32.NewRectPtr(x, y, width, height)), skia.Direction(direction))
}

// RectBoundsWithDirection adds a rectangle to the path. The rectangle is a complete contour, i.e. it starts with a
// MoveTo and ends with a Close operation.
func (p *Path) RectBoundsWithDirection(bounds geom32.Rect, direction Direction) {
	skia.PathAddRect(p.path, skia.RectToSkRect(&bounds), skia.Direction(direction))
}

// RoundedRect adds a rectangle with curved corners to the path with a clockwise direction. The rectangle is a complete
// contour, i.e. it starts with a MoveTo and ends with a Close operation.
func (p *Path) RoundedRect(x, y, width, height, radius float32) {
	skia.PathAddRoundedRect(p.path, skia.RectToSkRect(geom32.NewRectPtr(x, y, width, height)), radius, radius, skia.Direction(Clockwise))
}

// RoundedRectXY adds a rectangle with curved corners to the path with a clockwise direction. The rectangle is a
// complete contour, i.e. it starts with a MoveTo and ends with a Close operation.
func (p *Path) RoundedRectXY(x, y, width, height, rx, ry float32) {
	skia.PathAddRoundedRect(p.path, skia.RectToSkRect(geom32.NewRectPtr(x, y, width, height)), rx, ry, skia.Direction(Clockwise))
}

// RoundedRectBounds adds a rectangle with curved corners to the path with a clockwise direction. The rectangle is a
// complete contour, i.e. it starts with a MoveTo and ends with a Close operation.
func (p *Path) RoundedRectBounds(bounds geom32.Rect, radius float32) {
	skia.PathAddRoundedRect(p.path, skia.RectToSkRect(&bounds), radius, radius, skia.Direction(Clockwise))
}

// RoundedRectBoundsXY adds a rectangle with curved corners to the path with a clockwise direction. The rectangle is a
// complete contour, i.e. it starts with a MoveTo and ends with a Close operation.
func (p *Path) RoundedRectBoundsXY(bounds geom32.Rect, radius geom32.Point) {
	skia.PathAddRoundedRect(p.path, skia.RectToSkRect(&bounds), radius.X, radius.Y, skia.Direction(Clockwise))
}

// RoundedRectWithDirection adds a rectangle with curved corners to the path. The rectangle is a complete contour, i.e.
// it starts with a MoveTo and ends with a Close operation.
func (p *Path) RoundedRectWithDirection(x, y, width, height, radius float32, direction Direction) {
	skia.PathAddRoundedRect(p.path, skia.RectToSkRect(geom32.NewRectPtr(x, y, width, height)), radius, radius, skia.Direction(direction))
}

// RoundedRectXYWithDirection adds a rectangle with curved corners to the path. The rectangle is a complete contour,
// i.e. it starts with a MoveTo and ends with a Close operation.
func (p *Path) RoundedRectXYWithDirection(x, y, width, height, rx, ry float32, direction Direction) {
	skia.PathAddRoundedRect(p.path, skia.RectToSkRect(geom32.NewRectPtr(x, y, width, height)), rx, ry, skia.Direction(direction))
}

// RoundedRectBoundsWithDirection adds a rectangle with curved corners to the path. The rectangle is a complete contour,
// i.e. it starts with a MoveTo and ends with a Close operation.
func (p *Path) RoundedRectBoundsWithDirection(bounds geom32.Rect, radius float32, direction Direction) {
	skia.PathAddRoundedRect(p.path, skia.RectToSkRect(&bounds), radius, radius, skia.Direction(direction))
}

// RoundedRectXYBoundsWithDirection adds a rectangle with curved corners to the path. The rectangle is a complete
// contour, i.e. it starts with a MoveTo and ends with a Close operation.
func (p *Path) RoundedRectXYBoundsWithDirection(bounds geom32.Rect, radius geom32.Point, direction Direction) {
	skia.PathAddRoundedRect(p.path, skia.RectToSkRect(&bounds), radius.X, radius.Y, skia.Direction(direction))
}

// Rotate the path.
func (p *Path) Rotate(radians float32) {
	skia.PathTransform(p.path, skia.Matrix2DtoMatrix(geom32.NewRotationMatrix2D(radians)))
}

// RotateByDegrees rotates the path.
func (p *Path) RotateByDegrees(degrees float32) {
	skia.PathTransform(p.path, skia.Matrix2DtoMatrix(geom32.NewRotationByDegreesMatrix2D(degrees)))
}

// Scale the path.
func (p *Path) Scale(sx, sy float32) {
	skia.PathTransform(p.path, skia.Matrix2DtoMatrix(geom32.NewScaleMatrix2D(sx, sy)))
}

// ScaleSize scales the path.
func (p *Path) ScaleSize(size geom32.Size) {
	skia.PathTransform(p.path, skia.Matrix2DtoMatrix(geom32.NewScaleMatrix2D(size.Width, size.Height)))
}

// Transform the path by the provided matrix.
func (p *Path) Transform(matrix *geom32.Matrix2D) {
	skia.PathTransform(p.path, skia.Matrix2DtoMatrix(matrix))
}

// Translate the path.
func (p *Path) Translate(x, y float32) {
	skia.PathTransform(p.path, skia.Matrix2DtoMatrix(geom32.NewTranslationMatrix2D(x, y)))
}

// TranslatePt the path.
func (p *Path) TranslatePt(pt geom32.Point) {
	skia.PathTransform(p.path, skia.Matrix2DtoMatrix(geom32.NewTranslationMatrix2D(pt.X, pt.Y)))
}

// NewRotated creates a copy of this path and then rotates it.
func (p *Path) NewRotated(radians float32) *Path {
	path := NewPath()
	skia.PathTransformToDest(p.path, path.path, skia.Matrix2DtoMatrix(geom32.NewRotationMatrix2D(radians)))
	return path
}

// NewRotatedByDegrees creates a copy of this path and then rotates it.
func (p *Path) NewRotatedByDegrees(degrees float32) *Path {
	path := NewPath()
	skia.PathTransformToDest(p.path, path.path, skia.Matrix2DtoMatrix(geom32.NewRotationByDegreesMatrix2D(degrees)))
	return path
}

// NewScaled creates a copy of this path and then scales it.
func (p *Path) NewScaled(sx, sy float32) *Path {
	path := NewPath()
	skia.PathTransformToDest(p.path, path.path, skia.Matrix2DtoMatrix(geom32.NewScaleMatrix2D(sx, sy)))
	return path
}

// NewScaledSize creates a copy of this path and then scales it.
func (p *Path) NewScaledSize(size geom32.Size) *Path {
	path := NewPath()
	skia.PathTransformToDest(p.path, path.path, skia.Matrix2DtoMatrix(geom32.NewScaleMatrix2D(size.Width, size.Height)))
	return path
}

// NewTransformed creates a copy of this path and then transforms it by the provided matrix.
func (p *Path) NewTransformed(matrix *geom32.Matrix2D) *Path {
	path := NewPath()
	skia.PathTransformToDest(p.path, path.path, skia.Matrix2DtoMatrix(matrix))
	return path
}

// NewTranslated creates a copy of this path and then translates it.
func (p *Path) NewTranslated(x, y float32) *Path {
	path := NewPath()
	skia.PathTransformToDest(p.path, path.path, skia.Matrix2DtoMatrix(geom32.NewTranslationMatrix2D(x, y)))
	return path
}

// NewTranslatedPt creates a copy of this path and then translates it.
func (p *Path) NewTranslatedPt(pt geom32.Point) *Path {
	path := NewPath()
	skia.PathTransformToDest(p.path, path.path, skia.Matrix2DtoMatrix(geom32.NewTranslationMatrix2D(pt.X, pt.Y)))
	return path
}

// Reset the path, as if it was newly created.
func (p *Path) Reset() {
	skia.PathReset(p.path)
}

// Rewind resets the path, as if it was newly created, but retains any allocated memory for future use, improving
// performance at the cost of memory.
func (p *Path) Rewind() {
	skia.PathRewind(p.path)
}

// Contains returns true if the point is within the path, taking into account the FillType.
func (p *Path) Contains(x, y float32) bool {
	return skia.PathContains(p.path, x, y)
}

// ContainsPt returns true if the point is within the path, taking into account the FillType.
func (p *Path) ContainsPt(pt geom32.Point) bool {
	return skia.PathContains(p.path, pt.X, pt.Y)
}

// CurrentPt returns the current point.
func (p *Path) CurrentPt() geom32.Point {
	return skia.PathGetLastPoint(p.path)
}

// CombinePaths combines two or more paths into a new path.
func CombinePaths(ops []PathOpPair) (*Path, error) {
	b := skia.OpBuilderNew()
	defer skia.OpBuilderDestroy(b)
	for _, pair := range ops {
		skia.OpBuilderAdd(b, pair.Path.path, skia.PathOp(pair.Op))
	}
	path := NewPath()
	if !skia.OpBuilderResolve(b, path.path) {
		return nil, errs.New("unable to resolve path combination")
	}
	return path, nil
}

func pathAddMode(extend bool) skia.PathAddMode {
	if extend {
		return skia.PathAddModeExtend
	}
	return skia.PathAddModeAppend
}
