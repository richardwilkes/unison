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
	"math"
	"syscall"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"golang.org/x/sys/windows"
)

// uiaTextRangeFromPointCall calls ITextProvider::RangeFromPoint the way UI Automation calls it on arm64: a UiaPoint is
// a homogeneous floating-point aggregate there, so its two doubles arrive in floating-point registers and the out
// pointer is the second integer argument. The shim is what puts them there — syscall.SyscallN cannot — and it then
// tail-jumps to the slot's thunk, so what this exercises is the real path, registers and all.
func uiaTextRangeFromPointCall(t *testing.T, this uintptr, x, y float64, out uintptr) uint64 {
	t.Helper()
	uiaEnsureVtbls()
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	result, _, _ := syscall.SyscallN(uiaTextFromPointShimAddr(), this, uintptr(math.Float64bits(x)),
		uintptr(math.Float64bits(y)), out)
	return uint64(result)
}

// TestUIATextFromPointThunk verifies the thunk in front of ITextProvider::RangeFromPoint, which is the arm64 half of
// the one provider method that takes a structure by value: the two doubles arrive in floating-point registers and have
// to reach a callback that reads integer registers, and the out pointer has to survive the move, since it starts out in
// the register the first double has to end up in.
//
// It works the way TestFromPointThunk does: the call goes through syscall.SyscallN into the shim, so the runtime
// switches to the system stack on the way in exactly as it does for a real call out to UI Automation, and the shim
// scribbles over the integer registers it took the coordinates from so that a thunk which failed to move the
// floating-point registers cannot accidentally pass.
func TestUIATextFromPointThunk(t *testing.T) {
	c := check.New(t)
	uiaEnsureVtbls()
	c.True(uiaTextFromPointCallback != 0)
	if uiaThunksEnabled {
		c.Equal(uiaTextFromPointThunkAddr(), uiaTextVtbl[uiaUnknownSlots+3])
	}
	c.True(uiaTextFromPointShimAddr() != 0)

	var got uiaThunkCall
	saved := uiaTextFromPointCallback
	defer func() { uiaTextFromPointCallback = saved }()
	uiaTextFromPointCallback = windows.NewCallback(func(this, x, y, out uintptr) uint64 {
		got = uiaThunkCall{this: this, first: x, last: y, out: out}
		return COM_S_OK
	})

	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	r, _, _ := syscall.SyscallN(uiaTextFromPointShimAddr(), 0x2468, uintptr(math.Float64bits(640.25)),
		uintptr(math.Float64bits(-0.5)), 0xFEDC)
	c.Equal(uintptr(COM_S_OK), r)
	c.Equal(uintptr(0x2468), got.this)
	c.Equal(640.25, math.Float64frombits(uint64(got.first)))
	c.Equal(-0.5, math.Float64frombits(uint64(got.last)))
	c.Equal(uintptr(0xFEDC), got.out)
}
