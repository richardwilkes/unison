// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import "bytes"

func (m Modifiers) platformString() string {
	if m == 0 {
		return ""
	}
	var buffer bytes.Buffer
	if m&ControlModifier == ControlModifier {
		buffer.WriteRune('⌃')
	}
	if m&OptionModifier == OptionModifier {
		buffer.WriteRune('⌥')
	}
	if m&ShiftModifier == ShiftModifier {
		buffer.WriteRune('⇧')
	}
	if m&CapsLockModifier == CapsLockModifier {
		buffer.WriteRune('⇪')
	}
	if m&NumLockModifier == NumLockModifier {
		buffer.WriteRune('⇭')
	}
	if m&CommandModifier == CommandModifier {
		buffer.WriteRune('⌘')
	}
	return buffer.String()
}
