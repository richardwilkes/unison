// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package x11

import (
	"log/slog"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/richardwilkes/toolbox/v2/errs"
)

type (
	// Display represents an X11 display connection. This is a separate connection from the pure-Go wire-protocol
	// connection used by Conn; XIDs are server-global, so windows created on the Conn connection remain valid GLX
	// drawables on this one.
	Display unsafe.Pointer
	// FBConfig represents a GLX framebuffer configuration.
	FBConfig unsafe.Pointer
	// GLXContext represents a GLX rendering context.
	GLXContext unsafe.Pointer
	// GLXWindow represents a GLX drawable window.
	GLXWindow uintptr
)

// GLX constants from GL/glx.h and glxext.h.
const (
	glxDoubleBuffer           = 5
	glxRedSize                = 8
	glxGreenSize              = 9
	glxBlueSize               = 10
	glxAlphaSize              = 11
	glxDepthSize              = 12
	glxStencilSize            = 13
	glxTransparentType        = 0x23
	glxNone                   = 0x8000
	glxDrawableType           = 0x8010
	glxRenderType             = 0x8011
	glxXRenderable            = 0x8012
	glxWindowBit              = 0x00000001
	glxRGBABit                = 0x00000001
	glxContextMajorVersionARB = 0x2091
	glxContextMinorVersionARB = 0x2092
)

// xVisualInfo matches the memory layout of Xlib's XVisualInfo struct. C pointer and "unsigned long" fields map to
// uintptr and C "int" fields map to int32, which reproduces the C layout (including padding) on both 32-bit and 64-bit
// Linux. Only visualID and depth are read; the blank fields exist solely to keep the layout correct.
type xVisualInfo struct {
	_        uintptr // visual
	visualID uintptr
	_        int32 // screen
	depth    int32
	_        int32   // class
	_        uintptr // redMask
	_        uintptr // greenMask
	_        uintptr // blueMask
	_        int32   // colormapSize
	_        int32   // bitsPerRGB
}

var (
	glxInitOnce sync.Once
	glxInitErr  error

	xOpenDisplay     func(name *byte) Display
	xCloseDisplay    func(display Display) int32
	xFree            func(ptr unsafe.Pointer) int32
	xSync            func(display Display, discard int32) int32
	xSetErrorHandler func(handler uintptr) uintptr

	// glxNoopErrorHandler is a purego callback for an Xlib error handler that ignores the error, created in initGLX.
	glxNoopErrorHandler uintptr

	glXChooseFBConfig          func(display Display, screen int32, attribs, count *int32) *FBConfig
	glXGetVisualFromFBConfig   func(display Display, config FBConfig) *xVisualInfo
	glXGetFBConfigAttrib       func(display Display, config FBConfig, attribute int32, value *int32) int32
	glXCreateWindow            func(display Display, config FBConfig, window uintptr, attribs *int32) GLXWindow
	glXMakeContextCurrent      func(display Display, draw, read GLXWindow, context GLXContext) int32
	glXSwapBuffers             func(display Display, drawable GLXWindow)
	glXDestroyWindow           func(display Display, window GLXWindow)
	glXDestroyContext          func(display Display, context GLXContext)
	glXGetProcAddressARB       func(name string) uintptr
	glXCreateContextAttribsARB func(display Display, config FBConfig, share GLXContext, direct int32, attribs *int32) GLXContext
)

