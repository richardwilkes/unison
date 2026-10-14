// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
)

// TestW32ClipboardText verifies the text put on the clipboard: NUL-terminated UTF-16, with a surrogate pair where one
// is needed, and cut at a NUL in the text rather than refused because of it.
func TestW32ClipboardText(t *testing.T) {
	c := check.New(t)
	c.Equal([]uint16{0}, w32ClipboardText(""))
	c.Equal([]uint16{'h', 'i', 0}, w32ClipboardText("hi"))
	c.Equal([]uint16{0xD83D, 0xDE00, 0}, w32ClipboardText("\U0001F600"))
	c.Equal([]uint16{'a', 'b', 0}, w32ClipboardText("ab\x00cd"), "cut at the NUL, which ends what a reader sees")
	c.Equal([]uint16{0}, w32ClipboardText("\x00cd"))
}
