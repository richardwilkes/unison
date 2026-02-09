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
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	shcore                     = windows.NewLazySystemDLL("shcore.dll")
	getDpiForMonitorProc       = shcore.NewProc("GetDpiForMonitor")
	setProcessDpiAwarenessProc = shcore.NewProc("SetProcessDpiAwareness")
)

type PROCESS_DPI_AWARENESS int32

const (
	PROCESS_DPI_UNAWARE PROCESS_DPI_AWARENESS = iota
	PROCESS_SYSTEM_DPI_AWARE
	PROCESS_PER_MONITOR_DPI_AWARE
)

// GetDpiForMonitor https://learn.microsoft.com/en-us/windows/win32/api/shellscalingapi/nf-shellscalingapi-getdpiformonitor
func GetDpiForMonitor(hmonitor HMONITOR, dpiType MONITOR_DPI_TYPE) (dpiX, dpiY uint32) {
	r, _, _ := getDpiForMonitorProc.Call(uintptr(hmonitor), uintptr(dpiType), uintptr(unsafe.Pointer(&dpiX)),
		uintptr(unsafe.Pointer(&dpiY)))
	if uint32(r) != uint32(windows.S_OK) {
		return 0, 0
	}
	return dpiX, dpiY
}

// SetProcessDpiAwareness https://learn.microsoft.com/en-us/windows/win32/api/shellscalingapi/nf-shellscalingapi-setprocessdpiawareness
func SetProcessDpiAwareness(value PROCESS_DPI_AWARENESS) int32 {
	ret, _, _ := setProcessDpiAwarenessProc.Call(uintptr(value))
	return int32(ret)
}
