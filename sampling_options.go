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
	"github.com/richardwilkes/canvas/shaders"
	"github.com/richardwilkes/unison/enums/filtermode"
	"github.com/richardwilkes/unison/enums/mipmapmode"
)

// All of the structures and constants in this file must match the equivalents in canvas

var defaultSampling SamplingOptions

// CubicResampler holds the parameters for cubic resampling.
type CubicResampler struct {
	B float32
	C float32
}

// MitchellResampler is the standard "Mitchell" filter.
func MitchellResampler() CubicResampler {
	return CubicResampler{B: float32(1.0 / 3.0), C: float32(1.0 / 3.0)}
}

// CatmullRomResampler is the standard "Catmull-Rom" filter.
func CatmullRomResampler() CubicResampler {
	return CubicResampler{C: 0.5}
}

// SamplingOptions controls how images are sampled.
type SamplingOptions struct {
	MaxAniso       int32
	UseCubic       bool
	_              [3]bool
	CubicResampler CubicResampler
	FilterMode     filtermode.Enum
	MipMapMode     mipmapmode.Enum
}

// TODO: Replace with direct use
func (s *SamplingOptions) skSamplingOptions() *shaders.SamplingOptions {
	if s == nil {
		return defaultSampling.skSamplingOptions()
	}
	return &shaders.SamplingOptions{
		MaxAniso: s.MaxAniso,
		UseCubic: s.UseCubic,
		Cubic:    shaders.CubicResampler{B: s.CubicResampler.B, C: s.CubicResampler.C},
		Filter:   shaders.FilterMode(s.FilterMode),
		Mipmap:   shaders.MipmapMode(s.MipMapMode),
	}
}
