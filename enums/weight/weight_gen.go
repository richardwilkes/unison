// Code generated from "enum.go.tmpl" - DO NOT EDIT.

// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package weight

import (
	"strings"

	"github.com/richardwilkes/toolbox/i18n"
)

// Possible values.
const (
	Invisible Enum = iota * 100
	Thin
	ExtraLight
	Light
	Regular
	Medium
	SemiBold
	Bold
	ExtraBold
	Black
	ExtraBlack
)

// All possible values.
var All = []Enum{
	Invisible,
	Thin,
	ExtraLight,
	Light,
	Regular,
	Medium,
	SemiBold,
	Bold,
	ExtraBold,
	Black,
	ExtraBlack,
}

// Enum holds the wegith of a font.
type Enum int32

// EnsureValid ensures this is of a known value.
func (e Enum) EnsureValid() Enum {
	for _, one := range All {
		if one == e {
			return e
		}
	}
	return Regular
}

// Key returns the key used in serialization.
func (e Enum) Key() string {
	switch e {
	case Invisible:
		return "invisible"
	case Thin:
		return "thin"
	case ExtraLight:
		return "extra-light"
	case Light:
		return "light"
	case Regular:
		return "regular"
	case Medium:
		return "medium"
	case SemiBold:
		return "semi-bold"
	case Bold:
		return "bold"
	case ExtraBold:
		return "extra-bold"
	case Black:
		return "black"
	case ExtraBlack:
		return "extra-black"
	default:
		return Regular.Key()
	}
}

// String implements fmt.Stringer.
func (e Enum) String() string {
	switch e {
	case Invisible:
		return i18n.Text("Invisible")
	case Thin:
		return i18n.Text("Thin")
	case ExtraLight:
		return i18n.Text("Extra-Light")
	case Light:
		return i18n.Text("Light")
	case Regular:
		return i18n.Text("Regular")
	case Medium:
		return i18n.Text("Medium")
	case SemiBold:
		return i18n.Text("Semi-Bold")
	case Bold:
		return i18n.Text("Bold")
	case ExtraBold:
		return i18n.Text("Extra-Bold")
	case Black:
		return i18n.Text("Black")
	case ExtraBlack:
		return i18n.Text("Extra-Black")
	default:
		return Regular.String()
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
	return Regular
}
