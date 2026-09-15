// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

// The arm64 halves of the two UI Automation vtable slots that take doubles. See uia_thunk_windows.go for the whole
// story; the register plan for this file is:
//
//	ElementProviderFromPoint(this, double x, double y, out) arrives as X0=this, D0=x, D1=y, X1=out, because the ARM64
//	convention numbers integer and floating-point arguments separately: a double takes the next D register and no
//	integer register at all, so the out pointer is the *second* integer argument. The Go callback reads X0, X1, X2 and
//	X3, so the out pointer has to move to X3 before the doubles overwrite X1 and X2 — that order is the whole reason
//	this thunk is three instructions rather than two.
//
//	SetValue(this, double value) arrives as X0=this, D0=value, so the value's bits have to move to X1.
//
// R16 is the only other register touched. It is volatile in the ARM64 convention, unlike R27, the Go assembler's own
// scratch register, which is callee-saved there — which is why the address of each callback variable is materialized in
// R16 explicitly instead of being left to the assembler. The branch is a B rather than a BL so that the runtime's
// callback trampoline finds LR still holding UI Automation's return address, exactly as it would had UI Automation
// branched to it directly.

#include "textflag.h"

// uiaFromPointThunk is the ElementProviderFromPoint vtable slot.
TEXT uiaFromPointThunk<>(SB), NOSPLIT|NOFRAME, $0
	MOVD  R1, R3
	FMOVD F0, R1
	FMOVD F1, R2
	MOVD  $·uiaFromPointCallback(SB), R16
	MOVD  (R16), R16
	B     (R16)

// uiaRangeValueSetValueThunk is the IRangeValueProvider::SetValue vtable slot.
TEXT uiaRangeValueSetValueThunk<>(SB), NOSPLIT|NOFRAME, $0
	FMOVD F0, R1
	MOVD  $·uiaRangeValueSetValueCallback(SB), R16
	MOVD  (R16), R16
	B     (R16)

// uiaFromPointShim calls uiaFromPointThunk the way UI Automation does. It is reached through syscall.SyscallN, so it is
// entered with X0=this, X1=the bits of x, X2=the bits of y and X3=out. It moves the two sets of bits into D0 and D1,
// puts the out pointer where a real call would have left it, fills X2 and X3 with a value the test cannot mistake for a
// coordinate, and tail-jumps to the thunk.
TEXT uiaFromPointShim<>(SB), NOSPLIT|NOFRAME, $0
	FMOVD R1, F0
	FMOVD R2, F1
	MOVD  R3, R1
	MOVD  $-1, R2
	MOVD  $-1, R3
	B     uiaFromPointThunk<>(SB)

// uiaRangeValueSetValueShim calls uiaRangeValueSetValueThunk the way UI Automation does, entered with X0=this and
// X1=the bits of the value.
TEXT uiaRangeValueSetValueShim<>(SB), NOSPLIT|NOFRAME, $0
	FMOVD R1, F0
	MOVD  $-1, R1
	B     uiaRangeValueSetValueThunk<>(SB)

// func uiaFromPointThunkAddr() uintptr
TEXT ·uiaFromPointThunkAddr(SB), NOSPLIT|NOFRAME, $0-8
	MOVD $uiaFromPointThunk<>(SB), R0
	MOVD R0, ret+0(FP)
	RET

// func uiaRangeValueSetValueThunkAddr() uintptr
TEXT ·uiaRangeValueSetValueThunkAddr(SB), NOSPLIT|NOFRAME, $0-8
	MOVD $uiaRangeValueSetValueThunk<>(SB), R0
	MOVD R0, ret+0(FP)
	RET

// func uiaFromPointShimAddr() uintptr
TEXT ·uiaFromPointShimAddr(SB), NOSPLIT|NOFRAME, $0-8
	MOVD $uiaFromPointShim<>(SB), R0
	MOVD R0, ret+0(FP)
	RET

// func uiaRangeValueSetValueShimAddr() uintptr
TEXT ·uiaRangeValueSetValueShimAddr(SB), NOSPLIT|NOFRAME, $0-8
	MOVD $uiaRangeValueSetValueShim<>(SB), R0
	MOVD R0, ret+0(FP)
	RET
