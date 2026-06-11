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

	"github.com/richardwilkes/toolbox/v2/xos"
	"golang.org/x/sys/windows"
)

var (
	clsidDragDropHelper  = xos.Must(windows.GUIDFromString("{4657278A-411B-11D2-839A-00C04FD918D0}"))
	iidIDragSourceHelper = xos.Must(windows.GUIDFromString("{DE5BF786-477A-11D2-839D-00C04FD918D0}"))
	iidIDropTargetHelper = xos.Must(windows.GUIDFromString("{4657278B-411B-11D2-839A-00C04FD918D0}"))
)

// SHDRAGIMAGE https://learn.microsoft.com/en-us/windows/win32/api/shobjidl_core/ns-shobjidl_core-shdragimage
type SHDRAGIMAGE struct {
	SizeDragImage SIZE
	PtOffset      POINT
	HbmpDragImage HBITMAP
	CrColorKey    uint32
}

// IDragSourceHelper wraps an IDragSourceHelper COM interface pointer received from Windows.
// https://learn.microsoft.com/en-us/windows/win32/api/shobjidl_core/nn-shobjidl_core-idragsourcehelper
type IDragSourceHelper struct {
	Unknown
}

type vmtIDragSourceHelper struct {
	vmtUnknown
	InitializeFromBitmap uintptr
	InitializeFromWindow uintptr
}

func (obj *IDragSourceHelper) vmt() *vmtIDragSourceHelper {
	return (*vmtIDragSourceHelper)(obj.UnsafeVirtualMethodTable)
}

// InitializeFromBitmap stores the given drag image into the data object so the shell can render it during a drag.
// On success, ownership of the bitmap in shdi passes to the system; on failure, the caller retains ownership.
func (obj *IDragSourceHelper) InitializeFromBitmap(shdi *SHDRAGIMAGE, dataObj unsafe.Pointer) bool {
	r, _, _ := syscall.SyscallN(obj.vmt().InitializeFromBitmap,
		uintptr(unsafe.Pointer(obj)),
		uintptr(unsafe.Pointer(shdi)),
		uintptr(dataObj))
	return r == 0
}

// NewDragSourceHelper creates the shell's drag-drop helper object and returns its IDragSourceHelper interface.
// Caller must Release the result. Returns nil on failure.
func NewDragSourceHelper() *IDragSourceHelper {
	unknown := CoCreateInstance(clsidDragDropHelper, iidIDragSourceHelper)
	if unknown == nil {
		return nil
	}
	return (*IDragSourceHelper)(unsafe.Pointer(unknown))
}

// IDropTargetHelper wraps an IDropTargetHelper COM interface pointer received from Windows.
// https://learn.microsoft.com/en-us/windows/win32/api/shobjidl_core/nn-shobjidl_core-idroptargethelper
type IDropTargetHelper struct {
	Unknown
}

type vmtIDropTargetHelper struct {
	vmtUnknown
	DragEnter uintptr
	DragLeave uintptr
	DragOver  uintptr
	Drop      uintptr
	Show      uintptr
}

func (obj *IDropTargetHelper) vmt() *vmtIDropTargetHelper {
	return (*vmtIDropTargetHelper)(obj.UnsafeVirtualMethodTable)
}

// NewDropTargetHelper creates the shell's drag-drop helper object and returns its IDropTargetHelper interface.
// Caller must Release the result. Returns nil on failure.
func NewDropTargetHelper() *IDropTargetHelper {
	unknown := CoCreateInstance(clsidDragDropHelper, iidIDropTargetHelper)
	if unknown == nil {
		return nil
	}
	return (*IDropTargetHelper)(unsafe.Pointer(unknown))
}

// DragEnter notifies the helper that the drag entered the given window, allowing it to render the drag image.
// pt is in screen coordinates.
func (obj *IDropTargetHelper) DragEnter(hwnd windows.HWND, dataObj uintptr, pt *POINT, effect DropEffect) {
	//nolint:errcheck // Nothing we can do about an error here
	syscall.SyscallN(obj.vmt().DragEnter,
		uintptr(unsafe.Pointer(obj)),
		uintptr(hwnd),
		dataObj,
		uintptr(unsafe.Pointer(pt)),
		uintptr(effect))
}

// DragLeave notifies the helper that the drag left the window.
func (obj *IDropTargetHelper) DragLeave() {
	//nolint:errcheck // Nothing we can do about an error here
	syscall.SyscallN(obj.vmt().DragLeave, uintptr(unsafe.Pointer(obj)))
}

// DragOver notifies the helper of drag movement so it can reposition the drag image. pt is in screen coordinates.
func (obj *IDropTargetHelper) DragOver(pt *POINT, effect DropEffect) {
	//nolint:errcheck // Nothing we can do about an error here
	syscall.SyscallN(obj.vmt().DragOver,
		uintptr(unsafe.Pointer(obj)),
		uintptr(unsafe.Pointer(pt)),
		uintptr(effect))
}

// Drop notifies the helper that the drop occurred. pt is in screen coordinates.
func (obj *IDropTargetHelper) Drop(dataObj uintptr, pt *POINT, effect DropEffect) {
	//nolint:errcheck // Nothing we can do about an error here
	syscall.SyscallN(obj.vmt().Drop,
		uintptr(unsafe.Pointer(obj)),
		dataObj,
		uintptr(unsafe.Pointer(pt)),
		uintptr(effect))
}

// InitializeDragImage creates a drag source helper and stores the given bitmap into the data object as the drag
// image. The bitmap must be a 32-bit bitmap with premultiplied alpha, sized in physical pixels. offset is the
// position of the cursor within the image, also in physical pixels. On success, ownership of the bitmap passes to
// the system and true is returned; on failure, the caller retains ownership of the bitmap.
func InitializeDragImage(dataObj *DataObject, bmp HBITMAP, size SIZE, offset POINT) bool {
	helper := NewDragSourceHelper()
	if helper == nil {
		return false
	}
	defer helper.Release()
	shdi := SHDRAGIMAGE{
		SizeDragImage: size,
		PtOffset:      offset,
		HbmpDragImage: bmp,
		CrColorKey:    0xFFFFFFFF, // CLR_NONE; the alpha channel is used instead
	}
	return helper.InitializeFromBitmap(&shdi, unsafe.Pointer(dataObj))
}
