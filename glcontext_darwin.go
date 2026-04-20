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
	"github.com/richardwilkes/unison/internal/mac"
)

type apiGLContext struct {
	pixelFormat mac.OpenGLPixelFormatRef
	ctx         mac.OpenGLContextRef
}

func (c *apiGLContext) apiCreate(wnd *Window, transparent bool) error {
	pixFmt := mac.NewOpenGLPixelFormat()
	if pixFmt == 0 {
		return errs.New("failed to create OpenGL pixel format")
	}
	ctx := mac.NewOpenGLContext(wnd.wnd.view, pixFmt, 0, transparent)
	if ctx == 0 {
		pixFmt.Release()
		return errs.New("failed to create OpenGL context")
	}
	c.pixelFormat = pixFmt
	c.ctx = ctx
	return nil
}

func (c *apiGLContext) apiMakeCurrent() {
	c.ctx.MakeCurrent()
}

func (c *apiGLContext) apiSwapBuffers() {
	c.ctx.FlushBuffer()
}

func (c *apiGLContext) apiDestroy() {
	if c.ctx != 0 {
		c.ctx.Release()
		c.ctx = 0
	}
	if c.pixelFormat != 0 {
		c.pixelFormat.Release()
		c.pixelFormat = 0
	}
}

func apiClearOpenGLCurrentContext() {
	mac.ClearOpenGLCurrentContext()
}
