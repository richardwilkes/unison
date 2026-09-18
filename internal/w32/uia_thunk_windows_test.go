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

// uiaThunkCall holds the arguments one of the thunks delivered to its callback.
type uiaThunkCall struct {
	this  uintptr
	first uintptr
	last  uintptr
	out   uintptr
}

// TestUIAThunkSlots verifies that the two vtable slots that take doubles on both architectures hold the thunks rather
// than a callback, and that each thunk has a callback to jump to. A slot holding a plain callback would read the
// coordinates out of whichever integer registers happened to be in use.
//
// The third slot that arrives with a double, ITextProvider::RangeFromPoint, is thunked on arm64 alone — a UiaPoint is
// passed in two floating-point registers there and by reference on amd64 — so which of the two shapes it holds is
// pinned per architecture, by TestUIATextFromPointThunk and TestUIATextRangeFromPointSlotIsACallback. What both have in
// common is checked here: the slot is filled at all.
func TestUIAThunkSlots(t *testing.T) {
	c := check.New(t)
	uiaEnsureVtbls()
	c.True(uiaFromPointCallback != 0)
	c.True(uiaRangeValueSetValueCallback != 0)
	if uiaThunksEnabled {
		c.Equal(uiaFromPointThunkAddr(), uiaFragmentRootVtbl[uiaUnknownSlots])
		c.Equal(uiaRangeValueSetValueThunkAddr(), uiaRangeValueVtbl[uiaUnknownSlots])
	}
	c.True(uiaFromPointThunkAddr() != 0)
	c.True(uiaRangeValueSetValueThunkAddr() != 0)
	c.True(uiaFromPointShimAddr() != 0)
	c.True(uiaRangeValueSetValueShimAddr() != 0)
	c.True(uiaTextVtbl[uiaUnknownSlots+3] != 0, "the RangeFromPoint slot is filled on either architecture")
}

// TestFromPointThunk verifies the thunk in front of IRawElementProviderFragmentRoot::ElementProviderFromPoint: the two
// doubles arrive in floating-point registers and have to reach a callback that reads integer registers, and the out
// pointer has to survive the move (on arm64 it starts out in the register the first double has to end up in).
//
// The call goes through syscall.SyscallN into the shim, so the runtime switches to the system stack on the way in
// exactly as it does for a real call out to UI Automation, and the thunk is entered with the stack and the registers a
// call from UI Automation would have set up. The shim scribbles over the integer registers it took the coordinates
// from, so a thunk that failed to move the floating-point registers cannot accidentally pass.
func TestFromPointThunk(t *testing.T) {
	c := check.New(t)
	uiaEnsureVtbls()
	var got uiaThunkCall
	saved := uiaFromPointCallback
	defer func() { uiaFromPointCallback = saved }()
	uiaFromPointCallback = windows.NewCallback(func(this, x, y, out uintptr) uint64 {
		got = uiaThunkCall{this: this, first: x, last: y, out: out}
		return COM_S_OK
	})

	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	r, _, _ := syscall.SyscallN(uiaFromPointShimAddr(), 0x1234, uintptr(math.Float64bits(1536.5)),
		uintptr(math.Float64bits(-7.25)), 0xABCD)
	c.Equal(uintptr(COM_S_OK), r)
	c.Equal(uintptr(0x1234), got.this)
	c.Equal(1536.5, math.Float64frombits(uint64(got.first)))
	c.Equal(-7.25, math.Float64frombits(uint64(got.last)))
	c.Equal(uintptr(0xABCD), got.out)
}

// TestRangeValueSetValueThunk verifies the thunk in front of IRangeValueProvider::SetValue, the other vtable slot that
// takes a double. It works the same way TestFromPointThunk does.
func TestRangeValueSetValueThunk(t *testing.T) {
	c := check.New(t)
	uiaEnsureVtbls()
	var got uiaThunkCall
	saved := uiaRangeValueSetValueCallback
	defer func() { uiaRangeValueSetValueCallback = saved }()
	uiaRangeValueSetValueCallback = windows.NewCallback(func(this, value uintptr) uint64 {
		got = uiaThunkCall{this: this, first: value}
		return COM_S_OK
	})

	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	r, _, _ := syscall.SyscallN(uiaRangeValueSetValueShimAddr(), 0x5678, uintptr(math.Float64bits(0.125)))
	c.Equal(uintptr(COM_S_OK), r)
	c.Equal(uintptr(0x5678), got.this)
	c.Equal(0.125, math.Float64frombits(uint64(got.first)))
}
