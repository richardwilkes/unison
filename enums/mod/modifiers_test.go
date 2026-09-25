// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package mod_test

import (
	"runtime"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/unison/enums/mod"
)

const neutralAll = "Ctrl+Alt+Shift+CapsLock+NumLock+Super+"

// checkNative verifies that the platform's own convention is in effect.
func checkNative(c check.Checker) {
	if runtime.GOOS == xos.MacOS {
		c.Equal(mod.Command, mod.OSMenuCommand())
		c.True(mod.Command.OSMenuCommandDown())
		c.False(mod.Control.OSMenuCommandDown())
		c.Equal("⌃⌥⇧⇪⇭⌘", mod.All.String())
		c.Equal("⇧⌘", (mod.Shift | mod.Command).String())
	} else {
		c.Equal(mod.Control, mod.OSMenuCommand())
		c.True(mod.Control.OSMenuCommandDown())
		c.False(mod.Command.OSMenuCommandDown())
		c.Equal(neutralAll, mod.All.String())
		c.Equal("Ctrl+Shift+", (mod.Shift | mod.Control).String())
	}
	c.Equal("", mod.None.String())
}

func TestPlatformNeutral(t *testing.T) {
	c := check.New(t)
	c.False(mod.PlatformNeutral(), "the convention should be the platform's own until something asks otherwise")
	t.Cleanup(func() { mod.SetPlatformNeutral(false) })
	checkNative(c)

	c.False(mod.SetPlatformNeutral(true), "the previous setting should be handed back")
	c.True(mod.PlatformNeutral())
	c.Equal(mod.Control, mod.OSMenuCommand(), "the menu command key should be Control on every platform")
	c.True(mod.Control.OSMenuCommandDown())
	c.True((mod.Control | mod.Shift).OSMenuCommandDown())
	c.False(mod.Command.OSMenuCommandDown())
	c.Equal(neutralAll, mod.All.String(), "and the modifiers should be spelled out rather than drawn as glyphs")
	c.Equal("Ctrl+Shift+", (mod.Shift | mod.Control).String())
	c.Equal("Super+", mod.Command.String())
	c.Equal("", mod.None.String())
	c.True(mod.SetPlatformNeutral(true), "setting it again should report that it was already on")

	c.True(mod.SetPlatformNeutral(false))
	c.False(mod.PlatformNeutral())
	checkNative(c)
}

func TestKeyRoundTrip(t *testing.T) {
	c := check.New(t)
	t.Cleanup(func() { mod.SetPlatformNeutral(false) })
	for _, neutral := range []bool{false, true} {
		mod.SetPlatformNeutral(neutral)
		for _, m := range mod.List() {
			c.Equal(m, mod.FromKey(m.Key()), "Key() is the serialized form and must not follow the convention")
		}
		c.Equal(mod.All, mod.FromKey(mod.All.Key()))
		c.Equal("ctrl+alt+shift+caps+num+cmd", mod.All.Key())
	}
}
