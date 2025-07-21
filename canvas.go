// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/geom/poly"
	"github.com/richardwilkes/toolbox/v2/xmath"
	"github.com/richardwilkes/unison/enums/blendmode"
	"github.com/richardwilkes/unison/enums/filltype"
	"github.com/richardwilkes/unison/enums/filtermode"
	"github.com/richardwilkes/unison/enums/pathop"
	"github.com/richardwilkes/unison/enums/pointmode"
	"github.com/richardwilkes/unison/internal/skia"
)

// Canvas is a drawing surface.
type Canvas struct {
	canvas  skia.Canvas
	surface *surface
}

// SaveCount returns the number of saved states, which equals the number of save calls minus the number of Restore()
// calls plus 1. The SaveCount() of a new Canvas is 1.
func (c *Canvas) SaveCount() int {
	return skia.CanvasGetSaveCount(c.canvas)
}

// Save pushes the current transformation matrix and clip onto a stack and returns the current count. Multiple save
// calls should be balanced by an equal number of calls to Restore().
func (c *Canvas) Save() int {
	return skia.CanvasSave(c.canvas)
}

// SaveLayer pushes the current transformation matrix and clip onto a stack and returns the current count. The provided
// paint will be applied to all subsequent drawing until the corresponding call to Restore(). Multiple save calls should
// be balanced by an equal number of calls to Restore().
func (c *Canvas) SaveLayer(paint *Paint) int {
	return skia.CanvasSaveLayer(c.canvas, paint.paint)
}

// SaveWithOpacity pushes the current transformation matrix and clip onto a stack and returns the current count. The
// opacity (a value between 0 and 1, where 0 is fully transparent and 1 is fully opaque) will be applied to all
// subsequent drawing until the corresponding call to Restore(). Multiple save calls should be balanced by an equal
// number of calls to Restore().
func (c *Canvas) SaveWithOpacity(opacity float32) int {
	return skia.CanvasSaveLayerAlpha(c.canvas, byte(clamp0To1AndScale255(opacity)))
}

// Restore removes changes to the transformation matrix and clip since the last call to Save() or SaveWithOpacity().
// Does nothing if the stack is empty.
func (c *Canvas) Restore() {
	skia.CanvasRestore(c.canvas)
}

// RestoreToCount restores the transformation matrix and clip to the state they where in when the specified count was
// returned from a call to Save() or SaveWithOpacity(). Does nothing if count is greater than the current state stack
// count. Restores the state to the initial values if count is <= 1.
func (c *Canvas) RestoreToCount(count int) {
	skia.CanvasRestoreToCount(c.canvas, count)
}

// Translate the coordinate system.
func (c *Canvas) Translate(delta geom.Point) {
	skia.CanvasTranslate(c.canvas, delta.X, delta.Y)
}

// Scale the coordinate system.
func (c *Canvas) Scale(scale geom.Point) {
	skia.CanvasScale(c.canvas, scale.X, scale.Y)
}

// Rotate the coordinate system.
func (c *Canvas) Rotate(degrees float32) {
	skia.CanvasRotateRadians(c.canvas, degrees*xmath.DegreesToRadians)
}

// Skew the coordinate system. A positive value of skew.X skews the drawing right as y-axis values increase; a positive
// value of skew.Y skews the drawing down as x-axis values increase.
func (c *Canvas) Skew(skew geom.Point) {
	skia.CanvasSkew(c.canvas, skew.X, skew.Y)
}

// Concat the matrix.
func (c *Canvas) Concat(matrix geom.Matrix) {
	skia.CanvasConcat(c.canvas, matrix)
}

// ResetMatrix sets the current transform matrix to the identity matrix.
func (c *Canvas) ResetMatrix() {
	skia.CanvasResetMatrix(c.canvas)
}

// Matrix returns the current transform matrix.
func (c *Canvas) Matrix() geom.Matrix {
	return skia.CanvasGetTotalMatrix(c.canvas)
}

