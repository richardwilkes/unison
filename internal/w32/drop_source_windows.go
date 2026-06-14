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
	"time"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/toolbox/v2/xruntime"
	"golang.org/x/sys/windows"
)

var iidIDropSource = xos.Must(windows.GUIDFromString("{00000121-0000-0000-C000-000000000046}"))

// dropSourceContinuousInterval is the minimum time between synthesized drag-over updates. The OLE drag loop calls
// QueryContinueDrag very frequently, so this throttles the synthesized updates to a reasonable rate.
const dropSourceContinuousInterval = 50 * time.Millisecond

// DropSource is a Go-implemented COM IDropSource used when initiating a drag.
type DropSource struct {
	lpVtbl           uintptr // MUST BE FIRST: points to dropSrcVtbl
	refCount         int32
	pinner           runtime.Pinner
	lastContinuousAt time.Time
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
	guid := xruntime.PtrFromUintptr[windows.GUID](riid)
	if *guid == iidUnknown || *guid == iidIDropSource {
		*xruntime.PtrFromUintptr[uintptr](ppvObject) = this
		dropSrcAddRef(this)
		return COM_S_OK
	}
	*xruntime.PtrFromUintptr[uintptr](ppvObject) = 0
	return COM_E_NOINTERFACE
}

func dropSrcAddRef(this uintptr) uintptr {
	src := xruntime.PtrFromUintptr[DropSource](this)
	return uintptr(atomic.AddInt32(&src.refCount, 1))
}

func dropSrcRelease(this uintptr) uintptr {
	src := xruntime.PtrFromUintptr[DropSource](this)
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
	// The OLE drag loop calls this continuously, even when the mouse is stationary, so use it to re-run the drag-over
	// logic on the current drop target at a throttled rate when continuous updates were requested.
	if dt := ActiveDropTarget(); dt != nil {
		src := xruntime.PtrFromUintptr[DropSource](this)
		if now := time.Now(); now.Sub(src.lastContinuousAt) >= dropSourceContinuousInterval {
			src.lastContinuousAt = now
			var pt POINT
			if GetCursorPos(&pt) {
				dt.RefreshDragOver(pt, dropKeyStateMods(grfKeyState))
			}
		}
	}
	return COM_S_OK
}

func dropSrcGiveFeedback(_ uintptr, _ uintptr) uint64 {
	return COM_DRAGDROP_S_USEDEFAULTCURSORS
}
