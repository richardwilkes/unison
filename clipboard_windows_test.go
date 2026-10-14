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
	"testing"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/xruntime"
	"github.com/richardwilkes/unison/internal/w32"
	"golang.org/x/sys/windows"
)

// TestW32ClipboardText verifies the text put on the clipboard: NUL-terminated UTF-16, with a surrogate pair where one
// is needed, and cut at a NUL in the text rather than refused because of it.
func TestW32ClipboardText(t *testing.T) {
	c := check.New(t)
	c.Equal([]uint16{0}, w32ClipboardText(""))
	c.Equal([]uint16{'h', 'i', 0}, w32ClipboardText("hi"))
	c.Equal([]uint16{0xD83D, 0xDE00, 0}, w32ClipboardText("\U0001F600"))
	c.Equal([]uint16{'a', 'b', 0}, w32ClipboardText("ab\x00cd"), "cut at the NUL, which ends what a reader sees")
	c.Equal([]uint16{0}, w32ClipboardText("\x00cd"))
}

// TestW32ClipboardBytes verifies the memory block an entry other than text is put on the clipboard in: one holding the
// data, or a single zero byte for empty data, such as a marker type's. GlobalAlloc(GMEM_MOVEABLE, 0) returns a
// discarded block that GlobalLock cannot lock, so asking for none would keep the entry off the clipboard and leave the
// previous contents to be pasted.
func TestW32ClipboardBytes(t *testing.T) {
	c := check.New(t)
	for _, data := range [][]byte{nil, {}, {0xAB, 0xCD}} {
		h := w32ClipboardBytes(data)
		c.True(h != 0, "%v", data)
		if h == 0 {
			continue
		}
		want := data
		if len(want) == 0 {
			want = []byte{0}
		}
		c.True(w32.GlobalSize(h) >= uint64(len(want)), "%v", data)
		buf := w32.GlobalLock(h)
		c.True(buf != 0, "%v", data)
		if buf != 0 {
			c.Equal(want, unsafe.Slice(xruntime.PtrFromUintptr[byte](buf), len(want)), "%v", data)
			w32.GlobalUnlock(h)
		}
		w32.GlobalFree(h)
	}
}

// TestW32ClipboardWriteHWND verifies the window the clipboard is opened with for writing: the first top-level window,
// or else a message-only window kept for the purpose, never NULL. OpenClipboard's documentation says that after it is
// called with NULL, EmptyClipboard sets the clipboard owner to NULL, which causes SetClipboardData to fail, so a copy
// made with no window open would empty the clipboard and put nothing on it.
func TestW32ClipboardWriteHWND(t *testing.T) {
	c := check.New(t)
	priorWindows := windowList
	t.Cleanup(func() { windowList = priorWindows })
	windowList = nil
	hidden := w32ClipboardWriteHWND()
	c.True(hidden != 0, "a message-only window stands in for the missing top-level window")
	c.Equal(hidden, w32ClipboardWriteHWND(), "and is kept for the next time")
	top := &Window{wnd: &apiWindow{}}
	top.wnd.wnd = 7
	windowList = []*Window{{wnd: &apiWindow{}, owner: top, kind: WindowKindMenu}, top}
	c.Equal(windows.HWND(7), w32ClipboardWriteHWND(), "the first top-level window is used when there is one")
	top.wnd.wnd = 0
	c.Equal(hidden, w32ClipboardWriteHWND(), "but not one without a native window")
}
