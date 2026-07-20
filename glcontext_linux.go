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
	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/unison/internal/x11"
)

// glxAPI is the subset of x11.GLX that apiGLContext uses, expressed as an interface so tests can substitute a fake
// implementation that exercises the failure paths without a running X server.
type glxAPI interface {
	Visual() x11.VisualID
	Depth() byte
	CreateContext() x11.GLXContext
	CreateWindow(windowID x11.WindowID) x11.GLXWindow
	MakeCurrent(window x11.GLXWindow, context x11.GLXContext)
	ReleaseCurrent()
	SwapBuffers(window x11.GLXWindow)
	DestroyWindow(window x11.GLXWindow)
	DestroyContext(context x11.GLXContext)
	Close()
}

type apiGLContext struct {
	glx       glxAPI
	context   x11.GLXContext
	window    x11.GLXWindow
	visual    x11.VisualID
	depth     byte
	hasVisual bool
}

func (c *apiGLContext) x11PrepareWindow(wnd *Window) error {
	// Assign through a typed local so a failed NewGLX leaves c.glx a nil interface rather than an interface wrapping a
	// nil *x11.GLX, which would defeat the nil checks in apiCreate and apiDestroy.
	glx, err := x11Conn.NewGLX(wnd.transparent)
	if err != nil {
		return err
	}
	c.glx = glx
	// The window must be created with the visual & depth of the framebuffer configuration that GLX chose; otherwise
	// glXCreateWindow will fail with BadMatch. Propagate them so apiInit uses them instead of the screen defaults.
	c.visual = c.glx.Visual()
	c.depth = c.glx.Depth()
	c.hasVisual = true
	return nil
}

func (c *apiGLContext) apiCreate(wnd *Window) error {
	if c.glx == nil {
		return errs.New("failed to prepare GLX resources")
	}
	c.context = c.glx.CreateContext()
	if c.context == nil {
		return errs.New("failed to create OpenGL context")
	}
	// No flush of the main connection is needed before glXCreateWindow references the X window: apiInit, which created
	// that window, always flushes before returning, and NewWindow calls apiCreate immediately afterward.
	c.window = c.glx.CreateWindow(wnd.wnd.id)
	if c.window == 0 {
		// Clear the reference after destroying the context, since NewWindow's error path will invoke apiDestroy, which
		// would otherwise destroy the same context a second time and raise GLXBadContext.
		c.glx.DestroyContext(c.context)
		c.context = nil
		return errs.New("failed to create GLX window for the OpenGL context")
	}
	return nil
}

func (c *apiGLContext) apiMakeCurrent() {
	c.glx.MakeCurrent(c.window, c.context)
}

func (c *apiGLContext) apiReleaseCurrent() {
	c.glx.ReleaseCurrent()
}

func (c *apiGLContext) apiSwapBuffers() {
	c.glx.SwapBuffers(c.window)
}

func (c *apiGLContext) apiDestroy() {
	if c.window != 0 {
		c.glx.DestroyWindow(c.window)
		c.window = 0
	}
	if c.context != nil {
		c.glx.DestroyContext(c.context)
		c.context = nil
	}
	if c.glx != nil {
		c.glx.Close()
		c.glx = nil
	}
	c.visual = 0
	c.depth = 0
	c.hasVisual = false
}
