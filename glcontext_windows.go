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
	"fmt"
	"unsafe"

	"github.com/richardwilkes/canvas/gpu/gl"
	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/unison/internal/w32"
	"golang.org/x/sys/windows"
)

type nativeGLContext struct {
	hwnd windows.HWND
	dc   w32.HDC
	rc   w32.HGLRC
}

// w32GLPipelineVerified is set once the full rendering pipeline (an OpenGL 3.2 context plus the library's GL direct
// context) has been proven to work on this machine, letting later window creations skip the redundant direct-context
// probe. Only accessed on the UI thread.
var w32GLPipelineVerified bool

// glPixelFormat is the pixel format selected for the OpenGL rendering pipeline, together with where it stood among
// the candidates the driver offered, so that a failure anywhere in context creation can report exactly what was asked
// of the driver.
type glPixelFormat struct {
	pfd        w32.PIXELFORMATDESCRIPTOR
	index      int32 // 1-based pixel format index
	rank       int32 // 1-based position among the driver's candidates
	candidates int32 // how many candidates the driver offered
}

func (f *glPixelFormat) String() string {
	return fmt.Sprintf("pixel format %d (candidate %d of %d, %v)", f.index, f.rank, f.candidates, &f.pfd)
}

func (c *nativeGLContext) nativeCreate(wnd *Window) error {
	hwnd := wnd.wnd.wnd
	dc := w32.GetDC(hwnd)
	if dc == 0 {
		return errs.New("failed to get device context for window")
	}
	success := false
	defer func() {
		if !success {
			w32.ReleaseDC(hwnd, dc)
		}
	}()
	// Select the pixel format and prove that context creation actually works on disposable hidden windows before
	// touching this window. SetPixelFormat is irreversible for a window, and a GL-capable format requires
	// PFD_DOUBLEBUFFER, which excludes PFD_SUPPORT_GDI — so once the format is committed, GDI can no longer paint the
	// window, and GDI is exactly what nativePresentCPUPixels uses when rendering falls back to the CPU. Probing first
	// means a failure at any step leaves this window format-free and therefore still paintable by the fallback.
	f, rc, err := w32CreateGLContextOnProbeWindows()
	if err != nil {
		return errs.Newf("%s [OpenGL modules: %s]", err, w32.OpenGLModuleReport())
	}
	if err = w32.SetPixelFormat(dc, f.index, &f.pfd); err != nil {
		// A SetPixelFormat failure leaves the window without a pixel format, so the CPU fallback remains safe.
		w32.WglDeleteContext(rc)
		return errs.Newf("failed to set %v for OpenGL context: %s", &f, err)
	}
	c.hwnd = hwnd
	c.dc = dc
	c.rc = rc
	success = true
	return nil
}

// w32GLProbeWindow is a hidden, throwaway window on which pixel formats and contexts can be exercised without risk to
// a real window.
type w32GLProbeWindow struct {
	hwnd windows.HWND
	dc   w32.HDC
}

func w32NewGLProbeWindow() (*w32GLProbeWindow, error) {
	hwnd := w32.CreateWindowExW(0, wndProcClassName, "", w32.WS_CLIPSIBLINGS|w32.WS_CLIPCHILDREN, 0, 0, 1, 1, 0, 0,
		w32MainInstance, 0)
	if hwnd == 0 {
		return nil, errs.New("failed to create probe window for OpenGL context")
	}
	dc := w32.GetDC(hwnd)
	if dc == 0 {
		w32.DestroyWindow(hwnd)
		return nil, errs.New("failed to get device context for probe window")
	}
	return &w32GLProbeWindow{hwnd: hwnd, dc: dc}, nil
}

// destroy releases the window. No context may be current on its device context at this point.
func (p *w32GLProbeWindow) destroy() {
	w32.ReleaseDC(p.hwnd, p.dc)
	w32.DestroyWindow(p.hwnd)
}

