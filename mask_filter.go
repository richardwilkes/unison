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
	"github.com/richardwilkes/canvas/maskfilter"
	"github.com/richardwilkes/unison/enums/blur"
)

// MaskFilter performs a transformation on the mask before drawing it.
type MaskFilter struct {
	filter maskfilter.MaskFilter
}

func newMaskFilter(filter maskfilter.MaskFilter) *MaskFilter {
	if filter == nil {
		return nil
	}
	return &MaskFilter{filter: filter}
}

func (f *MaskFilter) filterOrNil() maskfilter.MaskFilter {
	if f == nil {
		return nil
	}
	return f.filter
}

// NewBlurMaskFilter returns a new blur mask filter. sigma is the standard deviation of the gaussian blur to apply. Must
// be greater than 0. If respectMatrix is true, the blur's sigma is modified by the current matrix.
func NewBlurMaskFilter(style blur.Enum, sigma float32, respectMatrix bool) *MaskFilter {
	if sigma <= 0 {
		sigma = 1
	}
	return newMaskFilter(maskfilter.NewBlur(maskfilter.BlurStyle(style), sigma, respectMatrix))
}

// NewTableMaskFilter returns a new table mask filter. The table should be 256 elements long. If shorter, it will be
// expanded to 256 elements and the new entries will be filled with 0.
func NewTableMaskFilter(table []byte) *MaskFilter {
	if len(table) < 256 {
		t := make([]byte, 256)
		copy(t, table)
		table = t
	}
	return newMaskFilter(maskfilter.NewTable((*[256]uint8)(table)))
}

// NewGammaMaskFilter returns a new gamma mask filter.
func NewGammaMaskFilter(gamma float32) *MaskFilter {
	return newMaskFilter(maskfilter.NewGamma(gamma))
}

// NewClipMaskFilter returns a new clip mask filter.
func NewClipMaskFilter(minimum, maximum byte) *MaskFilter {
	return newMaskFilter(maskfilter.NewClip(minimum, maximum))
}

// NewShaderMaskFilter returns a new shader mask filter.
func NewShaderMaskFilter(shader *Shader) *MaskFilter {
	return newMaskFilter(maskfilter.NewShader(shader.shader))
}
