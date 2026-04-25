// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

func x11TranslateModifierState(state uint16) Modifiers {
	var m Modifiers
	if state&0x0001 != 0 {
		m |= ShiftModifier
	}
	if state&0x0002 != 0 {
		m |= CapsLockModifier
	}
	if state&0x0004 != 0 {
		m |= ControlModifier
	}
	if state&0x0008 != 0 {
		m |= OptionModifier
	}
	if state&0x0010 != 0 {
		m |= CommandModifier
	}
	return m
}
