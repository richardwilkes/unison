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
	"github.com/richardwilkes/toolbox/v2/xruntime"
	"golang.org/x/sys/windows"
)

// DragScrollHandler is called for each mouse wheel event that occurs while a drag-and-drop operation is in progress.
// pt is in screen coordinates. horizontal is true for horizontal wheels (WM_MOUSEHWHEEL) and false for vertical
// wheels (WM_MOUSEWHEEL). delta is the wheel movement expressed in notches.
type DragScrollHandler func(pt POINT, horizontal bool, delta float32)

var (
	dragScrollHook        HHOOK
	dragScrollHandler     DragScrollHandler
	dragScrollHookProcPtr = windows.NewCallback(dragScrollHookProc)
)

// InstallDragScrollHook installs a thread-local mouse hook that forwards mouse wheel events to the supplied handler.
// Windows runs its own modal message loop inside DoDragDrop and does not deliver WM_MOUSEWHEEL messages to the
// window procedure, so without this hook the scroll wheel is inert during a drag. Call RemoveDragScrollHook once the
// drag completes. This must be called on the UI thread, and only one hook may be active at a time.
func InstallDragScrollHook(handler DragScrollHandler) {
	if dragScrollHook != 0 {
		return
	}
	dragScrollHandler = handler
	dragScrollHook = SetWindowsHookExW(WH_MOUSE, dragScrollHookProcPtr, 0, windows.GetCurrentThreadId())
}

// RemoveDragScrollHook removes the hook previously installed by InstallDragScrollHook.
func RemoveDragScrollHook() {
	if dragScrollHook != 0 {
		UnhookWindowsHookEx(dragScrollHook)
		dragScrollHook = 0
	}
	dragScrollHandler = nil
}

func dragScrollHookProc(code int, wParam WPARAM, lParam LPARAM) uintptr {
	if code == HC_ACTION && dragScrollHandler != nil {
		switch wParam {
		case WM_MOUSEWHEEL, WM_MOUSEHWHEEL:
			info := xruntime.PtrFromUintptr[MOUSEHOOKSTRUCTEX](lParam)
			delta := float32(int16((info.MouseData>>16)&0xFFFF)) / float32(WHEEL_DELTA)
			dragScrollHandler(info.Pt, wParam == WM_MOUSEHWHEEL, delta)
		}
	}
	return CallNextHookEx(0, code, wParam, lParam)
}
