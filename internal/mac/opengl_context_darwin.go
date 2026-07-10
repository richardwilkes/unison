// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package mac

import (
	"runtime"
	"unsafe"

	"github.com/ebitengine/purego/objc"
)

// nsOpenGLContextParameterSurfaceOpacity is AppKit's NSOpenGLContextParameterSurfaceOpacity value
// (NSOpenGLContextParameter is NSInteger; the value was verified against the macOS SDK headers by compiling and
// running an Objective-C program that printed it).
const nsOpenGLContextParameterSurfaceOpacity = 236

// OpenGLContextRef is a handle to an NSOpenGLContext. NewOpenGLContext returns an owned (+1) reference; balance it
// with Release.
type OpenGLContextRef objc.ID

// NewOpenGLContext returns a new OpenGL context rendering into the given view, using the given pixel format and
// optionally sharing object state with shareCtx (0 for none), or 0 if the context could not be created. When
// transparent is true, the context's surface opacity is disabled so the window behind shows through, and the view is
// switched to a best-resolution (retina) surface either way — both exactly as the old Objective-C bridge did.
func NewOpenGLContext(view View, pixFmt OpenGLPixelFormatRef, shareCtx OpenGLContextRef, transparent bool) OpenGLContextRef {
	ctx := objc.ID(Cls("NSOpenGLContext")).Send(Sel("alloc")).Send(Sel("initWithFormat:shareContext:"),
		objc.ID(pixFmt), objc.ID(shareCtx))
	if ctx == 0 {
		return 0
	}
	if transparent {
		opaque := int32(0)
		ctx.Send(Sel("setValues:forParameter:"), unsafe.Pointer(&opaque),
			int64(nsOpenGLContextParameterSurfaceOpacity))
		runtime.KeepAlive(&opaque)
	}
	objc.ID(view).Send(Sel("setWantsBestResolutionOpenGLSurface:"), true)
	ctx.Send(Sel("setView:"), objc.ID(view))
	return OpenGLContextRef(ctx)
}

// MakeCurrent makes the context the calling thread's current OpenGL context. Matching the old bridge, calling it on
// a 0 handle clears the current context instead.
func (c OpenGLContextRef) MakeCurrent() {
	if c == 0 {
		ClearOpenGLCurrentContext()
		return
	}
	objc.ID(c).Send(Sel("makeCurrentContext"))
}

// FlushBuffer copies the back buffer to the visible surface.
func (c OpenGLContextRef) FlushBuffer() {
	objc.ID(c).Send(Sel("flushBuffer"))
}

// Update synchronizes the context with the current state of the view it renders into. It must be called when the
// view's window moves or resizes.
func (c OpenGLContextRef) Update() {
	objc.ID(c).Send(Sel("update"))
}

// Release releases the context.
func (c OpenGLContextRef) Release() {
	Release(objc.ID(c))
}

// ClearOpenGLCurrentContext clears the calling thread's current OpenGL context.
func ClearOpenGLCurrentContext() {
	objc.ID(Cls("NSOpenGLContext")).Send(Sel("clearCurrentContext"))
}
