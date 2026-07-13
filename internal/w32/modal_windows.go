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

	"golang.org/x/sys/windows"
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

// Show displays the modal window. The owner may be 0, but providing one ensures the window is positioned relative to
// it (and therefore on the same display) rather than being placed at a system-chosen location.
func (obj *ModalWindow) Show(owner windows.HWND) bool {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	r1, _, _ := syscall.SyscallN(obj.vmt().Show, uintptr(unsafe.Pointer(obj)), uintptr(owner))
	return r1&0xff == 0
}
