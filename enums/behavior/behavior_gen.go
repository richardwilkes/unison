// Code generated from "enum.go.tmpl" - DO NOT EDIT.

// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package behavior

import (
	"strings"

	"github.com/richardwilkes/toolbox/i18n"
)

// Possible values.
const (
	Unmodified Enum = iota
	Fill            // If the content is smaller than the available space, expand it
	Follow          // Fix the content to the view size
	HintedFill      // Uses hints to try and fix the content to the view size, but if the resulting content is smaller than the available space, expands it
)

// All possible values.
var All = []Enum{
	Unmodified,
	Fill,
	Follow,
	HintedFill,
}

// Enum controls how auto-sizing of the scroll content's preferred size is handled.
type Enum byte

// EnsureValid ensures this is of a known value.
func (e Enum) EnsureValid() Enum {
	if e <= HintedFill {
		return e
	}
	return Unmodified
}

// Key returns the key used in serialization.
func (e Enum) Key() string {
	switch e {
	case Unmodified:
		return "unmodified"
	case Fill:
		return "fill"
	case Follow:
		return "follow"
	case HintedFill:
		return "hinted-fill"
	default:
		return Unmodified.Key()
	}
}

// String implements fmt.Stringer.
func (e Enum) String() string {
	switch e {
	case Unmodified:
		return i18n.Text("Unmodified")
	case Fill:
		return i18n.Text("Fill")
	case Follow:
		return i18n.Text("Follow")
	case HintedFill:
		return i18n.Text("Hinted-Fill")
	default:
		return Unmodified.String()
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
	return Unmodified
}
