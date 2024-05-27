// Copyright (c) 2021-2024 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"github.com/richardwilkes/toolbox/xmath"
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
func (c *Canvas) Translate(dx, dy float32) {
	skia.CanvasTranslate(c.canvas, dx, dy)
}

// Scale the coordinate system.
func (c *Canvas) Scale(x, y float32) {
	skia.CanvasScale(c.canvas, x, y)
}

// Rotate the coordinate system.
func (c *Canvas) Rotate(degrees float32) {
	skia.CanvasRotateRadians(c.canvas, degrees*xmath.DegreesToRadians)
}

// Skew the coordinate system. A positive value of sx skews the drawing right as y-axis values increase; a positive
// value of sy skews the drawing down as x-axis values increase.
func (c *Canvas) Skew(sx, sy float32) {
	skia.CanvasSkew(c.canvas, sx, sy)
}

// Concat the matrix.
func (c *Canvas) Concat(matrix Matrix) {
	skia.CanvasConcat(c.canvas, matrix)
}

// ResetMatrix sets the current transform matrix to the identity matrix.
func (c *Canvas) ResetMatrix() {
	skia.CanvasResetMatrix(c.canvas)
}

// Matrix returns the current transform matrix.
func (c *Canvas) Matrix() Matrix {
	return skia.CanvasGetTotalMatrix(c.canvas)
}

// SetMatrix replaces the current matrix with the given matrix.
func (c *Canvas) SetMatrix(matrix Matrix) {
	skia.CanvasSetMatrix(c.canvas, matrix)
}

// QuickRejectPath returns true if the path, after transformations by the current matrix, can be quickly determined to
// be outside of the current clip. May return false even though the path is outside of the clip.
func (c *Canvas) QuickRejectPath(path *Path) bool {
	return skia.CanvasQuickRejectPath(c.canvas, path.path)
}