// SetMatrix replaces the current matrix with the given matrix.
func (c *Canvas) SetMatrix(matrix geom.Matrix) {
	skia.CanvasSetMatrix(c.canvas, matrix)
}

// QuickRejectPath returns true if the path, after transformations by the current matrix, can be quickly determined to
// be outside of the current clip. May return false even though the path is outside of the clip.
func (c *Canvas) QuickRejectPath(path *Path) bool {
	return skia.CanvasQuickRejectPath(c.canvas, path.path)
}

// QuickRejectRect returns true if the rect, after transformations by the current matrix, can be quickly determined to
// be outside of the current clip. May return false even though the rect is outside of the clip.
func (c *Canvas) QuickRejectRect(rect geom.Rect) bool {
	return skia.CanvasQuickRejectRect(c.canvas, rect)
}

// Clear fills the clip with the color.
func (c *Canvas) Clear(color Color) {
	skia.CanvasClear(c.canvas, skia.Color(color))
}

// DrawPaint fills the clip with Paint. Any MaskFilter or PathEffect in the Paint is ignored.
func (c *Canvas) DrawPaint(paint *Paint) {
	skia.CanvasDrawPaint(c.canvas, paint.paint)
}

// DrawRect draws the rectangle with Paint.
func (c *Canvas) DrawRect(rect geom.Rect, paint *Paint) {
	skia.CanvasDrawRect(c.canvas, rect, paint.paint)
}

// DrawRoundedRect draws a rounded rectangle with Paint.
func (c *Canvas) DrawRoundedRect(rect geom.Rect, radius geom.Size, paint *Paint) {
	skia.CanvasDrawRoundRect(c.canvas, rect, radius.Width, radius.Height, paint.paint)
}

// DrawCircle draws the circle with Paint.
func (c *Canvas) DrawCircle(center geom.Point, radius float32, paint *Paint) {
	skia.CanvasDrawCircle(c.canvas, center.X, center.Y, radius, paint.paint)
}

// DrawOval draws the oval with Paint.
func (c *Canvas) DrawOval(rect geom.Rect, paint *Paint) {
	skia.CanvasDrawOval(c.canvas, rect, paint.paint)
}

// DrawPath draws the path with Paint.
func (c *Canvas) DrawPath(path *Path, paint *Paint) {
	skia.CanvasDrawPath(c.canvas, path.path, paint.paint)
}

// DrawImage draws the image at the specified location using its logical size. paint may be nil.
func (c *Canvas) DrawImage(img *Image, upperLeft geom.Point, sampling *SamplingOptions, paint *Paint) {
	c.DrawImageInRect(img, geom.Rect{Point: upperLeft, Size: img.LogicalSize()}, sampling, paint)
}

// DrawImageInRect draws the image into the area specified by the rect, scaling if necessary. paint may be nil.
func (c *Canvas) DrawImageInRect(img *Image, rect geom.Rect, sampling *SamplingOptions, paint *Paint) {
	c.DrawImageRectInRect(img, geom.Rect{Size: img.Size()}, rect, sampling, paint)
}

// DrawImageRectInRect draws a portion of the image into the area specified, scaling if necessary. srcRect should be in
// raw pixel coordinates, not logical coordinates. dstRect should be in logical coordinates. paint may be nil.
func (c *Canvas) DrawImageRectInRect(img *Image, srcRect, dstRect geom.Rect, sampling *SamplingOptions, paint *Paint) {
	skia.CanvasDrawImageRect(c.canvas, img.skiaImageForCanvas(c), srcRect, dstRect, sampling.skSamplingOptions(),
		paint.paintOrNil())
}

