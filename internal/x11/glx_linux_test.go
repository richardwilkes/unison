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
