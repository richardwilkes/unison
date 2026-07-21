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
	"unsafe"

	"github.com/richardwilkes/canvas/gpu/gl"
	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/unison/internal/w32"
	"golang.org/x/sys/windows"
)

type apiGLContext struct {
	hwnd windows.HWND
	dc   w32.HDC
	rc   w32.HGLRC
}

// w32GLPipelineVerified is set once the full rendering pipeline (an OpenGL 3.2 context plus the library's GL direct
// context) has been proven to work on this machine, letting later window creations skip the redundant direct-context
// probe. Only accessed on the UI thread.
var w32GLPipelineVerified bool

func (c *apiGLContext) apiCreate(wnd *Window) error {
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
	format, pfd := w32ChooseGLPixelFormat(dc)
	if format == 0 {
		return errs.New("failed to choose pixel format for OpenGL context")
	}
	// Prove that the pixel format and context creation actually work on a disposable hidden window before touching
	// this window. SetPixelFormat is irreversible for a window, and a GL-capable format requires PFD_DOUBLEBUFFER,
	// which excludes PFD_SUPPORT_GDI — so once the format is committed, GDI can no longer paint the window, and GDI is
	// exactly what apiPresentCPUPixels uses when rendering falls back to the CPU. Probing first means a failure at any
	// step leaves this window format-free and therefore still paintable by the fallback. Since a GL context may be
	// made current with any DC that has the same pixel format on the same device, the probe's context is usable with
	// this window once the same format has been committed to it below.
	rc, err := w32CreateGLContextOnProbeWindow(format, &pfd)
	if err != nil {
		return err
	}
	if !w32.SetPixelFormat(dc, format, &pfd) {
		// A SetPixelFormat failure leaves the window without a pixel format, so the CPU fallback remains safe.
		w32.WglDeleteContext(rc)
		return errs.New("failed to set pixel format for OpenGL context")
	}
	c.hwnd = hwnd
	c.dc = dc
	c.rc = rc
	success = true
	return nil
}

// w32ChooseGLPixelFormat returns the index and descriptor of the first pixel format suitable for the OpenGL rendering
// pipeline, or 0 if there is none.
func w32ChooseGLPixelFormat(dc w32.HDC) (int32, w32.PIXELFORMATDESCRIPTOR) {
	var pfd w32.PIXELFORMATDESCRIPTOR
	count := w32.DescribePixelFormat(dc, 1, uint32(unsafe.Sizeof(pfd)), nil)
	for i := int32(1); i <= count; i++ {
		w32.DescribePixelFormat(dc, i, uint32(unsafe.Sizeof(pfd)), &pfd)
		if w32.PixelFormatSuitableForOpenGL(&pfd) {
			return i, pfd
		}
	}
	return 0, pfd
}

// w32CreateGLContextOnProbeWindow creates an OpenGL 3.2 context for the given pixel format using a hidden, throwaway
// window, so that no failure along the way ever commits the format to a real window. The first successful call also
// verifies that the library's GL direct context can be created, since that failure would otherwise be discovered at
// first canvas preparation, after a real window's format had already been committed.
func w32CreateGLContextOnProbeWindow(format int32, pfd *w32.PIXELFORMATDESCRIPTOR) (w32.HGLRC, error) {
	probe := w32.CreateWindowExW(0, wndProcClassName, "", w32.WS_CLIPSIBLINGS|w32.WS_CLIPCHILDREN, 0, 0, 1, 1, 0, 0,
		w32MainInstance, 0)
	if probe == 0 {
		return 0, errs.New("failed to create probe window for OpenGL context")
	}
	defer w32.DestroyWindow(probe)
	dc := w32.GetDC(probe)
	if dc == 0 {
		return 0, errs.New("failed to get device context for probe window")
	}
	defer w32.ReleaseDC(probe, dc)
	if !w32.SetPixelFormat(dc, format, pfd) {
		return 0, errs.New("failed to set pixel format for OpenGL context")
	}
	fakeRC := w32.WglCreateContext(dc)
	if fakeRC == 0 {
		return 0, errs.New("failed to create fake OpenGL context")
	}
	defer w32.WglDeleteContext(fakeRC)
	if !w32.WglMakeCurrent(dc, fakeRC) {
		return 0, errs.New("failed to make fake OpenGL context current")
	}
	defer w32.WglMakeCurrent(0, 0)
	rc := w32.WglCreateContextAttribsARB(dc, 0, []int32{
		w32.WGL_CONTEXT_MAJOR_VERSION_ARB, 3,
		w32.WGL_CONTEXT_MINOR_VERSION_ARB, 2,
		0,
	})
	if rc == 0 {
		return 0, errs.New("failed to create OpenGL context")
	}
	if !w32GLPipelineVerified {
		if !w32.WglMakeCurrent(dc, rc) {
			w32.WglDeleteContext(rc)
			return 0, errs.New("failed to make OpenGL context current")
		}
		ctx := gl.MakeGLDirectContext(defaultOpenGL(), nil)
		if ctx == nil {
			// Deleting a context that is current on the calling thread implicitly makes it not current first.
			w32.WglDeleteContext(rc)
			return 0, errs.New("unable to create an OpenGL rendering context")
		}
		ctx.Destroy() // Destroy requires the context to be current, which it still is here.
		w32GLPipelineVerified = true
	}
	return rc, nil
}

func (c *apiGLContext) apiMakeCurrent() {
	w32.WglMakeCurrent(c.dc, c.rc)
}

func (c *apiGLContext) apiReleaseCurrent() {
	w32.WglMakeCurrent(0, 0)
}

func (c *apiGLContext) apiSwapBuffers() {
	w32.SwapBuffers(c.dc)
}

func (c *apiGLContext) apiDestroy() {
	if c.rc != 0 {
		w32.WglDeleteContext(c.rc)
		c.rc = 0
	}
	if c.dc != 0 {
		w32.ReleaseDC(c.hwnd, c.dc)
		c.dc = 0
	}
}
