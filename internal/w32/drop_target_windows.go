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

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/toolbox/v2/xruntime"
	"github.com/richardwilkes/unison/drag"
	"github.com/richardwilkes/unison/enums/mod"
	"golang.org/x/sys/windows"
)

var iidIDropTarget = xos.Must(windows.GUIDFromString("{00000122-0000-0000-C000-000000000046}"))

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

// DropTarget is a Go-implemented COM IDropTarget registered with a window.
type DropTarget struct {
	lpVtbl   uintptr // MUST BE FIRST: points to dropTargetVtbl
	window   DragTargetWindow
	helper   *IDropTargetHelper
	refCount int32
	pinner   runtime.Pinner
	opMask   drag.Op      // source's allowed ops, set in DragEnter
	lastOp   drag.Op      // op returned by the window's most recent DragEntered/DragUpdated, reported back on Drop
	dataObj  *IDataObject // current drag's data object (valid between DragEnter and DragLeave/Drop)
}

var dropTargetVtbl [7]uintptr

// activeDropTarget is the drop target the cursor is currently over during a drag, or nil when no drag is in progress
// or the cursor is not over any registered drop target. OLE keeps exactly one target "entered" at a time, so this
// identifies the window beneath the cursor. Accessed only on the UI thread.
var activeDropTarget *DropTarget

func init() {
	dropTargetVtbl[0] = windows.NewCallback(dropTargetQueryInterface)
	dropTargetVtbl[1] = windows.NewCallback(dropTargetAddRef)
	dropTargetVtbl[2] = windows.NewCallback(dropTargetRelease)
	dropTargetVtbl[3] = windows.NewCallback(dropTargetDragEnter)
	dropTargetVtbl[4] = windows.NewCallback(dropTargetDragOver)
	dropTargetVtbl[5] = windows.NewCallback(dropTargetDragLeave)
	dropTargetVtbl[6] = windows.NewCallback(dropTargetDrop)
}

// NewDropTarget creates a new DropTarget for the given window.
func NewDropTarget(w DragTargetWindow) *DropTarget {
	dt := &DropTarget{
		window:   w,
		helper:   NewDropTargetHelper(),
		refCount: 1,
	}
	dt.lpVtbl = uintptr(unsafe.Pointer(&dropTargetVtbl[0]))
	dt.pinner.Pin(dt)
	return dt
}

// Revoke releases the pinner and clears the data object reference.
func (dt *DropTarget) Revoke() {
	if dt == nil {
		return
	}
	if dt.helper != nil {
		dt.helper.Release()
		dt.helper = nil
	}
	dt.dataObj = nil
	if activeDropTarget == dt {
		activeDropTarget = nil
	}
	dt.pinner.Unpin()
}

// ActiveDropTarget returns the drop target the cursor is currently over during a drag, or nil if there is none.
func ActiveDropTarget() *DropTarget {
	return activeDropTarget
}

// HWND returns the window handle this drop target is registered with.
func (dt *DropTarget) HWND() windows.HWND {
	return dt.window.HWND()
}

func dropTargetQueryInterface(this, riid, ppvObject uintptr) uint64 {
	guid := xruntime.PtrFromUintptr[windows.GUID](riid)
	if *guid == iidUnknown || *guid == iidIDropTarget {
		*xruntime.PtrFromUintptr[uintptr](ppvObject) = this
		dropTargetAddRef(this)
		return COM_S_OK
	}
	*xruntime.PtrFromUintptr[uintptr](ppvObject) = 0
	return COM_E_NOINTERFACE
}

func dropTargetAddRef(this uintptr) uintptr {
	dt := xruntime.PtrFromUintptr[DropTarget](this)
	return uintptr(atomic.AddInt32(&dt.refCount, 1))
}

func dropTargetRelease(this uintptr) uintptr {
	dt := xruntime.PtrFromUintptr[DropTarget](this)
	n := atomic.AddInt32(&dt.refCount, -1)
	return uintptr(n)
}

func dropTargetDragEnter(this, pDataObj uintptr, grfKeyState MKDnD, pt uintptr, pdwEffect *DropEffect) uint64 {
	dt := xruntime.PtrFromUintptr[DropTarget](this)
	dt.opMask = dropEffectToOp(*pdwEffect)
	dt.dataObj = xruntime.PtrFromUintptr[IDataObject](pDataObj)
	dt.dataObj.AddRef()
	activeDropTarget = dt
	info := &dragInfo{obj: dt.dataObj, opMask: dt.opMask}
	op := dt.window.DragEntered(info, dropTargetClientPt(dt.window, pt), dropKeyStateMods(grfKeyState))
	dt.lastOp = op
	*pdwEffect = opToDropEffect(op)
	if dt.helper != nil {
		screenPt := packedScreenPt(pt)
		dt.helper.DragEnter(dt.window.HWND(), pDataObj, &screenPt, *pdwEffect)
	}
	return COM_S_OK
}

