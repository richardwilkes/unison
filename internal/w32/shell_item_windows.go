// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
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
)

const SIGDN_FILESYSPATH = 0x80058000

var (
	shell32                         = syscall.NewLazyDLL("Shell32.dll")
	shCreateItemFromParsingNameProc = shell32.NewProc("SHCreateItemFromParsingName")
	shellItemIID                    = NewGUID("43826D1E-E718-42EE-BC55-A1E261C37BFE")
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

func NewShellItem(path string) *ShellItem {
	var item *ShellItem
	if r1, _, _ := shCreateItemFromParsingNameProc.Call(uintptr(unsafe.Pointer(SysAllocString(path))), 0,
		uintptr(unsafe.Pointer(&shellItemIID)), uintptr(unsafe.Pointer(&item))); r1 != 0 {
		return nil
	}
	return item
}

func (obj *ShellItem) vmt() *vmtShellItem {
	return (*vmtShellItem)(obj.UnsafeVirtualMethodTable)
}

func (obj *ShellItem) DisplayName() string {
	var p *uint16
	r1, _, _ := syscall.SyscallN(obj.vmt().GetDisplayName, uintptr(unsafe.Pointer(obj)), SIGDN_FILESYSPATH,
		uintptr(unsafe.Pointer(&p)))
	if r1 != 0 {
		return ""
	}
	defer CoTaskMemFree(uintptr(unsafe.Pointer(p)))
	return syscall.UTF16ToString((*[1 << 30]uint16)(unsafe.Pointer(p))[:])
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
	r1, _, _ := syscall.SyscallN(obj.vmt().GetCount, uintptr(unsafe.Pointer(obj)), uintptr(unsafe.Pointer(&count)))
	if r1 != 0 {
		return 0
	}
	return int(count)
}

func (obj *ShellItemArray) Item(index int) string {
	var item *ShellItem
	r1, _, _ := syscall.SyscallN(obj.vmt().GetItemAt, uintptr(unsafe.Pointer(obj)), uintptr(index),
		uintptr(unsafe.Pointer(&item)))
	if r1 != 0 {
		return ""
	}
	defer item.Release()
	return item.DisplayName()
}
