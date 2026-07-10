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
	"reflect"

	"github.com/richardwilkes/canvas/canvas"
	skgeom "github.com/richardwilkes/canvas/geom"
	"github.com/richardwilkes/canvas/raster"
	"github.com/richardwilkes/canvas/skcolor"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/enums/blendmode"
	"github.com/richardwilkes/unison/enums/paintstyle"
	"github.com/richardwilkes/unison/enums/strokecap"
	"github.com/richardwilkes/unison/enums/strokejoin"
)

// Paint controls options applied when drawing.
type Paint struct {
	paint *canvas.Paint
}

func newPaint(paint *canvas.Paint) *Paint {
	return &Paint{paint: paint}
}

// NewPaint creates a new Paint.
func NewPaint() *Paint {
	p := newPaint(canvas.NewPaint())
	p.SetAntialias(true)
	return p
}

func (p *Paint) paintOrNil() *canvas.Paint {
	if p == nil {
		return nil
	}
	return p.paint
}

// Clone the Paint.
func (p *Paint) Clone() *Paint {
	clone := *p.paint
	return &Paint{paint: &clone}
}

// Equivalent returns true if these Paint objects are equivalent.
func (p *Paint) Equivalent(other *Paint) bool {
	if p == nil {
		return other == nil
	}
	if other == nil {
		return false
	}
	if p.paint == other.paint {
		return true
	}
	a := *p.paint
	b := *other.paint
	// The five effect fields (PathEffect, Shader, ColorFilter, MaskFilter, ImageFilter) are interfaces whose dynamic
	// types may not be comparable (e.g. a gradient shader carrying a []Color slice), so a plain a == b on the whole
	// struct could panic. Compare the effects by identity (matching the old SkPaint::operator== pointer semantics),
	// then clear them so the remaining all-scalar fields can be compared safely with ==.
	if !sameEffect(a.PathEffect, b.PathEffect) ||
		!sameEffect(a.Shader, b.Shader) ||
		!sameEffect(a.ColorFilter, b.ColorFilter) ||
		!sameEffect(a.MaskFilter, b.MaskFilter) ||
		!sameEffect(a.ImageFilter, b.ImageFilter) {
		return false
	}
	a.PathEffect, a.Shader, a.ColorFilter, a.MaskFilter, a.ImageFilter = nil, nil, nil, nil, nil
	b.PathEffect, b.Shader, b.ColorFilter, b.MaskFilter, b.ImageFilter = nil, nil, nil, nil, nil
	return a == b
}

// sameEffect reports whether two Paint effect values refer to the same underlying effect. Effects are treated as
// immutable and compared by identity: pointer-typed effects (the common case) by pointer, and any non-pointer effect
// via a panic-safe deep comparison rather than ==, which would panic on a non-comparable dynamic type.
func sameEffect(a, b any) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	va := reflect.ValueOf(a)
	vb := reflect.ValueOf(b)
	if va.Type() != vb.Type() {
		return false
	}
	switch va.Kind() {
	case reflect.Pointer, reflect.UnsafePointer, reflect.Chan, reflect.Func, reflect.Map:
		return va.Pointer() == vb.Pointer()
	default:
		return reflect.DeepEqual(a, b)
	}
}

// Reset the Paint back to its default state.
func (p *Paint) Reset() {
	p.paint = canvas.NewPaint()
}

// Antialias returns true if pixels on the active edges of a path may be drawn with partial transparency.
func (p *Paint) Antialias() bool {
	return p.paint.AntiAlias
}

// SetAntialias sets whether pixels on the active edges of a path may be drawn with partial transparency.
func (p *Paint) SetAntialias(enabled bool) {
	p.paint.AntiAlias = enabled
}

// Dither returns true if color error may be distributed to smooth color transition.
func (p *Paint) Dither() bool {
	return p.paint.Dither
}

// SetDither sets whether color error may be distributed to smooth color transition.
func (p *Paint) SetDither(enabled bool) {
	p.paint.Dither = enabled
}

// Color returns the current color.
func (p *Paint) Color() Color {
	return Color(p.paint.Color)
}

// SetColor sets the color.
func (p *Paint) SetColor(color Color) {
	p.paint.Color = skcolor.Color(color)
}

// Style returns the current PaintStyle.
func (p *Paint) Style() paintstyle.Enum {
	return paintstyle.Enum(p.paint.Style)
}

// SetStyle sets the PaintStyle.
func (p *Paint) SetStyle(style paintstyle.Enum) {
	p.paint.Style = canvas.Style(style)
}

