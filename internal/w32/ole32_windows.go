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
	"syscall"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/xos"
	"golang.org/x/sys/windows"
)

var (
	ole32                = syscall.NewLazyDLL("ole32.dll")
	coCreateInstanceProc = ole32.NewProc("CoCreateInstance")
	doDragDropProc       = ole32.NewProc("DoDragDrop")
	oleInitializeProc    = ole32.NewProc("OleInitialize")
	registerDragDropProc = ole32.NewProc("RegisterDragDrop")
	releaseStgMediumProc = ole32.NewProc("ReleaseStgMedium")
	revokeDragDropProc   = ole32.NewProc("RevokeDragDrop")
	iidUnknown           = xos.Must(windows.GUIDFromString("{00000000-0000-0000-C000-000000000046}"))
	nullGUID             windows.GUID
)

const (
	COM_S_OK                         uint64 = 0
	COM_S_FALSE                      uint64 = 1
	COM_E_NOTIMPL                    uint64 = 0x80004001
	COM_E_NOINTERFACE                uint64 = 0x80004002
	COM_DV_E_FORMATETC               uint64 = 0x80040064
	COM_DV_E_TYMED                   uint64 = 0x80040069
	COM_OLE_E_ADVISENOTSUPPORTED     uint64 = 0x80040003
	COM_DRAGDROP_S_DROP              uint64 = 0x00040100
	COM_DRAGDROP_S_CANCEL            uint64 = 0x00040101
	COM_DRAGDROP_S_USEDEFAULTCURSORS uint64 = 0x00040102
	COM_DATA_S_SAMEFORMATETC         uint64 = 0x00040130
)

// TyMed represents the type of storage medium used in a drag-and-drop operation.
type TyMed uint32

// Possible values for TyMed.
const (
	TyMedHGlobal TyMed = 1 << iota
	TyMedFile
	TyMedIStream
	TyMedNull TyMed = 0
)

// STGMEDIUM https://learn.microsoft.com/en-us/windows/win32/api/objidl/ns-objidl-ustgmedium-r1
type STGMEDIUM struct {
	Tymed          TyMed
	_              uint32
	Data           uintptr // union: HGLOBAL, IStream*, etc.; Go inserts 4 bytes padding before this
	PUnkForRelease uintptr // IUnknown*
}

// CoCreateInstance https://learn.microsoft.com/en-us/windows/win32/api/combaseapi/nf-combaseapi-cocreateinstance
func CoCreateInstance(classID, instanceID windows.GUID) *Unknown {
	if instanceID == nullGUID {
		instanceID = iidUnknown
	}
	var unknown *Unknown
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	if r1, _, _ := coCreateInstanceProc.Call(uintptr(unsafe.Pointer(&classID)), 0,
		windows.CLSCTX_INPROC_SERVER|windows.CLSCTX_LOCAL_SERVER|windows.CLSCTX_REMOTE_SERVER,
		uintptr(unsafe.Pointer(&instanceID)), uintptr(unsafe.Pointer(&unknown))); r1 != 0 {
		return nil
	}
	return unknown
}

// DoDragDrop initiates a drag-and-drop operation. Blocks until the drag ends.
// Returns COM_DRAGDROP_S_DROP on drop, COM_DRAGDROP_S_CANCEL on cancel.
func DoDragDrop(dataObj, dropSrc unsafe.Pointer, okEffects uintptr, effect *uint32) uintptr {
	r, _, _ := doDragDropProc.Call(uintptr(dataObj), uintptr(dropSrc), okEffects, uintptr(unsafe.Pointer(effect)))
	return r
}

// OleInitialize initializes the COM library on the current thread with a single-threaded apartment. Returns true on
// success.
func OleInitialize() error {
	r, _, _ := oleInitializeProc.Call(0)
	if r != 0 && r != 1 { // 0 = S_OK, 1 = S_FALSE (already initialized)
		return errs.Newf("OleInitialize failed: 0x%X", r)
	}
	return nil
}

// RegisterDragDrop registers a window as a drop target.
func RegisterDragDrop(hwnd windows.HWND, target *DropTarget) uintptr {
	r, _, _ := registerDragDropProc.Call(uintptr(hwnd), uintptr(unsafe.Pointer(target)))
	return r
}

// ReleaseStgMedium releases resources held by a STGMEDIUM.
func ReleaseStgMedium(medium *STGMEDIUM) {
	//nolint:errcheck // Nothing we can do about an error here
	releaseStgMediumProc.Call(uintptr(unsafe.Pointer(medium)))
}

// RevokeDragDrop revokes a window's drop target registration.
func RevokeDragDrop(hwnd windows.HWND) {
	//nolint:errcheck // Nothing we can do about an error here
	revokeDragDropProc.Call(uintptr(hwnd))
}
