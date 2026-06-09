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
	"runtime"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/uti"
	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/unison/drag"
	"golang.org/x/sys/windows"
)

var iidIDataObject = xos.Must(windows.GUIDFromString("{0000010E-0000-0000-C000-000000000046}"))

// DataDir represents the direction of data flow.
type DataDir uint32

// Possible values for DataDir.
const (
	DataDirGet DataDir = 1 << iota
	DataDirSet
)

type dragDataEntry struct {
	fmtEtc FORMATETC
	data   []byte // data in Windows format (UTF-16LE for text, raw bytes otherwise)
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

// DataObject is a Go-implemented COM IDataObject that carries drag data.
type DataObject struct {
	lpVtbl   uintptr // MUST BE FIRST: points to dataObjVtbl
	entries  []dragDataEntry
	enumFmt  *enumFORMATETC
	refCount int32
	pinner   runtime.Pinner
}

var dataObjVtbl [12]uintptr

func init() {
	dataObjVtbl[0] = windows.NewCallback(dataObjQueryInterface)
	dataObjVtbl[1] = windows.NewCallback(dataObjAddRef)
	dataObjVtbl[2] = windows.NewCallback(dataObjRelease)
	dataObjVtbl[3] = windows.NewCallback(dataObjGetData)
	dataObjVtbl[4] = windows.NewCallback(dataObjGetDataHere)
	dataObjVtbl[5] = windows.NewCallback(dataObjQueryGetData)
	dataObjVtbl[6] = windows.NewCallback(dataObjGetCanonicalFormatEtc)
	dataObjVtbl[7] = windows.NewCallback(dataObjSetData)
	dataObjVtbl[8] = windows.NewCallback(dataObjEnumFormatEtc)
	dataObjVtbl[9] = windows.NewCallback(dataObjDAdvise)
	dataObjVtbl[10] = windows.NewCallback(dataObjDUnadvise)
	dataObjVtbl[11] = windows.NewCallback(dataObjEnumDAdvise)
}

func (obj *IDataObject) vmt() *vmtIDataObject {
	return (*vmtIDataObject)(obj.UnsafeVirtualMethodTable)
}

// GetData retrieves data described by the given format. Caller must call ReleaseStgMedium on success.
func (obj *IDataObject) GetData(fmtEtc *FORMATETC) (STGMEDIUM, bool) {
	var stg STGMEDIUM
	r, _, _ := syscall.SyscallN(obj.vmt().GetData,
		uintptr(unsafe.Pointer(obj)),
		uintptr(unsafe.Pointer(fmtEtc)),
		uintptr(unsafe.Pointer(&stg)))
	return stg, r == 0
}

// QueryGetData checks whether a data object can supply data for the given format.
func (obj *IDataObject) QueryGetData(fmtEtc *FORMATETC) bool {
	r, _, _ := syscall.SyscallN(obj.vmt().QueryGetData,
		uintptr(unsafe.Pointer(obj)),
		uintptr(unsafe.Pointer(fmtEtc)))
	return r == 0
}

// EnumFormatEtc returns an enumerator of the formats the data object supports.
// Caller must Release the returned enumerator.
func (obj *IDataObject) EnumFormatEtc(direction DataDir) *IEnumFORMATETC {
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

// NewDataObject creates a DataObject carrying the given drag data.
func NewDataObject(data []drag.Data, opMask drag.Op) *DataObject {
	entries := make([]dragDataEntry, 0, len(data))
	for _, d := range data {
		cf := LookupDataType(d.Type.UTI)
		if cf == CFNone {
			continue
		}
		var raw []byte
		if uti.UTF8PlainText.ConformsTo(d.Type) {
			s, err := windows.UTF16FromString(string(d.Data))
			if err != nil {
				errs.Log(err)
				continue
			}
			raw = make([]byte, len(s)*2)
			for i, v := range s {
				raw[i*2] = byte(v)
				raw[i*2+1] = byte(v >> 8)
			}
		} else {
			raw = d.Data
		}
		entries = append(entries, dragDataEntry{
			fmtEtc: FORMATETC{
				CfFormat: uint16(cf),
				DwAspect: DVAspectContent,
				Lindex:   -1,
				Tymed:    TyMedHGlobal,
			},
			data: raw,
		})
	}
	obj := &DataObject{entries: entries, refCount: 1}
	obj.lpVtbl = uintptr(unsafe.Pointer(&dataObjVtbl[0]))
	obj.enumFmt = newEnumFORMATETC(obj)
	obj.pinner.Pin(obj)
	obj.pinner.Pin(obj.enumFmt)
	return obj
}

// Release unpins the DataObject and its internal enumerator from the Go garbage collector.
func (obj *DataObject) Release() {
	obj.pinner.Unpin()
}

func (obj *DataObject) findEntry(cf uint16) ([]byte, bool) {
	for _, e := range obj.entries {
		if e.fmtEtc.CfFormat == cf {
			return e.data, true
		}
	}
	return nil, false
}

func dataObjQueryInterface(this, riid, ppvObject uintptr) uint64 {
	guid := (*windows.GUID)(unsafe.Pointer(riid))
	if *guid == iidUnknown || *guid == iidIDataObject {
		*(*uintptr)(unsafe.Pointer(ppvObject)) = this
		dataObjAddRef(this)
		return COM_S_OK
	}
	*(*uintptr)(unsafe.Pointer(ppvObject)) = 0
	return COM_E_NOINTERFACE
}

func dataObjAddRef(this uintptr) uintptr {
	obj := (*DataObject)(unsafe.Pointer(this))
	return uintptr(atomic.AddInt32(&obj.refCount, 1))
}

func dataObjRelease(this uintptr) uintptr {
	obj := (*DataObject)(unsafe.Pointer(this))
	return uintptr(atomic.AddInt32(&obj.refCount, -1))
}

func dataObjGetData(this, pformatetcIn, pmedium uintptr) uint64 {
	obj := (*DataObject)(unsafe.Pointer(this))
	fe := (*FORMATETC)(unsafe.Pointer(pformatetcIn))
	data, ok := obj.findEntry(fe.CfFormat)
	if !ok {
		return COM_DV_E_FORMATETC
	}
	if fe.Tymed&TyMedHGlobal == 0 {
		return COM_DV_E_TYMED
	}
	h := GlobalAlloc(GMemMoveable, len(data))
	if h == 0 {
		return COM_E_NOTIMPL
	}
	buf := GlobalLock(h)
	if buf == 0 {
		GlobalFree(h)
		return COM_E_NOTIMPL
	}
	copy(unsafe.Slice((*byte)(unsafe.Pointer(buf)), len(data)), data)
	GlobalUnlock(h)
	stg := (*STGMEDIUM)(unsafe.Pointer(pmedium))
	stg.Tymed = TyMedHGlobal
	stg.Data = uintptr(h)
	stg.PUnkForRelease = 0
	return COM_S_OK
}

func dataObjGetDataHere(_, _, _ uintptr) uint64 { return COM_E_NOTIMPL }

func dataObjQueryGetData(this, pformatetc uintptr) uint64 {
	obj := (*DataObject)(unsafe.Pointer(this))
	fe := (*FORMATETC)(unsafe.Pointer(pformatetc))
	_, ok := obj.findEntry(fe.CfFormat)
	if !ok {
		return COM_DV_E_FORMATETC
	}
	if fe.Tymed&TyMedHGlobal == 0 {
		return COM_DV_E_TYMED
	}
	return COM_S_OK
}

func dataObjGetCanonicalFormatEtc(_, _, pformatetcOut uintptr) uint64 {
	// Indicate we don't canonicalize.
	(*FORMATETC)(unsafe.Pointer(pformatetcOut)).Ptd = 0
	return COM_DATA_S_SAMEFORMATETC
}

func dataObjSetData(_, _, _, _ uintptr) uint64 { return COM_E_NOTIMPL }

func dataObjEnumFormatEtc(this, dwDirection, ppenumFormatetc uintptr) uint64 {
	if dwDirection != 1 { // DATADIR_GET = 1
		return COM_E_NOTIMPL
	}
	obj := (*DataObject)(unsafe.Pointer(this))
	obj.enumFmt.Reset()
	enumAddRef(uintptr(unsafe.Pointer(obj.enumFmt)))
	*(*uintptr)(unsafe.Pointer(ppenumFormatetc)) = uintptr(unsafe.Pointer(obj.enumFmt))
	return COM_S_OK
}

func dataObjDAdvise(_, _, _, _, _ uintptr) uint64 { return COM_OLE_E_ADVISENOTSUPPORTED }
func dataObjDUnadvise(_, _ uintptr) uint64        { return COM_OLE_E_ADVISENOTSUPPORTED }
func dataObjEnumDAdvise(_, _ uintptr) uint64      { return COM_OLE_E_ADVISENOTSUPPORTED }
