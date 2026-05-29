// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package mod

import (
	"bytes"
)

func apiOSMenuCmdModifier() Modifiers {
	return Command
}

func (m Modifiers) apiString() string {
	if m == 0 {
		return ""
	}
	var buffer bytes.Buffer
	if m.ControlDown() {
		buffer.WriteRune('⌃')
	}
	if m.OptionDown() {
		buffer.WriteRune('⌥')
	}
	if m.ShiftDown() {
		buffer.WriteRune('⇧')
	}
	if m.CapsLockDown() {
		buffer.WriteRune('⇪')
	}
	if m.NumLockDown() {
		buffer.WriteRune('⇭')
	}
	if m.CommandDown() {
		buffer.WriteRune('⌘')
	}
	return buffer.String()
}
