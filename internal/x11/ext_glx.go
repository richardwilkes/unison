// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package x11

import "github.com/richardwilkes/toolbox/v2/errs"

const (
	glxOpRender = 1 + iota
	glxOpRenderLarge
	glxOpCreateContext
	glxOpDestroyContext
	glxOpMakeCurrent
	glxOpIsDirect
	glxOpQueryVersion
	glxOpWaitGL
	glxOpWaitX
	glxOpCopyContext
	glxOpSwapBuffers
	glxOpUseXFont
	glxOpCreateGLXPixmap
	glxOpGetVisualConfigs
	glxOpDestroyGLXPixmap
	glxOpVendorPrivate
	glxOpVendorPrivateWithReply
	glxOpQueryExtensionsString
	glxOpQueryServerString
	glxOpClientInfo
	glxOpGetFBConfigs
	glxOpCreatePixmap
	glxOpDestroyPixmap
	glxOpCreateNewContext
	glxOpQueryContext
	glxOpMakeContextCurrent
	glxOpCreatePbuffer
	glxOpDestroyPbuffer
	glxOpGetDrawableAttributes
	glxOpChangeDrawableAttributes
	glxOpCreateWindow
	glxOpDestroyWindow
	glxOpSetClientInfoARB
	glxOpCreateContextAttribsARB
	glxOpSetClientInfo2ARB
)

// Constants for GLX framebuffer configuration attributes.
const (
	FBAttrBufferSize            = 0x0002
	FBAttrLevel                 = 0x0003
	FBAttrDoubleBuffer          = 0x0005
	FBAttrStereo                = 0x0006
	FBAttrAuxBuffers            = 0x0007
	FBAttrRedSize               = 0x0008
	FBAttrGreenSize             = 0x0009
	FBAttrBlueSize              = 0x000A
	FBAttrAlphaSize             = 0x000B
	FBAttrDepthSize             = 0x000C
	FBAttrStencilSize           = 0x000D
	FBAttrAccumRedSize          = 0x000E
	FBAttrAccumGreenSize        = 0x000F
	FBAttrAccumBlueSize         = 0x0010
	FBAttrAccumAlphaSize        = 0x0011
	FBAttrVisualID              = 0x800B
	FBAttrRenderType            = 0x8011
	FBAttrDrawableType          = 0x8010
	FBAttrXRenderable           = 0x8012
	FBAttrFBConfigID            = 0x8013
	FBAttrMaxPBufferWidth       = 0x8016
	FBAttrMaxPBufferHeight      = 0x8017
	FBAttrMaxPBufferPixels      = 0x8018
	FBAttrConfigCaveat          = 0x0020
	FBAttrXVisualType           = 0x0022
	FBAttrTransparentType       = 0x0023
	FBAttrTransparentIndexValue = 0x0024
	FBAttrTransparentRedValue   = 0x0025
	FBAttrTransparentGreenValue = 0x0026
	FBAttrTransparentBlueValue  = 0x0027
	FBAttrTransparentAlphaValue = 0x0028
)

// Constants for GLX context attributes.
const (
	GLXContextMajorVersionARB = 0x2091
	GLXContextMinorVersionARB = 0x2092
)

// Possible values for FBAttrRenderType.
const (
	RenderTypeRGBABit = 1 << iota
	RenderTypeColorIndexBit
	RenderTypeRGBAFloatBit
	RenderTypeRGBAUnsignedFloatBitExt
)

// Possible values for FBAttrDrawableType.
const (
	DrawableTypeWindowBit = 1 << iota
	DrawableTypePixmapBit
	DrawableTypePbufferBit
)

// ExtGLX provides access to the GLX extension. Note that only those calls that I need have been implemented.
type ExtGLX struct {
	conn *Conn
	extensionInfo
}

func newExtGLX(conn *Conn) *ExtGLX {
	info := conn.hasExtension("GLX", glxOpQueryVersion, false, 1, 4)
	return &ExtGLX{
		conn:          conn,
		extensionInfo: info,
	}
}

// FBConfig represents a GLX framebuffer configuration.
type FBConfig struct {
	propertyList []uint32
}

// Property retrieves the value of the specified attribute, returning the value and whether it was found.
func (c *FBConfig) Property(attr uint32) (value uint32, found bool) {
	for i := 0; i < len(c.propertyList); i += 2 {
		if c.propertyList[i] == attr {
			return c.propertyList[i+1], true
		}
	}
	return 0, false
}

