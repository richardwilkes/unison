// Copyright ©2021 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"unsafe"

	"github.com/richardwilkes/unison/internal/skia"
)

// All of the structures and constants in this file must match the equivalents in Skia

var defaultSampling SamplingOptions

// FilterMode holds the type of sampling to be done.
type FilterMode int32

// Possible values for FilterMode.
const (
	FilterModeNearest FilterMode = iota // single sample point (nearest neighbor)
	FilterModeLinear                    // interporate between 2x2 sample points (bilinear interpolation)
)

// MipMapMode holds the type of mipmapping to be done.
type MipMapMode int32

// Possible values for MipMapMode.
const (
	MipMapModeNone    MipMapMode = iota // ignore mipmap levels, sample from the "base"
	MipMapModeNearest                   // sample from the nearest level
	MipMapModeLinear                    // interpolate between the two nearest levels
)

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
	UseCubic       bool
	_              [3]bool
	CubicResampler CubicResampler
	FilterMode     FilterMode
	MipMapMode     MipMapMode
}

func (s *SamplingOptions) skSamplingOptions() skia.SamplingOptions {
	if s == nil {
		return defaultSampling.skSamplingOptions()
	}
	return skia.SamplingOptions(unsafe.Pointer(s)) //nolint:gosec // Needed to cast to the skia type
}
