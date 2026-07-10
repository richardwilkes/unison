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
	"github.com/richardwilkes/canvas/raster"
	"github.com/richardwilkes/canvas/shaders"
	"github.com/richardwilkes/canvas/skcolor"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/enums/blendmode"
	"github.com/richardwilkes/unison/enums/tilemode"
)

// Shader specifies the source color(s) for what is being drawn. If a paint has no shader, then the paint's color is
// used. If the paint has a shader, then the shader's color(s) are use instead, but they are modulated by the paint's
// alpha. This makes it easy to create a shader once (e.g. bitmap tiling or gradient) and then change its transparency
// without having to modify the original shader... only the paint's alpha needs to be modified.
type Shader struct {
	shader shaders.Shader
}

func newShader(shader shaders.Shader) *Shader {
	if shader == nil {
		return nil
	}
	return &Shader{shader: shader}
}

func (s *Shader) shaderOrNil() shaders.Shader {
	if s == nil {
		return nil
	}
	return s.shader
}

// NewColorShader creates a new color Shader.
func NewColorShader(color Color) *Shader {
	return newShader(shaders.NewColor(skcolor.Color(color)))
}

// NewBlendShader creates a new blend Shader.
func NewBlendShader(blendMode blendmode.Enum, dst, src *Shader) *Shader {
	return newShader(shaders.NewBlend(raster.BlendMode(blendMode), dst.shader, src.shader))
}

// NewLinearGradientShader creates a new linear gradient Shader. matrix may be nil.
func NewLinearGradientShader(start, end geom.Point, colors []Color, colorPos []float32, tileMode tilemode.Enum, matrix geom.Matrix) *Shader {
	return newShader(shaders.NewLinearGradient(toSkPoint(start), toSkPoint(end), toSkColors(colors), colorPos,
		shaders.TileMode(tileMode), toSkMatrixPtr(matrix)))
}

// NewRadialGradientShader creates a new radial gradient Shader. matrix may be nil.
func NewRadialGradientShader(center geom.Point, radius float32, colors []Color, colorPos []float32, tileMode tilemode.Enum, matrix geom.Matrix) *Shader {
	return newShader(shaders.NewRadialGradient(toSkPoint(center), radius, toSkColors(colors), colorPos,
		shaders.TileMode(tileMode), toSkMatrixPtr(matrix)))
}

// NewSweepGradientShader creates a new sweep gradient Shader. matrix may be nil.
func NewSweepGradientShader(center geom.Point, startAngle, endAngle float32, colors []Color, colorPos []float32, tileMode tilemode.Enum, matrix geom.Matrix) *Shader {
	return newShader(shaders.NewSweepGradient(toSkPoint(center), toSkColors(colors), colorPos,
		shaders.TileMode(tileMode), startAngle, endAngle, toSkMatrixPtr(matrix)))
}

// New2PtConicalGradientShader creates a new 2-point conical gradient Shader. matrix may be nil.
func New2PtConicalGradientShader(startPt, endPt geom.Point, startRadius, endRadius float32, colors []Color, colorPos []float32, tileMode tilemode.Enum, matrix geom.Matrix) *Shader {
	return newShader(shaders.NewTwoPointConicalGradient(toSkPoint(startPt), startRadius, toSkPoint(endPt),
		endRadius, toSkColors(colors), colorPos, shaders.TileMode(tileMode), toSkMatrixPtr(matrix)))
}

// NewFractalPerlinNoiseShader creates a new fractal perlin noise Shader.
func NewFractalPerlinNoiseShader(baseFreqX, baseFreqY, seed float32, numOctaves, tileWidth, tileHeight int) *Shader {
	return newShader(shaders.NewFractalNoise(baseFreqX, baseFreqY, numOctaves, seed, int32(tileWidth),
		int32(tileHeight)))
}

// NewTurbulencePerlinNoiseShader creates a new turbulence perlin noise Shader.
func NewTurbulencePerlinNoiseShader(baseFreqX, baseFreqY, seed float32, numOctaves, tileWidth, tileHeight int) *Shader {
	return newShader(shaders.NewTurbulence(baseFreqX, baseFreqY, numOctaves, seed, int32(tileWidth),
		int32(tileHeight)))
}

// NewImageShader creates a new image Shader. If canvas is not nil, a hardware-accellerated image will be used if
// possible.
func NewImageShader(canvas *Canvas, img *Image, tileModeX, tileModeY tilemode.Enum, sampling *SamplingOptions, matrix geom.Matrix) *Shader {
	return newShader(shaders.NewImageDrawable(img.imageForCanvas(canvas), shaders.TileMode(tileModeX),
		shaders.TileMode(tileModeY), sampling.skSamplingOptions(), toSkMatrixPtr(matrix)))
}

// NewWithLocalMatrix creates a new copy of this shader with a local matrix applied.
func (s *Shader) NewWithLocalMatrix(matrix geom.Matrix) *Shader {
	return newShader(shaders.NewWithLocalMatrix(s.shader, toSkMatrix(matrix)))
}

// NewWithColorFilter creates a new copy of this shader with a color filter applied.
func (s *Shader) NewWithColorFilter(filter *ColorFilter) *Shader {
	return newShader(shaders.NewWithColorFilter(s.shader, filter.filter))
}
