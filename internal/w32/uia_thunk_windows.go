// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package w32

import "golang.org/x/sys/windows"

// Two of the vtable slots a UI Automation provider has to fill take a double:
//
//	IRawElementProviderFragmentRoot::ElementProviderFromPoint(this, double x, double y, out)
//	IRangeValueProvider::SetValue(this, double value)
//
// syscall.NewCallback (windows.NewCallback) refuses to build a trampoline for a Go function with a float64 parameter,
// and it is right to: the Go callback reads its arguments out of the integer argument registers, and on both Windows
// ABIs a double is passed in a floating-point register instead. Giving the slot a callback that declares uintptr
// parameters would therefore read whatever happened to be left in an integer register rather than the coordinate.
//
// The fix is one assembly instruction per double. Each slot holds a thunk that moves the floating-point registers into
// the integer argument registers the callback will read, and then tail-jumps — not calls — to the NewCallback
// trampoline held in one of the variables below. A tail jump leaves the stack exactly as a direct call from UI
// Automation would have, which is what the trampoline expects: on amd64 the return address is still at 0(SP) with the
// caller's 32 bytes of shadow space above it, and on arm64 the return address is still in LR. The callback then
// receives the raw bits of each double as a uintptr and decodes them with math.Float64frombits.
//
// The register plan, worked out from the Windows calling conventions:
//
// amd64 — integer and floating-point arguments share the four register slots positionally: argument 1 is RCX or XMM0,
// argument 2 is RDX or XMM1, argument 3 is R8 or XMM2, argument 4 is R9 or XMM3. The callback reads RCX, RDX, R8, R9.
//
//	ElementProviderFromPoint(this, x, y, out):  this=RCX ✓  x=XMM1→RDX  y=XMM2→R8  out=R9 ✓
//	    MOVQ X1, DX; MOVQ X2, R8; JMP trampoline
//	SetValue(this, value):                      this=RCX ✓  value=XMM1→RDX
//	    MOVQ X1, DX; JMP trampoline
//
// arm64 — integer and floating-point arguments are numbered independently: integers fill X0-X7 and doubles fill D0-D7,
// so a double does not consume an integer slot. The callback reads X0, X1, X2, X3.
//
//	ElementProviderFromPoint(this, x, y, out):  this=X0 ✓  x=D0  y=D1  out=X1 (the second *integer* argument)
//	    the out pointer has to move out of the way first, so: MOVD R1, R3; FMOVD F0, R1; FMOVD F1, R2; B (R16)
//	SetValue(this, value):                      this=X0 ✓  value=D0
//	    FMOVD F0, R1; B (R16)
//
// Both thunks are NOSPLIT|NOFRAME and clobber only registers the ABI makes volatile: AX on amd64, R16 on arm64. R27,
// the Go assembler's scratch register, is callee-saved in the ARM64 ABI and is deliberately not used, which is why the
// arm64 thunks materialize the address of the variable in R16 rather than letting the assembler pick.
//
// Fallback. If a thunk ever misbehaves on some target, set uiaThunksEnabled to false. ElementProviderFromPoint then
// reports the point as unhandled, costing a screen reader the ability to identify the element under the mouse — UI
// Automation falls back to the window itself — and IRangeValueProvider::SetValue reports the operation as unsupported,
// costing the ability to set a slider or spinner to a typed value. Nothing else depends on either.

// uiaThunksEnabled says whether the two vtable slots that take doubles use the assembly thunks. See the fallback note
// above for what turning it off costs.
const uiaThunksEnabled = true

// The NewCallback trampolines the thunks tail-jump to. They are plain uintptr rather than pointers because the
// assembly loads them directly, and they are filled in while the vtables are built, before either thunk's address is
// installed anywhere UI Automation can reach it.
var (
	uiaFromPointCallback          uintptr
	uiaRangeValueSetValueCallback uintptr
)

// uiaFromPointSlot returns what belongs in the ElementProviderFromPoint vtable slot.
func uiaFromPointSlot() uintptr {
	if uiaThunksEnabled {
		return uiaFromPointThunkAddr()
	}
	return windows.NewCallback(uiaFloatArgumentUnavailable)
}

// uiaRangeValueSetValueSlot returns what belongs in the IRangeValueProvider::SetValue vtable slot.
func uiaRangeValueSetValueSlot() uintptr {
	if uiaThunksEnabled {
		return uiaRangeValueSetValueThunkAddr()
	}
	return windows.NewCallback(uiaFloatArgumentUnavailable)
}

// uiaFloatArgumentUnavailable is what a slot that takes a double answers when the thunks are turned off. It declares no
// parameters at all, since on arm64 the out-parameter of ElementProviderFromPoint does not arrive in the register a
// four-argument Go callback would read it from, and it touches no out-parameter: UI Automation left them NULL, which is
// the answer wanted here.
func uiaFloatArgumentUnavailable() uint64 {
	return UIA_E_NOTSUPPORTED
}

// uiaFromPointThunkAddr returns the address of the ElementProviderFromPoint thunk, for the vtable slot. Implemented in
// uia_thunk_windows_amd64.s and uia_thunk_windows_arm64.s.
func uiaFromPointThunkAddr() uintptr

// uiaRangeValueSetValueThunkAddr returns the address of the IRangeValueProvider::SetValue thunk, for the vtable slot.
// Implemented in uia_thunk_windows_amd64.s and uia_thunk_windows_arm64.s.
func uiaRangeValueSetValueThunkAddr() uintptr

// uiaFromPointShimAddr returns the address of the shim that calls the ElementProviderFromPoint thunk the way UI
// Automation does. It exists for TestFromPointThunk, which cannot otherwise put a double in a floating-point register:
// the test passes the bits of each coordinate through syscall.SyscallN, and the shim — which is native code as far as
// the Go runtime is concerned, so the runtime switches to the system stack on the way in, exactly as it does for a real
// call out to UI Automation — loads them into the floating-point registers, scribbles over the integer registers it
// took them from so that a thunk which failed to move them cannot go unnoticed, and tail-jumps to the thunk.
// Implemented in uia_thunk_windows_amd64.s and uia_thunk_windows_arm64.s.
func uiaFromPointShimAddr() uintptr

// uiaRangeValueSetValueShimAddr returns the address of the shim that calls the IRangeValueProvider::SetValue thunk the
// way UI Automation does, for the same reason uiaFromPointShimAddr exists. Implemented in
// uia_thunk_windows_amd64.s and uia_thunk_windows_arm64.s.
func uiaRangeValueSetValueShimAddr() uintptr
