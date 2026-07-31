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
	dwmapi                        = windows.NewLazySystemDLL("dwmapi.dll")
	dwmEnableBlurBehindWindowProc = dwmapi.NewProc("DwmEnableBlurBehindWindow")
)

// DWM Blur Behind Constants https://learn.microsoft.com/en-us/windows/win32/dwm/dwm-bb-constants
const (
	DWM_BB_ENABLE = 1 << iota
	DWM_BB_BLURREGION
	DWM_BB_TRANSITIONONMAXIMIZED
)

// DWM_BLURBEHIND https://learn.microsoft.com/en-us/windows/win32/api/dwmapi/ns-dwmapi-dwm_blurbehind
type DWM_BLURBEHIND struct {
	Flags                 uint32
	Enable                int32
	RgnBlur               HRGN
	TransitionOnMaximized int32
}

// DwmEnableBlurBehindWindow https://learn.microsoft.com/en-us/windows/win32/api/dwmapi/nf-dwmapi-dwmenableblurbehindwindow
func DwmEnableBlurBehindWindow(hwnd windows.HWND, blurBehind *DWM_BLURBEHIND) bool {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	ret, _, _ := dwmEnableBlurBehindWindowProc.Call(uintptr(hwnd), uintptr(unsafe.Pointer(blurBehind)))
	return hresultSucceeded(ret)
}
