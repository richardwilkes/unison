// Code generated from "enum.go.tmpl" - DO NOT EDIT.

// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package slant

import (
	"strings"

	"github.com/richardwilkes/toolbox/i18n"
)

// Possible values.
const (
	Upright Enum = iota
	Italic
	Oblique
)

// All possible values.
var All = []Enum{
	Upright,
	Italic,
	Oblique,
}

// Enum holds the slant of a font.
type Enum byte

// EnsureValid ensures this is of a known value.
func (e Enum) EnsureValid() Enum {
	if e <= Oblique {
		return e
	}
	return Upright
}

// Key returns the key used in serialization.
func (e Enum) Key() string {
	switch e {
	case Upright:
		return "upright"
	case Italic:
		return "italic"
	case Oblique:
		return "oblique"
	default:
		return Upright.Key()
	}
}

// String implements fmt.Stringer.
func (e Enum) String() string {
	switch e {
	case Upright:
		return i18n.Text("Upright")
	case Italic:
		return i18n.Text("Italic")
	case Oblique:
		return i18n.Text("Oblique")
	default:
		return Upright.String()
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
	return Upright
}