// StrokeWidth returns the current stroke width.
func (p *Paint) StrokeWidth() float32 {
	return p.paint.StrokeWidth
}

// SetStrokeWidth sets the stroke width.
func (p *Paint) SetStrokeWidth(width float32) {
	p.paint.StrokeWidth = width
}

// StrokeMiter returns the current stroke miter limit for sharp corners.
func (p *Paint) StrokeMiter() float32 {
	return p.paint.MiterLimit
}

// SetStrokeMiter sets the miter limit for sharp corners.
func (p *Paint) SetStrokeMiter(miter float32) {
	p.paint.MiterLimit = miter
}

// StrokeCap returns the current StrokeCap.
func (p *Paint) StrokeCap() strokecap.Enum {
	return strokecap.Enum(p.paint.Cap)
}

// SetStrokeCap sets the StrokeCap.
func (p *Paint) SetStrokeCap(strokeCap strokecap.Enum) {
	p.paint.Cap = canvas.StrokeCap(strokeCap)
}

// StrokeJoin returns the current StrokeJoin.
func (p *Paint) StrokeJoin() strokejoin.Enum {
	return strokejoin.Enum(p.paint.Join)
}

// SetStrokeJoin sets the StrokeJoin.
func (p *Paint) SetStrokeJoin(strokeJoin strokejoin.Enum) {
	p.paint.Join = canvas.StrokeJoin(strokeJoin)
}

// BlendMode returns the current BlendMode.
func (p *Paint) BlendMode() blendmode.Enum {
	return blendmode.Enum(p.paint.BlendMode)
}

// SetBlendMode sets the BlendMode.
func (p *Paint) SetBlendMode(blendMode blendmode.Enum) {
	p.paint.BlendMode = raster.BlendMode(blendMode)
}

// Shader returns the current Shader.
func (p *Paint) Shader() *Shader {
	return newShader(p.paint.Shader)
}

// SetShader sets the Shader.
func (p *Paint) SetShader(shader *Shader) {
	p.paint.Shader = shader.shaderOrNil()
}

// ColorFilter returns the current ColorFilter.
func (p *Paint) ColorFilter() *ColorFilter {
	return newColorFilter(p.paint.ColorFilter)
}

// SetColorFilter sets the ColorFilter.
func (p *Paint) SetColorFilter(filter *ColorFilter) {
	p.paint.ColorFilter = filter.filterOrNil()
}

// MaskFilter returns the current MaskFilter.
func (p *Paint) MaskFilter() *MaskFilter {
	return newMaskFilter(p.paint.MaskFilter)
}

// SetMaskFilter sets the MaskFilter.
func (p *Paint) SetMaskFilter(filter *MaskFilter) {
	p.paint.MaskFilter = filter.filterOrNil()
}

// ImageFilter returns the current ImageFilter.
func (p *Paint) ImageFilter() *ImageFilter {
	return newImageFilter(p.paint.ImageFilter)
}

// SetImageFilter sets the ImageFilter.
func (p *Paint) SetImageFilter(filter *ImageFilter) {
	p.paint.ImageFilter = filter.filterOrNil()
}

// PathEffect returns the current PathEffect.
func (p *Paint) PathEffect() *PathEffect {
	return newPathEffect(p.paint.PathEffect)
}

// SetPathEffect sets the PathEffect.
func (p *Paint) SetPathEffect(effect *PathEffect) {
	p.paint.PathEffect = effect.effectOrNil()
}

// FillPath returns a path representing the path if it was stroked. resScale determines the precision used. Values >1
// increase precision, while those <1 reduce precision to favor speed and size. If hairline returns true, the path
// represents a hairline, otherwise it represents a fill.
func (p *Paint) FillPath(path *Path, resScale float32) (result *Path, hairline bool) {
	result = NewPath()
	isFill := p.paint.FillPath(path.path, result.path, nil, resScale)
	return result, !isFill
}

// FillPathWithCull returns a path representing the path if it was stroked. cullRect, if not nil, will prune any parts
// outside of the rect. resScale determines the precision used. Values >1 increase precision, while those <1 reduce
// precision to favor speed and size. If hairline returns true, the path represents a hairline, otherwise it represents
// a fill.
func (p *Paint) FillPathWithCull(path *Path, cullRect *geom.Rect, resScale float32) (result *Path, hairline bool) {
	result = NewPath()
	var cull *skgeom.Rect
	if cullRect != nil {
		r := toSkRect(*cullRect)
		cull = &r
	}
	isFill := p.paint.FillPath(path.path, result.path, cull, resScale)
	return result, !isFill
}
