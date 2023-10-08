// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
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
	"github.com/richardwilkes/unison/enums/arcsize"
	"github.com/richardwilkes/unison/enums/direction"
	"github.com/richardwilkes/unison/enums/filltype"
	"github.com/richardwilkes/unison/enums/pathop"
	"github.com/richardwilkes/unison/internal/skia"
)

// PathOpPair holds the combination of a Path and a PathOp.
type PathOpPair struct {
	Path *Path
	Op   pathop.Enum
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
func (p *Path) ToSVGString(useAbsoluteValues bool) string {
	ss := skia.PathToSVGString(p.path, useAbsoluteValues)
	defer skia.StringDelete(ss)
	return skia.StringGetString(ss)
}

// FillType returns the FillType for this path.
func (p *Path) FillType() filltype.Enum {
	return filltype.Enum(skia.PathGetFillType(p.path))
}

// SetFillType sets the FillType for this path.
func (p *Path) SetFillType(fillType filltype.Enum) {
	skia.PathSetFillType(p.path, skia.FillType(fillType))
}

// ArcTo appends an arc. rotation is in degrees.
func (p *Path) ArcTo(x, y, rx, ry, rotation float32, arcSize arcsize.Enum, dir direction.Enum) {
	skia.PathArcTo(p.path, x, y, rx, ry, rotation, skia.ArcSize(arcSize), skia.Direction(dir))
}

// ArcToFromTangent appends an arc. The arc is contained by the tangent from the current point to (x1, y1) and the
// tangent from (x1, y1) to (x2, y2). The arc is part of the circle sized to radius, positioned so it touches both
// tangent lines.
func (p *Path) ArcToFromTangent(x1, y1, x2, y2, radius float32) {
	skia.PathArcToWithPoints(p.path, x1, y1, x2, y2, radius)
}

// ArcToRelative appends an arc. The destination point is relative to the current point. rotation is in degrees.
func (p *Path) ArcToRelative(dx, dy, rx, ry, rotation float32, arcSize arcsize.Enum, dir direction.Enum) {
	skia.PathRArcTo(p.path, dx, dy, rx, ry, rotation, skia.ArcSize(arcSize), skia.Direction(dir))
}

// ArcToOval appends an arc bounded by an oval. Both startAngle and sweepAngle are in degrees. A positive sweepAngle
// extends clockwise while a negative value extends counter-clockwise. If forceMoveTo is true, a new contour is started.
func (p *Path) ArcToOval(bounds Rect, startAngle, sweepAngle float32, forceMoveTo bool) {
	skia.PathArcToWithOval(p.path, bounds, startAngle, sweepAngle, forceMoveTo)
}

// Bounds returns the bounding rectangle of the path. This is an approximation and may be different than the actual area
// covered when drawn.
func (p *Path) Bounds() Rect {
	return skia.PathGetBounds(p.path)
}

// ComputeTightBounds returns the bounding rectangle of the path. This is an approximation and may be different than the
// actual area covered when drawn. When a path contains only lines, this method is functionally equivalent a call to
// Bounds(), though slower. When a path contains curves, the computed bounds includes the maximum extent of the quad,
// conic, or cubic.
func (p *Path) ComputeTightBounds() Rect {
	return skia.PathComputeTightBounds(p.path)
}

// Circle adds a circle to the path with a clockwise direction. The circle is a complete contour, i.e. it starts with a
// MoveTo and ends with a Close operation.
func (p *Path) Circle(x, y, radius float32) {
	skia.PathAddCircle(p.path, x, y, radius, skia.Direction(direction.Clockwise))
}

// CircleWithDirection adds a circle to the path. The circle is a complete contour, i.e. it starts with a MoveTo and
// ends with a Close operation.
func (p *Path) CircleWithDirection(x, y, radius float32, dir direction.Enum) {
	skia.PathAddCircle(p.path, x, y, radius, skia.Direction(dir))
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

// ConicToRelative appends a conic curve. The control point and end point are relative to the current point.
func (p *Path) ConicToRelative(cpdx, cpdy, dx, dy, weight float32) {
	skia.PathRConicTo(p.path, cpdx, cpdy, dx, dy, weight)
}

// CubicTo appends a cubic curve.
func (p *Path) CubicTo(cp1x, cp1y, cp2x, cp2y, x, y float32) {
	skia.PathCubicTo(p.path, cp1x, cp1y, cp2x, cp2y, x, y)
}

// CubicToRelative appends a cubic curve. The control point and end point are relative to the current point.
func (p *Path) CubicToRelative(cp1dx, cp1dy, cp2dx, cp2dy, dx, dy float32) {
	skia.PathRCubicTo(p.path, cp1dx, cp1dy, cp2dx, cp2dy, dx, dy)
}

// LineTo appends a straight line segment.
func (p *Path) LineTo(x, y float32) {
	skia.PathLineTo(p.path, x, y)
}

// LineToRelative appends a straight line segment. The end point is relative to the current point.
func (p *Path) LineToRelative(x, y float32) {
	skia.PathRLineTo(p.path, x, y)
}

// MoveTo begins a new contour at the specified point.
func (p *Path) MoveTo(x, y float32) {
	skia.PathMoveTo(p.path, x, y)
}

// MoveToRelative begins a new contour at the specified point, which is relative to the current point.
func (p *Path) MoveToRelative(x, y float32) {
	skia.PathRMoveTo(p.path, x, y)
}

// Oval adds an oval to the path with a clockwise direction. The oval is a complete contour, i.e. it starts with a
// MoveTo and ends with a Close operation.
func (p *Path) Oval(bounds Rect) {
	skia.PathAddOval(p.path, bounds, skia.Direction(direction.Clockwise))
}

// OvalWithDirection adds an oval to the path. The oval is a complete contour, i.e. it starts with a MoveTo and ends
// with a Close operation.
func (p *Path) OvalWithDirection(bounds Rect, dir direction.Enum) {
	skia.PathAddOval(p.path, bounds, skia.Direction(dir))
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
func (p *Path) PathRotated(path *Path, degrees float32, extend bool) {
	skia.PathAddPathMatrix(p.path, path.path, NewRotationByDegreesMatrix(degrees), pathAddMode(extend))
}

// PathScaled appends a path after scaling it. If extend is true, a line from the current point to the start of the
// added path is created.
func (p *Path) PathScaled(path *Path, sx, sy float32, extend bool) {
	skia.PathAddPathMatrix(p.path, path.path, NewScaleMatrix(sx, sy), pathAddMode(extend))
}

// PathTransformed appends a path after transforming it. If extend is true, a line from the current point to the start
// of the added path is created.
func (p *Path) PathTransformed(path *Path, matrix Matrix, extend bool) {
	skia.PathAddPathMatrix(p.path, path.path, matrix, pathAddMode(extend))
}

// PathTranslated appends a path after translating it with the given offset. If extend is true, a line from the current
// point to the start of the added path is created.
func (p *Path) PathTranslated(path *Path, offsetX, offsetY float32, extend bool) {
	skia.PathAddPathOffset(p.path, path.path, offsetX, offsetY, pathAddMode(extend))
}

// Poly appends the line segments represented by pts to the path.
func (p *Path) Poly(pts []Point, closePath bool) {
	if len(pts) > 0 {
		skia.PathAddPoly(p.path, pts, closePath)
	}
}

// Polygon appends the polygon to the path.
func (p *Path) Polygon(poly Polygon) {
	for _, contour := range poly {
		p.Poly(contour, true)
	}
}

// QuadTo appends a quadratic curve.
func (p *Path) QuadTo(cpx, cpy, x, y float32) {
	skia.PathQuadTo(p.path, cpx, cpy, x, y)
}

// Rect adds a rectangle to the path with a clockwise direction. The rectangle is a complete contour, i.e. it starts
// with a MoveTo and ends with a Close operation.
func (p *Path) Rect(bounds Rect) {
	skia.PathAddRect(p.path, bounds, skia.Direction(direction.Clockwise))
}

// RectWithDirection adds a rectangle to the path. The rectangle is a complete contour, i.e. it starts with a MoveTo and
// ends with a Close operation.
func (p *Path) RectWithDirection(bounds Rect, dir direction.Enum) {
	skia.PathAddRect(p.path, bounds, skia.Direction(dir))
}

// RoundedRect adds a rectangle with curved corners to the path with a clockwise direction. The rectangle is a complete
// contour, i.e. it starts with a MoveTo and ends with a Close operation.
func (p *Path) RoundedRect(bounds Rect, radiusX, radiusY float32) {
	skia.PathAddRoundedRect(p.path, bounds, radiusX, radiusY, skia.Direction(direction.Clockwise))
}

// RoundedRectWithDirection adds a rectangle with curved corners to the path. The rectangle is a complete contour, i.e.
// it starts with a MoveTo and ends with a Close operation.
func (p *Path) RoundedRectWithDirection(bounds Rect, radiusX, radiusY float32, dir direction.Enum) {
	skia.PathAddRoundedRect(p.path, bounds, radiusX, radiusY, skia.Direction(dir))
}

// Rotate the path.
func (p *Path) Rotate(degrees float32) {
	skia.PathTransform(p.path, NewRotationByDegreesMatrix(degrees))
}

// Scale the path.
func (p *Path) Scale(sx, sy float32) {
	skia.PathTransform(p.path, NewScaleMatrix(sx, sy))
}

// Transform the path by the provided matrix.
func (p *Path) Transform(matrix Matrix) {
	skia.PathTransform(p.path, matrix)
}

// Translate the path.
func (p *Path) Translate(x, y float32) {
	skia.PathTransform(p.path, NewTranslationMatrix(x, y))
}

// NewRotated creates a copy of this path and then rotates it.
func (p *Path) NewRotated(degrees float32) *Path {
	path := NewPath()
	skia.PathTransformToDest(p.path, path.path, NewRotationByDegreesMatrix(degrees))
	return path
}

// NewScaled creates a copy of this path and then scales it.
func (p *Path) NewScaled(sx, sy float32) *Path {
	path := NewPath()
	skia.PathTransformToDest(p.path, path.path, NewScaleMatrix(sx, sy))
	return path
}

// NewTransformed creates a copy of this path and then transforms it by the provided matrix.
func (p *Path) NewTransformed(matrix Matrix) *Path {
	path := NewPath()
	skia.PathTransformToDest(p.path, path.path, matrix)
	return path
}

// NewTranslated creates a copy of this path and then translates it.
func (p *Path) NewTranslated(x, y float32) *Path {
	path := NewPath()
	skia.PathTransformToDest(p.path, path.path, NewTranslationMatrix(x, y))
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

// CurrentPt returns the current point.
func (p *Path) CurrentPt() Point {
	return skia.PathGetLastPoint(p.path)
}

// Union this path with the other path. Returns true if successful. Path is left unmodified if not successful.
func (p *Path) Union(other *Path) bool {
	return skia.PathCompute(p.path, other.path, skia.PathOp(pathop.Union))
}

// Subtract the other path from this path. Returns true if successful. Path is left unmodified if not successful.
func (p *Path) Subtract(other *Path) bool {
	return skia.PathCompute(p.path, other.path, skia.PathOp(pathop.Difference))
}

// Intersect this path with the other path. Returns true if successful. Path is left unmodified if not successful.
func (p *Path) Intersect(other *Path) bool {
	return skia.PathCompute(p.path, other.path, skia.PathOp(pathop.Intersect))
}

// Xor this path with the other path. Returns true if successful. Path is left unmodified if not successful.
func (p *Path) Xor(other *Path) bool {
	return skia.PathCompute(p.path, other.path, skia.PathOp(pathop.Xor))
}

// Simplify this path. Returns true if successful. Path is left unmodified if not successful.
func (p *Path) Simplify() bool {
	return skia.PathSimplify(p.path)
}

// CombinePaths combines two or more paths into a new path. There is an implied empty path as the starting point.
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
