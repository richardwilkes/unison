// Code generated from "enum.go.tmpl" - DO NOT EDIT.

// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package mipmapmode

import (
	"strings"

	"github.com/richardwilkes/toolbox/v2/i18n"
)

// Possible values.
const (
	None    Enum = iota // Ignore mipmap levels, sample from the 'base'
	Nearest             // Sample from the nearest level
	Linear              // Interpolate between the two nearest levels
)

// All possible values.
var All = []Enum{
	None,
	Nearest,
	Linear,
}

// Enum holds the type of mipmapping to be done.
type Enum int32

// EnsureValid ensures this is of a known value.
func (e Enum) EnsureValid() Enum {
	if e >= None && e <= Linear {
		return e
	}
	return None
}

// Key returns the key used in serialization.
func (e Enum) Key() string {
	switch e {
	case None:
		return "none"
	case Nearest:
		return "nearest"
	case Linear:
		return "linear"
	default:
		return None.Key()
	}
}

// String implements fmt.Stringer.
func (e Enum) String() string {
	switch e {
	case None:
		return i18n.Text("None")
	case Nearest:
		return i18n.Text("Nearest")
	case Linear:
		return i18n.Text("Linear")
	default:
		return None.String()
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
	return None
}
