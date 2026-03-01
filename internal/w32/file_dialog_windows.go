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
	"strings"
	"syscall"
	"unsafe"
)

// https://learn.microsoft.com/en-us/windows/win32/api/shobjidl_core/ne-shobjidl_core-_fileopendialogoptions
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

// FileFilter represents a single filter for a file dialog, with a user-friendly name and a pattern to match against
// file names.
type FileFilter struct {
	Name    string
	Pattern string
}

type filterSpec struct {
	name    *int16
	pattern *int16
}

// FileDialog https://learn.microsoft.com/en-us/windows/win32/api/shobjidl_core/nn-shobjidl_core-ifiledialog
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
	SetClientGUID       uintptr
	ClearClientData     uintptr
	SetFilter           uintptr
}

func (obj *FileDialog) vmt() *vmtFileDialog {
	return (*vmtFileDialog)(obj.UnsafeVirtualMethodTable)
}

// SetFolder sets the initial folder for the file dialog to open in. The path should be an absolute path to a folder.
func (obj *FileDialog) SetFolder(path string) {
	if item := NewShellItem(path); item != nil {
		defer item.Release()
		//nolint:errcheck // Nothing we can do about an error here
		syscall.SyscallN(obj.vmt().SetFolder, uintptr(unsafe.Pointer(obj)), uintptr(unsafe.Pointer(item)))
	}
}

// SetOptions sets the options for the file dialog, using a bitwise combination of the FOS* constants defined above.
func (obj *FileDialog) SetOptions(options int) {
	//nolint:errcheck // Nothing we can do about an error here
	syscall.SyscallN(obj.vmt().SetOptions, uintptr(unsafe.Pointer(obj)), uintptr(options))
}

// SetFileTypes sets the file type filters for the file dialog. Each filter consists of a user-friendly name and a
// pattern to match against file names. The pattern can include wildcards, such as "*.txt" to match all text files. If
// no filters are set, all files will be shown.
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
	//nolint:errcheck // Nothing we can do about an error here
	syscall.SyscallN(obj.vmt().SetFileTypes, uintptr(unsafe.Pointer(obj)), uintptr(len(specs)),
		uintptr(unsafe.Pointer(&specs[0])))
}

// SetDefaultExtension sets the default file extension for the file dialog. This is the extension that will be
// automatically appended to the file name if the user does not specify an extension. The extension should be specified
// without a leading dot, e.g. "txt" for text files.
func (obj *FileDialog) SetDefaultExtension(ext string) {
	//nolint:errcheck // Nothing we can do about an error here
	syscall.SyscallN(obj.vmt().SetDefaultExtension, uintptr(unsafe.Pointer(obj)),
		uintptr(unsafe.Pointer(SysAllocString(strings.TrimPrefix(ext, ".")))))
}

// SetFileName sets the initial file name for the file dialog. This is the file name that will be pre-filled in the file
// name input field when the dialog is shown.
func (obj *FileDialog) SetFileName(fileName string) {
	//nolint:errcheck // Nothing we can do about an error here
	syscall.SyscallN(obj.vmt().SetFileName, uintptr(unsafe.Pointer(obj)),
		uintptr(unsafe.Pointer(SysAllocString(fileName))))
}

// GetResult retrieves the file path selected by the user in the file dialog. If the user cancels the dialog or an error
// occurs, an empty string is returned.
func (obj *FileDialog) GetResult() string {
	var item *ShellItem
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	r1, _, _ := syscall.SyscallN(obj.vmt().GetResult, uintptr(unsafe.Pointer(obj)), uintptr(unsafe.Pointer(&item)))
	if r1 != 0 || item == nil {
		return ""
	}
	defer item.Release()
	return item.DisplayName()
}
