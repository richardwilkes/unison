// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
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

	"golang.org/x/sys/windows"
)

var (
	fileSaveDialogCLSID = NewGUID("C0B4E2F3-BA21-4773-8DBA-335EC946EB8B")
	fileSaveDialogIID   = NewGUID("84BCCD23-5FDE-4CDB-AEA4-AF64B83D78AB")
)

type FileSaveDialog struct {
	FileDialog
}

type vmtFileSaveDialog struct {
	vmtFileDialog
	SetSaveAsItem          uintptr
	SetProperties          uintptr
	SetCollectedProperties uintptr
	GetProperties          uintptr
	ApplyProperties        uintptr
}

func (obj *FileSaveDialog) vmt() *vmtFileSaveDialog {
	return (*vmtFileSaveDialog)(obj.UnsafeVirtualMethodTable)
}

func NewSaveDialog() *FileSaveDialog {
	CoInitialize(windows.COINIT_MULTITHREADED | windows.COINIT_DISABLE_OLE1DDE)
	return (*FileSaveDialog)(unsafe.Pointer(CoCreateInstance(fileSaveDialogCLSID, fileSaveDialogIID)))
}
