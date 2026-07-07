// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package x11

/*
#cgo linux pkg-config: x11 gl

#include <stdlib.h>
#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <GL/glx.h>

typedef GLXContext (*createContextProc)(Display*, GLXFBConfig, GLXContext, Bool, const int*);

static GLXContext createContext(Display* display, GLXFBConfig fbConfig) {
	const char *name = "glXCreateContextAttribsARB";
	createContextProc createContextAttribs = (createContextProc)glXGetProcAddressARB((const GLubyte *)name);
	if (!createContextAttribs) {
		return NULL;
	}
	int attrs[] = {
		GLX_CONTEXT_MAJOR_VERSION_ARB, 3,
		GLX_CONTEXT_MINOR_VERSION_ARB, 2,
		None,
	};
	return createContextAttribs(display, fbConfig, NULL, True, attrs);
}
*/
import "C"

import (
	"log/slog"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/errs"
)

type (
	// Display represents an X11 display connection.
	Display = *C.Display
	// FBConfig represents a GLX framebuffer configuration.
	FBConfig = C.GLXFBConfig
	// GLXContext represents a GLX rendering context.
	GLXContext = C.GLXContext
	// GLXWindow represents a GLX drawable window.
	GLXWindow = C.GLXWindow
)

// GLX represents an OpenGL context and associated framebuffer configuration.
type GLX struct {
	display  Display
	fbConfig FBConfig
	visual   VisualID
	depth    byte
}

// Visual returns the X11 visual ID associated with the chosen framebuffer configuration. The window that a GLX drawable
// will be created for must be created with this visual, or glXCreateWindow will fail with BadMatch.
func (glx *GLX) Visual() VisualID {
	return glx.visual
}

// Depth returns the color depth associated with the chosen framebuffer configuration's visual.
func (glx *GLX) Depth() byte {
	return glx.depth
}

// NewGLX creates a new GLX context with the specified transparency requirement.
func (c *Conn) NewGLX(transparent bool) (*GLX, error) {
	var glx GLX
	glx.display = C.XOpenDisplay(nil)
	if glx.display == nil {
		return nil, errs.New("failed to open X11 display")
	}
	attrs := []C.int{
		C.GLX_X_RENDERABLE, C.True,
		C.GLX_DRAWABLE_TYPE, C.GLX_WINDOW_BIT,
		C.GLX_RENDER_TYPE, C.GLX_RGBA_BIT,
		C.GLX_RED_SIZE, 8,
		C.GLX_GREEN_SIZE, 8,
		C.GLX_BLUE_SIZE, 8,
		C.GLX_DEPTH_SIZE, 24,
		C.GLX_STENCIL_SIZE, 8,
		C.GLX_DOUBLEBUFFER, C.True,
	}
	// Only request an alpha channel when transparency is actually needed. Requesting one otherwise biases the driver
	// toward 32-bit ARGB visuals that frequently differ from the screen's default visual, which is a common source of
	// BadMatch failures from glXCreateWindow on some drivers (e.g. NVIDIA).
	if transparent {
		attrs = append(attrs, C.GLX_ALPHA_SIZE, 8)
	}
	attrs = append(attrs, C.None)
	var count C.int
	configs := C.glXChooseFBConfig(glx.display, C.int(c.DefaultScreen), &attrs[0], &count)
	if configs == nil || count == 0 {
		if configs != nil {
			C.XFree(unsafe.Pointer(configs))
		}
		C.XCloseDisplay(glx.display)
		return nil, errs.New("failed to choose GLX framebuffer configuration")
	}
	defer C.XFree(unsafe.Pointer(configs))
	cfgs := unsafe.Slice(configs, count)
	var chosen FBConfig
	var chosenVisual *C.XVisualInfo
	for i := 0; i < int(count); i++ {
		visual := C.glXGetVisualFromFBConfig(glx.display, cfgs[i])
		if visual == nil {
			continue
		}
		if transparent {
			var transparentType C.int
			C.glXGetFBConfigAttrib(glx.display, cfgs[i], C.GLX_TRANSPARENT_TYPE, &transparentType)
			if transparentType == C.GLX_NONE {
				C.XFree(unsafe.Pointer(visual))
				continue
			}
		}
		chosen = cfgs[i]
		chosenVisual = visual
		break
	}
	if chosenVisual == nil && !transparent {
		for i := 0; i < int(count); i++ {
			visual := C.glXGetVisualFromFBConfig(glx.display, cfgs[i])
			if visual != nil {
				chosen = cfgs[i]
				chosenVisual = visual
				break
			}
		}
	}
	if chosen == nil || chosenVisual == nil {
		if chosenVisual != nil {
			C.XFree(unsafe.Pointer(chosenVisual))
		}
		C.XFree(unsafe.Pointer(configs))
		C.XCloseDisplay(glx.display)
		return nil, errs.New("failed to find suitable GLX framebuffer configuration")
	}
	glx.fbConfig = chosen
	glx.visual = VisualID(chosenVisual.visualid)
	glx.depth = byte(chosenVisual.depth)
	C.XFree(unsafe.Pointer(chosenVisual))
	return &glx, nil
}

// CreateContext creates a new GLX rendering context.
func (glx *GLX) CreateContext() GLXContext {
	if glx.display == nil || glx.fbConfig == nil {
		return nil
	}
	return C.createContext(glx.display, glx.fbConfig)
}

// CreateWindow creates a new GLX drawable window for the specified X11 window ID.
func (glx *GLX) CreateWindow(windowID WindowID) GLXWindow {
	if glx.display == nil || glx.fbConfig == nil {
		return 0
	}
	C.XSync(glx.display, C.False)
	return C.glXCreateWindow(glx.display, glx.fbConfig, C.Window(windowID), nil)
}

// MakeCurrent makes the specified GLX context current to the specified GLX window.
func (glx *GLX) MakeCurrent(window GLXWindow, context GLXContext) {
	if glx.display != nil {
		if C.glXMakeContextCurrent(glx.display, window, window, context) == 0 {
			slog.Error("failed to make OpenGL context current")
		}
	}
}

// ReleaseCurrent releases the current GLX context from the current thread.
func (glx *GLX) ReleaseCurrent() {
	if glx.display != nil {
		C.glXMakeContextCurrent(glx.display, C.None, C.None, nil)
	}
}

// SwapBuffers swaps the front and back buffers of the specified GLX window.
func (glx *GLX) SwapBuffers(window GLXWindow) {
	if glx.display != nil && window != 0 {
		C.glXSwapBuffers(glx.display, window)
	}
}

// DestroyWindow destroys the specified GLX window.
func (glx *GLX) DestroyWindow(window GLXWindow) {
	if glx.display != nil && window != 0 {
		C.glXDestroyWindow(glx.display, window)
	}
}

// DestroyContext destroys the specified GLX rendering context.
func (glx *GLX) DestroyContext(context GLXContext) {
	if glx.display != nil && context != nil {
		C.glXDestroyContext(glx.display, context)
	}
}

// Close closes the X11 display connection associated with this GLX instance. Note that this is a separate connection
// from the one used by the main X11 connection, and should be closed when the GLX instance is no longer needed to free
// resources.
func (glx *GLX) Close() {
	if glx.display != nil {
		C.XCloseDisplay(glx.display)
	}
}