// QuickRejectRect returns true if the rect, after transformations by the current matrix, can be quickly determined to
// be outside of the current clip. May return false even though the rect is outside of the clip.
func (c *Canvas) QuickRejectRect(rect Rect) bool {
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
func (c *Canvas) DrawRect(rect Rect, paint *Paint) {
	skia.CanvasDrawRect(c.canvas, rect, paint.paint)
}

// DrawRoundedRect draws a rounded rectangle with Paint.
func (c *Canvas) DrawRoundedRect(rect Rect, radiusX, radiusY float32, paint *Paint) {
	skia.CanvasDrawRoundRect(c.canvas, rect, radiusX, radiusY, paint.paint)
}

// DrawCircle draws the circle with Paint.
func (c *Canvas) DrawCircle(cx, cy, radius float32, paint *Paint) {
	skia.CanvasDrawCircle(c.canvas, cx, cy, radius, paint.paint)
}

// DrawOval draws the oval with Paint.
func (c *Canvas) DrawOval(rect Rect, paint *Paint) {
	skia.CanvasDrawOval(c.canvas, rect, paint.paint)
}

// DrawPath draws the path with Paint.
func (c *Canvas) DrawPath(path *Path, paint *Paint) {
	skia.CanvasDrawPath(c.canvas, path.path, paint.paint)
}

// DrawImage draws the image at the specified location using its logical size. paint may be nil.
func (c *Canvas) DrawImage(img *Image, x, y float32, sampling *SamplingOptions, paint *Paint) {
	c.DrawImageInRect(img, Rect{Point: Point{X: x, Y: y}, Size: img.LogicalSize()}, sampling, paint)
}

// DrawImageInRect draws the image into the area specified by the rect, scaling if necessary. paint may be nil.
func (c *Canvas) DrawImageInRect(img *Image, rect Rect, sampling *SamplingOptions, paint *Paint) {
	c.DrawImageRectInRect(img, Rect{Size: img.Size()}, rect, sampling, paint)
}

// DrawImageRectInRect draws a portion of the image into the area specified, scaling if necessary. srcRect should be in
// raw pixel coordinates, not logical coordinates. dstRect should be in logical coordinates. paint may be nil.
func (c *Canvas) DrawImageRectInRect(img *Image, srcRect, dstRect Rect, sampling *SamplingOptions, paint *Paint) {
	skia.CanvasDrawImageRect(c.canvas, img.ref().contextImg(c.surface), srcRect, dstRect, sampling.skSamplingOptions(),
		paint.paintOrNil())
}

// DrawImageNine draws an image stretched proportionally to fit into dstRect. 'center' divides the image into nine
// sections: four sides, four corners, and the center. Corners are unmodified or scaled down proportionately if their
// sides are larger than dstRect; center and four sides are scaled to fit remaining space, if any. paint may be nil.
func (c *Canvas) DrawImageNine(img *Image, centerRect, dstRect Rect, filter filtermode.Enum, paint *Paint) {
	skia.CanvasDrawImageNine(c.canvas, img.ref().contextImg(c.surface), centerRect, dstRect, skia.FilterMode(filter),
		paint.paintOrNil())
}

// DrawColor fills the clip with the color.
func (c *Canvas) DrawColor(color Color, mode blendmode.Enum) {
	skia.CanvasDrawColor(c.canvas, skia.Color(color), skia.BlendMode(mode))
}

// DrawPoint draws a point.
func (c *Canvas) DrawPoint(x, y float32, paint *Paint) {
	skia.CanvasDrawPoint(c.canvas, x, y, paint.paint)
}

// DrawPoints draws the points using the given mode.
func (c *Canvas) DrawPoints(pts []Point, paint *Paint, mode pointmode.Enum) {
	skia.CanvasDrawPoints(c.canvas, skia.PointMode(mode), pts, paint.paint)
}

// DrawLine draws a line.
func (c *Canvas) DrawLine(sx, sy, ex, ey float32, paint *Paint) {
	skia.CanvasDrawLine(c.canvas, sx, sy, ex, ey, paint.paint)
}

// DrawPolygon draws a polygon.
func (c *Canvas) DrawPolygon(poly Polygon, mode filltype.Enum, paint *Paint) {
	path := NewPath()
	path.SetFillType(mode)
	path.Polygon(poly)
	c.DrawPath(path, paint)
}

// DrawArc draws an arc. startAngle and sweepAngle are in degrees. If useCenter is true, this will draw a wedge that
// includes lines from the oval center to the arc end points. If useCenter is false, then just and arc between the end
// points will be drawn.
func (c *Canvas) DrawArc(oval Rect, startAngle, sweepAngle float32, paint *Paint, useCenter bool) {
	skia.CanvasDrawArc(c.canvas, oval, startAngle, sweepAngle, useCenter, paint.paint)
}

// DrawSimpleString draws a string. It does not do any processing of embedded line endings nor tabs. It also does not do
// any font fallback. y is the baseline for the text.
func (c *Canvas) DrawSimpleString(str string, x, y float32, font Font, paint *Paint) {
	if str != "" {
		skia.CanvasDrawSimpleText(c.canvas, str, x, y, font.skiaFont(), paint.paint)
	}
}

// DrawTextBlob draws text from a text blob.
func (c *Canvas) DrawTextBlob(blob *TextBlob, x, y float32, paint *Paint) {
	skia.CanvasDrawTextBlob(c.canvas, blob.blob, x, y, paint.paint)
}

// ClipRect replaces the clip with the intersection of difference of the current clip and rect.
func (c *Canvas) ClipRect(rect Rect, op pathop.Enum, antialias bool) {
	skia.CanavasClipRectWithOperation(c.canvas, rect, skia.ClipOp(op), antialias)
}

// ClipPath replaces the clip with the intersection of difference of the current clip and path.
func (c *Canvas) ClipPath(path *Path, op pathop.Enum, antialias bool) {
	skia.CanavasClipPathWithOperation(c.canvas, path.path, skia.ClipOp(op), antialias)
}

// ClipBounds returns the clip bounds.
func (c *Canvas) ClipBounds() Rect {
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
