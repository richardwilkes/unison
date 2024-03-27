// Code generated from "enum.go.tmpl" - DO NOT EDIT.

/*
 * Copyright ©2021-2023 by Richard A. Wilkes. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, version 2.0. If a copy of the MPL was not distributed with
 * this file, You can obtain one at http://mozilla.org/MPL/2.0/.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as
 * defined by the Mozilla Public License, version 2.0.
 */

package invertstyle

import (
	"strings"

	"github.com/richardwilkes/unison/i18n"
)

// Possible values.
const (
	None Enum = iota
	Brightness
	Lightness
)

// All possible values.
var All = []Enum{
	None,
	Brightness,
	Lightness,
}

// Enum holds the type of image inversion.
type Enum byte

// EnsureValid ensures this is of a known value.
func (e Enum) EnsureValid() Enum {
	if e <= Lightness {
		return e
	}
	return None
}

// Key returns the key used in serialization.
func (e Enum) Key() string {
	switch e {
	case None:
		return "none"
	case Brightness:
		return "brightness"
	case Lightness:
		return "lightness"
	default:
		return None.Key()
	}
}

// String implements fmt.Stringer.
func (e Enum) String() string {
	switch e {
	case None:
		return i18n.Text("None")
	case Brightness:
		return i18n.Text("Brightness")
	case Lightness:
		return i18n.Text("Lightness")
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
