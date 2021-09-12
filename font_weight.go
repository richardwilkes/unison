// Copyright ©2021 by Richard A. Wilkes. All rights reserved.
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

// FontWeight holds the weight of a font.
type FontWeight int32

// Possible values for FontWeight.
const (
	InvisibleFontWeight FontWeight = iota * 100
	ThinFontWeight
	ExtraLightFontWeight
	LightFontWeight
	NormalFontWeight
	MediumFontWeight
	SemiBoldFontWeight
	BoldFontWeight
	ExtraBoldFontWeight
	BlackFontWeight
	ExtraBlackFontWeight
)

// FontWeights holds the set of possible FontWeight values.
var FontWeights = []FontWeight{
	InvisibleFontWeight,
	ThinFontWeight,
	ExtraLightFontWeight,
	LightFontWeight,
	NormalFontWeight,
	MediumFontWeight,
	SemiBoldFontWeight,
	BoldFontWeight,
	ExtraBoldFontWeight,
	BlackFontWeight,
	ExtraBlackFontWeight,
}

// MarshalText implements the encoding.TextMarshaler interface.
func (w FontWeight) MarshalText() (text []byte, err error) {
	return []byte(strings.ToLower(w.String())), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (w *FontWeight) UnmarshalText(text []byte) error {
	*w = WeightFromString(string(text))
	return nil
}

// WeightFromString extracts the FontWeight from a string.
func WeightFromString(str string) FontWeight {
	if str == "" {
		return NormalFontWeight
	}
	for w := InvisibleFontWeight; w <= ExtraBlackFontWeight; w += 100 {
		if strings.EqualFold(w.String(), str) {
			return w
		}
	}
	return NormalFontWeight
}

func (w FontWeight) String() string {
	switch w {
	case InvisibleFontWeight:
		return "invisible"
	case ThinFontWeight:
		return "thin"
	case ExtraLightFontWeight:
		return "extra-light"
	case LightFontWeight:
		return "light"
	case MediumFontWeight:
		return "medium"
	case SemiBoldFontWeight:
		return "semi-bold"
	case BoldFontWeight:
		return "bold"
	case ExtraBoldFontWeight:
		return "extra-bold"
	case BlackFontWeight:
		return "black"
	case ExtraBlackFontWeight:
		return "extra-black"
	default:
		return "regular"
	}
}

// Localized returns the localized name.
func (w FontWeight) Localized() string {
	switch w {
	case InvisibleFontWeight:
		return i18n.Text("Invisible")
	case ThinFontWeight:
		return i18n.Text("Thin")
	case ExtraLightFontWeight:
		return i18n.Text("Extra-Light")
	case LightFontWeight:
		return i18n.Text("Light")
	case MediumFontWeight:
		return i18n.Text("Medium")
	case SemiBoldFontWeight:
		return i18n.Text("Semi-Bold")
	case BoldFontWeight:
		return i18n.Text("Bold")
	case ExtraBoldFontWeight:
		return i18n.Text("Extra-Bold")
	case BlackFontWeight:
		return i18n.Text("Black")
	case ExtraBlackFontWeight:
		return i18n.Text("Extra-Black")
	default:
		return i18n.Text("Regular")
	}
}
