// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package w32

import (
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/xos"
	"golang.org/x/sys/windows"
)

var (
	fileSaveDialogCLSID = xos.Must(windows.GUIDFromString("{C0B4E2F3-BA21-4773-8DBA-335EC946EB8B}"))
	fileSaveDialogIID   = xos.Must(windows.GUIDFromString("{84BCCD23-5FDE-4CDB-AEA4-AF64B83D78AB}"))
)

type FileSaveDialog struct {
	FileDialog
}

// NewSaveDialog creates a new IFileSaveDialog instance. The caller owns the returned (+1) reference and must call
// Release when done with it. COM is already initialized with a single-threaded apartment on the UI thread via
// OleInitialize during startup, which is what IFileDialog (an STA-only object) requires, so no CoInitializeEx call is
// made here.
func NewSaveDialog() *FileSaveDialog {
	return (*FileSaveDialog)(unsafe.Pointer(CoCreateInstance(fileSaveDialogCLSID, fileSaveDialogIID)))
}
