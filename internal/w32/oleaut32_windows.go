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
)

var (
	oleaut32           = syscall.NewLazyDLL("oleaut32.dll")
	sysAllocStringProc = oleaut32.NewProc("SysAllocString")
)

func SysAllocString(str string) *int16 {
	p, err := syscall.UTF16PtrFromString(str)
	if err != nil {
		return nil
	}
	r1, _, _ := sysAllocStringProc.Call(uintptr(unsafe.Pointer(p)))
	return (*int16)(unsafe.Pointer(r1))
}
