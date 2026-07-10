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
	"github.com/richardwilkes/canvas/path"
	"github.com/richardwilkes/canvas/pathops"
	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/enums/arcsize"
	"github.com/richardwilkes/unison/enums/direction"
	"github.com/richardwilkes/unison/enums/filltype"
	"github.com/richardwilkes/unison/enums/pathop"
)

// PathOpPair holds the combination of a Path and a PathOp.
type PathOpPair struct {
	Path *Path
	Op   pathop.Enum
}

// Path holds geometry.
type Path struct {
	path *path.Path
}

func newPath(p *path.Path) *Path {
	return &Path{path: p}
}

// NewPath creates a new, empty path.
func NewPath() *Path {
	return newPath(path.New())
}

// NewPathFromSVGString attempts to create a path from the given SVG string.
func NewPathFromSVGString(svg string) (*Path, error) {
	if parsed := path.ParseSVGString(svg); parsed != nil {
		return newPath(parsed), nil
	}
	return nil, errs.New("unable to parse SVG string into path")
}

// Empty returns true if the path is empty.
func (p *Path) Empty() bool {
	return p.path.IsEmpty()
}

// ToSVGString returns an SVG string that represents this path.
func (p *Path) ToSVGString(useAbsoluteValues bool) string {
	return p.path.ToSVGString(useAbsoluteValues)
}

// FillType returns the FillType for this path.
func (p *Path) FillType() filltype.Enum {
	return filltype.Enum(p.path.FillType())
}

// SetFillType sets the FillType for this path.
func (p *Path) SetFillType(fillType filltype.Enum) {
	p.path.SetFillType(path.FillType(fillType))
}

// ArcTo appends an arc. rotation is in degrees.
func (p *Path) ArcTo(pt geom.Point, radius geom.Size, rotation float32, arcSize arcsize.Enum, dir direction.Enum) {
	p.path.ArcToRotated(radius.Width, radius.Height, rotation, path.ArcSize(arcSize), skgeom.PathDirection(dir),
		pt.X, pt.Y)
}

// ArcToFromTangent appends an arc. The arc is contained by the tangent from the current point to pt1 and the tangent
// from pt1 to pt2. The arc is part of the circle sized to radius, positioned so it touches both tangent lines.
func (p *Path) ArcToFromTangent(pt1, pt2 geom.Point, radius float32) {
	p.path.ArcToTangent(pt1.X, pt1.Y, pt2.X, pt2.Y, radius)
}

// ArcToRelative appends an arc. The destination point is relative to the current point. rotation is in degrees.
func (p *Path) ArcToRelative(destPt geom.Point, radius geom.Size, rotation float32, arcSize arcsize.Enum, dir direction.Enum) {
	p.path.RArcToRotated(radius.Width, radius.Height, rotation, path.ArcSize(arcSize), skgeom.PathDirection(dir),
		destPt.X, destPt.Y)
}

// ArcToOval appends an arc bounded by an oval. Both startAngle and sweepAngle are in degrees. A positive sweepAngle
// extends clockwise while a negative value extends counter-clockwise. If forceMoveTo is true, a new contour is started.
func (p *Path) ArcToOval(bounds geom.Rect, startAngle, sweepAngle float32, forceMoveTo bool) {
	p.path.ArcToOval(toSkRect(bounds), startAngle, sweepAngle, forceMoveTo)
}

// Bounds returns the bounding rectangle of the path. This is an approximation and may be different than the actual area
// covered when drawn.
func (p *Path) Bounds() geom.Rect {
	return fromSkRect(p.path.Bounds())
}

// ComputeTightBounds returns the bounding rectangle of the path. This is an approximation and may be different than the
// actual area covered when drawn. When a path contains only lines, this method is functionally equivalent a call to
// Bounds(), though slower. When a path contains curves, the computed bounds includes the maximum extent of the quad,
// conic, or cubic.
func (p *Path) ComputeTightBounds() geom.Rect {
	return fromSkRect(p.path.ComputeTightBounds())
}

// Circle adds a circle to the path with a clockwise direction. The circle is a complete contour, i.e. it starts with a
// MoveTo and ends with a Close operation.
func (p *Path) Circle(center geom.Point, radius float32) {
	p.path.AddCircle(center.X, center.Y, radius, skgeom.PathDirection(direction.Clockwise))
}

// CircleWithDirection adds a circle to the path. The circle is a complete contour, i.e. it starts with a MoveTo and
// ends with a Close operation.
func (p *Path) CircleWithDirection(center geom.Point, radius float32, dir direction.Enum) {
	p.path.AddCircle(center.X, center.Y, radius, skgeom.PathDirection(dir))
}

// Clone this path.
func (p *Path) Clone() *Path {
	return newPath(p.path.Clone())
}

// Close the current contour.
func (p *Path) Close() {
	p.path.Close()
}

// ConicTo appends a conic curve.
func (p *Path) ConicTo(ctrlPt, endPt geom.Point, weight float32) {
	p.path.ConicTo(ctrlPt.X, ctrlPt.Y, endPt.X, endPt.Y, weight)
}

