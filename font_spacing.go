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

// FontSpacing holds the text spacing of a font.
type FontSpacing int32

// Possible values for FontSpacing.
const (
	UltraCondensedSpacing FontSpacing = 1 + iota
	ExtraCondensedSpacing
	CondensedSpacing
	SemiCondensedSpacing
	StandardSpacing
	SemiExpandedSpacing
	ExpandedSpacing
	ExtraExpandedSpacing
	UltraExpandedSpacing
)

// Spacings holds the set of possible FontSpacing values.
var Spacings = []FontSpacing{
	UltraCondensedSpacing,
	ExtraCondensedSpacing,
	CondensedSpacing,
	SemiCondensedSpacing,
	StandardSpacing,
	SemiExpandedSpacing,
	ExpandedSpacing,
	ExtraExpandedSpacing,
	UltraExpandedSpacing,
}

// MarshalText implements the encoding.TextMarshaler interface.
func (s FontSpacing) MarshalText() (text []byte, err error) {
	return []byte(s.Key()), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (s *FontSpacing) UnmarshalText(text []byte) error {
	*s = SpacingFromString(string(text))
	return nil
}

// SpacingFromString extracts the FontSpacing from a string.
func SpacingFromString(str string) FontSpacing {
	if str == "" {
		return StandardSpacing
	}
	for s := UltraCondensedSpacing; s <= UltraExpandedSpacing; s++ {
		if strings.EqualFold(s.Key(), str) {
			return s
		}
	}
	return StandardSpacing
}

// Key returns the key that is used when serializing.
func (s FontSpacing) Key() string {
	switch s {
	case UltraCondensedSpacing:
		return "ultra-condensed"
	case ExtraCondensedSpacing:
		return "extra-condensed"
	case CondensedSpacing:
		return "condensed"
	case SemiCondensedSpacing:
		return "semi-condensed"
	case SemiExpandedSpacing:
		return "semi-expanded"
	case ExpandedSpacing:
		return "expanded"
	case ExtraExpandedSpacing:
		return "extra-expanded"
	case UltraExpandedSpacing:
		return "ultra-expanded"
	default:
		return "standard"
	}
}

func (s FontSpacing) String() string {
	switch s {
	case UltraCondensedSpacing:
		return i18n.Text("Ultra-Condensed")
	case ExtraCondensedSpacing:
		return i18n.Text("Extra-Condensed")
	case CondensedSpacing:
		return i18n.Text("Condensed")
	case SemiCondensedSpacing:
		return i18n.Text("Semi-Condensed")
	case SemiExpandedSpacing:
		return i18n.Text("Semi-Expanded")
	case ExpandedSpacing:
		return i18n.Text("Expanded")
	case ExtraExpandedSpacing:
		return i18n.Text("Extra-Expanded")
	case UltraExpandedSpacing:
		return i18n.Text("Ultra-Expanded")
	default:
		return i18n.Text("Standard")
	}
}
