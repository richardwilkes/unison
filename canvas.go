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
	"log/slog"

	"github.com/richardwilkes/canvas/canvas"
	"github.com/richardwilkes/canvas/font"
	"github.com/richardwilkes/canvas/raster"
	"github.com/richardwilkes/canvas/shaders"
	"github.com/richardwilkes/canvas/skcolor"
	"github.com/richardwilkes/canvas/textblob"
	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/enums/blendmode"
	"github.com/richardwilkes/unison/enums/filtermode"
	"github.com/richardwilkes/unison/enums/pathop"
	"github.com/richardwilkes/unison/enums/pointmode"
)

// Canvas is a drawing surface.
type Canvas struct {
	canvas  *canvas.Canvas
	surface *surface
}

// SaveCount returns the number of saved states, which equals the number of save calls minus the number of Restore()
// calls plus 1. The SaveCount() of a new Canvas is 1.
func (c *Canvas) SaveCount() int {
	return c.canvas.SaveCount()
}

// Save pushes the current transformation matrix and clip onto a stack and returns the current count. Multiple save
// calls should be balanced by an equal number of calls to Restore().
func (c *Canvas) Save() int {
	return c.canvas.Save()
}

// SaveLayer pushes the current transformation matrix and clip onto a stack and returns the current count. The provided
// paint will be applied to all subsequent drawing until the corresponding call to Restore(). Multiple save calls should
// be balanced by an equal number of calls to Restore().
func (c *Canvas) SaveLayer(paint *Paint) int {
	return c.canvas.SaveLayer(nil, paint.paint)
}

// SaveWithOpacity pushes the current transformation matrix and clip onto a stack and returns the current count. The
// opacity (a value between 0 and 1, where 0 is fully transparent and 1 is fully opaque) will be applied to all
// subsequent drawing until the corresponding call to Restore(). Multiple save calls should be balanced by an equal
// number of calls to Restore().
func (c *Canvas) SaveWithOpacity(opacity float32) int {
	return c.canvas.SaveLayerAlpha(nil, byte(clamp0To1AndScale255(opacity)))
}

// Restore removes changes to the transformation matrix and clip since the last call to Save() or SaveWithOpacity().
// Does nothing if the stack is empty.
func (c *Canvas) Restore() {
	c.canvas.Restore()
}

// RestoreToCount restores the transformation matrix and clip to the state they where in when the specified count was
// returned from a call to Save() or SaveWithOpacity(). Does nothing if count is greater than the current state stack
// count. Restores the state to the initial values if count is <= 1.
func (c *Canvas) RestoreToCount(count int) {
	c.canvas.RestoreToCount(count)
}

// Translate the coordinate system.
func (c *Canvas) Translate(delta geom.Point) {
	c.canvas.Translate(delta.X, delta.Y)
}

// Scale the coordinate system.
func (c *Canvas) Scale(scale geom.Point) {
	c.canvas.Scale(scale.X, scale.Y)
}

// Rotate the coordinate system.
func (c *Canvas) Rotate(degrees float32) {
	c.canvas.Rotate(degrees)
}

// Skew the coordinate system. A positive value of skew.X skews the drawing right as y-axis values increase; a positive
// value of skew.Y skews the drawing down as x-axis values increase.
func (c *Canvas) Skew(skew geom.Point) {
	c.canvas.Skew(skew.X, skew.Y)
}

// Concat the matrix.
func (c *Canvas) Concat(matrix geom.Matrix) {
	m := toSkMatrix(matrix)
	c.canvas.Concat(&m)
}

// ResetMatrix sets the current transform matrix to the identity matrix.
func (c *Canvas) ResetMatrix() {
	c.canvas.ResetMatrix()
}

// Matrix returns the current transform matrix.
func (c *Canvas) Matrix() geom.Matrix {
	return fromSkMatrix(c.canvas.TotalMatrix())
}

// SetMatrix replaces the current matrix with the given matrix.
func (c *Canvas) SetMatrix(matrix geom.Matrix) {
	m := toSkMatrix(matrix)
	c.canvas.SetMatrix(&m)
}

// QuickRejectPath returns true if the path, after transformations by the current matrix, can be quickly determined to
// be outside of the current clip. May return false even though the path is outside of the clip.
func (c *Canvas) QuickRejectPath(path *Path) bool {
	return c.canvas.QuickRejectPath(path.path)
}

// QuickRejectRect returns true if the rect, after transformations by the current matrix, can be quickly determined to
// be outside of the current clip. May return false even though the rect is outside of the clip.
func (c *Canvas) QuickRejectRect(rect geom.Rect) bool {
	return c.canvas.QuickReject(toSkRect(rect))
}

// Clear fills the clip with the color.
func (c *Canvas) Clear(color Color) {
	c.canvas.Clear(skcolor.Color(color))
}

// DrawPaint fills the clip with Paint. Any MaskFilter or PathEffect in the Paint is ignored.
func (c *Canvas) DrawPaint(paint *Paint) {
	c.canvas.DrawPaint(paint.paint)
}

// DrawRect draws the rectangle with Paint.
func (c *Canvas) DrawRect(rect geom.Rect, paint *Paint) {
	c.canvas.DrawRect(toSkRect(rect), paint.paint)
}

// DrawRoundedRect draws a rounded rectangle with Paint.
func (c *Canvas) DrawRoundedRect(rect geom.Rect, radius geom.Size, paint *Paint) {
	c.canvas.DrawRoundRect(toSkRect(rect), radius.Width, radius.Height, paint.paint)
}

