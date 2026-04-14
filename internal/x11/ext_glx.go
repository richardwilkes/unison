// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package x11

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