// ConicToRelative appends a conic curve. The control point and end point are relative to the current point.
func (p *Path) ConicToRelative(ctrlPt, endPt geom.Point, weight float32) {
	p.path.RConicTo(ctrlPt.X, ctrlPt.Y, endPt.X, endPt.Y, weight)
}

// CubicTo appends a cubic curve.
func (p *Path) CubicTo(cp1, cp2, endPt geom.Point) {
	p.path.CubicTo(cp1.X, cp1.Y, cp2.X, cp2.Y, endPt.X, endPt.Y)
}

// CubicToRelative appends a cubic curve. The control point and end point are relative to the current point.
func (p *Path) CubicToRelative(cp1, cp2, endPt geom.Point) {
	p.path.RCubicTo(cp1.X, cp1.Y, cp2.X, cp2.Y, endPt.X, endPt.Y)
}

// LineTo appends a straight line segment.
func (p *Path) LineTo(pt geom.Point) {
	p.path.LineTo(pt.X, pt.Y)
}

// LineToRelative appends a straight line segment. The end point is relative to the current point.
func (p *Path) LineToRelative(pt geom.Point) {
	p.path.RLineTo(pt.X, pt.Y)
}

// MoveTo begins a new contour at the specified point.
func (p *Path) MoveTo(pt geom.Point) {
	p.path.MoveTo(pt.X, pt.Y)
}

// MoveToRelative begins a new contour at the specified point, which is relative to the current point.
func (p *Path) MoveToRelative(pt geom.Point) {
	p.path.RMoveTo(pt.X, pt.Y)
}

// Oval adds an oval to the path with a clockwise direction. The oval is a complete contour, i.e. it starts with a
// MoveTo and ends with a Close operation.
func (p *Path) Oval(bounds geom.Rect) {
	p.path.AddOval(toSkRect(bounds), skgeom.PathDirection(direction.Clockwise))
}

// OvalWithDirection adds an oval to the path. The oval is a complete contour, i.e. it starts with a MoveTo and ends
// with a Close operation.
func (p *Path) OvalWithDirection(bounds geom.Rect, dir direction.Enum) {
	p.path.AddOval(toSkRect(bounds), skgeom.PathDirection(dir))
}

// Path appends a path. If extend is true, a line from the current point to the start of the added path is created.
func (p *Path) Path(other *Path, extend bool) {
	p.path.AddPath(other.path, pathAddMode(extend))
}

// PathReverse appends a path in reverse order.
func (p *Path) PathReverse(other *Path) {
	p.path.ReverseAddPath(other.path)
}

// PathRotated appends a path after rotating it. If extend is true, a line from the current point to the start of the
// added path is created.
func (p *Path) PathRotated(other *Path, degrees float32, extend bool) {
	m := toSkMatrix(geom.NewRotationMatrix(degrees))
	p.path.AddPathMatrix(other.path, &m, pathAddMode(extend))
}

// PathScaled appends a path after scaling it. If extend is true, a line from the current point to the start of the
// added path is created.
func (p *Path) PathScaled(other *Path, pt geom.Point, extend bool) {
	m := toSkMatrix(geom.NewScaleMatrix(pt.X, pt.Y))
	p.path.AddPathMatrix(other.path, &m, pathAddMode(extend))
}

// PathTransformed appends a path after transforming it. If extend is true, a line from the current point to the start
// of the added path is created.
func (p *Path) PathTransformed(other *Path, matrix geom.Matrix, extend bool) {
	m := toSkMatrix(matrix)
	p.path.AddPathMatrix(other.path, &m, pathAddMode(extend))
}

// PathTranslated appends a path after translating it with the given offset. If extend is true, a line from the current
// point to the start of the added path is created.
func (p *Path) PathTranslated(other *Path, offset geom.Point, extend bool) {
	p.path.AddPathOffset(other.path, offset.X, offset.Y, pathAddMode(extend))
}

// Poly appends the line segments represented by pts to the path.
func (p *Path) Poly(pts []geom.Point, closePath bool) {
	if len(pts) > 0 {
		p.path.AddPoly(toSkPoints(pts), closePath)
	}
}

// QuadTo appends a quadratic curve.
func (p *Path) QuadTo(ctrlPt, endPt geom.Point) {
	p.path.QuadTo(ctrlPt.X, ctrlPt.Y, endPt.X, endPt.Y)
}

// Rect adds a rectangle to the path with a clockwise direction. The rectangle is a complete contour, i.e. it starts
// with a MoveTo and ends with a Close operation.
func (p *Path) Rect(bounds geom.Rect) {
	p.path.AddRect(toSkRect(bounds), skgeom.PathDirection(direction.Clockwise))
}

// RectWithDirection adds a rectangle to the path. The rectangle is a complete contour, i.e. it starts with a MoveTo and
// ends with a Close operation.
func (p *Path) RectWithDirection(bounds geom.Rect, dir direction.Enum) {
	p.path.AddRect(toSkRect(bounds), skgeom.PathDirection(dir))
}

