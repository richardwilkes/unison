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

type ModalWindow struct {
	Unknown
}

type vmtModalWindow struct {
	vmtUnknown
	Show uintptr
}

func (obj *ModalWindow) vmt() *vmtModalWindow {
	return (*vmtModalWindow)(obj.UnsafeVirtualMethodTable)
}

func (obj *ModalWindow) Show() bool {
	r1, _, _ := syscall.SyscallN(obj.vmt().Show, uintptr(unsafe.Pointer(obj)), 0)
	return r1 == 0
}
