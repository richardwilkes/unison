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
	"github.com/richardwilkes/toolbox/v2/xruntime"
	"github.com/richardwilkes/unison/internal/w32"
	"golang.org/x/sys/windows"
)

// The amd64 half of ITextProvider::RangeFromPoint, the one method in the provider API that takes a structure by value.
//
// A Point is sixteen bytes, and the Windows x64 convention passes any argument that is not one, two, four or eight
// bytes wide by reference: the caller makes a copy and hands over a pointer to it. So the method really arrives as
// RangeFromPoint(this, const Point *point, ITextRangeProvider **out) — three integer arguments in RCX, RDX and R8 —
// which windows.NewCallback describes exactly, and none of the assembly the other two double-taking slots need is
// wanted here. The arm64 half is a different story; see text_point_windows_arm64.go.

// textRangeFromPointSlot returns what belongs in the RangeFromPoint vtable slot.
func textRangeFromPointSlot() uintptr {
	return windows.NewCallback(textProviderRangeFromPointByRef)
}

// textProviderRangeFromPointByRef implements ITextProvider::RangeFromPoint on amd64, where the point arrives as a
// pointer to a copy UI Automation owns. The copy is read here and not held: it is valid for the length of the call,
// which is all that is needed, since the offset it names is worked out before this returns.
func textProviderRangeFromPointByRef(this, point, out uintptr) uint64 {
	if point == 0 {
		return w32.COM_E_POINTER
	}
	pt := xruntime.PtrFromUintptr[Point](point)
	return textProviderRangeFromPointAt(this, pt.X, pt.Y, out)
}
