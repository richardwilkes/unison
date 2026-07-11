// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package cocoa

import (
	"runtime"
	"unsafe"

	"github.com/ebitengine/purego/objc"
)

// NSOpenGLPixelFormatAttribute values (uint32). All values were verified against the macOS SDK headers by compiling
// and running an Objective-C program that printed them. The API is deprecated by Apple but fully functional; unison
// keeps using it deliberately (see plan.md's non-goals).
const (
	nsOpenGLPFAColorSize          = 8
	nsOpenGLPFAAlphaSize          = 11
	nsOpenGLPFADepthSize          = 12
	nsOpenGLPFAStencilSize        = 13
	nsOpenGLPFAAccelerated        = 73
	nsOpenGLPFAClosestPolicy      = 74
	nsOpenGLPFAOpenGLProfile      = 99
	nsOpenGLProfileVersion3_2Core = 0x3200
)

// OpenGLPixelFormatRef is a handle to an NSOpenGLPixelFormat. NewOpenGLPixelFormat returns an owned (+1) reference;
// balance it with Release.
type OpenGLPixelFormatRef objc.ID

// NewOpenGLPixelFormat returns a pixel format describing the hardware-accelerated OpenGL 3.2 core profile
// configuration unison renders with (24-bit color, 8-bit alpha, 24-bit depth, 8-bit stencil, closest-match policy),
// or 0 if no matching configuration exists.
func NewOpenGLPixelFormat() OpenGLPixelFormatRef {
	attribs := [...]uint32{
		nsOpenGLPFAAccelerated,
		nsOpenGLPFAClosestPolicy,
		nsOpenGLPFAOpenGLProfile, nsOpenGLProfileVersion3_2Core,
		nsOpenGLPFAColorSize, 24,
		nsOpenGLPFAAlphaSize, 8,
		nsOpenGLPFADepthSize, 24,
		nsOpenGLPFAStencilSize, 8,
		0,
	}
	f := objc.ID(Cls("NSOpenGLPixelFormat")).Send(Sel("alloc")).Send(Sel("initWithAttributes:"),
		unsafe.Pointer(&attribs[0]))
	runtime.KeepAlive(&attribs)
	return OpenGLPixelFormatRef(f)
}

// Release releases the pixel format.
func (f OpenGLPixelFormatRef) Release() {
	Release(objc.ID(f))
}
