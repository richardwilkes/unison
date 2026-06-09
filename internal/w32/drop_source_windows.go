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
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/xos"
	"golang.org/x/sys/windows"
)

var iidIDropSource = xos.Must(windows.GUIDFromString("{00000121-0000-0000-C000-000000000046}"))

// DropSource is a Go-implemented COM IDropSource used when initiating a drag.
type DropSource struct {
	lpVtbl   uintptr // MUST BE FIRST: points to dropSrcVtbl
	refCount int32
	pinner   runtime.Pinner
}

var dropSrcVtbl [5]uintptr

func init() {
	dropSrcVtbl[0] = windows.NewCallback(dropSrcQueryInterface)
	dropSrcVtbl[1] = windows.NewCallback(dropSrcAddRef)
	dropSrcVtbl[2] = windows.NewCallback(dropSrcRelease)
	dropSrcVtbl[3] = windows.NewCallback(dropSrcQueryContinueDrag)
	dropSrcVtbl[4] = windows.NewCallback(dropSrcGiveFeedback)
}

// NewDropSource creates a new DropSource for initiating a drag.
func NewDropSource() *DropSource {
	src := &DropSource{refCount: 1}
	src.lpVtbl = uintptr(unsafe.Pointer(&dropSrcVtbl[0]))
	src.pinner.Pin(src)
	return src
}

// Release unpins the DropSource from the Go garbage collector.
func (src *DropSource) Release() {
	src.pinner.Unpin()
}

func dropSrcQueryInterface(this, riid, ppvObject uintptr) uint64 {
	guid := (*windows.GUID)(unsafe.Pointer(riid))
	if *guid == iidUnknown || *guid == iidIDropSource {
		*(*uintptr)(unsafe.Pointer(ppvObject)) = this
		dropSrcAddRef(this)
		return COM_S_OK
	}
	*(*uintptr)(unsafe.Pointer(ppvObject)) = 0
	return COM_E_NOINTERFACE
}

func dropSrcAddRef(this uintptr) uintptr {
	src := (*DropSource)(unsafe.Pointer(this))
	return uintptr(atomic.AddInt32(&src.refCount, 1))
}

func dropSrcRelease(this uintptr) uintptr {
	src := (*DropSource)(unsafe.Pointer(this))
	return uintptr(atomic.AddInt32(&src.refCount, -1))
}

// dropSrcQueryContinueDrag is called repeatedly during a drag to check whether to continue.
// fEscapePressed: non-zero if Escape was pressed; grfKeyState: current mouse/key state.
func dropSrcQueryContinueDrag(this, fEscapePressed uintptr, grfKeyState MKDnD) uint64 {
	if fEscapePressed != 0 {
		return COM_DRAGDROP_S_CANCEL
	}
	// Drop when the left mouse button is released (not held in grfKeyState).
	if grfKeyState&MKDnDLButton == 0 {
		return COM_DRAGDROP_S_DROP
	}
	return COM_S_OK
}

func dropSrcGiveFeedback(_ uintptr, _ uintptr) uint64 {
	return COM_DRAGDROP_S_USEDEFAULTCURSORS
}
