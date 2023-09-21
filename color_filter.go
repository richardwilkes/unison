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

	"github.com/richardwilkes/unison/internal/skia"
)

// InvertStyle holds the type inversion.
type InvertStyle int32

// Possible values for InvertStyle.
const (
	NoInvert InvertStyle = iota
	InvertBrightness
	InvertLightness
)

// ColorFilter is called with source colors and return new colors, which are then passed onto the next stage.
type ColorFilter struct {
	filter skia.ColorFilter
}

func newColorFilter(filter skia.ColorFilter) *ColorFilter {
	if filter == nil {
		return nil
	}
	f := &ColorFilter{filter: filter}
	runtime.SetFinalizer(f, func(obj *ColorFilter) {
		ReleaseOnUIThread(func() {
			skia.ColorFilterUnref(obj.filter)
		})
	})
	return f
}

func (f *ColorFilter) filterOrNil() skia.ColorFilter {
	if f == nil {
		return nil
	}
	return f.filter
}

// NewBlendColorFilter returns a new blend color filter.
func NewBlendColorFilter(color Color, blendMode BlendMode) *ColorFilter {
	return newColorFilter(skia.ColorFilterNewMode(skia.Color(color), skia.BlendMode(blendMode)))
}

// NewLightingColorFilter returns a new lighting color filter.
func NewLightingColorFilter(mul, add Color) *ColorFilter {
	return newColorFilter(skia.ColorFilterNewLighting(skia.Color(mul), skia.Color(add)))
}

// NewComposeColorFilter returns a new color filter that combines two other color filters.
func NewComposeColorFilter(outer, inner *ColorFilter) *ColorFilter {
	return newColorFilter(skia.ColorFilterNewCompose(outer.filter, inner.filter))
}

// NewMatrixColorFilter returns a new matrix color filter. array should be 20 long. If smaller, it will be filled with
// zeroes to make it 20.
func NewMatrixColorFilter(array []float32) *ColorFilter {
	if len(array) < 20 {
		a := make([]float32, 20)
		copy(a, array)
		array = a
	}
	return newColorFilter(skia.ColorFilterNewColorMatrix(array))
}

// NewLumaColorFilter returns a new luma color filter.
func NewLumaColorFilter() *ColorFilter {
	return newColorFilter(skia.ColorFilterNewLumaColor())
}

// NewHighContrastColorFilter returns a new high contrast color filter.
func NewHighContrastColorFilter(contrast float32, style InvertStyle, grayscale bool) *ColorFilter {
	return newColorFilter(skia.ColorFilterNewHighContrast(&skia.HighContrastConfig{
		Grayscale:   grayscale,
		InvertStyle: skia.InvertStyle(style),
		Contrast:    contrast,
	}))
}

// NewAlphaFilter returns a new ColorFilter that applies an alpha blend.
func NewAlphaFilter(alpha float32) *ColorFilter {
	return NewMatrixColorFilter([]float32{
		1, 0, 0, 0, 0,
		0, 1, 0, 0, 0,
		0, 0, 1, 0, 0,
		0, 0, 0, alpha, 0,
	})
}

var grayscale30Filter *ColorFilter

// Grayscale30Filter returns a ColorFilter that transforms colors to grayscale and applies a 30% alpha blend.
func Grayscale30Filter() *ColorFilter {
	if grayscale30Filter == nil {
		grayscale30Filter = NewMatrixColorFilter([]float32{
			0.2126, 0.7152, 0.0722, 0, 0,
			0.2126, 0.7152, 0.0722, 0, 0,
			0.2126, 0.7152, 0.0722, 0, 0,
			0, 0, 0, 0.3, 0,
		})
	}
	return grayscale30Filter
}

var alpha30Filter *ColorFilter

// Alpha30Filter returns a ColorFilter that transforms colors by applying a 30% alpha blend.
func Alpha30Filter() *ColorFilter {
	if alpha30Filter == nil {
		alpha30Filter = NewAlphaFilter(0.3)
	}
	return alpha30Filter
}
