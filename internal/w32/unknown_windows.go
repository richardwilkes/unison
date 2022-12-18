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

// Unknown https://docs.microsoft.com/en-us/windows/win32/api/unknwn/nn-unknwn-iunknown
type Unknown struct {
	UnsafeVirtualMethodTable unsafe.Pointer
}

type vmtUnknown struct {
	QueryInterface uintptr
	AddRef         uintptr
	Release        uintptr
}

func (obj *Unknown) vmt() *vmtUnknown {
	return (*vmtUnknown)(obj.UnsafeVirtualMethodTable)
}

// QueryInterface https://docs.microsoft.com/en-us/windows/win32/api/unknwn/nf-unknwn-iunknown-queryinterface(refiid_void)
func (obj *Unknown) QueryInterface(guid *GUID) unsafe.Pointer {
	var dest unsafe.Pointer
	if ret, _, _ := syscall.SyscallN(obj.vmt().QueryInterface, uintptr(unsafe.Pointer(obj)),
		uintptr(unsafe.Pointer(guid)), uintptr(unsafe.Pointer(&dest))); ret != 0 {
		return nil
	}
	return dest
}

// AddRef https://docs.microsoft.com/en-us/windows/win32/api/unknwn/nf-unknwn-iunknown-addref
func (obj *Unknown) AddRef() {
	syscall.SyscallN(obj.vmt().AddRef, uintptr(unsafe.Pointer(obj)))
}

// Release https://docs.microsoft.com/en-us/windows/win32/api/unknwn/nf-unknwn-iunknown-release
func (obj *Unknown) Release() {
	syscall.SyscallN(obj.vmt().Release, uintptr(unsafe.Pointer(obj)))
}
