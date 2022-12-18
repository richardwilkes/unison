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

// IUnknown https://docs.microsoft.com/en-us/windows/win32/api/unknwn/nn-unknwn-iunknown
type IUnknown struct {
	UnsafeVirtualMethodTable unsafe.Pointer
}

// VMTIUnknown holds the virtual dispatch table entries for IUnknown.
type VMTIUnknown struct {
	QueryInterface uintptr
	AddRef         uintptr
	Release        uintptr
}

func (obj *IUnknown) vmt() *VMTIUnknown {
	return (*VMTIUnknown)(obj.UnsafeVirtualMethodTable)
}

// QueryInterface https://docs.microsoft.com/en-us/windows/win32/api/unknwn/nf-unknwn-iunknown-queryinterface(refiid_void)
func (obj *IUnknown) QueryInterface(guid *GUID) unsafe.Pointer {
	var dest unsafe.Pointer
	if ret, _, _ := syscall.SyscallN(obj.vmt().QueryInterface, uintptr(unsafe.Pointer(obj)),
		uintptr(unsafe.Pointer(guid)), uintptr(unsafe.Pointer(&dest))); ret != 0 {
		return nil
	}
	return dest
}

// AddRef https://docs.microsoft.com/en-us/windows/win32/api/unknwn/nf-unknwn-iunknown-addref
func (obj *IUnknown) AddRef() {
	syscall.SyscallN(obj.vmt().AddRef, uintptr(unsafe.Pointer(obj)))
}

// Release https://docs.microsoft.com/en-us/windows/win32/api/unknwn/nf-unknwn-iunknown-release
func (obj *IUnknown) Release() {
	syscall.SyscallN(obj.vmt().Release, uintptr(unsafe.Pointer(obj)))
}