// DrawImageNine draws an image stretched proportionally to fit into dstRect. 'center' divides the image into nine
// sections: four sides, four corners, and the center. Corners are unmodified or scaled down proportionately if their
// sides are larger than dstRect; center and four sides are scaled to fit remaining space, if any. paint may be nil.
func (c *Canvas) DrawImageNine(img *Image, centerRect, dstRect geom.Rect, filter filtermode.Enum, paint *Paint) {
	skia.CanvasDrawImageNine(c.canvas, img.skiaImageForCanvas(c), centerRect, dstRect, skia.FilterMode(filter),
		paint.paintOrNil())
}

// DrawColor fills the clip with the color.
func (c *Canvas) DrawColor(color Color, mode blendmode.Enum) {
	skia.CanvasDrawColor(c.canvas, skia.Color(color), skia.BlendMode(mode))
}

// DrawPoint draws a point.
func (c *Canvas) DrawPoint(pt geom.Point, paint *Paint) {
	skia.CanvasDrawPoint(c.canvas, pt, paint.paint)
}

// DrawPoints draws the points using the given mode.
func (c *Canvas) DrawPoints(pts []geom.Point, paint *Paint, mode pointmode.Enum) {
	skia.CanvasDrawPoints(c.canvas, skia.PointMode(mode), pts, paint.paint)
}

// DrawLine draws a line.
func (c *Canvas) DrawLine(start, end geom.Point, paint *Paint) {
	skia.CanvasDrawLine(c.canvas, start.X, start.Y, end.X, end.Y, paint.paint)
}

// DrawPolygon draws a polygon.
func (c *Canvas) DrawPolygon(polygon poly.Polygon, mode filltype.Enum, paint *Paint) {
	path := NewPath()
	path.SetFillType(mode)
	path.Polygon(polygon)
	c.DrawPath(path, paint)
}

// DrawArc draws an arc. startAngle and sweepAngle are in degrees. If useCenter is true, this will draw a wedge that
// includes lines from the oval center to the arc end points. If useCenter is false, then just and arc between the end
// points will be drawn.
func (c *Canvas) DrawArc(oval geom.Rect, startAngle, sweepAngle float32, paint *Paint, useCenter bool) {
	skia.CanvasDrawArc(c.canvas, oval, startAngle, sweepAngle, useCenter, paint.paint)
}

// DrawSimpleString draws a string. It does not do any processing of embedded line endings nor tabs. It also does not do
// any font fallback. pt.Y is the baseline for the text.
func (c *Canvas) DrawSimpleString(str string, pt geom.Point, font Font, paint *Paint) {
	if str != "" {
		skia.CanvasDrawSimpleText(c.canvas, str, pt, font.skiaFont(), paint.paint)
	}
}

// DrawTextBlob draws text from a text blob.
func (c *Canvas) DrawTextBlob(blob *TextBlob, pt geom.Point, paint *Paint) {
	skia.CanvasDrawTextBlob(c.canvas, blob.blob, pt, paint.paint)
}

// ClipRect replaces the clip with the intersection of difference of the current clip and rect.
func (c *Canvas) ClipRect(rect geom.Rect, op pathop.Enum, antialias bool) {
	skia.CanavasClipRectWithOperation(c.canvas, rect, skia.ClipOp(op), antialias)
}

// ClipPath replaces the clip with the intersection of difference of the current clip and path.
func (c *Canvas) ClipPath(path *Path, op pathop.Enum, antialias bool) {
	skia.CanavasClipPathWithOperation(c.canvas, path.path, skia.ClipOp(op), antialias)
}

// ClipBounds returns the clip bounds.
func (c *Canvas) ClipBounds() geom.Rect {
	return skia.CanvasGetLocalClipBounds(c.canvas)
}

// IsClipEmpty returns true if the clip is empty, i.e. nothing will draw.
func (c *Canvas) IsClipEmpty() bool {
	return skia.CanvasIsClipEmpty(c.canvas)
}

// IsClipRect returns true if the clip is a rectangle and not empty.
func (c *Canvas) IsClipRect() bool {
	return skia.CanvasIsClipRect(c.canvas)
}

// Flush any drawing.
func (c *Canvas) Flush() {
	c.surface.flush(true)
}
