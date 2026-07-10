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
	"testing"

	"github.com/richardwilkes/canvas/shaders"
	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/unison/enums/filtermode"
	"github.com/richardwilkes/unison/enums/mipmapmode"
)

// TestSkSamplingOptionsConversion verifies that skSamplingOptions faithfully copies every field into the canvas
// equivalent, both for a populated receiver and for the nil-receiver default path.
func TestSkSamplingOptionsConversion(t *testing.T) {
	c := check.New(t)

	opts := &SamplingOptions{
		MaxAniso:       4,
		UseCubic:       true,
		CubicResampler: MitchellResampler(),
		FilterMode:     filtermode.Linear,
		MipMapMode:     mipmapmode.Linear,
	}
	got := opts.skSamplingOptions()
	c.Equal(shaders.SamplingOptions{
		MaxAniso: opts.MaxAniso,
		UseCubic: opts.UseCubic,
		Cubic:    shaders.CubicResampler{B: opts.CubicResampler.B, C: opts.CubicResampler.C},
		Filter:   shaders.FilterMode(opts.FilterMode),
		Mipmap:   shaders.MipmapMode(opts.MipMapMode),
	}, got)

	// The nil receiver must resolve to the zero-value default rather than panic.
	var nilOpts *SamplingOptions
	c.Equal(shaders.SamplingOptions{}, nilOpts.skSamplingOptions())
}

// TestSkSamplingOptionsNoEscape guards the efficiency fix: skSamplingOptions returns by value, so neither the populated
// nor the nil-receiver default path should heap-allocate on the hot image-draw route.
func TestSkSamplingOptionsNoEscape(t *testing.T) {
	c := check.New(t)

	opts := &SamplingOptions{FilterMode: filtermode.Linear, MipMapMode: mipmapmode.Linear}
	var sink shaders.SamplingOptions
	c.Equal(0.0, testing.AllocsPerRun(100, func() {
		sink = opts.skSamplingOptions()
	}))

	var nilOpts *SamplingOptions
	c.Equal(0.0, testing.AllocsPerRun(100, func() {
		sink = nilOpts.skSamplingOptions()
	}))
	_ = sink
}
