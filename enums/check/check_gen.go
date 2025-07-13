// Code generated from "enum.go.tmpl" - DO NOT EDIT.

// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package check

import (
	"strings"

	"github.com/richardwilkes/toolbox/v2/i18n"
)

// Possible values.
const (
	Off Enum = iota
	On
	Mixed
)

// All possible values.
var All = []Enum{
	Off,
	On,
	Mixed,
}

// Enum represents the current state of something like a check box or mark.
type Enum byte

// EnsureValid ensures this is of a known value.
func (e Enum) EnsureValid() Enum {
	if e <= Mixed {
		return e
	}
	return Off
}

// Key returns the key used in serialization.
func (e Enum) Key() string {
	switch e {
	case Off:
		return "off"
	case On:
		return "on"
	case Mixed:
		return "mixed"
	default:
		return Off.Key()
	}
}

// String implements fmt.Stringer.
func (e Enum) String() string {
	switch e {
	case Off:
		return i18n.Text("Off")
	case On:
		return i18n.Text("On")
	case Mixed:
		return i18n.Text("Mixed")
	default:
		return Off.String()
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
	return Off
}
