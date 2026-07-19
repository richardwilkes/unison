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
)

// TestStuckModifierTableUsesVirtualKeyCodes verifies that the stuck-modifier-release hack queries GetKeyState with
// virtual-key codes. Prior to the fix, the table held the keys' scan codes (0x2A, 0x36, 0x15B, 0x15C), so
// GetKeyState(0x2A) actually tested VK_PRINT and GetKeyState(0x36) tested the '6' key, causing a spurious keyReleased
// to be synthesized on the next event-loop pass while Shift or a Win key was genuinely held.
func TestStuckModifierTableUsesVirtualKeyCodes(t *testing.T) {
	c := check.New(t)
	expected := map[KeyCode]int{
		KeyLShift:   0xA0, // VK_LSHIFT
		KeyRShift:   0xA1, // VK_RSHIFT
		KeyLCommand: 0x5B, // VK_LWIN
		KeyRCommand: 0x5C, // VK_RWIN
	}
	c.Equal(len(expected), len(w32StuckModifierKeys))
	for _, k := range w32StuckModifierKeys {
		c.Equal(expected[k.key], k.virtualKey)
	}
}

// TestCollectStuckModifiers verifies the decision logic: a key is only reported as stuck when the window believes it
// is pressed but the OS reports it as up.
func TestCollectStuckModifiers(t *testing.T) {
	c := check.New(t)

	// A fake GetKeyState that reports keys in the down set as pressed. The old bug queried scan codes, so simulating
	// "left Shift is physically held" by reporting only VK_LSHIFT as down distinguishes correct from broken lookups:
	// the buggy code queried 0x2A instead, saw "up", and synthesized a phantom release.
	fakeKeyState := func(down ...int) func(int) uint16 {
		return func(virtualKey int) uint16 {
			for _, d := range down {
				if d == virtualKey {
					return 0x8000
				}
			}
			return 0
		}
	}

	// Left Shift genuinely held: pressed in the window and reported down by the OS. Nothing should be released.
	pressed := map[KeyCode]bool{KeyLShift: true}
	c.Equal(0, len(w32CollectStuckModifiers(pressed, fakeKeyState(0xA0))))

	// Regression for the original finding: with the same physical state, a lookup keyed by the old scan code (0x2A)
	// finds nothing down and would flag left Shift as stuck.
	c.Equal([]KeyCode{KeyLShift}, w32CollectStuckModifiers(pressed, fakeKeyState(0x2A)))

	// Genuinely stuck: the window thinks right Shift and the right Win key are held, but the OS says everything is up.
	pressed = map[KeyCode]bool{KeyRShift: true, KeyRCommand: true}
	c.Equal([]KeyCode{KeyRShift, KeyRCommand}, w32CollectStuckModifiers(pressed, fakeKeyState()))

	// Keys the window never saw pressed are ignored no matter what the OS reports.
	c.Equal(0, len(w32CollectStuckModifiers(map[KeyCode]bool{}, fakeKeyState())))
}
