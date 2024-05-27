// Code generated from "enum.go.tmpl" - DO NOT EDIT.

// Copyright (c) 2021-2024 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package spacing

import (
	"strings"

	"github.com/richardwilkes/toolbox/i18n"
)

// Possible values.
const (
	UltraCondensed Enum = iota + 1
	ExtraCondensed
	Condensed
	SemiCondensed
	Standard
	SemiExpanded
	Expanded
	ExtraExpanded
	UltraExpanded
)

// All possible values.
var All = []Enum{
	UltraCondensed,
	ExtraCondensed,
	Condensed,
	SemiCondensed,
	Standard,
	SemiExpanded,
	Expanded,
	ExtraExpanded,
	UltraExpanded,
}

// Enum holds the text spacing of a font.
type Enum byte

// EnsureValid ensures this is of a known value.
func (e Enum) EnsureValid() Enum {
	if e >= UltraCondensed && e <= UltraExpanded {
		return e
	}
	return Standard
}

// Key returns the key used in serialization.
func (e Enum) Key() string {
	switch e {
	case UltraCondensed:
		return "ultra-condensed"
	case ExtraCondensed:
		return "extra-condensed"
	case Condensed:
		return "condensed"
	case SemiCondensed:
		return "semi-condensed"
	case Standard:
		return "standard"
	case SemiExpanded:
		return "semi-expanded"
	case Expanded:
		return "expanded"
	case ExtraExpanded:
		return "extra-expanded"
	case UltraExpanded:
		return "ultra-expanded"
	default:
		return Standard.Key()
	}
}

// String implements fmt.Stringer.
func (e Enum) String() string {
	switch e {
	case UltraCondensed:
		return i18n.Text("Ultra-Condensed")
	case ExtraCondensed:
		return i18n.Text("Extra-Condensed")
	case Condensed:
		return i18n.Text("Condensed")
	case SemiCondensed:
		return i18n.Text("Semi-Condensed")
	case Standard:
		return i18n.Text("Standard")
	case SemiExpanded:
		return i18n.Text("Semi-Expanded")
	case Expanded:
		return i18n.Text("Expanded")
	case ExtraExpanded:
		return i18n.Text("Extra-Expanded")
	case UltraExpanded:
		return i18n.Text("Ultra-Expanded")
	default:
		return Standard.String()
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
	return Standard
}
