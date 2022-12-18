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
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	fileOpenDialogCLSID = NewGUID("DC1C5A9C-E88A-4DDE-A5A1-60F82A20AEF7")
	fileOpenDialogIID   = NewGUID("D57C7288-D4AD-4768-BE02-9D969532D960")
)

type FileOpenDialog struct {
	FileDialog
}

type vmtFileOpenDialog struct {
	vmtFileDialog
	GetResults       uintptr
	GetSelectedItems uintptr
}

func (obj *FileOpenDialog) vmt() *vmtFileOpenDialog {
	return (*vmtFileOpenDialog)(obj.UnsafeVirtualMethodTable)
}

func NewOpenDialog() *FileOpenDialog {
	CoInitialize(windows.COINIT_MULTITHREADED | windows.COINIT_DISABLE_OLE1DDE)
	return (*FileOpenDialog)(unsafe.Pointer(CoCreateInstance(fileOpenDialogCLSID, fileOpenDialogIID)))
}

func (obj *FileOpenDialog) GetResults() []string {
	var array *ShellItemArray
	r1, _, _ := syscall.SyscallN(obj.vmt().GetResults, uintptr(unsafe.Pointer(obj)), uintptr(unsafe.Pointer(&array)))
	if r1 != 0 {
		return nil
	}
	defer array.Release()
	s := make([]string, array.Count())
	for i := range s {
		s[i] = array.Item(i)
	}
	return s
}
