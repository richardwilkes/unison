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
	"runtime"
	"syscall"
	"testing"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/check"
	"golang.org/x/sys/windows"
)

// uiaTextRangeFromPointCall calls ITextProvider::RangeFromPoint the way UI Automation calls it on amd64: through the
// vtable slot, with a pointer to the UiaPoint, since the x64 convention passes a sixteen-byte structure by reference.
// No assembly is involved on this architecture, so the call goes straight through the slot.
func uiaTextRangeFromPointCall(t *testing.T, this uintptr, x, y float64, out uintptr) uint64 {
	t.Helper()
	uiaEnsureVtbls()
	var pin runtime.Pinner
	defer pin.Unpin()
	point := &UiaPoint{X: x, Y: y}
	pin.Pin(point)
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	result, _, _ := syscall.SyscallN(uiaTextVtbl[uiaUnknownSlots+3], this, uintptr(unsafe.Pointer(point)), out)
	return uint64(result)
}

// TestUIATextRangeFromPointSlotIsACallback verifies that the RangeFromPoint slot holds an ordinary callback on amd64
// rather than one of the assembly thunks: the point arrives as a pointer there, so every argument is an integer one and
// windows.NewCallback describes the method exactly.
func TestUIATextRangeFromPointSlotIsACallback(t *testing.T) {
	c := check.New(t)
	uiaEnsureVtbls()
	slot := uiaTextVtbl[uiaUnknownSlots+3]
	c.True(slot != 0)
	c.Equal(windows.NewCallback(uiaTextProviderRangeFromPointByRef), slot)

	// A NULL point is a pointer error rather than a crash. UI Automation never passes one, but the copy is the caller's
	// and nothing here dereferences it without looking.
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	result, _, _ := syscall.SyscallN(slot, 0, 0, 0)
	c.Equal(COM_E_POINTER, uint64(result))
}
