// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"strings"

	"github.com/richardwilkes/toolbox/i18n"
)

// FontSlant holds the slant of a font.
type FontSlant int32

// Possible values for the slant of a font.
const (
	NoSlant FontSlant = iota
	ItalicSlant
	ObliqueSlant
)

// Slants holds the set of possible FontSlant values.
var Slants = []FontSlant{
	NoSlant,
	ItalicSlant,
	ObliqueSlant,
}

// MarshalText implements the encoding.TextMarshaler interface.
func (s FontSlant) MarshalText() (text []byte, err error) {
	return []byte(s.Key()), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (s *FontSlant) UnmarshalText(text []byte) error {
	*s = SlantFromString(string(text))
	return nil
}

// SlantFromString extracts the FontSlant from a string.
func SlantFromString(str string) FontSlant {
	if str == "" {
		return NoSlant
	}
	for s := NoSlant; s <= ObliqueSlant; s++ {
		if strings.EqualFold(s.Key(), str) {
			return s
		}
	}
	return NoSlant
}

// Key returns the key that is used when serializing.
func (s FontSlant) Key() string {
	switch s {
	case ItalicSlant:
		return "italic"
	case ObliqueSlant:
		return "oblique"
	default:
		return "upright"
	}
}

func (s FontSlant) String() string {
	switch s {
	case ItalicSlant:
		return i18n.Text("Italic")
	case ObliqueSlant:
		return i18n.Text("Oblique")
	default:
		return i18n.Text("Upright")
	}
}
