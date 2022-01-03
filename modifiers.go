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
	"runtime"

	"github.com/richardwilkes/toolbox"
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

// Modifiers contains flags indicating which modifier keys were down when an
// event occurred.
type Modifiers int

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

// OSMenuCmdModifier returns the OS's standard menu command key modifier.
func OSMenuCmdModifier() Modifiers {
	if runtime.GOOS == toolbox.MacOS {
		return CommandModifier
	}
	return ControlModifier
}

// OSMenuCmdModifierDown returns true if the OS's standard menu command key is
// being pressed.
func (m Modifiers) OSMenuCmdModifierDown() bool {
	mask := OSMenuCmdModifier()
	return m&mask == mask
}

// String returns a text representation of these modifiers.
func (m Modifiers) String() string {
	return m.platformString()
}
