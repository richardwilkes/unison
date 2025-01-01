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
	"runtime"

	"github.com/richardwilkes/unison/enums/blendmode"
	"github.com/richardwilkes/unison/enums/paintstyle"
	"github.com/richardwilkes/unison/enums/strokecap"
	"github.com/richardwilkes/unison/enums/strokejoin"
	"github.com/richardwilkes/unison/internal/skia"
)

// Paint controls options applied when drawing.
type Paint struct {
	paint skia.Paint
}

func newPaint(paint skia.Paint) *Paint {
	p := &Paint{paint: paint}
	runtime.SetFinalizer(p, func(obj *Paint) {
		ReleaseOnUIThread(func() {
			skia.PaintDelete(obj.paint)
		})
	})
	return p
}

// NewPaint creates a new Paint.
func NewPaint() *Paint {
	p := newPaint(skia.PaintNew())
	p.SetAntialias(true)
	return p
}

func (p *Paint) paintOrNil() skia.Paint {
	if p == nil {
		return nil
	}
	return p.paint
}

// Clone the Paint.
func (p *Paint) Clone() *Paint {
	return newPaint(skia.PaintClone(p.paint))
}

// Equivalent returns true if these Paint objects are equivalent.
func (p *Paint) Equivalent(other *Paint) bool {
	if p == nil {
		return other == nil
	}
	if other == nil {
		return false
	}
	return skia.PaintEquivalent(p.paint, other.paint)
}

// Reset the Paint back to its default state.
func (p *Paint) Reset() {
	skia.PaintReset(p.paint)
}

// Antialias returns true if pixels on the active edges of a path may be drawn with partial transparency.
func (p *Paint) Antialias() bool {
	return skia.PaintIsAntialias(p.paint)
}

// SetAntialias sets whether pixels on the active edges of a path may be drawn with partial transparency.
func (p *Paint) SetAntialias(enabled bool) {
	skia.PaintSetAntialias(p.paint, enabled)
}

// Dither returns true if color error may be distributed to smooth color transition.
func (p *Paint) Dither() bool {
	return skia.PaintIsDither(p.paint)
}

// SetDither sets whether color error may be distributed to smooth color transition.
func (p *Paint) SetDither(enabled bool) {
	skia.PaintSetDither(p.paint, enabled)
}

// Color returns the current color.
func (p *Paint) Color() Color {
	return Color(skia.PaintGetColor(p.paint))
}

// SetColor sets the color.
func (p *Paint) SetColor(color Color) {
	skia.PaintSetColor(p.paint, skia.Color(color))
}

// Style returns the current PaintStyle.
func (p *Paint) Style() paintstyle.Enum {
	return paintstyle.Enum(skia.PaintGetStyle(p.paint))
}

// SetStyle sets the PaintStyle.
func (p *Paint) SetStyle(style paintstyle.Enum) {
	skia.PaintSetStyle(p.paint, skia.PaintStyle(style))
}

// StrokeWidth returns the current stroke width.
func (p *Paint) StrokeWidth() float32 {
	return skia.PaintGetStrokeWidth(p.paint)
}

// SetStrokeWidth sets the stroke width.
func (p *Paint) SetStrokeWidth(width float32) {
	skia.PaintSetStrokeWidth(p.paint, width)
}

// StrokeMiter returns the current stroke miter limit for sharp corners.
func (p *Paint) StrokeMiter() float32 {
	return skia.PaintGetStrokeMiter(p.paint)
}

// SetStrokeMiter sets the miter limit for sharp corners.
func (p *Paint) SetStrokeMiter(miter float32) {
	skia.PaintSetStrokeMiter(p.paint, miter)
}

// StrokeCap returns the current StrokeCap.
func (p *Paint) StrokeCap() strokecap.Enum {
	return strokecap.Enum(skia.PaintGetStrokeCap(p.paint))
}

