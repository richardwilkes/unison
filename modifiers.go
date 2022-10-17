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
	"strings"
)

// Possible Modifiers values.
const (
	ShiftModifier Modifiers = 1 << iota
	ControlModifier
	OptionModifier
	CommandModifier
	CapsLockModifier
	NumLockModifier
	NonStickyModifiers           = ShiftModifier | ControlModifier | OptionModifier | CommandModifier
	StickyModifiers              = CapsLockModifier | NumLockModifier
	AllModifiers                 = StickyModifiers | NonStickyModifiers
	NoModifiers        Modifiers = 0
)

// Modifiers contains flags indicating which modifier keys were down when an event occurred.
type Modifiers byte

// ShiftDown returns true if the shift key is being pressed.
func (m Modifiers) ShiftDown() bool {
	return m&ShiftModifier == ShiftModifier
}

// ControlDown returns true if the control key is being pressed.
func (m Modifiers) ControlDown() bool {
	return m&ControlModifier == ControlModifier
}

// OptionDown returns true if the option/alt key is being pressed.
func (m Modifiers) OptionDown() bool {
	return m&OptionModifier == OptionModifier
}

// CommandDown returns true if the command/meta key is being pressed.
func (m Modifiers) CommandDown() bool {
	return m&CommandModifier == CommandModifier
}

// CapsLockDown returns true if the caps lock key is being pressed.
func (m Modifiers) CapsLockDown() bool {
	return m&CapsLockModifier == CapsLockModifier
}

// NumLockDown returns true if the num lock key is being pressed.
func (m Modifiers) NumLockDown() bool {
	return m&NumLockModifier == NumLockModifier
}

// DiscontiguousSelectionDown returns true if either the control or command/meta key is being pressed.
func (m Modifiers) DiscontiguousSelectionDown() bool {
	return m&(ControlModifier|CommandModifier) != 0
}

// OSMenuCmdModifierDown returns true if the OS's standard menu command key is being pressed.
func (m Modifiers) OSMenuCmdModifierDown() bool {
	mask := OSMenuCmdModifier()
	return m&mask == mask
}

// String returns a text representation of these modifiers.
func (m Modifiers) String() string {
	return m.platformString()
}

// MarshalText implements encoding.TextMarshaler.
func (m Modifiers) MarshalText() (text []byte, err error) {
	return []byte(m.Key()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (m *Modifiers) UnmarshalText(text []byte) error {
	*m = ModifiersFromKey(string(text))
	return nil
}

// ModifiersFromKey extracts Modifiers from a string created via a call to .Key().
func ModifiersFromKey(key string) Modifiers {
	var mods Modifiers
	for _, one := range strings.Split(strings.ToLower(key), "+") {
		switch one {
		case "ctrl":
			mods |= ControlModifier
		case "alt":
			mods |= OptionModifier
		case "shift":
			mods |= ShiftModifier
		case "caps":
			mods |= CapsLockModifier
		case "num":
			mods |= NumLockModifier
		case "cmd":
			mods |= CommandModifier
		}
	}
	return mods
}

// Key returns a string version of the Modifiers for the purpose of serialization.
func (m Modifiers) Key() string {
	if m == 0 {
		return ""
	}
	var buffer bytes.Buffer
	if m.ControlDown() {
		buffer.WriteString("ctrl")
	}
	if m.OptionDown() {
		if buffer.Len() != 0 {
			buffer.WriteByte('+')
		}
		buffer.WriteString("alt")
	}
	if m.ShiftDown() {
		if buffer.Len() != 0 {
			buffer.WriteByte('+')
		}
		buffer.WriteString("shift")
	}
	if m.CapsLockDown() {
		if buffer.Len() != 0 {
			buffer.WriteByte('+')
		}
		buffer.WriteString("caps")
	}
	if m.NumLockDown() {
		if buffer.Len() != 0 {
			buffer.WriteByte('+')
		}
		buffer.WriteString("num")
	}
	if m.CommandDown() {
		if buffer.Len() != 0 {
			buffer.WriteByte('+')
		}
		buffer.WriteString("cmd")
	}
	return buffer.String()
}
