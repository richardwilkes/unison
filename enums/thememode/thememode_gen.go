// Code generated from "enum.go.tmpl" - DO NOT EDIT.

/*
 * Copyright ©2021-2023 by Richard A. Wilkes. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, version 2.0. If a copy of the MPL was not distributed with
 * this file, You can obtain one at http://mozilla.org/MPL/2.0/.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as
 * defined by the Mozilla Public License, version 2.0.
 */

package thememode

import (
	"strings"

	"github.com/richardwilkes/toolbox/i18n"
)

// Possible values.
const (
	Auto Enum = iota
	Dark
	Light
)

// All possible values.
var All = []Enum{
	Auto,
	Dark,
	Light,
}

// Enum holds the theme display mode.
type Enum byte

// EnsureValid ensures this is of a known value.
func (e Enum) EnsureValid() Enum {
	if e <= Light {
		return e
	}
	return 0
}

// Key returns the key used in serialization.
func (e Enum) Key() string {
	switch e {
	case Auto:
		return "auto"
	case Dark:
		return "dark"
	case Light:
		return "light"
	default:
		return Enum(0).Key()
	}
}

// String implements fmt.Stringer.
func (e Enum) String() string {
	switch e {
	case Auto:
		return i18n.Text("Automatic")
	case Dark:
		return i18n.Text("Dark")
	case Light:
		return i18n.Text("Light")
	default:
		return Enum(0).String()
	}
}

// MarshalText implements the encoding.TextMarshaler interface.
func (e Enum) MarshalText() (text []byte, err error) {
	return []byte(e.Key()), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (e *Enum) UnmarshalText(text []byte) error {
	*e = Extract(string(text))
	return nil
}

// Extract the value from a string.
func Extract(str string) Enum {
	for _, e := range All {
		if strings.EqualFold(e.Key(), str) {
			return e
		}
	}
	return 0
}
