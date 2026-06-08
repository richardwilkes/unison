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

	"golang.org/x/sys/windows"
)

// DropEffect represents the effect of a drag-and-drop operation.
type DropEffect uint32

// Possible values for DropEffect.
const (
	DropEffectCopy DropEffect = 1 << iota
	DropEffectMove
	DropEffectLink
	DropEffectNone   DropEffect = 0
	DropEffectScroll DropEffect = 0x80000000
)

// TyMed represents the type of storage medium used in a drag-and-drop operation.
type TyMed uint32

// Possible values for TyMed.
const (
	TyMedNull TyMed = iota
	TyMedHGlobal
)

// DVAspect represents the aspect of data being transferred in a drag-and-drop operation.
type DVAspect uint32

// Possible values for DVAspect.
const (
	DVAspectContent DVAspect = 1 << iota
	DVAspectThumbnail
	DVAspectIcon
	DVAspectDocPrint
)

// MKDnD represents the key state and button modifiers passed to IDropTarget callbacks.
type MKDnD uint32

// Possible values for MKDnD.
const (
	MKDnDLButton MKDnD = 1 << iota
	MKDnDRButton
	MKDnDShift
	MKDnDControl
	MKDnDMButton
	MKDnDAlt
)

// COM HRESULT constants.
const (
	COM_S_OK                         = uintptr(0)
	COM_S_FALSE                      = uintptr(1)
	COM_E_NOTIMPL                    = uintptr(0x80004001)
	COM_E_NOINTERFACE                = uintptr(0x80004002)
	COM_DV_E_FORMATETC               = uintptr(0x80040064)
	COM_DV_E_TYMED                   = uintptr(0x80040069)
	COM_OLE_E_ADVISENOTSUPPORTED     = uintptr(0x80040003)
	COM_DRAGDROP_S_DROP              = uintptr(0x00040100)
	COM_DRAGDROP_S_CANCEL            = uintptr(0x00040101)
	COM_DRAGDROP_S_USEDEFAULTCURSORS = uintptr(0x00040102)
	COM_DATA_S_SAMEFORMATETC         = uintptr(0x00040130)
)

// FORMATETC https://learn.microsoft.com/en-us/windows/win32/api/objidl/ns-objidl-formatetc
// On 64-bit: CfFormat at 0 (2 bytes), 6 bytes implicit padding, Ptd at 8 (8 bytes),
// DwAspect at 16 (4 bytes), Lindex at 20 (4 bytes), Tymed at 24 (4 bytes), 4 bytes trailing. Total: 32 bytes.
type FORMATETC struct {
	CfFormat uint16
	Ptd      uintptr // *DVTARGETDEVICE; Go inserts 6 bytes of padding before this
	DwAspect DVAspect
	Lindex   int32
	Tymed    TyMed
	// Go adds 4 bytes of trailing padding to align to 8-byte struct alignment
}

// STGMEDIUM https://learn.microsoft.com/en-us/windows/win32/api/objidl/ns-objidl-ustgmedium-r1
// On 64-bit: Tymed at 0 (4 bytes), 4 bytes implicit padding, Data at 8 (8 bytes),
// PUnkForRelease at 16 (8 bytes). Total: 24 bytes.
type STGMEDIUM struct {
	Tymed          TyMed
	Data           uintptr // union: HGLOBAL, IStream*, etc.; Go inserts 4 bytes padding before this
	PUnkForRelease uintptr // IUnknown*
}

// DROPFILES https://learn.microsoft.com/en-us/windows/win32/api/shlobj_core/ns-shlobj_core-dropfiles
type DROPFILES struct {
	PFiles uint32
	PtX    int32
	PtY    int32
	FNC    uint32 // BOOL: non-client area flag
	FWide  uint32 // BOOL: TRUE = Unicode wide chars
}

var (
	oleInitializeProc    = ole32.NewProc("OleInitialize")
	registerDragDropProc = ole32.NewProc("RegisterDragDrop")
	revokeDragDropProc   = ole32.NewProc("RevokeDragDrop")
	doDragDropProc       = ole32.NewProc("DoDragDrop")
	releaseStgMediumProc = ole32.NewProc("ReleaseStgMedium")
)

// OleInitialize initializes the COM library on the current thread with a single-threaded apartment.
func OleInitialize() uintptr {
	r, _, _ := oleInitializeProc.Call(0)
	return r
}

// RegisterDragDrop registers a window as a drop target.
func RegisterDragDrop(hwnd windows.HWND, target unsafe.Pointer) uintptr {
	r, _, _ := registerDragDropProc.Call(uintptr(hwnd), uintptr(target))
	return r
}

