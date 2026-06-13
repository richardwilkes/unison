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

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/unison/internal/w32"
	"golang.org/x/sys/windows"
)

type apiGLContext struct {
	hwnd windows.HWND
	dc   w32.HDC
	rc   w32.HGLRC
}

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
	var pfd w32.PIXELFORMATDESCRIPTOR
	count := w32.DescribePixelFormat(dc, 1, uint32(unsafe.Sizeof(pfd)), nil)
	for i := int32(1); i <= count; i++ {
		w32.DescribePixelFormat(dc, i, uint32(unsafe.Sizeof(pfd)), &pfd)
		if pfd.DwFlags&w32.PFD_DRAW_TO_WINDOW == 0 || pfd.DwFlags&w32.PFD_SUPPORT_OPENGL == 0 {
			continue
		}
		if pfd.DwFlags&w32.PFD_GENERIC_ACCELERATED == 0 && pfd.DwFlags&w32.PFD_GENERIC_FORMAT != 0 {
			continue
		}
		if pfd.IPixelType != w32.PFD_TYPE_RGBA {
			continue
		}
		if pfd.DwFlags&w32.PFD_DOUBLEBUFFER == 0 {
			continue
		}
		if pfd.RedBits != 8 || pfd.GreenBits != 8 || pfd.BlueBits != 8 || pfd.AlphaBits != 8 {
			continue
		}
		if pfd.DepthBits != 24 || pfd.StencilBits != 8 {
			continue
		}
		if !w32.SetPixelFormat(dc, i, &pfd) {
			return errs.New("failed to set pixel format for OpenGL context")
		}
		fakeRC := w32.WglCreateContext(dc)
		if !w32.WglMakeCurrent(dc, fakeRC) {
			w32.WglDeleteContext(fakeRC)
			return errs.New("failed to make fake OpenGL context current")
		}
		rc := w32.WglCreateContextAttribsARB(dc, 0, []int32{
			w32.WGL_CONTEXT_MAJOR_VERSION_ARB, 3,
			w32.WGL_CONTEXT_MINOR_VERSION_ARB, 2,
			0,
		})
		w32.WglMakeCurrent(0, 0)
		w32.WglDeleteContext(fakeRC)
		if rc == 0 {
			return errs.New("failed to create OpenGL context")
		}
		c.hwnd = hwnd
		c.dc = dc
		c.rc = rc
		success = true
		return nil
	}
	return errs.New("failed to choose pixel format for OpenGL context")
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