func dropTargetDragOver(this uintptr, grfKeyState MKDnD, pt uintptr, pdwEffect *DropEffect) uint64 {
	dt := xruntime.PtrFromUintptr[DropTarget](this)
	if dt.dataObj == nil {
		*pdwEffect = DropEffectNone
		return COM_S_OK
	}
	info := &dragInfo{obj: dt.dataObj, opMask: dt.opMask}
	op := dt.window.DragUpdated(info, dropTargetClientPt(dt.window, pt), dropKeyStateMods(grfKeyState))
	dt.lastOp = op
	*pdwEffect = opToDropEffect(op)
	if dt.helper != nil {
		screenPt := packedScreenPt(pt)
		dt.helper.DragOver(&screenPt, *pdwEffect)
	}
	return COM_S_OK
}

func dropTargetDragLeave(this uintptr) uint64 {
	dt := xruntime.PtrFromUintptr[DropTarget](this)
	dt.lastOp = 0
	if dt.dataObj != nil {
		dt.dataObj.Release()
		dt.dataObj = nil
	}
	if activeDropTarget == dt {
		activeDropTarget = nil
	}
	dt.window.DragExited()
	if dt.helper != nil {
		dt.helper.DragLeave()
	}
	return COM_S_OK
}

func dropTargetDrop(this, pDataObj uintptr, grfKeyState MKDnD, pt, pdwEffect uintptr) uint64 {
	dt := xruntime.PtrFromUintptr[DropTarget](this)
	if dt.dataObj != nil {
		dt.dataObj.Release()
		dt.dataObj = nil
	}
	if activeDropTarget == dt {
		activeDropTarget = nil
	}
	dataObj := xruntime.PtrFromUintptr[IDataObject](pDataObj)
	info := &dragInfo{obj: dataObj, opMask: dt.opMask}
	accepted := dt.window.Drop(info, dropTargetClientPt(dt.window, pt), dropKeyStateMods(grfKeyState))
	// Report the effect that was actually performed back to the source. Reporting DROPEFFECT_NONE on an accepted drop
	// would tell a source performing a Move that nothing happened, so it would never delete the original.
	*xruntime.PtrFromUintptr[DropEffect](pdwEffect) = dropResultEffect(accepted, dt.lastOp)
	dt.lastOp = 0
	if dt.helper != nil {
		screenPt := packedScreenPt(pt)
		dt.helper.Drop(pDataObj, &screenPt, *xruntime.PtrFromUintptr[DropEffect](pdwEffect))
	}
	return COM_S_OK
}

// RefreshDragOver re-runs the drag-over logic at the given screen position without an actual mouse movement, so the
// drop feedback updates after the target view scrolls during a drag. Windows does not deliver a DragOver until the
// mouse actually moves, so this must be called explicitly when the content scrolls underneath a stationary cursor.
// It does nothing if no drag is currently over this target. pt is in screen coordinates.
func (dt *DropTarget) RefreshDragOver(pt POINT, mods mod.Modifiers) {
	if dt == nil || dt.dataObj == nil {
		return
	}
	info := &dragInfo{obj: dt.dataObj, opMask: dt.opMask}
	op := dt.window.DragUpdated(info, dropTargetClientPtFromScreen(dt.window, pt), mods)
	dt.lastOp = op
	if dt.helper != nil {
		screenPt := pt
		dt.helper.DragOver(&screenPt, opToDropEffect(op))
	}
}

func packedScreenPt(pt uintptr) POINT {
	return POINT{
		X: int32(pt & 0xFFFFFFFF),
		Y: int32(pt >> 32),
	}
}

func dropTargetClientPt(w DragTargetWindow, pt uintptr) geom.Point {
	return dropTargetClientPtFromScreen(w, packedScreenPt(pt))
}

func dropTargetClientPtFromScreen(w DragTargetWindow, screenPt POINT) geom.Point {
	ScreenToClient(w.HWND(), &screenPt)
	return w.ConvertRawMousePoint(geom.NewPoint(float32(screenPt.X), float32(screenPt.Y)))
}

func dropKeyStateMods(grfKeyState MKDnD) mod.Modifiers {
	var mods mod.Modifiers
	if grfKeyState&MKDnDShift != 0 {
		mods |= mod.Shift
	}
	if grfKeyState&MKDnDControl != 0 {
		mods |= mod.Control
	}
	if grfKeyState&MKDnDAlt != 0 {
		mods |= mod.Option
	}
	return mods
}
