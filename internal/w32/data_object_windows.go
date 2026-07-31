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
	"syscall"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/uti"
	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/toolbox/v2/xruntime"
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
	data   []byte  // when fmtEtc.Tymed == TyMedHGlobal: data in Windows format (UTF-16LE for text, raw bytes otherwise)
	stream uintptr // when fmtEtc.Tymed == TyMedIStream: an owned IStream* received via SetData
}

// releaseMedium drops the reference to an owned IStream, if any.
func (e *dragDataEntry) releaseMedium() {
	if e.stream != 0 {
		xruntime.PtrFromUintptr[Unknown](e.stream).Release()
		e.stream = 0
	}
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

// DataObject is a Go-implemented COM IDataObject that carries drag data. Its lifetime is governed by the COM
// reference count: the object stays pinned (and its mediums held) until every reference — the creator's initial one
// plus any taken by drop targets or the drag-drop helper — has been released, so a target that retains the
// IDataObject past the end of DoDragDrop still points at live memory.
type DataObject struct {
	lpVtbl   uintptr // MUST BE FIRST: points to dataObjVtbl
	entries  []dragDataEntry
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
	obj.pinner.Pin(obj)
	return obj
}

// Release drops the creator's reference from NewDataObject. Owned mediums are freed and the object is unpinned from
// the Go garbage collector only once every other holder (drop targets, the drag-drop helper, enumerators) has
// released its reference too.
func (obj *DataObject) Release() {
	obj.release()
}

func (obj *DataObject) addRef() uintptr {
	return comAddRef(&obj.refCount)
}

func (obj *DataObject) release() uintptr {
	remaining, final := comRelease(&obj.refCount)
	if final {
		for i := range obj.entries {
			obj.entries[i].releaseMedium()
		}
		obj.entries = nil
		obj.pinner.Unpin()
	}
	return remaining
}

func (obj *DataObject) findEntry(cf uint16) (*dragDataEntry, bool) {
	for i := range obj.entries {
		if obj.entries[i].fmtEtc.CfFormat == cf {
			return &obj.entries[i], true
		}
	}
	return nil, false
}

func dataObjQueryInterface(this, riid, ppvObject uintptr) uint64 {
	guid := xruntime.PtrFromUintptr[windows.GUID](riid)
	if *guid == iidUnknown || *guid == iidIDataObject {
		*xruntime.PtrFromUintptr[uintptr](ppvObject) = this
		dataObjAddRef(this)
		return COM_S_OK
	}
	*xruntime.PtrFromUintptr[uintptr](ppvObject) = 0
	return COM_E_NOINTERFACE
}

func dataObjAddRef(this uintptr) uintptr {
	return xruntime.PtrFromUintptr[DataObject](this).addRef()
}

func dataObjRelease(this uintptr) uintptr {
	return xruntime.PtrFromUintptr[DataObject](this).release()
}

func dataObjGetData(this, pformatetcIn, pmedium uintptr) uint64 {
	obj := xruntime.PtrFromUintptr[DataObject](this)
	fe := xruntime.PtrFromUintptr[FORMATETC](pformatetcIn)
	entry, ok := obj.findEntry(fe.CfFormat)
	if !ok {
		return COM_DV_E_FORMATETC
	}
	if fe.Tymed&entry.fmtEtc.Tymed == 0 {
		return COM_DV_E_TYMED
	}
	stg := xruntime.PtrFromUintptr[STGMEDIUM](pmedium)
	if entry.fmtEtc.Tymed == TyMedIStream {
		xruntime.PtrFromUintptr[Unknown](entry.stream).AddRef() // caller releases via ReleaseStgMedium
		stg.Tymed = TyMedIStream
		stg.Data = entry.stream
		stg.PUnkForRelease = 0
		return COM_S_OK
	}
	// A zero-length entry still needs a lockable medium: GlobalAlloc(GMEM_MOVEABLE, 0) returns a handle to a
	// zero-length, discarded block that GlobalLock cannot lock, which would turn a request for empty data into a
	// spurious E_OUTOFMEMORY. Allocate at least one byte so the requester always receives a valid, lockable HGLOBAL.
	size := max(len(entry.data), 1)
	h := GlobalAlloc(GMemMoveable, size)
	if h == 0 {
		return COM_E_OUTOFMEMORY
	}
	buf := GlobalLock(h)
	if buf == 0 {
		GlobalFree(h)
		return COM_E_OUTOFMEMORY
	}
	dst := unsafe.Slice(xruntime.PtrFromUintptr[byte](buf), size)
	dst[0] = 0 // so the padding byte of an empty entry reads as zero rather than allocator garbage
	copy(dst, entry.data)
	GlobalUnlock(h)
	stg.Tymed = TyMedHGlobal
	stg.Data = uintptr(h)
	stg.PUnkForRelease = 0
	return COM_S_OK
}

func dataObjGetDataHere(_, _, _ uintptr) uint64 { return COM_E_NOTIMPL }

func dataObjQueryGetData(this, pformatetc uintptr) uint64 {
	obj := xruntime.PtrFromUintptr[DataObject](this)
	fe := xruntime.PtrFromUintptr[FORMATETC](pformatetc)
	entry, ok := obj.findEntry(fe.CfFormat)
	if !ok {
		return COM_DV_E_FORMATETC
	}
	if fe.Tymed&entry.fmtEtc.Tymed == 0 {
		return COM_DV_E_TYMED
	}
	return COM_S_OK
}

func dataObjGetCanonicalFormatEtc(_, pformatetcIn, pformatetcOut uintptr) uint64 {
	if pformatetcOut == 0 {
		return COM_E_POINTER
	}
	out := xruntime.PtrFromUintptr[FORMATETC](pformatetcOut)
	if pformatetcIn == 0 {
		*out = FORMATETC{}
		return COM_E_POINTER
	}
	// We don't canonicalize: the output is the input minus any target device, per the DATA_S_SAMEFORMATETC contract,
	// which requires the out-struct to be fully filled in rather than left as whatever the caller passed.
	*out = *xruntime.PtrFromUintptr[FORMATETC](pformatetcIn)
	out.Ptd = 0
	return COM_DATA_S_SAMEFORMATETC
}

// dataObjSetData stores data in the data object. The shell's drag-drop helper relies on this to stash the drag
// image and its bookkeeping under private clipboard formats, which it reads back later via GetData. The helper uses
// both HGLOBAL and IStream mediums, so both must be accepted.
func dataObjSetData(this, pformatetc, pmedium, fRelease uintptr) uint64 {
	obj := xruntime.PtrFromUintptr[DataObject](this)
	fe := xruntime.PtrFromUintptr[FORMATETC](pformatetc)
	stg := xruntime.PtrFromUintptr[STGMEDIUM](pmedium)
	entry := dragDataEntry{
		fmtEtc: FORMATETC{
			CfFormat: fe.CfFormat,
			DwAspect: fe.DwAspect,
			Lindex:   fe.Lindex,
			Tymed:    stg.Tymed,
		},
	}
	switch {
	case fe.Tymed&TyMedHGlobal != 0 && stg.Tymed == TyMedHGlobal:
		if h := syscall.Handle(stg.Data); h != 0 {
			buf := GlobalLock(h)
			if buf == 0 {
				return COM_DV_E_TYMED
			}
			entry.data = make([]byte, GlobalSize(h))
			copy(entry.data, unsafe.Slice(xruntime.PtrFromUintptr[byte](buf), len(entry.data)))
			GlobalUnlock(h)
		}
		if fRelease != 0 {
			ReleaseStgMedium(stg)
		}
	case fe.Tymed&TyMedIStream != 0 && stg.Tymed == TyMedIStream:
		if stg.Data == 0 {
			return COM_DV_E_TYMED
		}
		entry.stream = stg.Data
		// Always take a direct reference of our own on the stream. Simply adopting the caller's reference when
		// fRelease is set would be wrong whenever PUnkForRelease is non-zero, since the medium must then be freed
		// through that object rather than by releasing the stream; taking our own reference and handing the medium
		// back through ReleaseStgMedium (which honors PUnkForRelease) is correct in every combination.
		xruntime.PtrFromUintptr[Unknown](entry.stream).AddRef()
		if fRelease != 0 {
			ReleaseStgMedium(stg)
		}
	default:
		return COM_DV_E_TYMED
	}
	for i := range obj.entries {
		if obj.entries[i].fmtEtc.CfFormat == fe.CfFormat {
			obj.entries[i].releaseMedium()
			obj.entries[i] = entry
			return COM_S_OK
		}
	}
	obj.entries = append(obj.entries, entry)
	return COM_S_OK
}

func dataObjEnumFormatEtc(this, dwDirection, ppenumFormatetc uintptr) uint64 {
	if ppenumFormatetc == 0 {
		return COM_E_POINTER
	}
	out := xruntime.PtrFromUintptr[uintptr](ppenumFormatetc)
	*out = 0
	if DataDir(dwDirection) != DataDirGet {
		return COM_E_NOTIMPL
	}
	// Each call hands out a fresh, independently positioned enumerator, as the IDataObject contract requires;
	// a single shared instance would let one consumer's iteration corrupt another's.
	obj := xruntime.PtrFromUintptr[DataObject](this)
	*out = uintptr(unsafe.Pointer(newEnumFORMATETC(obj, 0)))
	return COM_S_OK
}

func dataObjDAdvise(_, _, _, _, _ uintptr) uint64 { return COM_OLE_E_ADVISENOTSUPPORTED }
func dataObjDUnadvise(_, _ uintptr) uint64        { return COM_OLE_E_ADVISENOTSUPPORTED }
func dataObjEnumDAdvise(_, _ uintptr) uint64      { return COM_OLE_E_ADVISENOTSUPPORTED }
