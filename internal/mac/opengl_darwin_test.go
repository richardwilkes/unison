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
	"testing"
	"unsafe"

	"github.com/ebitengine/purego/objc"
)

// pixelFormatAttr reads one attribute back from a pixel format via getValues:forAttribute:forVirtualScreen:.
func pixelFormatAttr(f OpenGLPixelFormatRef, attr uint32) int32 {
	var val int32
	objc.ID(f).Send(Sel("getValues:forAttribute:forVirtualScreen:"), unsafe.Pointer(&val), attr, int32(0))
	runtime.KeepAlive(&val)
	return val
}

// contextSurfaceOpacity reads the surface-opacity parameter back from a context via getValues:forParameter:.
func contextSurfaceOpacity(c OpenGLContextRef) int32 {
	var val int32
	objc.ID(c).Send(Sel("getValues:forParameter:"), unsafe.Pointer(&val),
		int64(nsOpenGLContextParameterSurfaceOpacity))
	runtime.KeepAlive(&val)
	return val
}

// newTestPixelFormat returns the pixel format unison renders with, failing the test if none exists (every Mac that
// can run the tests has a hardware-accelerated GL 3.2 core configuration).
func newTestPixelFormat(t *testing.T) OpenGLPixelFormatRef {
	t.Helper()
	f := NewOpenGLPixelFormat()
	if f == 0 {
		t.Fatal("NewOpenGLPixelFormat returned 0")
	}
	return f
}

// TestNewOpenGLPixelFormat proves the attribute list crossed the purego boundary intact: every attribute the old
// Objective-C bridge requested must be queryable from the resulting format with at least the requested value.
func TestNewOpenGLPixelFormat(t *testing.T) {
	runOnMain(func() {
		f := newTestPixelFormat(t)
		defer f.Release()
		for _, c := range []struct {
			name string
			attr uint32
			min  int32
		}{
			{"color size", nsOpenGLPFAColorSize, 24},
			{"alpha size", nsOpenGLPFAAlphaSize, 8},
			{"depth size", nsOpenGLPFADepthSize, 24},
			{"stencil size", nsOpenGLPFAStencilSize, 8},
			{"accelerated", nsOpenGLPFAAccelerated, 1},
		} {
			if got := pixelFormatAttr(f, c.attr); got < c.min {
				t.Errorf("%s = %d, want >= %d", c.name, got, c.min)
			}
		}
		if got := pixelFormatAttr(f, nsOpenGLPFAOpenGLProfile); got != nsOpenGLProfileVersion3_2Core {
			t.Errorf("profile = %#x, want %#x", got, nsOpenGLProfileVersion3_2Core)
		}
	})
}

// TestNewOpenGLContext proves context creation against a real window/view pair the way unison's apiCreate does:
// the context must exist, be attached to the view, leave the view flagged for best-resolution surfaces, default to
// an opaque surface, and support the share-context creation path.
func TestNewOpenGLContext(t *testing.T) {
	runOnMain(func() {
		_, v, cleanup := newTestWindowAndView(t)
		defer cleanup()
		f := newTestPixelFormat(t)
		defer f.Release()
		ctx := NewOpenGLContext(v, f, 0, false)
		if ctx == 0 {
			t.Fatal("NewOpenGLContext returned 0")
		}
		defer ctx.Release()
		if got := View(objc.ID(ctx).Send(Sel("view"))); got != v {
			t.Errorf("context view = %#x, want %#x", got, v)
		}
		if !objc.Send[bool](objc.ID(v), Sel("wantsBestResolutionOpenGLSurface")) {
			t.Error("view wantsBestResolutionOpenGLSurface = false, want true")
		}
		if got := contextSurfaceOpacity(ctx); got != 1 {
			t.Errorf("opaque context surface opacity = %d, want 1", got)
		}
		// The share-context path (unison passes 0 today, but the parameter is part of the exported API).
		shared := NewOpenGLContext(v, f, ctx, false)
		if shared == 0 {
			t.Fatal("NewOpenGLContext with a share context returned 0")
		}
		shared.Release()
	})
}

// TestNewOpenGLContextTransparent proves the surface-opacity handling that moved from Objective-C to Go: a context
// created with transparent=true must read back opacity 0, exactly what the old bridge's setValues:forParameter: did.
func TestNewOpenGLContextTransparent(t *testing.T) {
	runOnMain(func() {
		w, v, cleanup := newTestWindowAndView(t)
		defer cleanup()
		w.SetTransparent() // matches production: unison marks the window transparent before creating the context
		f := newTestPixelFormat(t)
		defer f.Release()
		ctx := NewOpenGLContext(v, f, 0, true)
		if ctx == 0 {
			t.Fatal("NewOpenGLContext returned 0")
		}
		defer ctx.Release()
		if got := contextSurfaceOpacity(ctx); got != 0 {
			t.Errorf("transparent context surface opacity = %d, want 0", got)
		}
	})
}

// TestOpenGLContextCurrent proves the make-current contract unison's render loop depends on: MakeCurrent binds the
// context to the calling thread, MakeCurrent on a 0 handle clears it (the old bridge's openGLMakeCurrent(nil)
// behavior), and ClearOpenGLCurrentContext clears it explicitly. Update and FlushBuffer are exercised against the
// current context with the window ordered in, the shape of every frame unison draws.
func TestOpenGLContextCurrent(t *testing.T) {
	runOnMain(func() {
		w, v, cleanup := newTestWindowAndView(t)
		defer cleanup()
		f := newTestPixelFormat(t)
		defer f.Release()
		ctx := NewOpenGLContext(v, f, 0, false)
		if ctx == 0 {
			t.Fatal("NewOpenGLContext returned 0")
		}
		defer ctx.Release()
		defer ClearOpenGLCurrentContext() // never leak a current context into other tests
		currentCtx := func() objc.ID { return objc.ID(Cls("NSOpenGLContext")).Send(Sel("currentContext")) }

		ctx.MakeCurrent()
		if got := currentCtx(); got != objc.ID(ctx) {
			t.Fatalf("currentContext = %#x after MakeCurrent, want %#x", got, ctx)
		}
		OpenGLContextRef(0).MakeCurrent()
		if got := currentCtx(); got != 0 {
			t.Errorf("currentContext = %#x after MakeCurrent on 0 handle, want 0", got)
		}
		ctx.MakeCurrent()
		ClearOpenGLCurrentContext()
		if got := currentCtx(); got != 0 {
			t.Errorf("currentContext = %#x after ClearOpenGLCurrentContext, want 0", got)
		}

		// One frame's worth of calls with a live drawable: order the window in, sync the context to the view, draw
		// nothing, and flush. None of these report errors; this is a no-crash/no-hang proof through real msgSends.
		w.MakeKeyAndOrderFront()
		ctx.Update()
		ctx.MakeCurrent()
		ctx.FlushBuffer()
		w.OrderOut()
	})
}
