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
	"runtime"
	"unsafe"

	"github.com/richardwilkes/unison/enums/blendmode"
	"github.com/richardwilkes/unison/enums/tilemode"
	"github.com/richardwilkes/unison/internal/skia"
)

// Shader specifies the source color(s) for what is being drawn. If a paint has no shader, then the paint's color is
// used. If the paint has a shader, then the shader's color(s) are use instead, but they are modulated by the paint's
// alpha. This makes it easy to create a shader once (e.g. bitmap tiling or gradient) and then change its transparency
// without having to modify the original shader... only the paint's alpha needs to be modified.
type Shader struct {
	shader skia.Shader
}

func newShader(shader skia.Shader) *Shader {
	if shader == nil {
		return nil
	}
	s := &Shader{shader: shader}
	runtime.SetFinalizer(s, func(obj *Shader) {
		ReleaseOnUIThread(func() {
			skia.ShaderUnref(obj.shader)
		})
	})
	return s
}

func (s *Shader) shaderOrNil() skia.Shader {
	if s == nil {
		return nil
	}
	return s.shader
}

// NewColorShader creates a new color Shader.
func NewColorShader(color Color) *Shader {
	return newShader(skia.ShaderNewColor(skia.Color(color)))
}

// NewBlendShader creates a new blend Shader.
func NewBlendShader(blendMode blendmode.Enum, dst, src *Shader) *Shader {
	return newShader(skia.ShaderNewBlend(skia.BlendMode(blendMode), dst.shader, src.shader))
}

// NewLinearGradientShader creates a new linear gradient Shader. matrix may be nil.
func NewLinearGradientShader(start, end Point, colors []Color, colorPos []float32, tileMode tilemode.Enum, matrix Matrix) *Shader {
	return newShader(skia.ShaderNewLinearGradient(start, end,
		unsafe.Slice((*skia.Color)(unsafe.Pointer(&colors[0])), len(colors)),
		colorPos, skia.TileMode(tileMode), matrix))
}

// NewRadialGradientShader creates a new radial gradient Shader. matrix may be nil.
func NewRadialGradientShader(center Point, radius float32, colors []Color, colorPos []float32, tileMode tilemode.Enum, matrix Matrix) *Shader {
	return newShader(skia.ShaderNewRadialGradient(center, radius,
		unsafe.Slice((*skia.Color)(unsafe.Pointer(&colors[0])), len(colors)),
		colorPos, skia.TileMode(tileMode), matrix))
}

// NewSweepGradientShader creates a new sweep gradient Shader. matrix may be nil.
func NewSweepGradientShader(center Point, startAngle, endAngle float32, colors []Color, colorPos []float32, tileMode tilemode.Enum, matrix Matrix) *Shader {
	return newShader(skia.ShaderNewSweepGradient(center, startAngle, endAngle,
		unsafe.Slice((*skia.Color)(unsafe.Pointer(&colors[0])), len(colors)),
		colorPos, skia.TileMode(tileMode), matrix))
}

// New2PtConicalGradientShader creates a new 2-point conical gradient Shader. matrix may be nil.
func New2PtConicalGradientShader(startPt, endPt Point, startRadius, endRadius float32, colors []Color, colorPos []float32, tileMode tilemode.Enum, matrix Matrix) *Shader {
	return newShader(skia.ShaderNewTwoPointConicalGradient(startPt, endPt, startRadius, endRadius,
		unsafe.Slice((*skia.Color)(unsafe.Pointer(&colors[0])), len(colors)),
		colorPos, skia.TileMode(tileMode), matrix))
}

// NewFractalPerlinNoiseShader creates a new fractal perlin noise Shader.
func NewFractalPerlinNoiseShader(baseFreqX, baseFreqY, seed float32, numOctaves, tileWidth, tileHeight int) *Shader {
	return newShader(skia.ShaderNewPerlinNoiseFractalNoise(baseFreqX, baseFreqY, seed, numOctaves, skia.ISize{
		Width:  int32(tileWidth),
		Height: int32(tileHeight),
	}))
}

// NewTurbulencePerlinNoiseShader creates a new turbulence perlin noise Shader.
func NewTurbulencePerlinNoiseShader(baseFreqX, baseFreqY, seed float32, numOctaves, tileWidth, tileHeight int) *Shader {
	return newShader(skia.ShaderNewPerlinNoiseTurbulence(baseFreqX, baseFreqY, seed, numOctaves, skia.ISize{
		Width:  int32(tileWidth),
		Height: int32(tileHeight),
	}))
}

// NewImageShader creates a new image Shader. If canvas is not nil, a hardware-accellerated image will be used if
// possible.
func NewImageShader(canvas *Canvas, img *Image, tileModeX, tileModeY tilemode.Enum, sampling *SamplingOptions, matrix Matrix) *Shader {
	var image skia.Image
	ref := img.ref()
	if canvas == nil {
		image = ref.img
	} else {
		image = ref.contextImg(canvas.surface)
	}
	return newShader(skia.ImageMakeShader(image, skia.TileMode(tileModeX), skia.TileMode(tileModeY),
		sampling.skSamplingOptions(), matrix))
}

// NewWithLocalMatrix creates a new copy of this shader with a local matrix applied.
func (s *Shader) NewWithLocalMatrix(matrix Matrix) *Shader {
	return newShader(skia.ShaderWithLocalMatrix(s.shader, matrix))
}

// NewWithColorFilter creates a new copy of this shader with a color filter applied.
func (s *Shader) NewWithColorFilter(filter *ColorFilter) *Shader {
	return newShader(skia.ShaderWithColorFilter(s.shader, filter.filter))
}
