// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"time"
	"unsafe"

	"github.com/richardwilkes/unison/internal/w32"
	"golang.org/x/sys/windows"
)

func (c *Clipboard) getText() string {
	var wnd windows.HWND
	if len(windowList) != 0 {
		wnd = windowList[0].wnd.wnd
	}
	tries := 3
	for !w32.OpenClipboard(wnd) {
		time.Sleep(time.Millisecond)
		tries--
		if tries == 0 {
			return ""
		}
	}
	defer w32.CloseClipboard()
	obj := w32.GetClipboardData(w32.CFUnicodeText)
	if obj == 0 {
		return ""
	}
	buffer := w32.GlobalLock(obj)
	if buffer == 0 {
		return ""
	}
	defer w32.GlobalUnlock(obj)
	return windows.UTF16PtrToString((*uint16)(unsafe.Pointer(buffer)))
}

func (c *Clipboard) setText(str string) {
	s, err := windows.UTF16FromString(str)
	if err != nil {
		return
	}
	obj := w32.GlobalAlloc(w32.GMemMoveable, len(s)*2)
	if obj == 0 {
		return
	}
	buffer := w32.GlobalLock(obj)
	if buffer == 0 {
		w32.GlobalFree(obj)
		return
	}
	copy(unsafe.Slice((*uint16)(unsafe.Pointer(buffer)), len(s)), s)
	w32.GlobalUnlock(obj)
	var wnd windows.HWND
	if len(windowList) != 0 {
		wnd = windowList[0].wnd.wnd
	}
	tries := 3
	for !w32.OpenClipboard(wnd) {
		time.Sleep(time.Millisecond)
		tries--
		if tries == 0 {
			w32.GlobalFree(obj)
			return
		}
	}
	w32.EmptyClipboard()
	w32.SetClipboardData(w32.CFUnicodeText, obj)
	w32.CloseClipboard()
}
