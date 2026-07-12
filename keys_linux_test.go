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
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/unison/enums/mod"
)

// TestX11ModifiersFromKeymap verifies that the modifier bit indices probe a QueryKeymap() bit vector using the same
// raw-keycode indexing that QueryKeymap() itself uses. Prior to the fix, the indices were offset by the server's
// minimum keycode, so a pressed modifier (e.g. Shift) was never detected, which broke Shift-click multi-selection in
// popup menus on Linux. See https://github.com/richardwilkes/gcs/issues/1069.
func TestX11ModifiersFromKeymap(t *testing.T) {
	c := check.New(t)

	// Typical X11 keycodes for these keys on a server with a minimum keycode of 8. The exact values only matter in
	// that they are well above minKeyCode, so an off-by-minKeyCode error probes an entirely different bit.
	const (
		lShiftKeycode   = uint16(50)
		rShiftKeycode   = uint16(62)
		lControlKeycode = uint16(37)
		rControlKeycode = uint16(105)
		capsLockKeycode = uint16(66)
	)

	// Restore the package globals after the test so we don't perturb any live keyboard mapping.
	saved := [...]int{
		x11KeyLShiftBitIndex, x11KeyRShiftBitIndex,
		x11KeyLControlBitIndex, x11KeyRControlBitIndex,
		x11KeyLOptionBitIndex, x11KeyROptionBitIndex,
		x11KeyLCommandBitIndex, x11KeyRCommandBitIndex,
		x11KeyCapsLockBitIndex, x11KeyNumLockBitIndex,
	}
	t.Cleanup(func() {
		x11KeyLShiftBitIndex, x11KeyRShiftBitIndex = saved[0], saved[1]
		x11KeyLControlBitIndex, x11KeyRControlBitIndex = saved[2], saved[3]
		x11KeyLOptionBitIndex, x11KeyROptionBitIndex = saved[4], saved[5]
		x11KeyLCommandBitIndex, x11KeyRCommandBitIndex = saved[6], saved[7]
		x11KeyCapsLockBitIndex, x11KeyNumLockBitIndex = saved[8], saved[9]
	})

	// Assign the bit indices exactly as the keyboard-mapping setup does.
	x11KeyLShiftBitIndex = x11ModifierBitIndex(lShiftKeycode)
	x11KeyRShiftBitIndex = x11ModifierBitIndex(rShiftKeycode)
	x11KeyLControlBitIndex = x11ModifierBitIndex(lControlKeycode)
	x11KeyRControlBitIndex = x11ModifierBitIndex(rControlKeycode)
	x11KeyCapsLockBitIndex = x11ModifierBitIndex(capsLockKeycode)
	// Park the remaining indices on a guaranteed-clear bit so they never register.
	x11KeyLOptionBitIndex = 0
	x11KeyROptionBitIndex = 0
	x11KeyLCommandBitIndex = 0
	x11KeyRCommandBitIndex = 0
	x11KeyNumLockBitIndex = 0

	// pressKeymap builds a bit vector the way QueryKeymap() reports it: the bit for a keycode lives at byte
	// keycode>>3, bit keycode&7.
	pressKeymap := func(keycodes ...uint16) []byte {
		var keyMap [32]byte
		for _, keycode := range keycodes {
			keyMap[keycode>>3] |= 1 << (keycode & 7)
		}
		return keyMap[:]
	}

	c.Equal(mod.Modifiers(0), x11ModifiersFromKeymap(pressKeymap()), "nothing pressed")
	c.True(x11ModifiersFromKeymap(pressKeymap(lShiftKeycode)).ShiftDown(), "left shift")
	c.True(x11ModifiersFromKeymap(pressKeymap(rShiftKeycode)).ShiftDown(), "right shift")
	c.True(x11ModifiersFromKeymap(pressKeymap(lControlKeycode)).ControlDown(), "left control")
	c.True(x11ModifiersFromKeymap(pressKeymap(rControlKeycode)).ControlDown(), "right control")
	c.True(x11ModifiersFromKeymap(pressKeymap(capsLockKeycode)).CapsLockDown(), "caps lock")

	// Shift+Control held together, as when Shift-clicking a menu item.
	both := x11ModifiersFromKeymap(pressKeymap(lShiftKeycode, lControlKeycode))
	c.True(both.ShiftDown(), "shift when shift+control held")
	c.True(both.ControlDown(), "control when shift+control held")

	// A non-modifier key at the bit index an off-by-minKeyCode bug would have used for shift must not read as shift.
	c.False(x11ModifiersFromKeymap(pressKeymap(lShiftKeycode-8)).ShiftDown(),
		"a key minKeyCode below the shift keycode must not register as shift")
}
