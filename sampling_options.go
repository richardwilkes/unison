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
	"unsafe"

	"github.com/richardwilkes/unison/enums/filtermode"
	"github.com/richardwilkes/unison/enums/mipmapmode"
	"github.com/richardwilkes/unison/internal/skia"
)

// All of the structures and constants in this file must match the equivalents in Skia

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

func (s *SamplingOptions) skSamplingOptions() skia.SamplingOptions {
	if s == nil {
		return defaultSampling.skSamplingOptions()
	}
	return skia.SamplingOptions(unsafe.Pointer(s)) //nolint:gosec // Needed to cast to the skia type
}
