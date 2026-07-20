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

// TestFieldBlinkClearsPendingWhenDetached verifies that a blink task firing while the field is not in a valid window
// still clears the pending flag. Leaving it set would permanently suppress caret blinking after the field is
// re-attached to a window, since scheduleBlink refuses to schedule while pending is true.
func TestFieldBlinkClearsPendingWhenDetached(t *testing.T) {
	c := check.New(t)
	f := NewField()
	f.pending = true
	f.blink()
	c.False(f.pending)
}

// TestFieldDefaultKeyDownConsumesOSMenuCommandKeys verifies that DefaultKeyDown reports OS-menu-command key
// combinations it acted upon as handled, so that dispatch stops rather than continuing to ancestors (e.g. a Table,
// which treats arrow keys as navigation without checking the command modifier).
func TestFieldDefaultKeyDownConsumesOSMenuCommandKeys(t *testing.T) {
	c := check.New(t)
	f := NewField()
	f.SetText("hello")
	cmd := mod.OSMenuCommand()
	for _, keyCode := range []KeyCode{KeyLeft, KeyRight, KeyUp, KeyDown} {
		c.True(f.DefaultKeyDown(keyCode, cmd, false), "cmd+%v must be reported as handled", keyCode)
	}
	c.True(f.CanSelectAll())
	c.True(f.DefaultKeyDown(KeyA, cmd, false), "cmd+A must be reported as handled when select-all is possible")
	c.False(f.CanSelectAll())
	c.False(f.DefaultKeyDown(KeyA, cmd, false),
		"cmd+A must not be reported as handled when everything is already selected")
	c.False(f.DefaultKeyDown(KeyB, cmd, false), "an unbound command key must not be reported as handled")
}
