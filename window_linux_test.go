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
