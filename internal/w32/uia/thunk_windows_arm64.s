// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

// The arm64 halves of the three UI Automation vtable slots that arrive with a double in a floating-point register. See
// thunk_windows.go for the whole story; the register plan for this file is:
//
//	ElementProviderFromPoint(this, double x, double y, out) arrives as X0=this, D0=x, D1=y, X1=out, because the ARM64
//	convention numbers integer and floating-point arguments separately: a double takes the next D register and no
//	integer register at all, so the out pointer is the *second* integer argument. The Go callback reads X0, X1, X2 and
//	X3, so the out pointer has to move to X3 before the doubles overwrite X1 and X2 — that order is the whole reason
//	this thunk is three instructions rather than two.
//
//	SetValue(this, double value) arrives as X0=this, D0=value, so the value's bits have to move to X1.
//
//	RangeFromPoint(this, Point point, out) arrives as X0=this, D0=point.X, D1=point.Y, X1=out: a Point is two
//	doubles and nothing else, which makes it a homogeneous floating-point aggregate, and the ARM64 convention
//	passes one in consecutive D registers rather than by reference. That is ElementProviderFromPoint's plan exactly,
//	so the thunk is the same three instructions in the same order. This slot needs no assembly on amd64, where the
//	same structure is passed by reference; see text_point_windows_amd64.go.
//
// R16 is the only other register touched. It is volatile in the ARM64 convention, unlike R27, the Go assembler's own
// scratch register, which is callee-saved there — which is why the address of each callback variable is materialized in
// R16 explicitly instead of being left to the assembler. The branch is a B rather than a BL so that the runtime's
// callback trampoline finds LR still holding UI Automation's return address, exactly as it would had UI Automation
// branched to it directly.

#include "textflag.h"

// fromPointThunk is the ElementProviderFromPoint vtable slot.
TEXT fromPointThunk<>(SB), NOSPLIT|NOFRAME, $0
	MOVD  R1, R3
	FMOVD F0, R1
	FMOVD F1, R2
	MOVD  $·fromPointCallback(SB), R16
	MOVD  (R16), R16
	B     (R16)

// rangeValueSetValueThunk is the IRangeValueProvider::SetValue vtable slot.
TEXT rangeValueSetValueThunk<>(SB), NOSPLIT|NOFRAME, $0
	FMOVD F0, R1
	MOVD  $·rangeValueSetValueCallback(SB), R16
	MOVD  (R16), R16
	B     (R16)

// fromPointShim calls fromPointThunk the way UI Automation does. It is reached through syscall.SyscallN, so it is
// entered with X0=this, X1=the bits of x, X2=the bits of y and X3=out. It moves the two sets of bits into D0 and D1,
// puts the out pointer where a real call would have left it, fills X2 and X3 with a value the test cannot mistake for a
// coordinate, and tail-jumps to the thunk.
TEXT fromPointShim<>(SB), NOSPLIT|NOFRAME, $0
	FMOVD R1, F0
	FMOVD R2, F1
	MOVD  R3, R1
	MOVD  $-1, R2
	MOVD  $-1, R3
	B     fromPointThunk<>(SB)

// rangeValueSetValueShim calls rangeValueSetValueThunk the way UI Automation does, entered with X0=this and
// X1=the bits of the value.
TEXT rangeValueSetValueShim<>(SB), NOSPLIT|NOFRAME, $0
	FMOVD R1, F0
	MOVD  $-1, R1
	B     rangeValueSetValueThunk<>(SB)

// func fromPointThunkAddr() uintptr
TEXT ·fromPointThunkAddr(SB), NOSPLIT|NOFRAME, $0-8
	MOVD $fromPointThunk<>(SB), R0
	MOVD R0, ret+0(FP)
	RET

// func rangeValueSetValueThunkAddr() uintptr
TEXT ·rangeValueSetValueThunkAddr(SB), NOSPLIT|NOFRAME, $0-8
	MOVD $rangeValueSetValueThunk<>(SB), R0
	MOVD R0, ret+0(FP)
	RET

// func fromPointShimAddr() uintptr
TEXT ·fromPointShimAddr(SB), NOSPLIT|NOFRAME, $0-8
	MOVD $fromPointShim<>(SB), R0
	MOVD R0, ret+0(FP)
	RET

// func rangeValueSetValueShimAddr() uintptr
TEXT ·rangeValueSetValueShimAddr(SB), NOSPLIT|NOFRAME, $0-8
	MOVD $rangeValueSetValueShim<>(SB), R0
	MOVD R0, ret+0(FP)
	RET

// textFromPointThunk is the ITextProvider::RangeFromPoint vtable slot. A Point is a homogeneous floating-point
// aggregate, so it arrives in D0 and D1 rather than by reference, which leaves the out pointer as the second integer
// argument in X1: the same register plan as fromPointThunk above, and the same three instructions in the same order.
TEXT textFromPointThunk<>(SB), NOSPLIT|NOFRAME, $0
	MOVD  R1, R3
	FMOVD F0, R1
	FMOVD F1, R2
	MOVD  $·textFromPointCallback(SB), R16
	MOVD  (R16), R16
	B     (R16)

// textFromPointShim calls textFromPointThunk the way UI Automation does, entered with X0=this, X1=the bits of x,
// X2=the bits of y and X3=out. It works exactly as fromPointShim does.
TEXT textFromPointShim<>(SB), NOSPLIT|NOFRAME, $0
	FMOVD R1, F0
	FMOVD R2, F1
	MOVD  R3, R1
	MOVD  $-1, R2
	MOVD  $-1, R3
	B     textFromPointThunk<>(SB)

// func textFromPointThunkAddr() uintptr
TEXT ·textFromPointThunkAddr(SB), NOSPLIT|NOFRAME, $0-8
	MOVD $textFromPointThunk<>(SB), R0
	MOVD R0, ret+0(FP)
	RET

// func textFromPointShimAddr() uintptr
TEXT ·textFromPointShimAddr(SB), NOSPLIT|NOFRAME, $0-8
	MOVD $textFromPointShim<>(SB), R0
	MOVD R0, ret+0(FP)
	RET