// DrawCircle draws the circle with Paint.
func (c *Canvas) DrawCircle(center geom.Point, radius float32, paint *Paint) {
	c.canvas.DrawCircle(center.X, center.Y, radius, paint.paint)
}

// DrawOval draws the oval with Paint.
func (c *Canvas) DrawOval(rect geom.Rect, paint *Paint) {
	c.canvas.DrawOval(toSkRect(rect), paint.paint)
}

// DrawPath draws the path with Paint.
func (c *Canvas) DrawPath(path *Path, paint *Paint) {
	c.canvas.DrawPath(path.path, paint.paint)
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
	if img == nil {
		return
	}
	src := toSkRect(srcRect)
	c.canvas.DrawImageRect(img.imageForCanvas(c), src, toSkRect(dstRect), sampling.skSamplingOptions(),
		paint.paintOrNil(), canvas.ConstraintStrict)
}

// DrawImageNine draws an image stretched proportionally to fit into dstRect. 'center' divides the image into nine
// sections: four sides, four corners, and the center. Corners are unmodified or scaled down proportionately if their
// sides are larger than dstRect; center and four sides are scaled to fit remaining space, if any. paint may be nil.
func (c *Canvas) DrawImageNine(img *Image, centerRect, dstRect geom.Rect, filter filtermode.Enum, paint *Paint) {
	// DrawImageNine wants a raster image. img.image is always a raster *imagecore.Image, so asRaster is a plain type
	// assertion here; routing through imageForCanvas would instead force a GPU upload plus a full GPU->CPU readback on
	// every call for on-screen window canvases.
	c.canvas.DrawImageNine(asRaster(img.image), toSkIRect(centerRect), toSkRect(dstRect),
		shaders.FilterMode(filter), paint.paintOrNil())
}

// DrawColor fills the clip with the color.
func (c *Canvas) DrawColor(color Color, mode blendmode.Enum) {
	c.canvas.DrawColor(skcolor.Color(color), raster.BlendMode(mode))
}

// DrawPoint draws a point.
func (c *Canvas) DrawPoint(pt geom.Point, paint *Paint) {
	c.canvas.DrawPoint(pt.X, pt.Y, paint.paint)
}

// DrawPoints draws the points using the given mode.
func (c *Canvas) DrawPoints(pts []geom.Point, paint *Paint, mode pointmode.Enum) {
	c.canvas.DrawPoints(canvas.PointMode(mode), toSkPoints(pts), paint.paint)
}

// DrawLine draws a line.
func (c *Canvas) DrawLine(start, end geom.Point, paint *Paint) {
	c.canvas.DrawLine(start.X, start.Y, end.X, end.Y, paint.paint)
}

// DrawArc draws an arc. startAngle and sweepAngle are in degrees. If useCenter is true, this will draw a wedge that
// includes lines from the oval center to the arc end points. If useCenter is false, then just and arc between the end
// points will be drawn.
func (c *Canvas) DrawArc(oval geom.Rect, startAngle, sweepAngle float32, paint *Paint, useCenter bool) {
	c.canvas.DrawArc(toSkRect(oval), startAngle, sweepAngle, useCenter, paint.paint)
}

// DrawSimpleString draws a string. It does not do any processing of embedded line endings nor tabs. It also does not do
// any font fallback. pt.Y is the baseline for the text.
func (c *Canvas) DrawSimpleString(str string, pt geom.Point, f Font, paint *Paint) {
	if str != "" {
		c.canvas.DrawSimpleText([]byte(str), font.TextEncodingUTF8, pt.X, pt.Y, f.canvasFont(), paint.paint)
	}
}

// DrawTextBlob draws text from a text blob.
func (c *Canvas) DrawTextBlob(blob *textblob.Blob, pt geom.Point, paint *Paint) {
	c.canvas.DrawTextBlob(blob, pt.X, pt.Y, paint.paint)
}

// ClipRect replaces the clip with the intersection of difference of the current clip and rect.
func (c *Canvas) ClipRect(rect geom.Rect, op pathop.Enum, antialias bool) {
	if op.ValidForClip() {
		c.canvas.ClipRect(toSkRect(rect), raster.ClipOp(op), antialias)
	} else {
		errs.LogAttrs(errs.New("invalid op for clipping"), slog.String("op", op.String()))
	}
}

// ClipPath replaces the clip with the intersection of difference of the current clip and path.
func (c *Canvas) ClipPath(path *Path, op pathop.Enum, antialias bool) {
	if op.ValidForClip() {
		c.canvas.ClipPath(path.path, raster.ClipOp(op), antialias)
	} else {
		errs.LogAttrs(errs.New("invalid op for clipping"), slog.String("op", op.String()))
	}
}

// ClipBounds returns the clip bounds.
func (c *Canvas) ClipBounds() geom.Rect {
	return fromSkRect(c.canvas.LocalClipBounds())
}

// IsClipEmpty returns true if the clip is empty, i.e. nothing will draw.
func (c *Canvas) IsClipEmpty() bool {
	return c.canvas.IsClipEmpty()
}

// IsClipRect returns true if the clip is a rectangle and not empty.
func (c *Canvas) IsClipRect() bool {
	return c.canvas.IsClipRect()
}

// Flush any drawing.
func (c *Canvas) Flush() {
	c.surface.flush(true)
}
