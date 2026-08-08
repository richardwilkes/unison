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
	"sync"
	"testing"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/check"
)

// TestCreateContextBracketsCallWithNoopErrorHandler verifies that CreateContext installs the no-op Xlib error handler
// before calling glXCreateContextAttribsARB, syncs so any protocol errors raised by the call (e.g. GLXBadFBConfig on
// drivers that cannot provide the requested GL version) are processed while that handler is installed, and restores
// the previous handler afterward. Without this bracketing, Xlib's default error handler would print a message and
// terminate the process instead of letting CreateContext return nil.
func TestCreateContextBracketsCallWithNoopErrorHandler(t *testing.T) {
	c := check.New(t)
	savedSetErrorHandler := xSetErrorHandler
	savedSync := xSync
	savedCreate := glXCreateContextAttribsARB
	savedNoopHandler := glxNoopErrorHandler
	t.Cleanup(func() {
		xSetErrorHandler = savedSetErrorHandler
		xSync = savedSync
		glXCreateContextAttribsARB = savedCreate
		glxNoopErrorHandler = savedNoopHandler
	})
	const noopHandler = uintptr(7)
	const previousHandler = uintptr(42)
	glxNoopErrorHandler = noopHandler
	installed := previousHandler
	var order []string
	xSetErrorHandler = func(handler uintptr) uintptr {
		prev := installed
		installed = handler
		switch handler {
		case noopHandler:
			order = append(order, "install-noop")
		case previousHandler:
			order = append(order, "restore-previous")
		default:
			order = append(order, "install-unknown")
		}
		return prev
	}
	xSync = func(_ Display, _ int32) int32 {
		order = append(order, "sync")
		return 0
	}
	glXCreateContextAttribsARB = func(_ Display, _ FBConfig, _ GLXContext, _ int32, _ *int32) GLXContext {
		order = append(order, "create")
		return nil // What a driver returns after raising GLXBadFBConfig for an unsupported GL version.
	}
	var dummy int32
	glx := &GLX{
		display:  Display(unsafe.Pointer(&dummy)),
		fbConfig: FBConfig(unsafe.Pointer(&dummy)),
	}
	c.True(glx.CreateContext() == nil, "CreateContext must return nil when context creation fails")
	c.Equal([]string{"install-noop", "create", "sync", "restore-previous"}, order)
	c.Equal(previousHandler, installed, "the previously installed error handler must be restored")
}

// fakeGLXConfig describes one framebuffer configuration that the mocked glXChooseFBConfig will report.
type fakeGLXConfig struct {
	visualID    uintptr // 0 means glXGetVisualFromFBConfig fails for this configuration
	depth       int32
	transparent bool
}

// installFakeGLX points the Xlib and GLX entry points at in-process fakes reporting the given framebuffer
// configurations and marks GLX initialization as already complete, so NewGLX never dlopens anything. It returns a
// pointer to the number of XCloseDisplay calls made, which NewGLX performs only on failure.
func installFakeGLX(t *testing.T, cfgs []fakeGLXConfig) *int {
	t.Helper()
	savedOpen, savedClose, savedFree := xOpenDisplay, xCloseDisplay, xFree
	savedChoose := glXChooseFBConfig
	savedVisual := glXGetVisualFromFBConfig
	savedAttrib := glXGetFBConfigAttrib
	savedInitErr := glxInitErr
	t.Cleanup(func() {
		xOpenDisplay, xCloseDisplay, xFree = savedOpen, savedClose, savedFree
		glXChooseFBConfig = savedChoose
		glXGetVisualFromFBConfig = savedVisual
		glXGetFBConfigAttrib = savedAttrib
		glxInitErr = savedInitErr
		glxInitOnce = sync.Once{}
	})
	glxInitOnce = sync.Once{}
	glxInitOnce.Do(func() {}) // Consume the Once so initGLX() just returns glxInitErr.
	glxInitErr = nil

	// Separate backing storage gives each FBConfig a distinct, non-nil pointer value.
	slots := make([]int32, len(cfgs))
	configs := make([]FBConfig, len(cfgs))
	visuals := make([]xVisualInfo, len(cfgs))
	for i := range cfgs {
		configs[i] = FBConfig(unsafe.Pointer(&slots[i]))
		visuals[i] = xVisualInfo{visualID: cfgs[i].visualID, depth: cfgs[i].depth}
	}
	indexOf := func(config FBConfig) int {
		for i := range configs {
			if configs[i] == config {
				return i
			}
		}
		return -1
	}
	var display int32
	var closes int
	xOpenDisplay = func(_ *byte) Display { return Display(unsafe.Pointer(&display)) }
	xCloseDisplay = func(_ Display) int32 {
		closes++
		return 0
	}
	xFree = func(_ unsafe.Pointer) int32 { return 0 }
	glXChooseFBConfig = func(_ Display, _ int32, _, count *int32) *FBConfig {
		*count = int32(len(configs))
		if len(configs) == 0 {
			return nil
		}
		return &configs[0]
	}
	glXGetVisualFromFBConfig = func(_ Display, config FBConfig) *xVisualInfo {
		if i := indexOf(config); i >= 0 && cfgs[i].visualID != 0 {
			return &visuals[i]
		}
		return nil
	}
	glXGetFBConfigAttrib = func(_ Display, config FBConfig, attribute int32, value *int32) int32 {
		if attribute == glxTransparentType {
			*value = glxNone
			if i := indexOf(config); i >= 0 && cfgs[i].transparent {
				*value = 2 // GLX_TRANSPARENT_RGB
			}
		}
		return 0
	}
	return &closes
}

