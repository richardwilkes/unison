// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package w32

// hresultSucceeded reports whether an HRESULT indicates success, mirroring the Windows SUCCEEDED() macro: success codes
// (S_OK, S_FALSE, etc.) have the high bit clear, while failure codes (E_FAIL, etc.) have it set. This must be used for
// functions returning HRESULT rather than the BOOL idiom (ret&0xff != 0), which inverts the meaning: S_OK (0) would
// read as failure and most failure codes would read as success.
func hresultSucceeded(hr uintptr) bool {
	return int32(hr) >= 0
}

// wglProcAddressValid reports whether a value returned by wglGetProcAddress is a real function pointer. Per the
// documentation, some OpenGL implementations return 1, 2, 3, or -1 on failure in addition to NULL, so all of those
// sentinels must be rejected.
func wglProcAddressValid(addr uintptr) bool {
	switch addr {
	case 0, 1, 2, 3, ^uintptr(0):
		return false
	default:
		return true
	}
}
