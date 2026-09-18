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

	"golang.org/x/sys/windows"
)

// The arm64 half of ITextProvider::RangeFromPoint, the one method in the provider API that takes a structure by value.
//
// A UiaPoint is two doubles and nothing else, which makes it a homogeneous floating-point aggregate: the Microsoft
// ARM64 convention passes one in consecutive floating-point registers rather than by reference, so the method arrives
// as this=X0, point.X=D0, point.Y=D1, out=X1 — the out pointer being the *second* integer argument, because a double
// takes no integer register at all. That is the same register plan as
// IRawElementProviderFragmentRoot::ElementProviderFromPoint, so this slot holds a thunk built the same way: it moves
// the out pointer clear, moves the two doubles into the integer registers a Go callback reads, and tail-jumps to the
// callback below. See uia_thunk_windows.go for the whole story, including what turning the thunks off costs, and
// uia_text_point_windows_amd64.go for the other architecture, where the same argument arrives as a pointer.

// uiaTextFromPointCallback is the NewCallback trampoline the RangeFromPoint thunk tail-jumps to. It is a plain uintptr
// because the assembly loads it directly, and it is filled in before the thunk's address is installed anywhere UI
// Automation can reach it.
var uiaTextFromPointCallback uintptr

// uiaTextRangeFromPointSlot returns what belongs in the RangeFromPoint vtable slot, filling in the callback the thunk
// jumps to first.
//
// The callback is filled in whether or not the thunk ends up in the slot, which is what uiaBuildVtbls does for
// ElementProviderFromPoint's. The thunk is still reachable with the emergency switch off — the shim its test calls
// reaches it directly — and a thunk whose callback variable was left at zero branches to address zero, which takes the
// process down instead of failing.
func uiaTextRangeFromPointSlot() uintptr {
	uiaTextFromPointCallback = windows.NewCallback(uiaTextProviderRangeFromPointBits)
	if uiaThunksEnabled {
		return uiaTextFromPointThunkAddr()
	}
	return windows.NewCallback(uiaFloatArgumentUnavailable)
}

// uiaTextProviderRangeFromPointBits implements ITextProvider::RangeFromPoint on arm64. It is reached through the thunk,
// so the two coordinates arrive as the raw bits of the doubles rather than as doubles.
func uiaTextProviderRangeFromPointBits(this, xBits, yBits, out uintptr) uint64 {
	return uiaTextProviderRangeFromPointAt(this, math.Float64frombits(uint64(xBits)),
		math.Float64frombits(uint64(yBits)), out)
}

// uiaTextFromPointThunkAddr returns the address of the RangeFromPoint thunk, for the vtable slot. Implemented in
// uia_thunk_windows_arm64.s.
func uiaTextFromPointThunkAddr() uintptr

// uiaTextFromPointShimAddr returns the address of the shim that calls the RangeFromPoint thunk the way UI Automation
// does. It exists for the thunk's test, which cannot otherwise put a double in a floating-point register; see
// uiaFromPointShimAddr, which it is a copy of. Implemented in uia_thunk_windows_arm64.s.
func uiaTextFromPointShimAddr() uintptr