// TestNewGLXFallsBackToOpaqueConfig verifies that a transparency request is a preference rather than a requirement:
// when no framebuffer configuration on the system can provide transparency, an opaque one is used instead. The
// fallback loop used to be gated on !transparent, which made it unreachable, since the first loop already accepts any
// configuration with a visual when transparency is not wanted. A transparent window on such a system therefore failed
// outright, and glcontext_linux.go never retries with transparency turned off.
func TestNewGLXFallsBackToOpaqueConfig(t *testing.T) {
	c := check.New(t)
	closes := installFakeGLX(t, []fakeGLXConfig{
		{visualID: 0},               // glXGetVisualFromFBConfig fails for this one
		{visualID: 0x21, depth: 24}, // opaque
		{visualID: 0x22, depth: 32}, // opaque
	})
	glx, err := (&Conn{}).NewGLX(true)
	c.NoError(err)
	if glx == nil {
		t.Fatal("an opaque configuration must be used when none can provide transparency")
	}
	c.Equal(VisualID(0x21), glx.Visual(), "the first configuration with a visual must be used")
	c.Equal(byte(24), glx.Depth())
	c.Equal(0, *closes, "a successful NewGLX must leave the display open")
}

// TestNewGLXPrefersTransparentConfig verifies that the fallback does not preempt a configuration that actually
// supports transparency.
func TestNewGLXPrefersTransparentConfig(t *testing.T) {
	c := check.New(t)
	installFakeGLX(t, []fakeGLXConfig{
		{visualID: 0x21, depth: 24},
		{visualID: 0x22, depth: 32, transparent: true},
	})
	glx, err := (&Conn{}).NewGLX(true)
	c.NoError(err)
	if glx == nil {
		t.Fatal("the transparency-capable configuration must be used")
	}
	c.Equal(VisualID(0x22), glx.Visual())
	c.Equal(byte(32), glx.Depth())
}

// TestNewGLXOpaqueRequestUsesFirstUsableConfig verifies that an opaque request is unaffected: it takes the first
// configuration with a visual, transparency-capable or not.
func TestNewGLXOpaqueRequestUsesFirstUsableConfig(t *testing.T) {
	c := check.New(t)
	installFakeGLX(t, []fakeGLXConfig{
		{visualID: 0},
		{visualID: 0x21, depth: 24},
		{visualID: 0x22, depth: 32, transparent: true},
	})
	glx, err := (&Conn{}).NewGLX(false)
	c.NoError(err)
	if glx == nil {
		t.Fatal("an opaque request must accept the first configuration with a visual")
	}
	c.Equal(VisualID(0x21), glx.Visual())
	c.Equal(byte(24), glx.Depth())
}

// TestNewGLXFailsWhenNoConfigHasAVisual verifies that a genuinely unusable server is still reported as an error, with
// the display closed rather than leaked.
func TestNewGLXFailsWhenNoConfigHasAVisual(t *testing.T) {
	c := check.New(t)
	closes := installFakeGLX(t, []fakeGLXConfig{{visualID: 0}, {visualID: 0}})
	glx, err := (&Conn{}).NewGLX(true)
	c.HasError(err)
	c.True(glx == nil)
	c.Equal(1, *closes, "the display must be closed when NewGLX fails")
}
