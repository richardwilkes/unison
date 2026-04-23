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

type apiGLContext struct {
	glx     *x11.GLX
	context x11.GLXContext
	window  x11.GLXWindow
	visual  x11.VisualID
	depth   byte
}

func (c *apiGLContext) x11PrepareWindow(wnd *Window) error {
	var err error
	if c.glx, err = x11Conn.NewGLX(wnd.transparent); err != nil {
		return err
	}
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
	x11Conn.Flush()
	c.window = c.glx.CreateWindow(wnd.wnd.id)
	if c.window == 0 {
		c.glx.DestroyContext(c.context)
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
}
