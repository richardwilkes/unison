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
	"fmt"
	"strings"

	"github.com/richardwilkes/toolbox/errs"
	"github.com/richardwilkes/toolbox/txt"
)

// FontFaceDescriptor holds information necessary to construct a FontFace.
type FontFaceDescriptor struct {
	Family  string      `json:"family"`
	Weight  FontWeight  `json:"weight"`
	Spacing FontSpacing `json:"spacing"`
	Slant   FontSlant   `json:"slant"`
}

// Face returns the matching FontFace, if any.
func (ffd FontFaceDescriptor) Face() *FontFace {
	return MatchFontFace(ffd.Family, ffd.Weight, ffd.Spacing, ffd.Slant)
}

func (ffd FontFaceDescriptor) variants() string {
	variants := make([]string, 0, 3)
	if ffd.Weight != NormalFontWeight {
		variants = append(variants, ffd.Weight.String())
	}
	if ffd.Spacing != StandardSpacing {
		variants = append(variants, ffd.Spacing.String())
	}
	if ffd.Slant != NoSlant {
		variants = append(variants, ffd.Slant.String())
	}
	if len(variants) != 0 {
		return fmt.Sprintf(" (%s)", strings.Join(variants, ", "))
	}
	return ""
}

// String this returns a string suitable for display. It is not suitable for converting back into a FontFaceDescriptor.
func (ffd FontFaceDescriptor) String() string {
	return ffd.Family + ffd.variants()
}

// MarshalText implements the encoding.TextMarshaler interface.
func (ffd FontFaceDescriptor) MarshalText() (text []byte, err error) {
	return []byte(fmt.Sprintf("%s %s %s %s", ffd.Family, ffd.Weight.Key(), ffd.Spacing.Key(), ffd.Slant.Key())), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (ffd *FontFaceDescriptor) UnmarshalText(text []byte) error {
	parts := strings.Split(txt.CollapseSpaces(string(text)), " ")
	if len(parts) < 4 {
		return errs.Newf("invalid font face descriptor: %s", string(text))
	}
	ffd.Slant = SlantFromString(parts[len(parts)-1])
	ffd.Spacing = SpacingFromString(parts[len(parts)-2])
	ffd.Weight = WeightFromString(parts[len(parts)-3])
	ffd.Family = strings.Join(parts[:len(parts)-3], " ")
	return nil
}
