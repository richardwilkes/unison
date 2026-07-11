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
	"sync"
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

// Renderer modes the GL tests can run in, in preference order.
const (
	glModeNone        = iota // no OpenGL renderer at all — the tests skip
	glModeAccelerated        // hardware-accelerated renderer, via the production NewOpenGLPixelFormat
	glModeSoftware           // Apple software renderer, via the test-only fallback pixel format
)

// Constants used only by the software-renderer test fallback (production always requires NSOpenGLPFAAccelerated).
// All values were verified against the macOS SDK headers by compiling and running an Objective-C program that
// printed them: NSOpenGLPFARendererID=70, kCGLRendererGenericFloatID=0x20400, kCGLRendererIDMatchingMask=0xfe7f00.
// Renderer IDs read back from a live pixel format carry variant bits (0x1020400 for the software renderer on this
// SDK), so comparisons must apply the matching mask.
const (
	nsOpenGLPFARendererID     = 70
	cglRendererGenericFloatID = 0x20400
	cglRendererIDMatchingMask = 0xfe7f00
)

type glTestEnv struct {
	rendererID int32 // renderer of the pixel format the tests will use, read back from the probe format
	mode       int
}

// probeGLEnv determines once per process which renderer the GL tests run on. It prefers the hardware-accelerated
// renderer (present on every physical Mac, even with the login session locked): that probe requests the single
// attribute NSOpenGLPFAAccelerated through raw msgSends, independent of NewOpenGLPixelFormat's attribute-list
// handling, so a genuine regression in the ported code still fails — not skips — wherever a GPU exists. Headless CI
// VMs (both the Intel and Apple-silicon GitHub runners) offer no accelerated renderer, so there the probe falls back
// to the Apple software renderer, using the exact attribute list newSoftwareGLPixelFormat builds — success
// guarantees newTestPixelFormat succeeds later. Calls runOnMain, so it must first run from a test goroutine (which
// requireGL ensures); after that the cached value is safe to read anywhere.
var probeGLEnv = sync.OnceValue(func() glTestEnv {
	env := glTestEnv{mode: glModeNone}
	runOnMain(func() {
		attrs := [...]uint32{nsOpenGLPFAAccelerated, 0}
		f := objc.ID(Cls("NSOpenGLPixelFormat")).Send(Sel("alloc")).Send(Sel("initWithAttributes:"),
			unsafe.Pointer(&attrs[0]))
		runtime.KeepAlive(&attrs)
		if f != 0 {
			env.mode = glModeAccelerated
		} else {
			if f = objc.ID(newSoftwareGLPixelFormat()); f == 0 {
				return
			}
			env.mode = glModeSoftware
		}
		env.rendererID = pixelFormatAttr(OpenGLPixelFormatRef(f), nsOpenGLPFARendererID)
		Release(f)
	})
	return env
})

// requireGL skips the test only when the environment has no usable OpenGL renderer at all — neither
// hardware-accelerated nor the Apple software renderer — and logs which renderer the test runs on otherwise. It
// must be called from the test goroutine, before runOnMain: t.Skip calls runtime.Goexit and must never run inside a
// runOnMain closure.
func requireGL(t *testing.T) {
	t.Helper()
	switch env := probeGLEnv(); env.mode {
	case glModeAccelerated:
		t.Logf("running on the hardware-accelerated renderer (id %#x)", uint32(env.rendererID))
	case glModeSoftware:
		t.Logf("no hardware-accelerated renderer here; running on the Apple software renderer (id %#x)",
			uint32(env.rendererID))
	default:
		t.Skip("no OpenGL renderer in this environment, not even the software renderer")
	}
}

// newSoftwareGLPixelFormat builds the test-only fallback pixel format: NewOpenGLPixelFormat's exact attribute list
// with NSOpenGLPFAAccelerated replaced by an explicit request for the Apple software renderer
// (kCGLRendererGenericFloatID). It lets the GL tests exercise the ported context code on machines with no
// accelerated renderer; production never uses it.
func newSoftwareGLPixelFormat() OpenGLPixelFormatRef {
	attribs := [...]uint32{
		nsOpenGLPFARendererID, cglRendererGenericFloatID,
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

// newTestPixelFormat returns the pixel format the GL tests render with: the production NewOpenGLPixelFormat
// wherever a hardware-accelerated renderer exists, else the software fallback. Callers must have passed requireGL
// first, so a 0 here is a real defect — the probe already created this exact configuration — and the Fatal is
// pump-safe (see runPumped). requireGL also guarantees probeGLEnv is already cached, making it safe to read here
// inside a runOnMain closure.
func newTestPixelFormat(t *testing.T) OpenGLPixelFormatRef {
	t.Helper()
	var f OpenGLPixelFormatRef
	if probeGLEnv().mode == glModeAccelerated {
		f = NewOpenGLPixelFormat()
	} else {
		f = newSoftwareGLPixelFormat()
	}
	if f == 0 {
		t.Fatal("could not create the test pixel format")
	}
	return f
}

// TestNewOpenGLPixelFormat proves the attribute list crossed the purego boundary intact: every attribute requested
// must be queryable from the resulting format with at least the requested value. On machines with a GPU this
// exercises the production NewOpenGLPixelFormat list (including NSOpenGLPFAAccelerated); without one it exercises
// the software fallback list, asserting the explicitly requested renderer was honored instead.
func TestNewOpenGLPixelFormat(t *testing.T) {
	requireGL(t)
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
		} {
			if got := pixelFormatAttr(f, c.attr); got < c.min {
				t.Errorf("%s = %d, want >= %d", c.name, got, c.min)
			}
		}
		if got := pixelFormatAttr(f, nsOpenGLPFAOpenGLProfile); got != nsOpenGLProfileVersion3_2Core {
			t.Errorf("profile = %#x, want %#x", got, nsOpenGLProfileVersion3_2Core)
		}
		if probeGLEnv().mode == glModeAccelerated {
			if got := pixelFormatAttr(f, nsOpenGLPFAAccelerated); got < 1 {
				t.Errorf("accelerated = %d, want >= 1", got)
			}
		} else if got := pixelFormatAttr(f, nsOpenGLPFARendererID); got&cglRendererIDMatchingMask !=
			cglRendererGenericFloatID&cglRendererIDMatchingMask {
			t.Errorf("rendererID = %#x, want the generic-float software renderer (%#x under mask %#x)",
				uint32(got), uint32(cglRendererGenericFloatID), uint32(cglRendererIDMatchingMask))
		}
	})
}

// TestNewOpenGLContext proves context creation against a real window/view pair the way unison's apiCreate does:
// the context must exist, be attached to the view, leave the view flagged for best-resolution surfaces, default to
// an opaque surface, and support the share-context creation path.
func TestNewOpenGLContext(t *testing.T) {
	requireGL(t)
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
	requireGL(t)
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
	requireGL(t)
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
