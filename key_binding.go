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
	"strings"

	"github.com/richardwilkes/unison/enums/mod"
)

// KeyBinding holds a key code and/or modifier.
type KeyBinding struct {
	KeyCode   KeyCode
	Modifiers mod.Modifiers
}

// KeyBindingFromKey extracts a KeyBinding from a string created via a call to .Key(). Since .Key() always places the
// key code after the modifiers, the final "+"-separated part is treated as the key code if it can be parsed as one and
// any preceding parts are treated as modifiers; otherwise, the entire string is treated as modifiers. The tokens
// "caps" and "num" are used both as key codes (CapsLock, NumLock) and as modifiers, so a trailing "caps" or "num"
// always resolves to the key code.
func KeyBindingFromKey(key string) KeyBinding {
	if i := strings.LastIndex(key, "+"); i != -1 {
		if k := KeyCodeFromKey(key[i+1:]); k != KeyNone {
			return KeyBinding{
				KeyCode:   k,
				Modifiers: mod.FromKey(key[:i]),
			}
		}
		return KeyBinding{Modifiers: mod.FromKey(key)}
	}
	if k := KeyCodeFromKey(key); k != KeyNone {
		return KeyBinding{KeyCode: k}
	}
	return KeyBinding{Modifiers: mod.FromKey(key)}
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

// IsZero implements json.isZero.
func (b KeyBinding) IsZero() bool {
	return b.Modifiers == 0 && b.KeyCode == 0
}
