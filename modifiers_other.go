// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

//go:build !darwin

package unison

import (
	"bytes"

	"github.com/richardwilkes/unison/i18n"
)

// OSMenuCmdModifier returns the OS's standard menu command key modifier.
func OSMenuCmdModifier() Modifiers {
	return ControlModifier
}

func (m Modifiers) platformString() string {
	if m == 0 {
		return ""
	}
	var buffer bytes.Buffer
	if m.ControlDown() {
		buffer.WriteString(i18n.Text("Ctrl+"))
	}
	if m.OptionDown() {
		buffer.WriteString(i18n.Text("Alt+"))
	}
	if m.ShiftDown() {
		buffer.WriteString(i18n.Text("Shift+"))
	}
	if m.CapsLockDown() {
		buffer.WriteString(i18n.Text("CapsLock+"))
	}
	if m.NumLockDown() {
		buffer.WriteString(i18n.Text("NumLock+"))
	}
	if m.CommandDown() {
		buffer.WriteString(i18n.Text("Super+"))
	}
	return buffer.String()
}