// SetStrokeCap sets the StrokeCap.
func (p *Paint) SetStrokeCap(strokeCap strokecap.Enum) {
	skia.PaintSetStrokeCap(p.paint, skia.StrokeCap(strokeCap))
}

// StrokeJoin returns the current StrokeJoin.
func (p *Paint) StrokeJoin() strokejoin.Enum {
	return strokejoin.Enum(skia.PaintGetStrokeJoin(p.paint))
}

// SetStrokeJoin sets the StrokeJoin.
func (p *Paint) SetStrokeJoin(strokeJoin strokejoin.Enum) {
	skia.PaintSetStrokeJoin(p.paint, skia.StrokeJoin(strokeJoin))
}

// BlendMode returns the current BlendMode.
func (p *Paint) BlendMode() blendmode.Enum {
	return blendmode.Enum(skia.PaintGetBlendMode(p.paint))
}

// SetBlendMode sets the BlendMode.
func (p *Paint) SetBlendMode(blendMode blendmode.Enum) {
	skia.PaintSetBlendMode(p.paint, skia.BlendMode(blendMode))
}

// Shader returns the current Shader.
func (p *Paint) Shader() *Shader {
	return newShader(skia.PaintGetShader(p.paint))
}

// SetShader sets the Shader.
func (p *Paint) SetShader(shader *Shader) {
	skia.PaintSetShader(p.paint, shader.shaderOrNil())
}

// ColorFilter returns the current ColorFilter.
func (p *Paint) ColorFilter() *ColorFilter {
	return newColorFilter(skia.PaintGetColorFilter(p.paint))
}

// SetColorFilter sets the ColorFilter.
func (p *Paint) SetColorFilter(filter *ColorFilter) {
	skia.PaintSetColorFilter(p.paint, filter.filterOrNil())
}

// MaskFilter returns the current MaskFilter.
func (p *Paint) MaskFilter() *MaskFilter {
	return newMaskFilter(skia.PaintGetMaskFilter(p.paint))
}

// SetMaskFilter sets the MaskFilter.
func (p *Paint) SetMaskFilter(filter *MaskFilter) {
	skia.PaintSetMaskFilter(p.paint, filter.filterOrNil())
}

// ImageFilter returns the current ImageFilter.
func (p *Paint) ImageFilter() *ImageFilter {
	return newImageFilter(skia.PaintGetImageFilter(p.paint))
}

// SetImageFilter sets the ImageFilter.
func (p *Paint) SetImageFilter(filter *ImageFilter) {
	skia.PaintSetImageFilter(p.paint, filter.filterOrNil())
}

// PathEffect returns the current PathEffect.
func (p *Paint) PathEffect() *PathEffect {
	return newPathEffect(skia.PaintGetPathEffect(p.paint))
}

// SetPathEffect sets the PathEffect.
func (p *Paint) SetPathEffect(effect *PathEffect) {
	skia.PaintSetPathEffect(p.paint, effect.effectOrNil())
}

// FillPath returns a path representing the path if it was stroked. resScale determines the precision used. Values >1
// increase precision, while those <1 reduce precision to favor speed and size. If hairline returns true, the path
// represents a hairline, otherwise it represents a fill.
func (p *Paint) FillPath(path *Path, resScale float32) (result *Path, hairline bool) {
	result = NewPath()
	isFill := skia.PaintGetFillPath(p.paint, path.path, result.path, nil, resScale)
	return result, !isFill
}

// FillPathWithCull returns a path representing the path if it was stroked. cullRect, if not nil, will prune any parts
// outside of the rect. resScale determines the precision used. Values >1 increase precision, while those <1 reduce
// precision to favor speed and size. If hairline returns true, the path represents a hairline, otherwise it represents
// a fill.
func (p *Paint) FillPathWithCull(path *Path, cullRect *Rect, resScale float32) (result *Path, hairline bool) {
	result = NewPath()
	isFill := skia.PaintGetFillPath(p.paint, path.path, result.path, cullRect, resScale)
	return result, !isFill
}
