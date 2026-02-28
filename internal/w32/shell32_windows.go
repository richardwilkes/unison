// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
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

const SIGDN_FILESYSPATH = 0x80058000

var (
	shell32                         = syscall.NewLazyDLL("Shell32.dll")
	dragAcceptFilesProc             = shell32.NewProc("DragAcceptFiles")
	dragQueryFileWProc              = shell32.NewProc("DragQueryFileW")
	dragQueryPointProc              = shell32.NewProc("DragQueryPoint")
	dragFinishProc                  = shell32.NewProc("DragFinish")
	shCreateItemFromParsingNameProc = shell32.NewProc("SHCreateItemFromParsingName")
	shellItemIID                    = xos.Must(windows.GUIDFromString("{43826D1E-E718-42EE-BC55-A1E261C37BFE}"))
)

type ShellItem struct {
	Unknown
}

type vmtShellItem struct {
	vmtUnknown
	BindToHandler  uintptr
	GetParent      uintptr
	GetDisplayName uintptr
	GetAttributes  uintptr
	Compare        uintptr
}

// DragAcceptFiles https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-dragacceptfiles
func DragAcceptFiles(hwnd windows.HWND, accept bool) {
	var a uint32
	if accept {
		a = 1
	}
	//nolint:errcheck // Nothing we can do about an error here
	dragAcceptFilesProc.Call(uintptr(hwnd), uintptr(a))
}

// DragQueryFileCount https://docs.microsoft.com/en-us/windows/win32/api/shellapi/nf-shellapi-dragqueryfilew
func DragQueryFileCount(hdrop HDROP) uint32 {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	r1, _, _ := dragQueryFileWProc.Call(uintptr(hdrop), 0xFFFFFFFF, 0, 0)
	return uint32(r1)
}

// DragQueryFileW https://docs.microsoft.com/en-us/windows/win32/api/shellapi/nf-shellapi-dragqueryfilew
func DragQueryFileW(hdrop HDROP, index uint32) string {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	r1, _, _ := dragQueryFileWProc.Call(uintptr(hdrop), uintptr(index), 0, 0)
	if r1 == 0 {
		return ""
	}
	buf := make([]uint16, uint32(r1)+1)
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	r1, _, _ = dragQueryFileWProc.Call(uintptr(hdrop), uintptr(index), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if r1 == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}

// DragQueryPoint https://docs.microsoft.com/en-us/windows/win32/api/shellapi/nf-shellapi-dragquerypoint
func DragQueryPoint(hdrop HDROP) (POINT, bool) {
	var pt POINT
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	r1, _, _ := dragQueryPointProc.Call(uintptr(hdrop), uintptr(unsafe.Pointer(&pt)))
	return pt, r1&0xff != 0
}

// DragFinish https://docs.microsoft.com/en-us/windows/win32/api/shellapi/nf-shellapi-dragfinish
func DragFinish(hdrop HDROP) {
	//nolint:errcheck // Nothing we can do about an error here
	dragFinishProc.Call(uintptr(hdrop))
}

func NewShellItem(path string) *ShellItem {
	var item *ShellItem
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	if r1, _, _ := shCreateItemFromParsingNameProc.Call(uintptr(unsafe.Pointer(SysAllocString(path))), 0,
		uintptr(unsafe.Pointer(&shellItemIID)), uintptr(unsafe.Pointer(&item))); r1 != 0 {
		return nil
	}
	return item
}

func (obj *ShellItem) vmt() *vmtShellItem {
	return (*vmtShellItem)(obj.UnsafeVirtualMethodTable)
}

const (
	sizeofUint16   = unsafe.Sizeof(uint16(0))
	maxUint16Array = (1<<31 - sizeofUint16) / sizeofUint16
)

func (obj *ShellItem) DisplayName() string {
	var p *uint16
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	r1, _, _ := syscall.SyscallN(obj.vmt().GetDisplayName, uintptr(unsafe.Pointer(obj)), SIGDN_FILESYSPATH,
		uintptr(unsafe.Pointer(&p)))
	if r1 != 0 {
		return ""
	}
	defer windows.CoTaskMemFree(unsafe.Pointer(p))
	return syscall.UTF16ToString(unsafe.Slice(p, maxUint16Array))
}

type ShellItemArray struct {
	Unknown
}

type vmtShellItemArray struct {
	vmtUnknown
	BindToHandler              uintptr
	GetPropertyStore           uintptr
	GetPropertyDescriptionList uintptr
	GetAttributes              uintptr
	GetCount                   uintptr
	GetItemAt                  uintptr
	EnumItems                  uintptr
}

func (obj *ShellItemArray) vmt() *vmtShellItemArray {
	return (*vmtShellItemArray)(obj.UnsafeVirtualMethodTable)
}

func (obj *ShellItemArray) Count() int {
	var count uintptr
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	r1, _, _ := syscall.SyscallN(obj.vmt().GetCount, uintptr(unsafe.Pointer(obj)), uintptr(unsafe.Pointer(&count)))
	if r1 != 0 {
		return 0
	}
	return int(count)
}

func (obj *ShellItemArray) Item(index int) string {
	var item *ShellItem
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	r1, _, _ := syscall.SyscallN(obj.vmt().GetItemAt, uintptr(unsafe.Pointer(obj)), uintptr(index),
		uintptr(unsafe.Pointer(&item)))
	if r1 != 0 {
		return ""
	}
	defer item.Release()
	return item.DisplayName()
}