// RevokeDragDrop revokes a window's drop target registration.
func RevokeDragDrop(hwnd windows.HWND) {
	//nolint:errcheck // Nothing we can do about an error here
	revokeDragDropProc.Call(uintptr(hwnd))
}

// DoDragDrop initiates a drag-and-drop operation. Blocks until the drag ends.
// Returns COM_DRAGDROP_S_DROP on drop, COM_DRAGDROP_S_CANCEL on cancel.
func DoDragDrop(dataObj, dropSrc unsafe.Pointer, okEffects uintptr, effect *uint32) uintptr {
	r, _, _ := doDragDropProc.Call(uintptr(dataObj), uintptr(dropSrc), okEffects, uintptr(unsafe.Pointer(effect)))
	return r
}

// ReleaseStgMedium releases resources held by a STGMEDIUM.
func ReleaseStgMedium(medium *STGMEDIUM) {
	//nolint:errcheck // Nothing we can do about an error here
	releaseStgMediumProc.Call(uintptr(unsafe.Pointer(medium)))
}

// IDataObject wraps an IDataObject COM interface pointer received from Windows.
// https://learn.microsoft.com/en-us/windows/win32/api/objidl/nn-objidl-idataobject
type IDataObject struct {
	Unknown
}

type vmtIDataObject struct {
	vmtUnknown
	GetData               uintptr
	GetDataHere           uintptr
	QueryGetData          uintptr
	GetCanonicalFormatEtc uintptr
	SetData               uintptr
	EnumFormatEtc         uintptr
	DAdvise               uintptr
	DUnadvise             uintptr
	EnumDAdvise           uintptr
}

func (obj *IDataObject) vmt() *vmtIDataObject {
	return (*vmtIDataObject)(obj.UnsafeVirtualMethodTable)
}

// GetData retrieves data described by the given format. Caller must call ReleaseStgMedium on success.
func (obj *IDataObject) GetData(fmtEtc *FORMATETC) (STGMEDIUM, uintptr) {
	var stg STGMEDIUM
	r, _, _ := syscall.SyscallN(obj.vmt().GetData,
		uintptr(unsafe.Pointer(obj)),
		uintptr(unsafe.Pointer(fmtEtc)),
		uintptr(unsafe.Pointer(&stg)))
	return stg, r
}

// QueryGetData checks whether a data object can supply data for the given format.
func (obj *IDataObject) QueryGetData(fmtEtc *FORMATETC) uintptr {
	r, _, _ := syscall.SyscallN(obj.vmt().QueryGetData,
		uintptr(unsafe.Pointer(obj)),
		uintptr(unsafe.Pointer(fmtEtc)))
	return r
}

// EnumFormatEtc returns an enumerator of the formats the data object supports.
// Caller must Release the returned enumerator.
func (obj *IDataObject) EnumFormatEtc(direction uint32) *IEnumFORMATETC {
	var enumObj *IEnumFORMATETC
	r, _, _ := syscall.SyscallN(obj.vmt().EnumFormatEtc,
		uintptr(unsafe.Pointer(obj)),
		uintptr(direction),
		uintptr(unsafe.Pointer(&enumObj)))
	if r != 0 {
		return nil
	}
	return enumObj
}

// IEnumFORMATETC wraps an IEnumFORMATETC COM interface pointer received from Windows.
type IEnumFORMATETC struct {
	Unknown
}

type vmtIEnumFORMATETC struct {
	vmtUnknown
	Next  uintptr
	Skip  uintptr
	Reset uintptr
	Clone uintptr
}

func (obj *IEnumFORMATETC) vmt() *vmtIEnumFORMATETC {
	return (*vmtIEnumFORMATETC)(obj.UnsafeVirtualMethodTable)
}

// Next retrieves the next formats from the enumerator. Returns the count actually fetched.
func (obj *IEnumFORMATETC) Next(formats []FORMATETC) int {
	if len(formats) == 0 {
		return 0
	}
	var fetched uint32
	syscall.SyscallN(obj.vmt().Next, //nolint:errcheck // Return value is fetched count, checked below
		uintptr(unsafe.Pointer(obj)),
		uintptr(len(formats)),
		uintptr(unsafe.Pointer(&formats[0])),
		uintptr(unsafe.Pointer(&fetched)))
	return int(fetched)
}

// Reset resets the enumerator to the beginning.
func (obj *IEnumFORMATETC) Reset() {
	//nolint:errcheck // Nothing we can do about an error here
	syscall.SyscallN(obj.vmt().Reset, uintptr(unsafe.Pointer(obj)))
}
