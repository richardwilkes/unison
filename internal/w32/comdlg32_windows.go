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

var (
	comdlg32             = syscall.NewLazyDLL("comdlg32.dll")
	getOpenFileNameWProc = comdlg32.NewProc("GetOpenFileNameW")
	getSaveFileNameWProc = comdlg32.NewProc("GetSaveFileNameW")
)

// Constants from https://docs.microsoft.com/en-us/windows/win32/api/commdlg/ns-commdlg-openfilenamea
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

// OpenFileName https://docs.microsoft.com/en-us/windows/win32/api/commdlg/ns-commdlg-openfilenamew
//
//nolint:maligned // Can't do anything about Windows structs being poorly aligned
type OpenFileName struct {
	Size            uint32
	Owner           HWND
	Instance        syscall.Handle
	Filter          uintptr
	CustomFilter    uintptr
	MaxCustomFilter uint32
	FilterIndex     uint32
	FileName        uintptr
	MaxFileName     uint32
	FileTitle       uintptr
	MaxFileTitle    uint32
	InitialDir      uintptr
	Title           uintptr
	Flags           uint32
	FileOffset      uint16
	FileExtension   uint16
	DefExt          uintptr
	CustData        uintptr
	Hook            uintptr
	TemplateName    uintptr
	Reserved1       uintptr
	Reserved2       uint32
	FlagsEx         uint32
}

// GetOpenFileName https://docs.microsoft.com/en-us/windows/win32/api/commdlg/nf-commdlg-getopenfilenamew
func GetOpenFileName(ofn *OpenFileName) bool {
	b, _, _ := getOpenFileNameWProc.Call(uintptr(unsafe.Pointer(ofn)))
	return b != 0
}

// GetSaveFileName https://docs.microsoft.com/en-us/windows/win32/api/commdlg/nf-commdlg-getsavefilenamew
func GetSaveFileName(ofn *OpenFileName) bool {
	b, _, _ := getSaveFileNameWProc.Call(uintptr(unsafe.Pointer(ofn)))
	return b != 0
}
