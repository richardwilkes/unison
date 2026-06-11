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
	"github.com/richardwilkes/unison/drag"
	"github.com/richardwilkes/unison/enums/mod"
	"golang.org/x/sys/windows"
)

var iidIDropTarget = xos.Must(windows.GUIDFromString("{00000122-0000-0000-C000-000000000046}"))

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
	dataObj  *IDataObject // current drag's data object (valid between DragEnter and DragLeave/Drop)
}

var dropTargetVtbl [7]uintptr

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
	dt.pinner.Unpin()
}

func dropTargetQueryInterface(this, riid, ppvObject uintptr) uint64 {
	guid := (*windows.GUID)(unsafe.Pointer(riid))
	if *guid == iidUnknown || *guid == iidIDropTarget {
		*(*uintptr)(unsafe.Pointer(ppvObject)) = this
		dropTargetAddRef(this)
		return COM_S_OK
	}
	*(*uintptr)(unsafe.Pointer(ppvObject)) = 0
	return COM_E_NOINTERFACE
}

func dropTargetAddRef(this uintptr) uintptr {
	dt := (*DropTarget)(unsafe.Pointer(this))
	return uintptr(atomic.AddInt32(&dt.refCount, 1))
}

func dropTargetRelease(this uintptr) uintptr {
	dt := (*DropTarget)(unsafe.Pointer(this))
	n := atomic.AddInt32(&dt.refCount, -1)
	return uintptr(n)
}

func dropTargetDragEnter(this, pDataObj uintptr, grfKeyState MKDnD, pt uintptr, pdwEffect *DropEffect) uint64 {
	dt := (*DropTarget)(unsafe.Pointer(this))
	dt.opMask = dropEffectToOp(*pdwEffect)
	dt.dataObj = (*IDataObject)(unsafe.Pointer(pDataObj))
	dt.dataObj.AddRef()
	info := &dragInfo{obj: dt.dataObj, opMask: dt.opMask}
	op := dt.window.DragEntered(info, dropTargetClientPt(dt.window, pt), dropKeyStateMods(grfKeyState))
	*pdwEffect = opToDropEffect(op)
	if dt.helper != nil {
		screenPt := packedScreenPt(pt)
		dt.helper.DragEnter(dt.window.HWND(), pDataObj, &screenPt, *pdwEffect)
	}
	return COM_S_OK
}

func dropTargetDragOver(this uintptr, grfKeyState MKDnD, pt uintptr, pdwEffect *DropEffect) uint64 {
	dt := (*DropTarget)(unsafe.Pointer(this))
	if dt.dataObj == nil {
		*pdwEffect = DropEffectNone
		return COM_S_OK
	}
	info := &dragInfo{obj: dt.dataObj, opMask: dt.opMask}
	op := dt.window.DragUpdated(info, dropTargetClientPt(dt.window, pt), dropKeyStateMods(grfKeyState))
	*pdwEffect = opToDropEffect(op)
	if dt.helper != nil {
		screenPt := packedScreenPt(pt)
		dt.helper.DragOver(&screenPt, *pdwEffect)
	}
	return COM_S_OK
}

func dropTargetDragLeave(this uintptr) uint64 {
	dt := (*DropTarget)(unsafe.Pointer(this))
	if dt.dataObj != nil {
		dt.dataObj.Release()
		dt.dataObj = nil
	}
	dt.window.DragExited()
	if dt.helper != nil {
		dt.helper.DragLeave()
	}
	return COM_S_OK
}

func dropTargetDrop(this, pDataObj uintptr, grfKeyState MKDnD, pt, pdwEffect uintptr) uint64 {
	dt := (*DropTarget)(unsafe.Pointer(this))
	if dt.dataObj != nil {
		dt.dataObj.Release()
		dt.dataObj = nil
	}
	dataObj := (*IDataObject)(unsafe.Pointer(pDataObj))
	info := &dragInfo{obj: dataObj, opMask: dt.opMask}
	dt.window.Drop(info, dropTargetClientPt(dt.window, pt), dropKeyStateMods(grfKeyState))
	*(*DropEffect)(unsafe.Pointer(pdwEffect)) = DropEffectNone
	if dt.helper != nil {
		screenPt := packedScreenPt(pt)
		dt.helper.Drop(pDataObj, &screenPt, *(*DropEffect)(unsafe.Pointer(pdwEffect)))
	}
	return COM_S_OK
}

func packedScreenPt(pt uintptr) POINT {
	return POINT{
		X: int32(pt & 0xFFFFFFFF),
		Y: int32(pt >> 32),
	}
}

func dropTargetClientPt(w DragTargetWindow, pt uintptr) geom.Point {
	screenPt := packedScreenPt(pt)
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

func dropEffectToOp(effect DropEffect) drag.Op {
	var op drag.Op
	if effect&DropEffectCopy != 0 {
		op |= drag.Copy
	}
	if effect&DropEffectMove != 0 {
		op |= drag.Move
	}
	return op
}

func opToDropEffect(op drag.Op) DropEffect {
	switch {
	case op&drag.Copy != 0:
		return DropEffectCopy
	case op&drag.Move != 0:
		return DropEffectMove
	default:
		return DropEffectNone
	}
}

// OpMaskToDropEffect converts a drag.Op mask to a Windows DropEffect bitmask.
func OpMaskToDropEffect(op drag.Op) DropEffect {
	var effect DropEffect
	if op&drag.Copy != 0 {
		effect |= DropEffectCopy
	}
	if op&drag.Move != 0 {
		effect |= DropEffectMove
	}
	return effect
}
