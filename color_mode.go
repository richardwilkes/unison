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
	"github.com/richardwilkes/toolbox/i18n"
)

// Possible values.
const (
	AutomaticColorMode ColorMode = iota
	DarkColorMode
	LightColorMode
	LastColorMode = LightColorMode
)

const (
	automaticModeKey = "auto"
	darkModeKey      = "dark"
	lightModeKey     = "light"
)

// AllColorModes holds all possible values.
var AllColorModes = []ColorMode{
	AutomaticColorMode,
	DarkColorMode,
	LightColorMode,
}

// ColorMode holds the display mode.
type ColorMode byte

// EnsureValid ensures this is of a known value.
func (enum ColorMode) EnsureValid() ColorMode {
	if enum <= LastColorMode {
		return enum
	}
	return 0
}

// Key returns the key used in serialization.
func (enum ColorMode) Key() string {
	switch enum {
	case AutomaticColorMode:
		return automaticModeKey
	case DarkColorMode:
		return darkModeKey
	case LightColorMode:
		return lightModeKey
	default:
		return ColorMode(0).Key()
	}
}

// String implements fmt.Stringer.
func (enum ColorMode) String() string {
	switch enum {
	case AutomaticColorMode:
		return i18n.Text("Automatic")
	case DarkColorMode:
		return i18n.Text("Dark")
	case LightColorMode:
		return i18n.Text("Light")
	default:
		return ColorMode(0).Key()
	}
}

// ExtractMode extracts the value from a string.
func ExtractMode(str string) ColorMode {
	switch str {
	case automaticModeKey:
		return AutomaticColorMode
	case darkModeKey:
		return DarkColorMode
	case lightModeKey:
		return LightColorMode
	default:
		return ColorMode(0)
	}
}

// MarshalText implements the encoding.TextMarshaler interface.
func (enum ColorMode) MarshalText() (text []byte, err error) {
	return []byte(enum.Key()), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (enum *ColorMode) UnmarshalText(text []byte) error {
	*enum = ExtractMode(string(text))
	return nil
}
