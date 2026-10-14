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
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/uti"
	"github.com/richardwilkes/toolbox/v2/xruntime"
	"github.com/richardwilkes/unison/drag"
	"github.com/richardwilkes/unison/internal/w32"
	"golang.org/x/sys/windows"
)

func nativeClipboardAvailableDataTypes() []string {
	var wnd windows.HWND
	if w := firstTopLevelWindow(); w != nil {
		wnd = w.wnd.wnd
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
		if name := w32.ReverseDataType(f); name != "" {
			result = append(result, name)
		}
	}
	return result
}

func nativeClipboardHasDataType(dataType *uti.DataType) bool {
	t := w32.LookupDataType(dataType.UTI)
	if t == w32.CFNone {
		return false
	}
	return w32.IsClipboardFormatAvailable(t)
}

func nativeClipboardGetData(dataType *uti.DataType) []byte {
	t := w32.LookupDataType(dataType.UTI)
	if t == w32.CFNone {
		return nil
	}
	var wnd windows.HWND
	if w := firstTopLevelWindow(); w != nil {
		wnd = w.wnd.wnd
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

// w32ClipboardText converts text into the NUL-terminated UTF-16 that CF_UNICODETEXT holds. Text with a NUL in it is cut
// there, since the terminator ends what any reader of the clipboard sees, rather than being dropped from the clipboard
// altogether.
func w32ClipboardText(text string) []uint16 {
	if i := strings.IndexByte(text, 0); i >= 0 {
		text = text[:i]
	}
	return append(utf16.Encode([]rune(text)), 0)
}

// w32ClipboardBytes returns a movable memory block holding data for SetClipboardData, or 0 if one cannot be had. Empty
// data gets a single zero byte, since GlobalAlloc(GMEM_MOVEABLE, 0) returns a discarded block that GlobalLock cannot
// lock.
func w32ClipboardBytes(data []byte) syscall.Handle {
	size := max(len(data), 1)
	obj := w32.GlobalAlloc(w32.GMemMoveable, size)
	if obj == 0 {
		return 0
	}
	buf := w32.GlobalLock(obj)
	if buf == 0 {
		w32.GlobalFree(obj)
		return 0
	}
	dst := unsafe.Slice(xruntime.PtrFromUintptr[byte](buf), size)
	dst[0] = 0
	copy(dst, data)
	w32.GlobalUnlock(obj)
	return obj
}

// w32ClipboardOwner is the message-only window that w32ClipboardWriteHWND creates, or 0 until it has.
var w32ClipboardOwner windows.HWND

// w32ClipboardWriteHWND returns the window to open the clipboard with for writing: the first top-level window, or else
// a message-only window kept for the purpose. It must not be NULL, since OpenClipboard's documentation says that
// EmptyClipboard then sets the clipboard owner to NULL, which causes SetClipboardData to fail. Returns 0 only if the
// message-only window cannot be created.
func w32ClipboardWriteHWND() windows.HWND {
	if w := firstTopLevelWindow(); w != nil && w.wnd.wnd != 0 {
		return w.wnd.wnd
	}
	if w32ClipboardOwner == 0 && w32MainWndClass.register() == nil {
		w32ClipboardOwner = w32.CreateWindowExW(0, w32MainWndClass.name, "", 0, 0, 0, 0, 0, w32.HWND_MESSAGE, 0,
			w32MainInstance, 0)
	}
	return w32ClipboardOwner
}

func nativeClipboardSetData(data ...drag.Data) {
	type entry struct {
		format w32.ClipboardFormat
		obj    syscall.Handle
	}
	entries := make([]entry, 0, len(data))
	for _, d := range data {
		t := w32.LookupDataType(d.Type.UTI)
		if t == w32.CFNone {
			continue
		}
		var obj syscall.Handle
		if uti.UTF8PlainText.ConformsTo(d.Type) {
			s := w32ClipboardText(string(d.Data))
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
		} else if obj = w32ClipboardBytes(d.Data); obj == 0 {
			continue
		}
		entries = append(entries, entry{t, obj})
	}
	if len(entries) == 0 {
		return
	}
	wnd := w32ClipboardWriteHWND()
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
		// Windows owns the handle only after a successful SetClipboardData, so free it ourselves on failure.
		if w32.SetClipboardData(e.format, e.obj) == 0 {
			w32.GlobalFree(e.obj)
		}
	}
	w32.CloseClipboard()
}
