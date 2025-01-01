// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
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
