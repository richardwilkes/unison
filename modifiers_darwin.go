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
	"bytes"

	"github.com/richardwilkes/unison/internal/mac"
)

func apiOSMenuCmdModifier() Modifiers {
	return CommandModifier
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

func (m Modifiers) macEventModifierFlags() mac.EventModifierFlags {
	var mods mac.EventModifierFlags
	if m.ShiftDown() {
		mods |= mac.EventModifierFlagShift
	}
	if m.OptionDown() {
		mods |= mac.EventModifierFlagOption
	}
	if m.CommandDown() {
		mods |= mac.EventModifierFlagCommand
	}
	if m.ControlDown() {
		mods |= mac.EventModifierFlagControl
	}
	if m.CapsLockDown() {
		mods |= mac.EventModifierFlagCapsLock
	}
	return mods
}

func macModifiersFromEventModifierFlags(flags mac.EventModifierFlags) Modifiers {
	var mods Modifiers
	if flags&mac.EventModifierFlagShift != 0 {
		mods |= ShiftModifier
	}
	if flags&mac.EventModifierFlagOption != 0 {
		mods |= OptionModifier
	}
	if flags&mac.EventModifierFlagCommand != 0 {
		mods |= CommandModifier
	}
	if flags&mac.EventModifierFlagControl != 0 {
		mods |= ControlModifier
	}
	if flags&mac.EventModifierFlagCapsLock != 0 {
		mods |= CapsLockModifier
	}
	return mods
}
