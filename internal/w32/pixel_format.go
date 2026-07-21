// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package w32

// This file deliberately has no _windows suffix so that the pixel format selection logic can be tested on any
// platform.

// https://learn.microsoft.com/en-us/windows/win32/api/wingdi/ns-wingdi-pixelformatdescriptor
const (
	PFD_TYPE_RGBA             = 0
	PFD_TYPE_COLORINDEX       = 1
	PFD_DOUBLEBUFFER          = 0x00000001
	PFD_STEREO                = 0x00000002
	PFD_DRAW_TO_WINDOW        = 0x00000004
	PFD_DRAW_TO_BITMAP        = 0x00000008
	PFD_SUPPORT_GDI           = 0x00000010
	PFD_SUPPORT_OPENGL        = 0x00000020
	PFD_GENERIC_FORMAT        = 0x00000040
	PFD_NEED_PALETTE          = 0x00000080
	PFD_NEED_SYSTEM_PALETTE   = 0x00000100
	PFD_SWAP_EXCHANGE         = 0x00000200
	PFD_SWAP_COPY             = 0x00000400
	PFD_SWAP_LAYER_BUFFERS    = 0x00000800
	PFD_GENERIC_ACCELERATED   = 0x00001000
	PFD_DEPTH_DONTCARE        = 0x20000000
	PFD_DOUBLEBUFFER_DONTCARE = 0x40000000
	PFD_STEREO_DONTCARE       = 0x80000000
)

// PIXELFORMATDESCRIPTOR http://msdn.microsoft.com/en-us/library/windows/desktop/dd368826.aspx
type PIXELFORMATDESCRIPTOR struct {
	Size                   uint16
	Version                uint16
	DwFlags                uint32
	IPixelType             byte
	ColorBits              byte
	RedBits, RedShift      byte
	GreenBits, GreenShift  byte
	BlueBits, BlueShift    byte
	AlphaBits, AlphaShift  byte
	AccumBits              byte
	AccumRedBits           byte
	AccumGreenBits         byte
	AccumBlueBits          byte
	AccumAlphaBits         byte
	DepthBits, StencilBits byte
	AuxBuffers             byte
	ILayerType             byte
	Reserved               byte
	DwLayerMask            uint32
	DwVisibleMask          uint32
	DwDamageMask           uint32
}

// PixelFormatSuitableForOpenGL reports whether a pixel format is acceptable for the OpenGL rendering pipeline: a
// hardware-accelerated (either an ICD format, which sets neither generic flag, or a generic format the driver
// accelerates), double-buffered RGBA format drawable to a window, with 8 bits per channel, a 24-bit depth buffer and
// an 8-bit stencil buffer. Note that any format passing this test cannot also support GDI drawing: PFD_DOUBLEBUFFER
// and PFD_SUPPORT_GDI are mutually exclusive, which is why such a format must never be committed to a window that may
// end up painted by the CPU rendering fallback.
func PixelFormatSuitableForOpenGL(pfd *PIXELFORMATDESCRIPTOR) bool {
	if pfd.DwFlags&PFD_DRAW_TO_WINDOW == 0 || pfd.DwFlags&PFD_SUPPORT_OPENGL == 0 ||
		pfd.DwFlags&PFD_DOUBLEBUFFER == 0 {
		return false
	}
	if pfd.DwFlags&PFD_GENERIC_ACCELERATED == 0 && pfd.DwFlags&PFD_GENERIC_FORMAT != 0 {
		return false
	}
	return pfd.IPixelType == PFD_TYPE_RGBA &&
		pfd.RedBits == 8 && pfd.GreenBits == 8 && pfd.BlueBits == 8 && pfd.AlphaBits == 8 &&
		pfd.DepthBits == 24 && pfd.StencilBits == 8
}
