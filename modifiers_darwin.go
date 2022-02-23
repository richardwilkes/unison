// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
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

	"github.com/richardwilkes/unison/internal/ns"
)

// OSMenuCmdModifier returns the OS's standard menu command key modifier.
func OSMenuCmdModifier() Modifiers {
	return CommandModifier
}

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

func (m Modifiers) eventModifierFlags() ns.EventModifierFlags {
	var mods ns.EventModifierFlags
	if m.ShiftDown() {
		mods |= ns.EventModifierFlagShift
	}
	if m.OptionDown() {
		mods |= ns.EventModifierFlagOption
	}
	if m.CommandDown() {
		mods |= ns.EventModifierFlagCommand
	}
	if m.ControlDown() {
		mods |= ns.EventModifierFlagControl
	}
	if m.CapsLockDown() {
		mods |= ns.EventModifierFlagCapsLock
	}
	return mods
}

func modifiersFromEventModifierFlags(flags ns.EventModifierFlags) Modifiers {
	var mods Modifiers
	if flags&ns.EventModifierFlagShift != 0 {
		mods |= ShiftModifier
	}
	if flags&ns.EventModifierFlagOption != 0 {
		mods |= OptionModifier
	}
	if flags&ns.EventModifierFlagCommand != 0 {
		mods |= CommandModifier
	}
	if flags&ns.EventModifierFlagControl != 0 {
		mods |= ControlModifier
	}
	if flags&ns.EventModifierFlagCapsLock != 0 {
		mods |= CapsLockModifier
	}
	return mods
}
