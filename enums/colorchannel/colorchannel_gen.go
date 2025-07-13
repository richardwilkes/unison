// Code generated from "enum.go.tmpl" - DO NOT EDIT.

// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package colorchannel

import (
	"strings"

	"github.com/richardwilkes/toolbox/v2/i18n"
)

// Possible values.
const (
	Red Enum = iota
	Green
	Blue
	Alpha
)

// All possible values.
var All = []Enum{
	Red,
	Green,
	Blue,
	Alpha,
}

// Enum specifies a specific channel within an RGBA color.
type Enum byte

// EnsureValid ensures this is of a known value.
func (e Enum) EnsureValid() Enum {
	if e <= Alpha {
		return e
	}
	return Red
}

// Key returns the key used in serialization.
func (e Enum) Key() string {
	switch e {
	case Red:
		return "red"
	case Green:
		return "green"
	case Blue:
		return "blue"
	case Alpha:
		return "alpha"
	default:
		return Red.Key()
	}
}

// String implements fmt.Stringer.
func (e Enum) String() string {
	switch e {
	case Red:
		return i18n.Text("Red")
	case Green:
		return i18n.Text("Green")
	case Blue:
		return i18n.Text("Blue")
	case Alpha:
		return i18n.Text("Alpha")
	default:
		return Red.String()
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
	return Red
}
