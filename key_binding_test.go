// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison_test

import (
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/enums/mod"
)

func TestKeyBindingFromKey(t *testing.T) {
	chk := check.New(t)

	// Key code only
	chk.Equal(unison.KeyBinding{KeyCode: unison.KeyZ}, unison.KeyBindingFromKey("Z"))
	chk.Equal(unison.KeyBinding{KeyCode: unison.KeyF1}, unison.KeyBindingFromKey("F1"))

	// Single modifier only
	chk.Equal(unison.KeyBinding{Modifiers: mod.Shift}, unison.KeyBindingFromKey("shift"))

	// Multiple modifiers only
	chk.Equal(unison.KeyBinding{Modifiers: mod.Control | mod.Shift}, unison.KeyBindingFromKey("ctrl+shift"))
	chk.Equal(unison.KeyBinding{Modifiers: mod.Control | mod.Option | mod.Shift | mod.Command},
		unison.KeyBindingFromKey("ctrl+alt+shift+cmd"))

	// One modifier plus a key code
	chk.Equal(unison.KeyBinding{KeyCode: unison.KeyW, Modifiers: mod.Command}, unison.KeyBindingFromKey("cmd+W"))

	// Multiple modifiers plus a key code (e.g. the binding the library itself creates for Redo)
	chk.Equal(unison.KeyBinding{KeyCode: unison.KeyZ, Modifiers: mod.Shift | mod.Command},
		unison.KeyBindingFromKey("shift+cmd+Z"))
	chk.Equal(unison.KeyBinding{KeyCode: unison.KeyF5, Modifiers: mod.Control | mod.Option | mod.Shift | mod.Command},
		unison.KeyBindingFromKey("ctrl+alt+shift+cmd+F5"))

	// A trailing "caps" or "num" resolves to the key code, per the documented rule
	chk.Equal(unison.KeyBinding{KeyCode: unison.KeyCapsLock}, unison.KeyBindingFromKey("caps"))
	chk.Equal(unison.KeyBinding{KeyCode: unison.KeyNumLock, Modifiers: mod.Shift}, unison.KeyBindingFromKey("shift+num"))

	// "caps" and "num" act as modifiers when not in the final position
	chk.Equal(unison.KeyBinding{KeyCode: unison.KeyA, Modifiers: mod.CapsLock | mod.NumLock},
		unison.KeyBindingFromKey("caps+num+A"))

	// Empty and unrecognized input yield a zero binding
	chk.Equal(unison.KeyBinding{}, unison.KeyBindingFromKey(""))
	chk.Equal(unison.KeyBinding{}, unison.KeyBindingFromKey("bogus"))
	chk.True(unison.KeyBindingFromKey("bogus").IsZero())
}

func TestKeyBindingKeyRoundTrip(t *testing.T) {
	chk := check.New(t)
	for _, binding := range []unison.KeyBinding{
		{},
		{KeyCode: unison.KeyA},
		{KeyCode: unison.KeyNumPadAdd},
		{Modifiers: mod.Command},
		{Modifiers: mod.Control | mod.Shift},
		{KeyCode: unison.KeyW, Modifiers: mod.Command},
		{KeyCode: unison.KeyZ, Modifiers: mod.Shift | mod.Command},
		{KeyCode: unison.KeyDelete, Modifiers: mod.Control | mod.Option | mod.Shift | mod.Command},
		{KeyCode: unison.KeyCapsLock},
		{KeyCode: unison.KeyNumLock, Modifiers: mod.Shift},
		{KeyCode: unison.KeyP, Modifiers: mod.CapsLock | mod.NumLock | mod.Command},
	} {
		chk.Equal(binding, unison.KeyBindingFromKey(binding.Key()), "round trip of %q", binding.Key())
	}
}

func TestKeyBindingUnmarshalText(t *testing.T) {
	chk := check.New(t)
	original := unison.KeyBinding{KeyCode: unison.KeyZ, Modifiers: mod.Shift | mod.Command}
	text, err := original.MarshalText()
	chk.NoError(err)
	chk.Equal("shift+cmd+Z", string(text))
	var decoded unison.KeyBinding
	chk.NoError(decoded.UnmarshalText(text))
	chk.Equal(original, decoded)
}
