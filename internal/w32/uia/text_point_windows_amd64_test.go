// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package uia

import (
	"runtime"
	"syscall"
	"testing"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/unison/internal/w32"
	"golang.org/x/sys/windows"
)

// textRangeFromPointCall calls ITextProvider::RangeFromPoint the way UI Automation calls it on amd64: through the
// vtable slot, with a pointer to the Point, since the x64 convention passes a sixteen-byte structure by reference.
// No assembly is involved on this architecture, so the call goes straight through the slot.
func textRangeFromPointCall(t *testing.T, this uintptr, x, y float64, out uintptr) uint64 {
	t.Helper()
	ensureVtbls()
	var pin runtime.Pinner
	defer pin.Unpin()
	point := &Point{X: x, Y: y}
	pin.Pin(point)
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	result, _, _ := syscall.SyscallN(textVtbl[unknownSlots+3], this, uintptr(unsafe.Pointer(point)), out)
	return uint64(result)
}

// TestTextRangeFromPointSlotIsACallback verifies that the RangeFromPoint slot holds an ordinary callback on amd64
// rather than one of the assembly thunks: the point arrives as a pointer there, so every argument is an integer one and
// windows.NewCallback describes the method exactly.
func TestTextRangeFromPointSlotIsACallback(t *testing.T) {
	c := check.New(t)
	ensureVtbls()
	slot := textVtbl[unknownSlots+3]
	c.True(slot != 0)
	c.Equal(windows.NewCallback(textProviderRangeFromPointByRef), slot)

	// A NULL point is a pointer error rather than a crash. UI Automation never passes one, but the copy is the caller's
	// and nothing here dereferences it without looking.
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	result, _, _ := syscall.SyscallN(slot, 0, 0, 0)
	c.Equal(w32.COM_E_POINTER, uint64(result))
}
