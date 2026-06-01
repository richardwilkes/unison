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
	ole32                = syscall.NewLazyDLL("ole32.dll")
	coCreateInstanceProc = ole32.NewProc("CoCreateInstance")
	instanceIDUnknown    = xos.Must(windows.GUIDFromString("{00000000-0000-0000-C000-000000000046}"))
	nullGUID             windows.GUID
)

// CoCreateInstance https://learn.microsoft.com/en-us/windows/win32/api/combaseapi/nf-combaseapi-cocreateinstance
func CoCreateInstance(classID, instanceID windows.GUID) *Unknown {
	if instanceID == nullGUID {
		instanceID = instanceIDUnknown
	}
	var unknown *Unknown
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	if r1, _, _ := coCreateInstanceProc.Call(uintptr(unsafe.Pointer(&classID)), 0,
		windows.CLSCTX_INPROC_SERVER|windows.CLSCTX_LOCAL_SERVER|windows.CLSCTX_REMOTE_SERVER,
		uintptr(unsafe.Pointer(&instanceID)), uintptr(unsafe.Pointer(&unknown))); r1 != 0 {
		return nil
	}
	return unknown
}
