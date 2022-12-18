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
	"strings"
	"syscall"
	"unsafe"
)

const (
	FOSOverwritePrompt          = 0x00000002
	FOSStrictFileTypes          = 0x00000004
	FOSNoChangeDir              = 0x00000008
	FOSPickFolders              = 0x00000020
	FOSForceFileSystem          = 0x00000040
	FOSAllNonStorageItems       = 0x00000080
	FOSNoValidate               = 0x00000100
	FOSAllowMultiSelect         = 0x00000200
	FOSPathMustExist            = 0x00000800
	FOSFileMustExist            = 0x00001000
	FOSCreatePrompt             = 0x00002000
	FOSShareAware               = 0x00004000
	FOSNoReadOnlyReturn         = 0x00008000
	FOSNoTestFileCreate         = 0x00010000
	FOSHideMRUPlaces            = 0x00020000
	FOSHidePinnedPlaces         = 0x00040000
	FOSNoDereferenceLinks       = 0x00100000
	FOSOKBUttonNeedsInteraction = 0x00200000
	FOSDontAddToRecent          = 0x02000000
	FOSForceShowHidden          = 0x10000000
	FOSDefaultNoMiniMode        = 0x20000000
	FOSForcePreviewPaneOn       = 0x40000000
	FOSSupportsStreamableItems  = 0x80000000
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

func (obj *FileDialog) SetDefaultExtension(ext string) {
	syscall.SyscallN(obj.vmt().SetDefaultExtension, uintptr(unsafe.Pointer(obj)),
		uintptr(unsafe.Pointer(SysAllocString(strings.TrimPrefix(ext, ".")))))
}

func (obj *FileDialog) SetFileName(fileName string) {
	syscall.SyscallN(obj.vmt().SetFileName, uintptr(unsafe.Pointer(obj)),
		uintptr(unsafe.Pointer(SysAllocString(fileName))))
}

func (obj *FileDialog) GetResult() string {
	var item *ShellItem
	r1, _, _ := syscall.SyscallN(obj.vmt().GetResult, uintptr(unsafe.Pointer(obj)), uintptr(unsafe.Pointer(&item)))
	if r1 != 0 || item == nil {
		return ""
	}
	defer item.Release()
	return item.DisplayName()
}
