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
	window     x11.GLXWindowID
	context    x11.GLXContextID
	contextTag uint32
}

func (c *apiGLContext) apiCreate(wnd, share *Window, transparent bool) error {
	screen := uint32(x11Conn.DefaultScreen)
	cfgs := x11Conn.ExtGLX.GetFBConfigs(screen)
	which := -1
	maybe := -1
	for i := range cfgs {
		cfg := &cfgs[i]
		if renderType, ok := cfg.Property(x11.FBAttrRenderType); !ok || renderType&x11.RenderTypeRGBABit == 0 {
			continue
		}
		if drawableType, ok := cfg.Property(x11.FBAttrDrawableType); !ok || drawableType&x11.DrawableTypeWindowBit == 0 {
			continue
		}
		if doubleBuffered, ok := cfg.Property(x11.FBAttrDoubleBuffer); !ok || doubleBuffered == 0 {
			continue
		}
		if redBits, ok := cfg.Property(x11.FBAttrRedSize); !ok || redBits != 8 {
			continue
		}
		if greenBits, ok := cfg.Property(x11.FBAttrGreenSize); !ok || greenBits != 8 {
			continue
		}
		if blueBits, ok := cfg.Property(x11.FBAttrBlueSize); !ok || blueBits != 8 {
			continue
		}
		if alphaBits, ok := cfg.Property(x11.FBAttrAlphaSize); !ok || alphaBits != 8 {
			continue
		}
		if depthBits, ok := cfg.Property(x11.FBAttrDepthSize); !ok || depthBits != 24 {
			continue
		}
		if stencilBits, ok := cfg.Property(x11.FBAttrStencilSize); !ok || stencilBits != 8 {
			continue
		}
		if transparent {
			if transparentType, ok := cfg.Property(x11.FBAttrTransparentType); ok && transparentType != 0 {
				which = i
				break
			}
		} else {
			which = i
			break
		}
		if maybe != -1 {
			maybe = i
		}
	}
	if which == -1 {
		which = maybe
	}
	if which == -1 {
		return errs.New("failed to find suitable framebuffer configuration for the OpenGL context")
	}
	fbCfgID, ok := cfgs[which].Property(x11.FBAttrFBConfigID)
	if !ok {
		return errs.New("failed to retrieve framebuffer configuration ID for the OpenGL context")
	}
	var shareCtx x11.GLXContextID
	if share != nil {
		shareCtx = share.glCtx.context
	}
	if c.context = x11Conn.ExtGLX.CreateContextAttribsARB(fbCfgID, screen, shareCtx, true, []uint32{
		x11.GLXContextMajorVersionARB, 3,
		x11.GLXContextMinorVersionARB, 2,
		0,
	}); c.context == 0 {
		return errs.New("failed to create OpenGL context")
	}
	if c.window = x11Conn.ExtGLX.CreateWindow(fbCfgID, screen, wnd.wnd.id, nil); c.window == 0 {
		x11Conn.ExtGLX.DestroyContext(c.context)
		c.context = 0
		return errs.New("failed to create GLX window for the OpenGL context")
	}
	return nil
}

func (c *apiGLContext) apiMakeCurrent() {
	c.contextTag = x11Conn.ExtGLX.MakeCurrent(c.window, c.context, c.contextTag)
}

func (c *apiGLContext) apiSwapBuffers() {
	x11Conn.ExtGLX.SwapBuffers(c.window, c.contextTag)
}

func (c *apiGLContext) apiDestroy() {
	if c.window != 0 {
		x11Conn.ExtGLX.DestroyWindow(c.window)
		c.window = 0
	}
	if c.context != 0 {
		x11Conn.ExtGLX.DestroyContext(c.context)
		c.context = 0
	}
}

func apiClearOpenGLCurrentContext() {
	x11Conn.ExtGLX.MakeCurrent(0, 0, 0)
}
