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
	"os"
	"strings"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
)

// TestSaveDialogNeverRequestsFolderPicking verifies the Windows save dialog does not set FOS_PICKFOLDERS:
// IFileSaveDialog does not support that option, and since SetOptions' HRESULT is not surfaced, a rejected flag would
// silently drop every other option in the mask, including the overwrite prompt.
func TestSaveDialogNeverRequestsFolderPicking(t *testing.T) {
	c := check.New(t)
	data, err := os.ReadFile("save_dialog_windows.go")
	c.NoError(err)
	c.False(strings.Contains(string(data), "w32.FOSPickFolders"))
}
