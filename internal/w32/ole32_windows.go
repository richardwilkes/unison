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

	"golang.org/x/sys/windows"
)

var (
	ole32                = syscall.NewLazyDLL("ole32.dll")
	coInitializeExProc   = ole32.NewProc("CoInitializeEx")
	coCreateInstanceProc = ole32.NewProc("CoCreateInstance")
	coTaskMemFreeProc    = ole32.NewProc("CoTaskMemFree")
)

func CoInitialize(coInit int) {
	coInitializeExProc.Call(0, uintptr(coInit))
}

func CoCreateInstance(classID, instanceID GUID) *IUnknown {
	if instanceID == NullGUID {
		instanceID = InstanceIDUnknown
	}
	var unknown *IUnknown
	if r1, _, _ := coCreateInstanceProc.Call(uintptr(unsafe.Pointer(&classID)), 0,
		windows.CLSCTX_INPROC_SERVER|windows.CLSCTX_LOCAL_SERVER|windows.CLSCTX_REMOTE_SERVER,
		uintptr(unsafe.Pointer(&instanceID)), uintptr(unsafe.Pointer(&unknown))); r1 != 0 {
		return nil
	}
	return unknown
}

func CoTaskMemFree(ptr uintptr) {
	coTaskMemFreeProc.Call(ptr)
}