// w32CreateGLContextOnProbeWindows selects the pixel format for the OpenGL rendering pipeline and creates an OpenGL
// 3.2 context for it, using hidden, throwaway windows so that no failure along the way ever commits a format to a real
// window. It follows the bootstrap sequence the WGL extensions require, and that the mainstream toolkits use:
//
//  1. A legacy context is created on a first probe window using the plain format ChoosePixelFormat picks for a
//     double-buffered RGB window. That context exists only to make the extension entry points reachable, since
//     wglGetProcAddress cannot resolve them without a current context.
//  2. wglChoosePixelFormatARB asks the driver itself for the formats that satisfy the pipeline's requirements. This
//     matters: DescribePixelFormat cannot express attributes such as multisampling or the presentation model, so a
//     format that looks right in its PIXELFORMATDESCRIPTOR may be one the driver refuses to create a context on.
//     Selecting by scanning DescribePixelFormat failed exactly that way on Qualcomm Adreno, where wglCreateContext
//     rejected the first matching format with ERROR_INVALID_PIXEL_FORMAT.
//  3. The chosen format is committed to a second probe window, where the 3.2 context is created with
//     wglCreateContextAttribsARB. The first successful call also verifies that the library's GL direct context can be
//     created, since that failure would otherwise be discovered at first canvas preparation, after a real window's
//     format had already been committed.
//
// Since a GL context may be made current with any DC that has the same pixel format on the same device, the returned
// context is usable with a real window once the returned format has been committed to it.
//
// Every failure names the pixel format involved and carries the driver's error code, since a bug report is usually the
// only chance to learn why a driver refused.
func w32CreateGLContextOnProbeWindows() (glPixelFormat, w32.HGLRC, error) {
	var f glPixelFormat
	if err := w32.LoadOpenGL32(); err != nil {
		return f, 0, err
	}
	bootstrap, err := w32NewGLProbeWindow()
	if err != nil {
		return f, 0, err
	}
	defer bootstrap.destroy()
	fakeRC, err := w32CreateBootstrapGLContext(bootstrap.dc)
	if err != nil {
		return f, 0, err
	}
	defer w32.WglDeleteContext(fakeRC)
	defer w32.WglMakeCurrent(0, 0) //nolint:errcheck // Nothing useful can be done about a failure to release
	if f, err = w32ChooseGLPixelFormat(bootstrap.dc); err != nil {
		return f, 0, err
	}
	target, err := w32NewGLProbeWindow()
	if err != nil {
		return f, 0, err
	}
	defer target.destroy()
	rc, err := w32CreateGLContextForPixelFormat(target.dc, &f)
	if err != nil {
		return f, 0, err
	}
	return f, rc, nil
}

// w32CreateBootstrapGLContext creates a legacy OpenGL context on the device context and makes it current, using the
// plain pixel format that ChoosePixelFormat picks for a double-buffered RGB window: the same request every toolkit
// bootstraps with, and thus the one drivers are most certain to honor. A generic (software) format means no hardware
// OpenGL driver is available, which is reported rather than bootstrapped from, since the extensions needed later do not
// exist in the software implementation. The caller owns the returned context and must release it as current.
func w32CreateBootstrapGLContext(dc w32.HDC) (w32.HGLRC, error) {
	pfd := w32.PIXELFORMATDESCRIPTOR{
		Size:       uint16(unsafe.Sizeof(w32.PIXELFORMATDESCRIPTOR{})),
		Version:    1,
		DwFlags:    w32.PFD_DRAW_TO_WINDOW | w32.PFD_SUPPORT_OPENGL | w32.PFD_DOUBLEBUFFER,
		IPixelType: w32.PFD_TYPE_RGBA,
		ColorBits:  24,
	}
	index, err := w32.ChoosePixelFormat(dc, &pfd)
	if err != nil {
		return 0, errs.Newf("failed to choose bootstrap pixel format for OpenGL: %s", err)
	}
	w32.DescribePixelFormat(dc, index, uint32(unsafe.Sizeof(pfd)), &pfd)
	if pfd.DwFlags&w32.PFD_GENERIC_FORMAT != 0 && pfd.DwFlags&w32.PFD_GENERIC_ACCELERATED == 0 {
		return 0, errs.Newf("no hardware-accelerated OpenGL driver is available: bootstrap pixel format %d is the generic "+
			"software implementation (%v)", index, &pfd)
	}
	if err = w32.SetPixelFormat(dc, index, &pfd); err != nil {
		return 0, errs.Newf("failed to set bootstrap pixel format %d (%v) for OpenGL: %s", index, &pfd, err)
	}
	rc, err := w32.WglCreateContext(dc)
	if err != nil {
		// Whether the format that was just set is even visible to the context creation is the crux of any failure
		// here, so report what both GDI and opengl32 say the device context's format is.
		return 0, errs.Newf("failed to create bootstrap OpenGL context with pixel format %d (%v): %s [GetPixelFormat "+
			"reports %d via GDI and %d via opengl32]", index, &pfd, err, w32.GetPixelFormat(dc), w32.WglGetPixelFormat(dc))
	}
	if err = w32.WglMakeCurrent(dc, rc); err != nil {
		w32.WglDeleteContext(rc)
		return 0, errs.Newf("failed to make bootstrap OpenGL context current with pixel format %d (%v): %s", index, &pfd,
			err)
	}
	return rc, nil
}

