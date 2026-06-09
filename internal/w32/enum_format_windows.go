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
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/xos"
	"golang.org/x/sys/windows"
)

var iidIEnumFORMATETC = xos.Must(windows.GUIDFromString("{00000103-0000-0000-C000-000000000046}"))

// DVAspect represents the aspect of data being transferred in a drag-and-drop operation.
type DVAspect uint32

// Possible values for DVAspect.
const (
	DVAspectContent DVAspect = 1 << iota
	DVAspectThumbnail
	DVAspectIcon
	DVAspectDocPrint
)

// FORMATETC https://learn.microsoft.com/en-us/windows/win32/api/objidl/ns-objidl-formatetc
type FORMATETC struct {
	CfFormat uint16
	_        uint16
	_        uint32
	Ptd      uintptr // *DVTARGETDEVICE; Go inserts 6 bytes of padding before this
	DwAspect DVAspect
	Lindex   int32
	Tymed    TyMed
	_        uint32
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

// enumFORMATETC is a Go-implemented COM IEnumFORMATETC for a DataObject.
type enumFORMATETC struct {
	lpVtbl   uintptr // MUST BE FIRST: points to enumFmtVtbl
	obj      *DataObject
	pos      int
	refCount int32
}

var enumFmtVtbl [7]uintptr

func init() {
	enumFmtVtbl[0] = windows.NewCallback(enumQueryInterface)
	enumFmtVtbl[1] = windows.NewCallback(enumAddRef)
	enumFmtVtbl[2] = windows.NewCallback(enumRelease)
	enumFmtVtbl[3] = windows.NewCallback(enumNext)
	enumFmtVtbl[4] = windows.NewCallback(enumSkip)
	enumFmtVtbl[5] = windows.NewCallback(enumResetCB)
	enumFmtVtbl[6] = windows.NewCallback(enumClone)
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

func newEnumFORMATETC(obj *DataObject) *enumFORMATETC {
	e := &enumFORMATETC{obj: obj, refCount: 1}
	e.lpVtbl = uintptr(unsafe.Pointer(&enumFmtVtbl[0]))
	return e
}

func (e *enumFORMATETC) Reset() {
	e.pos = 0
}

func enumQueryInterface(this, riid, ppvObject uintptr) uint64 {
	guid := (*windows.GUID)(unsafe.Pointer(riid))
	if *guid == iidUnknown || *guid == iidIEnumFORMATETC {
		*(*uintptr)(unsafe.Pointer(ppvObject)) = this
		enumAddRef(this)
		return COM_S_OK
	}
	*(*uintptr)(unsafe.Pointer(ppvObject)) = 0
	return COM_E_NOINTERFACE
}

func enumAddRef(this uintptr) uintptr {
	e := (*enumFORMATETC)(unsafe.Pointer(this))
	return uintptr(atomic.AddInt32(&e.refCount, 1))
}

func enumRelease(this uintptr) uintptr {
	e := (*enumFORMATETC)(unsafe.Pointer(this))
	return uintptr(atomic.AddInt32(&e.refCount, -1))
}

func enumNext(this, celt, rgelt, pceltFetched uintptr) uint64 {
	e := (*enumFORMATETC)(unsafe.Pointer(this))
	count := int(celt)
	dst := unsafe.Slice((*FORMATETC)(unsafe.Pointer(rgelt)), count)
	fetched := 0
	for fetched < count && e.pos < len(e.obj.entries) {
		dst[fetched] = e.obj.entries[e.pos].fmtEtc
		fetched++
		e.pos++
	}
	if pceltFetched != 0 {
		*(*uint32)(unsafe.Pointer(pceltFetched)) = uint32(fetched)
	}
	if fetched == count {
		return COM_S_OK
	}
	return COM_S_FALSE
}

func enumSkip(this, celt uintptr) uint64 {
	e := (*enumFORMATETC)(unsafe.Pointer(this))
	count := int(celt)
	remaining := len(e.obj.entries) - e.pos
	if count > remaining {
		e.pos = len(e.obj.entries)
		return COM_S_FALSE
	}
	e.pos += count
	return COM_S_OK
}

func enumResetCB(this uintptr) uint64 {
	e := (*enumFORMATETC)(unsafe.Pointer(this))
	e.pos = 0
	return COM_S_OK
}

func enumClone(_ uintptr, _ uintptr) uint64 { return COM_E_NOTIMPL }
