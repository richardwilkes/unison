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

	"github.com/richardwilkes/toolbox/i18n"
)

// Possible values.
const (
	AutomaticColorMode ColorMode = iota
	DarkColorMode
	LightColorMode
	LastColorMode = LightColorMode
)

var (
	// AllColorModes holds all possible values.
	AllColorModes = []ColorMode{
		AutomaticColorMode,
		DarkColorMode,
		LightColorMode,
	}
	modeData = []struct {
		key    string
		string string
	}{
		{
			key:    "auto",
			string: i18n.Text("Automatic"),
		},
		{
			key:    "dark",
			string: i18n.Text("Dark"),
		},
		{
			key:    "light",
			string: i18n.Text("Light"),
		},
	}
)

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
	return modeData[enum.EnsureValid()].key
}

// String implements fmt.Stringer.
func (enum ColorMode) String() string {
	return modeData[enum.EnsureValid()].string
}

// ExtractMode extracts the value from a string.
func ExtractMode(str string) ColorMode {
	for i, one := range modeData {
		if strings.EqualFold(one.key, str) {
			return ColorMode(i)
		}
	}
	return 0
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
