// Code generated from "enum.go.tmpl" - DO NOT EDIT.

// Copyright (c) 2021-2024 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package pathop

import (
	"strings"

	"github.com/richardwilkes/toolbox/i18n"
)

// Possible values.
const (
	Difference Enum = iota
	Intersect
	Union
	Xor
	ReverseDifference
)

// All possible values.
var All = []Enum{
	Difference,
	Intersect,
	Union,
	Xor,
	ReverseDifference,
}

// Enum holds the possible operations that can be performed on a pair of paths.
type Enum byte

// EnsureValid ensures this is of a known value.
func (e Enum) EnsureValid() Enum {
	if e <= ReverseDifference {
		return e
	}
	return Difference
}

// Key returns the key used in serialization.
func (e Enum) Key() string {
	switch e {
	case Difference:
		return "difference"
	case Intersect:
		return "intersect"
	case Union:
		return "union"
	case Xor:
		return "xor"
	case ReverseDifference:
		return "reverse-difference"
	default:
		return Difference.Key()
	}
}

// String implements fmt.Stringer.
func (e Enum) String() string {
	switch e {
	case Difference:
		return i18n.Text("Difference")
	case Intersect:
		return i18n.Text("Intersect")
	case Union:
		return i18n.Text("Union")
	case Xor:
		return i18n.Text("Xor")
	case ReverseDifference:
		return i18n.Text("Reverse-Difference")
	default:
		return Difference.String()
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
	return Difference
}
