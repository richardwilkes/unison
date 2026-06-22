// Code generated from "enum.go.tmpl" - DO NOT EDIT.

// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package gradienttype

import (
	"strings"

	"github.com/richardwilkes/toolbox/v2/i18n"
)

// Possible values.
const (
	Linear Enum = iota
	Radial
	Sweep
	Conical
)

// All possible values.
var All = []Enum{
	Linear,
	Radial,
	Sweep,
	Conical,
}

// Enum specifies the type of gradient to use.
type Enum byte

// EnsureValid ensures this is of a known value.
func (e Enum) EnsureValid() Enum {
	if e <= Conical {
		return e
	}
	return Linear
}

// Key returns the key used in serialization.
func (e Enum) Key() string {
	switch e {
	case Linear:
		return "linear"
	case Radial:
		return "radial"
	case Sweep:
		return "sweep"
	case Conical:
		return "conical"
	default:
		return Linear.Key()
	}
}

// String implements fmt.Stringer.
func (e Enum) String() string {
	switch e {
	case Linear:
		return i18n.Text("Linear")
	case Radial:
		return i18n.Text("Radial")
	case Sweep:
		return i18n.Text("Sweep")
	case Conical:
		return i18n.Text("Conical")
	default:
		return Linear.String()
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
	return Linear
}