// dlopenFirst opens the first shared library from names that loads successfully, returning the error from the first
// attempt if none do.
func dlopenFirst(names ...string) (uintptr, error) {
	var firstErr error
	for _, name := range names {
		lib, err := purego.Dlopen(name, purego.RTLD_LAZY|purego.RTLD_GLOBAL)
		if err == nil {
			return lib, nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	return 0, firstErr
}

// registerLibFunc is purego.RegisterLibFunc, except that a missing symbol is reported as an error rather than a panic.
func registerLibFunc(fptr any, lib uintptr, name string) error {
	addr, err := purego.Dlsym(lib, name)
	if err != nil {
		return errs.NewWithCause("unable to resolve "+name, err)
	}
	purego.RegisterFunc(fptr, addr)
	return nil
}

// initGLX loads libX11 and libGL via dlopen and resolves the Xlib and GLX entry points used by this file. It replaces
// the compile-time linking the cgo version of this code got from `pkg-config: x11 gl`; the libraries are now a runtime
// requirement only.
func initGLX() error {
	glxInitOnce.Do(func() {
		libX11, err := dlopenFirst("libX11.so.6", "libX11.so")
		if err != nil {
			glxInitErr = errs.NewWithCause("unable to load libX11; install your distribution's libX11 package", err)
			return
		}
		libGL, err := dlopenFirst("libGL.so.1", "libGL.so")
		if err != nil {
			glxInitErr = errs.NewWithCause("unable to load libGL; install your distribution's Mesa/OpenGL package", err)
			return
		}
		for _, one := range []struct {
			fptr any
			name string
			lib  uintptr
		}{
			{&xOpenDisplay, "XOpenDisplay", libX11},
			{&xCloseDisplay, "XCloseDisplay", libX11},
			{&xFree, "XFree", libX11},
			{&xSync, "XSync", libX11},
			{&xSetErrorHandler, "XSetErrorHandler", libX11},
			{&glXChooseFBConfig, "glXChooseFBConfig", libGL},
			{&glXGetVisualFromFBConfig, "glXGetVisualFromFBConfig", libGL},
			{&glXGetFBConfigAttrib, "glXGetFBConfigAttrib", libGL},
			{&glXCreateWindow, "glXCreateWindow", libGL},
			{&glXMakeContextCurrent, "glXMakeContextCurrent", libGL},
			{&glXSwapBuffers, "glXSwapBuffers", libGL},
			{&glXDestroyWindow, "glXDestroyWindow", libGL},
			{&glXDestroyContext, "glXDestroyContext", libGL},
			{&glXGetProcAddressARB, "glXGetProcAddressARB", libGL},
		} {
			if glxInitErr = registerLibFunc(one.fptr, one.lib, one.name); glxInitErr != nil {
				return
			}
		}
		// glXCreateContextAttribsARB is an extension and must be resolved through glXGetProcAddressARB rather than
		// dlsym. A zero result leaves glXCreateContextAttribsARB nil and CreateContext returns nil, matching the cgo
		// version's behavior.
		if addr := glXGetProcAddressARB("glXCreateContextAttribsARB"); addr != 0 {
			purego.RegisterFunc(&glXCreateContextAttribsARB, addr)
		}
		// An Xlib error handler has the C signature int (*)(Display *, XErrorEvent *); the return value is ignored.
		glxNoopErrorHandler = purego.NewCallback(func(_, _ uintptr) uintptr { return 0 })
	})
	return glxInitErr
}

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
	if err := initGLX(); err != nil {
		return nil, err
	}
	var glx GLX
	glx.display = xOpenDisplay(nil)
	if glx.display == nil {
		return nil, errs.New("failed to open X11 display")
	}
	attrs := []int32{
		glxXRenderable, 1,
		glxDrawableType, glxWindowBit,
		glxRenderType, glxRGBABit,
		glxRedSize, 8,
		glxGreenSize, 8,
		glxBlueSize, 8,
		glxDepthSize, 24,
		glxStencilSize, 8,
		glxDoubleBuffer, 1,
	}
	// Only request an alpha channel when transparency is actually needed. Requesting one otherwise biases the driver
	// toward 32-bit ARGB visuals that frequently differ from the screen's default visual, which is a common source of
	// BadMatch failures from glXCreateWindow on some drivers (e.g. NVIDIA).
	if transparent {
		attrs = append(attrs, glxAlphaSize, 8)
	}
	attrs = append(attrs, 0) // None
	var count int32
	configs := glXChooseFBConfig(glx.display, int32(c.DefaultScreen), &attrs[0], &count)
	if configs == nil || count == 0 {
		if configs != nil {
			xFree(unsafe.Pointer(configs))
		}
		xCloseDisplay(glx.display)
		return nil, errs.New("failed to choose GLX framebuffer configuration")
	}
	defer xFree(unsafe.Pointer(configs))
	cfgs := unsafe.Slice(configs, count)
	var chosen FBConfig
	var chosenVisual *xVisualInfo
	for i := 0; i < int(count); i++ {
		visual := glXGetVisualFromFBConfig(glx.display, cfgs[i])
		if visual == nil {
			continue
		}
		if transparent {
			var transparentType int32
			glXGetFBConfigAttrib(glx.display, cfgs[i], glxTransparentType, &transparentType)
			if transparentType == glxNone {
				xFree(unsafe.Pointer(visual))
				continue
			}
		}
		chosen = cfgs[i]
		chosenVisual = visual
		break
	}
	if chosenVisual == nil && !transparent {
		for i := 0; i < int(count); i++ {
			visual := glXGetVisualFromFBConfig(glx.display, cfgs[i])
			if visual != nil {
				chosen = cfgs[i]
				chosenVisual = visual
				break
			}
		}
	}
	if chosen == nil || chosenVisual == nil {
		if chosenVisual != nil {
			xFree(unsafe.Pointer(chosenVisual))
		}
		xCloseDisplay(glx.display)
		return nil, errs.New("failed to find suitable GLX framebuffer configuration")
	}
	glx.fbConfig = chosen
	glx.visual = VisualID(chosenVisual.visualID)
	glx.depth = byte(chosenVisual.depth)
	xFree(unsafe.Pointer(chosenVisual))
	return &glx, nil
}

// CreateContext creates a new GLX rendering context.
func (glx *GLX) CreateContext() GLXContext {
	if glx.display == nil || glx.fbConfig == nil || glXCreateContextAttribsARB == nil {
		return nil
	}
	attrs := []int32{
		glxContextMajorVersionARB, 3,
		glxContextMinorVersionARB, 2,
		0, // None
	}
	// glXCreateContextAttribsARB raises X protocol errors (e.g. GLXBadFBConfig) when the requested GL version or
	// configuration is unsupported, and Xlib's default error handler prints a message and terminates the process.
	// Install a no-op handler around the call (as GLFW does) so such failures surface as a nil context the caller can
	// handle, and sync before restoring it so any error the call raised is processed while the no-op handler is still
	// installed.
	prev := xSetErrorHandler(glxNoopErrorHandler)
	context := glXCreateContextAttribsARB(glx.display, glx.fbConfig, nil, 1, &attrs[0])
	xSync(glx.display, 0)
	xSetErrorHandler(prev)
	return context
}

// CreateWindow creates a new GLX drawable window for the specified X11 window ID.
func (glx *GLX) CreateWindow(windowID WindowID) GLXWindow {
	if glx.display == nil || glx.fbConfig == nil {
		return 0
	}
	xSync(glx.display, 0)
	return glXCreateWindow(glx.display, glx.fbConfig, uintptr(windowID), nil)
}

// MakeCurrent makes the specified GLX context current to the specified GLX window.
func (glx *GLX) MakeCurrent(window GLXWindow, context GLXContext) {
	if glx.display != nil {
		if glXMakeContextCurrent(glx.display, window, window, context) == 0 {
			slog.Error("failed to make OpenGL context current")
		}
	}
}

// ReleaseCurrent releases the current GLX context from the current thread.
func (glx *GLX) ReleaseCurrent() {
	if glx.display != nil {
		glXMakeContextCurrent(glx.display, 0, 0, nil)
	}
}

// SwapBuffers swaps the front and back buffers of the specified GLX window.
func (glx *GLX) SwapBuffers(window GLXWindow) {
	if glx.display != nil && window != 0 {
		glXSwapBuffers(glx.display, window)
	}
}

// DestroyWindow destroys the specified GLX window.
func (glx *GLX) DestroyWindow(window GLXWindow) {
	if glx.display != nil && window != 0 {
		glXDestroyWindow(glx.display, window)
	}
}

// DestroyContext destroys the specified GLX rendering context.
func (glx *GLX) DestroyContext(context GLXContext) {
	if glx.display != nil && context != nil {
		glXDestroyContext(glx.display, context)
	}
}

// Close closes the X11 display connection associated with this GLX instance. Note that this is a separate connection
// from the one used by the main X11 connection, and should be closed when the GLX instance is no longer needed to free
// resources.
func (glx *GLX) Close() {
	if glx.display != nil {
		xCloseDisplay(glx.display)
	}
}
