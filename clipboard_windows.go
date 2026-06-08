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
	"syscall"
	"time"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/uti"
	"github.com/richardwilkes/toolbox/v2/xruntime"
	"github.com/richardwilkes/unison/drag"
	"github.com/richardwilkes/unison/internal/w32"
	"golang.org/x/sys/windows"
)

var (
	w32DataTypeMapLock sync.RWMutex
	w32DataTypeMap     = map[string]w32.ClipboardFormat{
		uti.UTF8PlainText.UTI: w32.CFUnicodeText,
	}
	w32ReverseDataTypeMap = map[w32.ClipboardFormat]string{
		w32.CFUnicodeText: uti.UTF8PlainText.UTI,
	}
)

func apiClipboardAvailableDataTypes() []string {
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
	var result []string
	for f := w32.EnumClipboardFormats(w32.CFNone); f != w32.CFNone; f = w32.EnumClipboardFormats(f) {
		w32DataTypeMapLock.RLock()
		name, ok := w32ReverseDataTypeMap[f]
		w32DataTypeMapLock.RUnlock()
		if ok {
			result = append(result, name)
			continue
		}
		if name = w32.GetClipboardFormatNameW(f); name != "" {
			result = append(result, name)
		}
	}
	return result
}

func apiClipboardHasDataType(dataType *uti.DataType) bool {
	t := w32LookupDataType(dataType.UTI)
	if t == w32.CFNone {
		return false
	}
	return w32.IsClipboardFormatAvailable(t)
}

func apiClipboardGetData(dataType *uti.DataType) []byte {
	t := w32LookupDataType(dataType.UTI)
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
	if uti.UTF8PlainText.ConformsTo(dataType) {
		// Windows stores CF_UNICODETEXT as UTF-16LE; convert to UTF-8 for the caller.
		u16 := unsafe.Slice(xruntime.PtrFromUintptr[uint16](buffer), size/2)
		// Strip any null terminator before decoding.
		end := len(u16)
		for end > 0 && u16[end-1] == 0 {
			end--
		}
		return []byte(windows.UTF16ToString(u16[:end]))
	}
	data := make([]byte, size)
	copy(data, unsafe.Slice(xruntime.PtrFromUintptr[byte](buffer), len(data)))
	return data
}

func apiClipboardSetData(data ...drag.Data) {
	type entry struct {
		format w32.ClipboardFormat
		obj    syscall.Handle
	}
	entries := make([]entry, 0, len(data))
	for _, d := range data {
		t := w32LookupDataType(d.Type.UTI)
		if t == w32.CFNone {
			continue
		}
		var obj syscall.Handle
		if uti.UTF8PlainText.ConformsTo(d.Type) {
			s, err := windows.UTF16FromString(string(d.Data))
			if err != nil {
				continue
			}
			obj = w32.GlobalAlloc(w32.GMemMoveable, len(s)*2)
			if obj == 0 {
				continue
			}
			buf := w32.GlobalLock(obj)
			if buf == 0 {
				w32.GlobalFree(obj)
				continue
			}
			copy(unsafe.Slice(xruntime.PtrFromUintptr[uint16](buf), len(s)), s)
			w32.GlobalUnlock(obj)
		} else {
			obj = w32.GlobalAlloc(w32.GMemMoveable, len(d.Data))
			if obj == 0 {
				continue
			}
			buf := w32.GlobalLock(obj)
			if buf == 0 {
				w32.GlobalFree(obj)
				continue
			}
			copy(unsafe.Slice(xruntime.PtrFromUintptr[byte](buf), len(d.Data)), d.Data)
			w32.GlobalUnlock(obj)
		}
		entries = append(entries, entry{t, obj})
	}
	if len(entries) == 0 {
		return
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
			for _, e := range entries {
				w32.GlobalFree(e.obj)
			}
			return
		}
	}
	w32.EmptyClipboard()
	for _, e := range entries {
		w32.SetClipboardData(e.format, e.obj)
		// Windows owns the handle after SetClipboardData — do not free it
	}
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
	w32ReverseDataTypeMap[f] = dataType
	w32DataTypeMapLock.Unlock()
	return f
}
