// Code generated from "enum.go.tmpl" - DO NOT EDIT.

// Copyright (c) 2021-2024 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package blur

import (
	"strings"

	"github.com/richardwilkes/toolbox/i18n"
)

// Possible values.
const (
	Normal Enum = iota
	Solid
	Outer
	Inner
)

// All possible values.
var All = []Enum{
	Normal,
	Solid,
	Outer,
	Inner,
}

// Enum holds the type of blur to apply.
type Enum byte

// EnsureValid ensures this is of a known value.
func (e Enum) EnsureValid() Enum {
	if e <= Inner {
		return e
	}
	return Normal
}

// Key returns the key used in serialization.
func (e Enum) Key() string {
	switch e {
	case Normal:
		return "normal"
	case Solid:
		return "solid"
	case Outer:
		return "outer"
	case Inner:
		return "inner"
	default:
		return Normal.Key()
	}
}

// String implements fmt.Stringer.
func (e Enum) String() string {
	switch e {
	case Normal:
		return i18n.Text("Normal")
	case Solid:
		return i18n.Text("Solid")
	case Outer:
		return i18n.Text("Outer")
	case Inner:
		return i18n.Text("Inner")
	default:
		return Normal.String()
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
	return Normal
}