// RoundedRect adds a rectangle with curved corners to the path with a clockwise direction. The rectangle is a complete
// contour, i.e. it starts with a MoveTo and ends with a Close operation.
func (p *Path) RoundedRect(bounds geom.Rect, radius geom.Size) {
	p.path.AddRoundRect(toSkRect(bounds), radius.Width, radius.Height, skgeom.PathDirection(direction.Clockwise))
}

// RoundedRectWithDirection adds a rectangle with curved corners to the path. The rectangle is a complete contour, i.e.
// it starts with a MoveTo and ends with a Close operation.
func (p *Path) RoundedRectWithDirection(bounds geom.Rect, radius geom.Size, dir direction.Enum) {
	p.path.AddRoundRect(toSkRect(bounds), radius.Width, radius.Height, skgeom.PathDirection(dir))
}

// Rotate the path.
func (p *Path) Rotate(degrees float32) {
	m := toSkMatrix(geom.NewRotationMatrix(degrees))
	p.path.Transform(&m)
}

// Scale the path.
func (p *Path) Scale(scale geom.Point) {
	m := toSkMatrix(geom.NewScaleMatrix(scale.X, scale.Y))
	p.path.Transform(&m)
}

// Transform the path by the provided matrix.
func (p *Path) Transform(matrix geom.Matrix) {
	m := toSkMatrix(matrix)
	p.path.Transform(&m)
}

// Translate the path.
func (p *Path) Translate(pt geom.Point) {
	m := toSkMatrix(geom.NewTranslationMatrix(pt.X, pt.Y))
	p.path.Transform(&m)
}

// NewRotated creates a copy of this path and then rotates it.
func (p *Path) NewRotated(degrees float32) *Path {
	result := NewPath()
	m := toSkMatrix(geom.NewRotationMatrix(degrees))
	p.path.TransformTo(&m, result.path)
	return result
}

// NewScaled creates a copy of this path and then scales it.
func (p *Path) NewScaled(scale geom.Point) *Path {
	result := NewPath()
	m := toSkMatrix(geom.NewScaleMatrix(scale.X, scale.Y))
	p.path.TransformTo(&m, result.path)
	return result
}

// NewTransformed creates a copy of this path and then transforms it by the provided matrix.
func (p *Path) NewTransformed(matrix geom.Matrix) *Path {
	result := NewPath()
	m := toSkMatrix(matrix)
	p.path.TransformTo(&m, result.path)
	return result
}

// NewTranslated creates a copy of this path and then translates it.
func (p *Path) NewTranslated(pt geom.Point) *Path {
	result := NewPath()
	m := toSkMatrix(geom.NewTranslationMatrix(pt.X, pt.Y))
	p.path.TransformTo(&m, result.path)
	return result
}

// Reset the path, as if it was newly created.
func (p *Path) Reset() {
	p.path.Reset()
}

// Rewind resets the path, as if it was newly created, but retains any allocated memory for future use, improving
// performance at the cost of memory.
func (p *Path) Rewind() {
	p.path.Rewind()
}

// Contains returns true if the point is within the path, taking into account the FillType.
func (p *Path) Contains(pt geom.Point) bool {
	return p.path.Contains(pt.X, pt.Y)
}

// CurrentPt returns the current point.
func (p *Path) CurrentPt() geom.Point {
	pt, _ := p.path.LastPt()
	return fromSkPoint(pt)
}

// applyOp applies the boolean operation op between this path and the other path. Returns true if successful. Path is
// left unmodified if not successful.
func (p *Path) applyOp(other *Path, op pathops.PathOp) bool {
	if res, ok := pathops.Op(p.path, other.path, op); ok {
		p.path = res
		return true
	}
	return false
}

// Union this path with the other path. Returns true if successful. Path is left unmodified if not successful.
func (p *Path) Union(other *Path) bool { return p.applyOp(other, pathops.Union) }

// Subtract the other path from this path. Returns true if successful. Path is left unmodified if not successful.
func (p *Path) Subtract(other *Path) bool { return p.applyOp(other, pathops.Difference) }

// Intersect this path with the other path. Returns true if successful. Path is left unmodified if not successful.
func (p *Path) Intersect(other *Path) bool { return p.applyOp(other, pathops.Intersect) }

// Xor this path with the other path. Returns true if successful. Path is left unmodified if not successful.
func (p *Path) Xor(other *Path) bool { return p.applyOp(other, pathops.XOR) }

// Simplify this path. Returns true if successful. Path is left unmodified if not successful.
func (p *Path) Simplify() bool {
	if res, ok := pathops.Simplify(p.path); ok {
		p.path = res
		return true
	}
	return false
}

// CombinePaths combines two or more paths into a new path. There is an implied empty path as the starting point.
func CombinePaths(ops []PathOpPair) (*Path, error) {
	var b pathops.Builder
	for _, pair := range ops {
		b.Add(pair.Path.path, pathops.PathOp(pair.Op))
	}
	if res, ok := b.Resolve(); ok {
		return newPath(res), nil
	}
	return nil, errs.New("unable to resolve path combination")
}

func pathAddMode(extend bool) path.AddPathMode {
	if extend {
		return path.AddPathExtend
	}
	return path.AddPathAppend
}
