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
	"testing"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/check"
)

// newTestDropTarget builds a DropTarget the way NewDropTarget does, minus the window and drag-drop helper, neither of
// which participates in the reference-count lifetime under test.
func newTestDropTarget() *DropTarget {
	dt := &DropTarget{refCount: 1}
	dt.lpVtbl = uintptr(unsafe.Pointer(&dropTargetVtbl[0]))
	dt.pinner.Pin(dt)
	return dt
}

// TestDropTargetReferenceCountLifetime verifies the drop target honors the COM reference count rather than treating
// the pin as its sole lifetime guard: a reference taken through QueryInterface must survive the creator's Revoke, with
// the count reaching zero — and only then triggering the unpin — when that holder releases too.
func TestDropTargetReferenceCountLifetime(t *testing.T) {
	c := check.New(t)
	dt := newTestDropTarget()
	this := uintptr(unsafe.Pointer(dt))
	var pin runtime.Pinner
	defer pin.Unpin()
	out := new(uintptr)
	guid := iidIDropTarget
	pin.Pin(out)
	pin.Pin(&guid)

	// QueryInterface for IDropTarget must return the same object and take a reference of its own.
	c.Equal(COM_S_OK, dropTargetQueryInterface(this, uintptr(unsafe.Pointer(&guid)), uintptr(unsafe.Pointer(out))))
	c.Equal(this, *out)
	c.Equal(int32(2), atomic.LoadInt32(&dt.refCount))

	// Revoke must only drop the creator's reference, leaving the QueryInterface holder's intact.
	dt.Revoke()
	c.Equal(int32(1), atomic.LoadInt32(&dt.refCount))

	// The QueryInterface holder's release drops the final reference.
	c.Equal(uintptr(0), dropTargetRelease(this))

	// AddRef/Release through the COM vtbl entries must report standard IUnknown counts.
	dt = newTestDropTarget()
	this = uintptr(unsafe.Pointer(dt))
	c.Equal(uintptr(2), dropTargetAddRef(this))
	c.Equal(uintptr(1), dropTargetRelease(this))
	c.Equal(uintptr(0), dropTargetRelease(this))
}

// TestDropTargetRevokeMidDrag verifies revoking a drop target while a drag is in progress — reachable when a
// DragEntered/DragUpdated/Drop handler disposes the window — releases the data-object reference taken at DragEnter
// instead of leaking it, and keeps the target itself alive until OLE's drag loop drops the AddRef'd pointer it still
// holds.
func TestDropTargetRevokeMidDrag(t *testing.T) {
	c := check.New(t)
	dt := newTestDropTarget()
	this := uintptr(unsafe.Pointer(dt))

	// Simulate OLE's DoDragDrop loop holding the reference it takes at DragEnter and releases only after
	// DragLeave/Drop returns.
	dropTargetAddRef(this)

	// Simulate the data-object reference dropTargetDragEnter takes for the duration of the drag.
	obj := &DataObject{refCount: 1}
	obj.lpVtbl = uintptr(unsafe.Pointer(&dataObjVtbl[0]))
	dt.dataObj = (*IDataObject)(unsafe.Pointer(obj))
	dt.dataObj.AddRef()
	c.Equal(int32(2), atomic.LoadInt32(&obj.refCount))
	activeDropTarget = dt

	dt.Revoke()

	// The DragEnter data-object reference must have been released, not leaked.
	c.Equal(int32(1), atomic.LoadInt32(&obj.refCount))
	c.Nil(dt.dataObj)
	c.Nil(ActiveDropTarget())

	// Only the creator's reference may have been dropped; OLE's must remain, so the object stays pinned and a
	// subsequent DragLeave/DragOver/Release still touches live memory.
	c.Equal(int32(1), atomic.LoadInt32(&dt.refCount))

	// OLE's release after the drag loop finishes drops the final reference.
	c.Equal(uintptr(0), dropTargetRelease(this))
}
