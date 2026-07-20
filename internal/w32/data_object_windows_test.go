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
	"testing"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/check"
)

// newTestDataObject builds a pinned DataObject directly, bypassing NewDataObject so no clipboard formats need to be
// registered with the OS. Every entry uses TyMedHGlobal with a trivial payload.
func newTestDataObject(formats ...uint16) *DataObject {
	entries := make([]dragDataEntry, 0, len(formats))
	for _, cf := range formats {
		entries = append(entries, dragDataEntry{
			fmtEtc: FORMATETC{CfFormat: cf, DwAspect: DVAspectContent, Lindex: -1, Tymed: TyMedHGlobal},
			data:   []byte{0},
		})
	}
	obj := &DataObject{entries: entries, refCount: 1}
	obj.lpVtbl = uintptr(unsafe.Pointer(&dataObjVtbl[0]))
	obj.pinner.Pin(obj)
	return obj
}

// TestDataObjectGetCanonicalFormatEtc verifies the out-struct is fully populated — copied from the input with the
// target device cleared — rather than having only Ptd assigned while the rest is left as caller stack garbage, and
// that a nil output pointer is rejected instead of dereferenced.
func TestDataObjectGetCanonicalFormatEtc(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	in := &FORMATETC{CfFormat: uint16(CFUnicodeText), Ptd: 0xdeadbeef, DwAspect: DVAspectContent, Lindex: -1, Tymed: TyMedHGlobal}
	out := &FORMATETC{CfFormat: 0xFFFF, Ptd: 1, DwAspect: 0xFF, Lindex: 42, Tymed: 0xFF}
	pin.Pin(in)
	pin.Pin(out)
	c.Equal(COM_E_POINTER, dataObjGetCanonicalFormatEtc(0, uintptr(unsafe.Pointer(in)), 0))
	c.Equal(COM_DATA_S_SAMEFORMATETC,
		dataObjGetCanonicalFormatEtc(0, uintptr(unsafe.Pointer(in)), uintptr(unsafe.Pointer(out))))
	expected := *in
	expected.Ptd = 0
	c.Equal(expected, *out)
}

// TestDataObjectReferenceCountLifetime verifies the reference count actually governs the object's lifetime: the
// creator's Release must not tear the object down while another COM holder (here an enumerator) still has a
// reference, and cleanup must run exactly once, when the last reference of all is dropped.
func TestDataObjectReferenceCountLifetime(t *testing.T) {
	c := check.New(t)
	obj := newTestDataObject(uint16(CFUnicodeText))
	this := uintptr(unsafe.Pointer(obj))
	c.Equal(uintptr(2), dataObjAddRef(this))
	c.Equal(uintptr(1), dataObjRelease(this))
	e := newEnumFORMATETC(obj, 0) // takes its own reference on obj
	obj.Release()                 // drops the creator's reference
	c.True(obj.entries != nil)    // still alive: the enumerator's reference must keep it so
	c.Equal(uintptr(0), enumRelease(uintptr(unsafe.Pointer(e))))
	c.True(obj.entries == nil) // the final release ran the cleanup
}

// TestDataObjectEnumeratorIndependence verifies each EnumFormatEtc call and each Clone yields an independently
// positioned enumerator, rather than the single shared instance previously handed to every consumer, whose position
// any of them could corrupt for the others; it also checks the non-GET direction and nil out-pointer error paths.
func TestDataObjectEnumeratorIndependence(t *testing.T) {
	c := check.New(t)
	obj := newTestDataObject(1, 2, 3)
	defer obj.Release()
	this := uintptr(unsafe.Pointer(obj))
	var pin runtime.Pinner
	defer pin.Unpin()
	outPtr := new(uintptr)
	fetched := new(uint32)
	buf := make([]FORMATETC, 2)
	pin.Pin(outPtr)
	pin.Pin(fetched)
	pin.Pin(&buf[0])

	c.Equal(COM_E_POINTER, dataObjEnumFormatEtc(this, uintptr(DataDirGet), 0))
	c.Equal(COM_S_OK, dataObjEnumFormatEtc(this, uintptr(DataDirGet), uintptr(unsafe.Pointer(outPtr))))
	e1 := *outPtr
	c.Equal(COM_E_NOTIMPL, dataObjEnumFormatEtc(this, uintptr(DataDirSet), uintptr(unsafe.Pointer(outPtr))))
	c.Equal(uintptr(0), *outPtr)

	c.Equal(COM_S_OK, enumNext(e1, 2, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(fetched))))
	c.Equal(uint32(2), *fetched)
	c.Equal(uint16(1), buf[0].CfFormat)
	c.Equal(uint16(2), buf[1].CfFormat)

	// A clone starts at the source's position, then the two advance independently: both must see the final entry.
	c.Equal(COM_E_POINTER, enumClone(e1, 0))
	c.Equal(COM_S_OK, enumClone(e1, uintptr(unsafe.Pointer(outPtr))))
	e2 := *outPtr
	c.Equal(COM_S_FALSE, enumNext(e2, 2, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(fetched))))
	c.Equal(uint32(1), *fetched)
	c.Equal(uint16(3), buf[0].CfFormat)
	c.Equal(COM_S_FALSE, enumNext(e1, 2, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(fetched))))
	c.Equal(uint32(1), *fetched)
	c.Equal(uint16(3), buf[0].CfFormat)

	c.Equal(uintptr(0), enumRelease(e1))
	c.Equal(uintptr(0), enumRelease(e2))
}
