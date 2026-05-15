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
	"sync"
	"time"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/unison/internal/w32"
	"golang.org/x/sys/windows"
)

var (
	w32DataTypeMapLock sync.RWMutex
	w32DataTypeMap     = map[string]w32.ClipboardFormat{
		"public.utf8-plain-text": w32.CFUnicodeText,
	}
)

func apiClipboardGetText() string {
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
	return windows.UTF16PtrToString((*uint16)(unsafe.Pointer(buffer))) //nolint:govet // No other choice
}

func apiClipboardSetText(text string) {
	s, err := windows.UTF16FromString(text)
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
	copy(unsafe.Slice((*uint16)(unsafe.Pointer(buffer)), len(s)), s) //nolint:govet // No other choice
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

func apiClipboardGetBytes(dataType string) []byte {
	t := w32LookupDataType(dataType)
	if t == w32.CFNone {
		return nil
	}
	var wnd windows.HWND
	if len(windowList) != 0 {
		wnd = windowList[0].wnd.wnd
	}
	tries := 3
	for !w32.OpenClipboard(wnd) {
		time.Sleep(time.Millisecond)
		tries--
		if tries == 0 {
			return nil
		}
	}
	defer w32.CloseClipboard()
	obj := w32.GetClipboardData(t)
	if obj == 0 {
		return nil
	}
	buffer := w32.GlobalLock(obj)
	if buffer == 0 {
		return nil
	}
	defer w32.GlobalUnlock(obj)
	size := w32.GlobalSize(obj)
	data := make([]byte, size)
	copy(data, unsafe.Slice((*byte)(unsafe.Pointer(buffer)), len(data))) //nolint:govet // No other choice
	return data
}

func apiClipboardSetBytes(dataType string, data []byte) {
	t := w32LookupDataType(dataType)
	if t == w32.CFNone {
		return
	}
	obj := w32.GlobalAlloc(w32.GMemMoveable, len(data))
	if obj == 0 {
		return
	}
	buffer := w32.GlobalLock(obj)
	if buffer == 0 {
		w32.GlobalFree(obj)
		return
	}
	copy(unsafe.Slice((*byte)(unsafe.Pointer(buffer)), len(data)), data) //nolint:govet // No other choice
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
	w32.SetClipboardData(t, obj)
	w32.CloseClipboard()
}

func w32LookupDataType(dataType string) w32.ClipboardFormat {
	w32DataTypeMapLock.RLock()
	f, ok := w32DataTypeMap[dataType]
	w32DataTypeMapLock.RUnlock()
	if ok {
		return f
	}
	if f = w32.RegisterClipboardFormatW(dataType); f == w32.CFNone {
		errs.Log(errs.Newf("unable to register clipboard format %q", dataType))
		return w32.CFNone
	}
	w32DataTypeMapLock.Lock()
	w32DataTypeMap[dataType] = f
	w32DataTypeMapLock.Unlock()
	return f
}
