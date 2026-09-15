// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

// The amd64 halves of the two UI Automation vtable slots that take doubles. See uia_thunk_windows.go for the whole
// story; the register plan for this file is:
//
//	ElementProviderFromPoint(this, double x, double y, out) arrives as RCX=this, XMM1=x, XMM2=y, R9=out, because the
//	Windows x64 convention numbers integer and floating-point arguments together: argument n goes in the n-th integer
//	register or the n-th XMM register. The Go callback reads RCX, RDX, R8, R9, so the doubles have to move to RDX and
//	R8, and the other two registers are already right.
//
//	SetValue(this, double value) arrives as RCX=this, XMM1=value, so value has to move to RDX.
//
// AX is the only other register touched, and the x64 convention makes it volatile. The jump is a JMP rather than a CALL
// so that the runtime's callback trampoline finds the stack exactly as it would had UI Automation called it directly:
// the return address at 0(SP), and the caller's 32 bytes of shadow space above it for the trampoline to spill the four
// integer argument registers into.

#include "textflag.h"

// uiaFromPointThunk is the ElementProviderFromPoint vtable slot.
TEXT uiaFromPointThunk<>(SB), NOSPLIT|NOFRAME, $0
	MOVQ X1, DX
	MOVQ X2, R8
	MOVQ ·uiaFromPointCallback(SB), AX
	JMP  AX

// uiaRangeValueSetValueThunk is the IRangeValueProvider::SetValue vtable slot.
TEXT uiaRangeValueSetValueThunk<>(SB), NOSPLIT|NOFRAME, $0
	MOVQ X1, DX
	MOVQ ·uiaRangeValueSetValueCallback(SB), AX
	JMP  AX

// uiaFromPointShim calls uiaFromPointThunk the way UI Automation does. It is reached through syscall.SyscallN, so it is
// entered with RCX=this, RDX=the bits of x, R8=the bits of y and R9=out. It moves the two sets of bits into XMM1 and
// XMM2, fills RDX and R8 with a value the test cannot mistake for a coordinate, and tail-jumps to the thunk.
TEXT uiaFromPointShim<>(SB), NOSPLIT|NOFRAME, $0
	MOVQ DX, X1
	MOVQ R8, X2
	MOVQ $-1, DX
	MOVQ $-1, R8
	JMP  uiaFromPointThunk<>(SB)

// uiaRangeValueSetValueShim calls uiaRangeValueSetValueThunk the way UI Automation does, entered with RCX=this and
// RDX=the bits of the value.
TEXT uiaRangeValueSetValueShim<>(SB), NOSPLIT|NOFRAME, $0
	MOVQ DX, X1
	MOVQ $-1, DX
	JMP  uiaRangeValueSetValueThunk<>(SB)

// func uiaFromPointThunkAddr() uintptr
TEXT ·uiaFromPointThunkAddr(SB), NOSPLIT|NOFRAME, $0-8
	LEAQ uiaFromPointThunk<>(SB), AX
	MOVQ AX, ret+0(FP)
	RET

// func uiaRangeValueSetValueThunkAddr() uintptr
TEXT ·uiaRangeValueSetValueThunkAddr(SB), NOSPLIT|NOFRAME, $0-8
	LEAQ uiaRangeValueSetValueThunk<>(SB), AX
	MOVQ AX, ret+0(FP)
	RET

// func uiaFromPointShimAddr() uintptr
TEXT ·uiaFromPointShimAddr(SB), NOSPLIT|NOFRAME, $0-8
	LEAQ uiaFromPointShim<>(SB), AX
	MOVQ AX, ret+0(FP)
	RET

// func uiaRangeValueSetValueShimAddr() uintptr
TEXT ·uiaRangeValueSetValueShimAddr(SB), NOSPLIT|NOFRAME, $0-8
	LEAQ uiaRangeValueSetValueShim<>(SB), AX
	MOVQ AX, ret+0(FP)
	RET
