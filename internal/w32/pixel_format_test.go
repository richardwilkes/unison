// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package w32

import (
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
)

// glCapablePFD returns a descriptor satisfying every requirement PixelFormatSuitableForOpenGL checks (the shape of a
// typical ICD hardware format, which sets neither generic flag); the tests perturb one property at a time.
func glCapablePFD() PIXELFORMATDESCRIPTOR {
	return PIXELFORMATDESCRIPTOR{
		DwFlags:     PFD_DRAW_TO_WINDOW | PFD_SUPPORT_OPENGL | PFD_DOUBLEBUFFER,
		IPixelType:  PFD_TYPE_RGBA,
		RedBits:     8,
		GreenBits:   8,
		BlueBits:    8,
		AlphaBits:   8,
		DepthBits:   24,
		StencilBits: 8,
	}
}

func TestPixelFormatSuitableForOpenGL(t *testing.T) {
	c := check.New(t)
	for i, one := range []struct {
		mutate   func(pfd *PIXELFORMATDESCRIPTOR)
		desc     string
		expected bool
	}{
		{
			desc:     "ICD hardware format is accepted",
			mutate:   func(_ *PIXELFORMATDESCRIPTOR) {},
			expected: true,
		},
		{
			desc:     "driver-accelerated generic (MCD) format is accepted",
			mutate:   func(pfd *PIXELFORMATDESCRIPTOR) { pfd.DwFlags |= PFD_GENERIC_FORMAT | PFD_GENERIC_ACCELERATED },
			expected: true,
		},
		{
			desc:     "unaccelerated generic software format is rejected",
			mutate:   func(pfd *PIXELFORMATDESCRIPTOR) { pfd.DwFlags |= PFD_GENERIC_FORMAT },
			expected: false,
		},
		{
			desc:     "single-buffered format is rejected",
			mutate:   func(pfd *PIXELFORMATDESCRIPTOR) { pfd.DwFlags &^= PFD_DOUBLEBUFFER },
			expected: false,
		},
		{
			desc: "single-buffered GDI-capable format is rejected",
			mutate: func(pfd *PIXELFORMATDESCRIPTOR) {
				pfd.DwFlags &^= PFD_DOUBLEBUFFER
				pfd.DwFlags |= PFD_SUPPORT_GDI
			},
			expected: false,
		},
		{
			desc:     "format that cannot draw to a window is rejected",
			mutate:   func(pfd *PIXELFORMATDESCRIPTOR) { pfd.DwFlags &^= PFD_DRAW_TO_WINDOW },
			expected: false,
		},
		{
			desc:     "format without OpenGL support is rejected",
			mutate:   func(pfd *PIXELFORMATDESCRIPTOR) { pfd.DwFlags &^= PFD_SUPPORT_OPENGL },
			expected: false,
		},
		{
			desc:     "color-index format is rejected",
			mutate:   func(pfd *PIXELFORMATDESCRIPTOR) { pfd.IPixelType = PFD_TYPE_COLORINDEX },
			expected: false,
		},
		{
			desc:     "format without an alpha channel is rejected",
			mutate:   func(pfd *PIXELFORMATDESCRIPTOR) { pfd.AlphaBits = 0 },
			expected: false,
		},
		{
			desc: "format with 10-bit color channels is rejected",
			mutate: func(pfd *PIXELFORMATDESCRIPTOR) {
				pfd.RedBits = 10
				pfd.GreenBits = 10
				pfd.BlueBits = 10
				pfd.AlphaBits = 2
			},
			expected: false,
		},
		{
			desc:     "format with a 16-bit depth buffer is rejected",
			mutate:   func(pfd *PIXELFORMATDESCRIPTOR) { pfd.DepthBits = 16 },
			expected: false,
		},
		{
			desc:     "format without a stencil buffer is rejected",
			mutate:   func(pfd *PIXELFORMATDESCRIPTOR) { pfd.StencilBits = 0 },
			expected: false,
		},
	} {
		pfd := glCapablePFD()
		one.mutate(&pfd)
		c.Equal(one.expected, PixelFormatSuitableForOpenGL(&pfd), "case %d: %s", i, one.desc)
	}
}

func TestPixelFormatDescriptorString(t *testing.T) {
	c := check.New(t)
	pfd := glCapablePFD()
	pfd.ColorBits = 32
	pfd.DwFlags |= PFD_SUPPORT_COMPOSITION
	c.Equal("flags=0x8025(DOUBLEBUFFER|DRAW_TO_WINDOW|SUPPORT_OPENGL|SUPPORT_COMPOSITION) type=RGBA color=32 "+
		"rgba=8/8/8/8 depth=24 stencil=8 accum=0 aux=0 layer=0", pfd.String())

	// Unknown bits are kept, as a raw value, and a zero descriptor still describes itself.
	pfd = PIXELFORMATDESCRIPTOR{DwFlags: PFD_GENERIC_FORMAT | 0x10000, IPixelType: PFD_TYPE_COLORINDEX}
	c.Equal("flags=0x10040(GENERIC_FORMAT|0x10000) type=COLORINDEX color=0 rgba=0/0/0/0 depth=0 stencil=0 accum=0 "+
		"aux=0 layer=0", pfd.String())
	pfd = PIXELFORMATDESCRIPTOR{IPixelType: 7}
	c.Equal("flags=0x0 type=7 color=0 rgba=0/0/0/0 depth=0 stencil=0 accum=0 aux=0 layer=0", pfd.String())
}