// w32ChooseGLPixelFormat asks the driver for the pixel formats satisfying the rendering pipeline's requirements and
// returns the best one: the first that PixelFormatSuitableForOpenGL also accepts, or failing that the driver's own
// first choice, since the driver's judgement of what it can render to outranks what a PIXELFORMATDESCRIPTOR shows. A
// context must be current on the calling thread.
func w32ChooseGLPixelFormat(dc w32.HDC) (glPixelFormat, error) {
	var f glPixelFormat
	candidates, err := w32.WglChoosePixelFormatARB(dc, []int32{
		w32.WGL_DRAW_TO_WINDOW_ARB, 1,
		w32.WGL_SUPPORT_OPENGL_ARB, 1,
		w32.WGL_DOUBLE_BUFFER_ARB, 1,
		w32.WGL_ACCELERATION_ARB, w32.WGL_FULL_ACCELERATION_ARB,
		w32.WGL_PIXEL_TYPE_ARB, w32.WGL_TYPE_RGBA_ARB,
		w32.WGL_RED_BITS_ARB, 8,
		w32.WGL_GREEN_BITS_ARB, 8,
		w32.WGL_BLUE_BITS_ARB, 8,
		w32.WGL_ALPHA_BITS_ARB, 8,
		w32.WGL_DEPTH_BITS_ARB, 24,
		w32.WGL_STENCIL_BITS_ARB, 8,
		0,
	}, 16)
	if err != nil {
		return f, errs.Newf("failed to choose pixel format for OpenGL context: %s", err)
	}
	if len(candidates) == 0 {
		return f, errs.New("failed to choose pixel format for OpenGL context: the driver offers no hardware-accelerated, " +
			"double-buffered RGBA8 format with 24-bit depth and 8-bit stencil buffers")
	}
	f.candidates = int32(len(candidates))
	size := uint32(unsafe.Sizeof(f.pfd))
	for i, index := range candidates {
		var pfd w32.PIXELFORMATDESCRIPTOR
		if w32.DescribePixelFormat(dc, index, size, &pfd) != 0 && w32.PixelFormatSuitableForOpenGL(&pfd) {
			f.index = index
			f.rank = int32(i + 1)
			f.pfd = pfd
			return f, nil
		}
	}
	f.index = candidates[0]
	f.rank = 1
	w32.DescribePixelFormat(dc, f.index, size, &f.pfd)
	return f, nil
}

// w32CreateGLContextForPixelFormat commits the pixel format to the device context and creates an OpenGL 3.2 context
// for it. The first successful call also verifies the library's GL direct context. No context is left current.
func w32CreateGLContextForPixelFormat(dc w32.HDC, f *glPixelFormat) (w32.HGLRC, error) {
	if err := w32.SetPixelFormat(dc, f.index, &f.pfd); err != nil {
		return 0, errs.Newf("failed to set %v for OpenGL context on probe window: %s", f, err)
	}
	rc, err := w32.WglCreateContextAttribsARB(dc, 0, []int32{
		w32.WGL_CONTEXT_MAJOR_VERSION_ARB, 3,
		w32.WGL_CONTEXT_MINOR_VERSION_ARB, 2,
		0,
	})
	if err != nil {
		return 0, errs.Newf("failed to create OpenGL 3.2 context with %v: %s", f, err)
	}
	if !w32GLPipelineVerified {
		if err = w32.WglMakeCurrent(dc, rc); err != nil {
			w32.WglDeleteContext(rc)
			return 0, errs.Newf("failed to make OpenGL context current with %v: %s", f, err)
		}
		ctx := gl.MakeGLDirectContext(defaultOpenGL(), nil)
		if ctx == nil {
			// Deleting a context that is current on the calling thread implicitly makes it not current first.
			w32.WglDeleteContext(rc)
			return 0, errs.Newf("unable to create an OpenGL rendering context with %v", f)
		}
		ctx.Destroy()            // Destroy requires the context to be current, which it still is here.
		w32.WglMakeCurrent(0, 0) //nolint:errcheck // Nothing useful can be done about a failure to release
		w32GLPipelineVerified = true
	}
	return rc, nil
}

func (c *nativeGLContext) nativeMakeCurrent() {
	w32.WglMakeCurrent(c.dc, c.rc) //nolint:errcheck // Rendering will simply produce nothing if this fails
}

func (c *nativeGLContext) nativeReleaseCurrent() {
	w32.WglMakeCurrent(0, 0) //nolint:errcheck // Nothing useful can be done about a failure to release
}

func (c *nativeGLContext) nativeSwapBuffers() {
	w32.SwapBuffers(c.dc)
}

func (c *nativeGLContext) nativeDestroy() {
	if c.rc != 0 {
		w32.WglDeleteContext(c.rc)
		c.rc = 0
	}
	if c.dc != 0 {
		w32.ReleaseDC(c.hwnd, c.dc)
		c.dc = 0
	}
}
