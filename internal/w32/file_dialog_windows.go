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
)

const (
	FileDialogOptionOverwritePrompt  = 0x2
	FileDialogOptionPickFolders      = 0x20
	FileDialogOptionAllowMultiSelect = 0x200
	FileDialogOptionPathMustExist    = 0x800
	FileDialogOptionFileMustExist    = 0x1000
)

type FileFilter struct {
	Name    string
	Pattern string
}

type filterSpec struct {
	name    *int16
	pattern *int16
}

type FileDialog struct {
	ModalWindow
}

type vmtFileDialog struct {
	vmtModalWindow
	SetFileTypes        uintptr
	SetFileTypeIndex    uintptr
	GetFileTypeIndex    uintptr
	Advise              uintptr
	Unadvise            uintptr
	SetOptions          uintptr
	GetOptions          uintptr
	SetDefaultFolder    uintptr
	SetFolder           uintptr
	GetFolder           uintptr
	GetCurrentSelection uintptr
	SetFileName         uintptr
	GetFileName         uintptr
	SetTitle            uintptr
	SetOkButtonLabel    uintptr
	SetFileNameLabel    uintptr
	GetResult           uintptr
	AddPlace            uintptr
	SetDefaultExtension uintptr
	Close               uintptr
	SetClientGuid       uintptr
	ClearClientData     uintptr
	SetFilter           uintptr
}

func (obj *FileDialog) vmt() *vmtFileDialog {
	return (*vmtFileDialog)(obj.UnsafeVirtualMethodTable)
}

func (obj *FileDialog) SetFolder(path string) {
	if item := NewShellItem(path); item != nil {
		defer item.Release()
		syscall.SyscallN(obj.vmt().SetFolder, uintptr(unsafe.Pointer(obj)), uintptr(unsafe.Pointer(item)))
	}
}

func (obj *FileDialog) SetOptions(options int) {
	syscall.SyscallN(obj.vmt().SetOptions, uintptr(unsafe.Pointer(obj)), uintptr(options))
}

func (obj *FileDialog) SetFileTypes(filters []FileFilter) {
	if len(filters) == 0 {
		return
	}
	specs := make([]filterSpec, len(filters))
	for i, one := range filters {
		specs[i] = filterSpec{
			name:    SysAllocString(one.Name),
			pattern: SysAllocString(one.Pattern),
		}
	}
	syscall.SyscallN(obj.vmt().SetFileTypes, uintptr(unsafe.Pointer(obj)), uintptr(len(specs)),
		uintptr(unsafe.Pointer(&specs[0])))
}
