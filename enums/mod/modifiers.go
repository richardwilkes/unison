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
	"strings"
	"sync/atomic"
)

// Possible Modifiers values.
const (
	Shift Modifiers = 1 << iota
	Control
	Option
	Command
	CapsLock
	NumLock
	NonSticky           = Shift | Control | Option | Command
	Sticky              = CapsLock | NumLock
	All                 = Sticky | NonSticky
	None      Modifiers = 0
)

// Modifiers contains flags indicating which modifier keys were down when an event occurred.
type Modifiers byte

var platformNeutral atomic.Bool

// SetPlatformNeutral makes OSMenuCommand() return Control and String() use the "Ctrl+Shift+" form on every platform
// when on is true, or the platform's own convention when false (the default). A headless session turns it on for its
// lifetime. Returns the previous setting. Safe to call from any goroutine.
func SetPlatformNeutral(on bool) (previous bool) {
	return platformNeutral.Swap(on)
}

// PlatformNeutral reports whether SetPlatformNeutral(true) is in effect. Safe to call from any goroutine.
func PlatformNeutral() bool {
	return platformNeutral.Load()
}

// ShiftDown returns true if the shift key is being pressed.
func (m Modifiers) ShiftDown() bool {
	return m&Shift == Shift
}

// ControlDown returns true if the control key is being pressed.
func (m Modifiers) ControlDown() bool {
	return m&Control == Control
}

// OptionDown returns true if the option/alt key is being pressed.
func (m Modifiers) OptionDown() bool {
	return m&Option == Option
}

// CommandDown returns true if the command/meta key is being pressed.
func (m Modifiers) CommandDown() bool {
	return m&Command == Command
}

// CapsLockDown returns true if the caps lock key is being pressed.
func (m Modifiers) CapsLockDown() bool {
	return m&CapsLock == CapsLock
}

// NumLockDown returns true if the num lock key is being pressed.
func (m Modifiers) NumLockDown() bool {
	return m&NumLock == NumLock
}

// DiscontiguousSelectionDown returns true if either the control or command/meta key is being pressed.
func (m Modifiers) DiscontiguousSelectionDown() bool {
	return m&(Control|Command) != 0
}

// OSMenuCommandDown returns true if the OS's standard menu command key is being pressed.
func (m Modifiers) OSMenuCommandDown() bool {
	mask := OSMenuCommand()
	return m&mask == mask
}

// String returns a text representation of these modifiers: the macOS glyphs (⌃⌥⇧⇪⇭⌘) on macOS, or the "Ctrl+Shift+"
// form everywhere else and while SetPlatformNeutral(true) is in effect.
func (m Modifiers) String() string {
	if platformNeutral.Load() {
		return m.neutralString()
	}
	return m.apiString()
}

// neutralString is the "Ctrl+Shift+" form of String().
func (m Modifiers) neutralString() string {
	if m == 0 {
		return ""
	}
	var buffer bytes.Buffer
	if m.ControlDown() {
		buffer.WriteString("Ctrl+")
	}
	if m.OptionDown() {
		buffer.WriteString("Alt+")
	}
	if m.ShiftDown() {
		buffer.WriteString("Shift+")
	}
	if m.CapsLockDown() {
		buffer.WriteString("CapsLock+")
	}
	if m.NumLockDown() {
		buffer.WriteString("NumLock+")
	}
	if m.CommandDown() {
		buffer.WriteString("Super+")
	}
	return buffer.String()
}

// MarshalText implements encoding.TextMarshaler.
func (m Modifiers) MarshalText() (text []byte, err error) {
	return []byte(m.Key()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (m *Modifiers) UnmarshalText(text []byte) error {
	*m = FromKey(string(text))
	return nil
}

// OSMenuCommand returns the OS's standard menu command key modifier: Command on macOS, Control everywhere else and
// while SetPlatformNeutral(true) is in effect.
func OSMenuCommand() Modifiers {
	if platformNeutral.Load() {
		return Control
	}
	return apiOSMenuCmdModifier()
}

// List returns a slice of all possible Modifiers values.
func List() []Modifiers {
	return []Modifiers{Shift, Control, Option, Command, CapsLock, NumLock}
}

// FromKey extracts Modifiers from a string created via a call to .Key().
func FromKey(key string) Modifiers {
	var mods Modifiers
	for one := range strings.SplitSeq(strings.ToLower(key), "+") {
		switch one {
		case "ctrl":
			mods |= Control
		case "alt":
			mods |= Option
		case "shift":
			mods |= Shift
		case "caps":
			mods |= CapsLock
		case "num":
			mods |= NumLock
		case "cmd":
			mods |= Command
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
