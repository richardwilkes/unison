// Code generated from "enum.go.tmpl" - DO NOT EDIT.

// Copyright (c) 2021-2024 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package filtermode

import (
	"strings"

	"github.com/richardwilkes/toolbox/i18n"
)

// Possible values.
const (
	Nearest Enum = iota // Single sample point (nearest neighbor)
	Linear              // Interpolate between 2x2 sample points (bilinear interpolation)
)

// All possible values.
var All = []Enum{
	Nearest,
	Linear,
}

// Enum holds the type of sampling to be done.
type Enum int32

// EnsureValid ensures this is of a known value.
func (e Enum) EnsureValid() Enum {
	if e >= Nearest && e <= Linear {
		return e
	}
	return Nearest
}

// Key returns the key used in serialization.
func (e Enum) Key() string {
	switch e {
	case Nearest:
		return "nearest"
	case Linear:
		return "linear"
	default:
		return Nearest.Key()
	}
}

// String implements fmt.Stringer.
func (e Enum) String() string {
	switch e {
	case Nearest:
		return i18n.Text("Nearest")
	case Linear:
		return i18n.Text("Linear")
	default:
		return Nearest.String()
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
	return Nearest
}
