// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package w32

// packPoint packs X and Y coordinates into the single value the Win64 ABI uses to pass an 8-byte POINT structure by
// value: X occupies the low 32 bits and Y the high 32 bits. The coordinates are converted through uint32 so that
// negative values (which occur on multi-monitor setups) do not sign-extend into the other half.
func packPoint(x, y int32) uintptr {
	return uintptr(uint32(x)) | uintptr(uint32(y))<<32
}

// unpackPoint is the inverse of packPoint, recovering the X and Y coordinates from a POINT passed by value in a single
// packed register, as is done for the pt parameter of the IDropTarget methods.
func unpackPoint(pt uintptr) (x, y int32) {
	return int32(uint32(pt)), int32(uint32(pt >> 32))
}
