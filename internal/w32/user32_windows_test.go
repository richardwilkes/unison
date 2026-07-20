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
	"testing"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/check"
)

// TestStructSizesMatchWin32 verifies the structure sizes Windows validates at API boundaries. GetWindowPlacement and
// SetWindowPlacement fail unless Length is exactly 44 (the #ifdef _MAC-only rcDevice field must not be present), and
// EnumDisplayDevicesW fails unless cb is exactly 840 (the size of the structure, not of a pointer to it).
func TestStructSizesMatchWin32(t *testing.T) {
	c := check.New(t)
	c.Equal(uintptr(44), unsafe.Sizeof(WINDOWPLACEMENT{}))
	c.Equal(uintptr(840), unsafe.Sizeof(DISPLAY_DEVICEW{}))
}
