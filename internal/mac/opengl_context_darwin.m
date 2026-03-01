// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

#import "macos.h"

NSOpenGLContextRef newOpenGLContext(NSViewRef view, NSOpenGLPixelFormatRef pixFmt, NSOpenGLContextRef shareCtx, bool transparent) {
	NSOpenGLContext* ctx = [[NSOpenGLContext alloc] initWithFormat:pixFmt shareContext:shareCtx];
	if (!ctx) {
		return nil;
	}
	if (transparent) {
		int opaque = 0;
		[ctx setValues:&opaque forParameter:NSOpenGLContextParameterSurfaceOpacity];
	}
	NSView* v = (NSView*)view;
	[v setWantsBestResolutionOpenGLSurface:true];
	[ctx setView:v];
	return ctx;
}

void openGLUpdate(NSOpenGLContextRef ctx) {
	[(NSOpenGLContext*)ctx update];
}

void openGLMakeCurrent(NSOpenGLContextRef ctx) {
	NSOpenGLContext* c = (NSOpenGLContext*)ctx;
	if (c) {
		[c makeCurrentContext];
	} else {
		[NSOpenGLContext clearCurrentContext];
	}
}

void openGLFlushBuffer(NSOpenGLContextRef ctx) {
	[(NSOpenGLContext*)ctx flushBuffer];
}