// GetFBConfigs retrieves the list of GLX framebuffer configurations for the specified screen.
func (e *ExtGLX) GetFBConfigs(screen uint32) []FBConfig {
	w := NewWriter(8)
	w.Byte(e.majorOpcode)
	w.Byte(glxOpGetFBConfigs)
	w.Uint16(2)
	w.Uint32(screen)
	var cfgs []FBConfig
	if err := e.conn.sendNewRequest(newReplyRequest(w, func(r *Reader) {
		r.Skip(8)
		numConfigs := r.Uint32()
		numProperties := int(r.Uint32()) * 2
		r.Skip(16)
		for range numConfigs {
			cfgs = append(cfgs, FBConfig{propertyList: r.Uint32Slice(numProperties)})
		}
	})); err != nil {
		errs.Log(err)
	}
	return cfgs
}

// CreateContextAttribsARB creates a new GLX context with the specified framebuffer configuration, screen, shared
// context, direct rendering flag, and attribute list, returning the new GLXContextID.
func (e *ExtGLX) CreateContextAttribsARB(fbconfigID, screen uint32, shareList GLXContextID, direct bool, attrs []uint32) GLXContextID {
	id := nextXID[GLXContextID](e.conn)
	if id == 0 {
		return 0
	}
	w := NewWriter(28 + len(attrs)*4)
	w.Byte(e.majorOpcode)
	w.Byte(glxOpCreateContextAttribsARB)
	w.Uint16(7 + uint16(len(attrs)))
	w.GLXContextID(id)
	w.Uint32(fbconfigID)
	w.Uint32(screen)
	w.GLXContextID(shareList)
	w.Bool(direct)
	w.Zero(3)
	w.Uint32(uint32(len(attrs) / 2))
	w.Uint32Slice(attrs)
	if err := e.conn.sendNewRequest(newCheckedRequest(w)); err != nil {
		errs.Log(err)
		id = 0
	}
	return id
}

// DestroyContext destroys the specified GLX context.
func (e *ExtGLX) DestroyContext(ctx GLXContextID) {
	w := NewWriter(8)
	w.Byte(e.majorOpcode)
	w.Byte(glxOpDestroyContext)
	w.Uint16(2)
	w.GLXContextID(ctx)
	if err := e.conn.sendNewRequest(newUncheckedRequest(w)); err != nil {
		errs.Log(err)
	}
}

// CreateWindow creates a new GLX window with the specified framebuffer configuration, screen, X11 window, and attribute
// list, returning the new GLXWindowID.
func (e *ExtGLX) CreateWindow(fbconfig, screen uint32, window WindowID, attrs []uint32) GLXWindowID {
	id := nextXID[GLXWindowID](e.conn)
	if id == 0 {
		return 0
	}
	w := NewWriter(24 + len(attrs)*4)
	w.Byte(e.majorOpcode)
	w.Byte(glxOpCreateWindow)
	w.Uint16(6 + uint16(len(attrs)))
	w.Uint32(screen)
	w.Uint32(fbconfig)
	w.WindowID(window)
	w.GLXWindowID(id)
	w.Uint32(uint32(len(attrs) / 2))
	w.Uint32Slice(attrs)
	if err := e.conn.sendNewRequest(newCheckedRequest(w)); err != nil {
		errs.Log(err)
		id = 0
	}
	return id
}

// DestroyWindow destroys the specified GLX window.
func (e *ExtGLX) DestroyWindow(window GLXWindowID) {
	w := NewWriter(8)
	w.Byte(e.majorOpcode)
	w.Byte(glxOpDestroyWindow)
	w.Uint16(2)
	w.GLXWindowID(window)
	if err := e.conn.sendNewRequest(newUncheckedRequest(w)); err != nil {
		errs.Log(err)
	}
}

// MakeCurrent makes the specified GLX context current to the specified GLX window, returning a new context tag.
func (e *ExtGLX) MakeCurrent(wnd GLXWindowID, ctx GLXContextID, tag uint32) uint32 {
	w := NewWriter(16)
	w.Byte(e.majorOpcode)
	w.Byte(glxOpMakeCurrent)
	w.Uint16(4)
	w.GLXWindowID(wnd)
	w.GLXContextID(ctx)
	w.Uint32(tag)
	var newTag uint32
	if err := e.conn.sendNewRequest(newReplyRequest(w, func(r *Reader) {
		r.Skip(8)
		newTag = r.Uint32()
		r.Skip(20)
	})); err != nil {
		errs.Log(err)
	}
	return newTag
}

// SwapBuffers swaps the front and back buffers of the specified GLX window, using the specified context tag.
func (e *ExtGLX) SwapBuffers(wnd GLXWindowID, tag uint32) {
	w := NewWriter(12)
	w.Byte(e.majorOpcode)
	w.Byte(glxOpSwapBuffers)
	w.Uint16(3)
	w.Uint32(tag)
	w.GLXWindowID(wnd)
	if err := e.conn.sendNewRequest(newUncheckedRequest(w)); err != nil {
		errs.Log(err)
	}
}
