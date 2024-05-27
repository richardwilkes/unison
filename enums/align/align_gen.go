// Code generated from "enum.go.tmpl" - DO NOT EDIT.

// Copyright (c) 2021-2024 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package align

import (
	"strings"

	"github.com/richardwilkes/toolbox/i18n"
)

// Possible values.
const (
	Start Enum = iota
	Middle
	End
	Fill
)

// All possible values.
var All = []Enum{
	Start,
	Middle,
	End,
	Fill,
}

// Enum specifies how to align an object within its available space.
type Enum byte

// EnsureValid ensures this is of a known value.
func (e Enum) EnsureValid() Enum {
	if e <= Fill {
		return e
	}
	return Start
}

// Key returns the key used in serialization.
func (e Enum) Key() string {
	switch e {
	case Start:
		return "start"
	case Middle:
		return "middle"
	case End:
		return "end"
	case Fill:
		return "fill"
	default:
		return Start.Key()
	}
}

// String implements fmt.Stringer.
func (e Enum) String() string {
	switch e {
	case Start:
		return i18n.Text("Start")
	case Middle:
		return i18n.Text("Middle")
	case End:
		return i18n.Text("End")
	case Fill:
		return i18n.Text("Fill")
	default:
		return Start.String()
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
	return Start
}
