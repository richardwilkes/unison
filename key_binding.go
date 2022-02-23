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
	"strings"

	"github.com/richardwilkes/json"
)

var _ json.Omitter = KeyBinding{}

// KeyBinding holds a key code and/or modifier.
type KeyBinding struct {
	KeyCode   KeyCode
	Modifiers Modifiers
}

// KeyBindingFromKey extracts a KeyBinding from a string created via a call to .Key().
func KeyBindingFromKey(key string) KeyBinding {
	parts := strings.Split(key, "+")
	switch len(parts) {
	case 1:
		if k := KeyCodeFromKey(parts[0]); k != 0 {
			return KeyBinding{KeyCode: k}
		}
		return KeyBinding{Modifiers: ModifiersFromKey(parts[0])}
	case 2:
		return KeyBinding{
			KeyCode:   KeyCodeFromKey(parts[1]),
			Modifiers: ModifiersFromKey(parts[0]),
		}
	default:
		return KeyBinding{}
	}
}

// Key returns a string version of the KeyCode for the purpose of serialization.
func (b KeyBinding) Key() string {
	m := b.Modifiers.Key()
	k := b.KeyCode.Key()
	if m == "" {
		return k
	}
	if k == "" {
		return m
	}
	return m + "+" + k
}

// MarshalText implements encoding.TextMarshaler.
func (b KeyBinding) MarshalText() (text []byte, err error) {
	return []byte(b.Key()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (b *KeyBinding) UnmarshalText(text []byte) error {
	*b = KeyBindingFromKey(string(text))
	return nil
}

func (b KeyBinding) String() string {
	return b.Modifiers.String() + b.KeyCode.String()
}

// ShouldOmit implements json.Omitter.
func (b KeyBinding) ShouldOmit() bool {
	return b.Modifiers == 0 && b.KeyCode.ShouldOmit()
}
