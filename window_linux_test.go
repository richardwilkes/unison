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
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/xos"
)

func TestWMClassData(t *testing.T) {
	c := check.New(t)

	savedCmd := xos.AppCmdName
	savedName := xos.AppName
	savedID := xos.AppIdentifier
	defer func() {
		xos.AppCmdName = savedCmd
		xos.AppName = savedName
		xos.AppIdentifier = savedID
	}()

	// Normal case: instance is the command name, class is the identifier. WM_CLASS is a pair of null-terminated
	// strings, so a .desktop file's StartupWMClass entry (which matches the class name) can associate with the window.
	xos.AppCmdName = "gcs"
	xos.AppName = "GCS"
	xos.AppIdentifier = "com.trollworks.gcs"
	c.Equal("gcs\x00com.trollworks.gcs\x00", string(wmClassData()))

	// Falls back to the application name when the command name is empty.
	xos.AppCmdName = ""
	c.Equal("GCS\x00com.trollworks.gcs\x00", string(wmClassData()))

	// Falls back to the instance name when the identifier is empty.
	xos.AppCmdName = "gcs"
	xos.AppIdentifier = ""
	c.Equal("gcs\x00gcs\x00", string(wmClassData()))
}

// TestX11WindowStateIsVisibleToPublicAPI verifies that the WM_STATE and _NET_WM_STATE property handlers record the
// state where IsMinimized and IsMaximized read it. The handlers used to write a platform-private copy of the flags
// instead, so those public methods always reported false on Linux no matter what the window manager did.
func TestX11WindowStateIsVisibleToPublicAPI(t *testing.T) {
	c := check.New(t)
	w := &Window{valid: true}
	var minimizedCalls, maximizedCalls []bool
	w.MinimizedCallback = func(minimized bool) { minimizedCalls = append(minimizedCalls, minimized) }
	w.MaximizedCallback = func(maximized bool) { maximizedCalls = append(maximizedCalls, maximized) }
	c.False(w.IsMinimized())
	c.False(w.IsMaximized())

	w.x11SetMinimized(true)
	c.True(w.IsMinimized())
	c.Equal([]bool{true}, minimizedCalls)

	// A repeat of the state already in effect must not re-notify.
	w.x11SetMinimized(true)
	c.Equal([]bool{true}, minimizedCalls)

	w.x11SetMinimized(false)
	c.False(w.IsMinimized())
	c.Equal([]bool{true, false}, minimizedCalls)

	w.x11SetMaximized(true)
	c.True(w.IsMaximized())
	c.Equal([]bool{true}, maximizedCalls)
	w.x11SetMaximized(true)
	c.Equal([]bool{true}, maximizedCalls)
	w.x11SetMaximized(false)
	c.False(w.IsMaximized())
	c.Equal([]bool{true, false}, maximizedCalls)
}
