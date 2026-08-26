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
	"github.com/richardwilkes/unison/internal/cocoa"
)

type nativeGLContext struct {
	pixelFormat cocoa.OpenGLPixelFormatRef
	ctx         cocoa.OpenGLContextRef
}

func (c *nativeGLContext) nativeCreate(wnd *Window) error {
	pixFmt := cocoa.NewOpenGLPixelFormat()
	if pixFmt == 0 {
		return errs.New("failed to create OpenGL pixel format")
	}
	ctx := cocoa.NewOpenGLContext(wnd.wnd.view, pixFmt, 0, wnd.transparent)
	if ctx == 0 {
		pixFmt.Release()
		return errs.New("failed to create OpenGL context")
	}
	c.pixelFormat = pixFmt
	c.ctx = ctx
	return nil
}

func (c *nativeGLContext) nativeMakeCurrent() {
	c.ctx.MakeCurrent()
}

func (c *nativeGLContext) nativeReleaseCurrent() {
	cocoa.ClearOpenGLCurrentContext()
}

func (c *nativeGLContext) nativeSwapBuffers() {
	c.ctx.FlushBuffer()
}

func (c *nativeGLContext) nativeDestroy() {
	if c.ctx != 0 {
		c.ctx.Release()
		c.ctx = 0
	}
	if c.pixelFormat != 0 {
		c.pixelFormat.Release()
		c.pixelFormat = 0
	}
}
