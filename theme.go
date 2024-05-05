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
	"github.com/richardwilkes/unison/enums/paintstyle"
)

var _ ColorProvider = &ThemeColor{}

// PrimaryTheme is the default theme.
var PrimaryTheme = NewDefaultTheme()

// Theme holds a set of colors for a theme.
type Theme struct {
	Primary          ThemeColor `json:"primary"`
	OnPrimary        ThemeColor `json:"on_primary"`
	PrimaryVariant   ThemeColor `json:"primary_variant"`
	Secondary        ThemeColor `json:"secondary"`
	OnSecondary      ThemeColor `json:"on_secondary"`
	SecondaryVariant ThemeColor `json:"secondary_variant"`
	Tertiary         ThemeColor `json:"tertiary"`
	OnTertiary       ThemeColor `json:"on_tertiary"`
	TertiaryVariant  ThemeColor `json:"tertiary_variant"`
	Surface          ThemeColor `json:"surface"`
	OnSurface        ThemeColor `json:"on_surface"`
	SurfaceAbove     ThemeColor `json:"surface_above"`
	SurfaceBelow     ThemeColor `json:"surface_below"`
	Error            ThemeColor `json:"error"`
	OnError          ThemeColor `json:"on_error"`
	Warning          ThemeColor `json:"warning"`
	OnWarning        ThemeColor `json:"on_warning"`
	Outline          ThemeColor `json:"outline"`
	OutlineVariant   ThemeColor `json:"outline_variant"`
	Shadow           ThemeColor `json:"shadow"`
}

// ThemeColor holds a pair of colors, one for light mode and one for dark mode.
type ThemeColor struct {
	Light Color `json:"light"`
	Dark  Color `json:"dark"`
}

// NewDefaultTheme returns a new Theme with default colors.
func NewDefaultTheme() Theme {
	return Theme{
		Primary:          ThemeColor{Light: RGB(0, 128, 204), Dark: RGB(0, 128, 204)},
		OnPrimary:        ThemeColor{Light: RGB(224, 224, 224), Dark: RGB(224, 224, 224)},
		PrimaryVariant:   ThemeColor{Light: RGB(0, 97, 153), Dark: RGB(0, 97, 153)},
		Secondary:        ThemeColor{Light: RGB(61, 97, 153), Dark: RGB(61, 97, 153)},
		OnSecondary:      ThemeColor{Light: RGB(224, 224, 224), Dark: RGB(224, 224, 224)},
		SecondaryVariant: ThemeColor{Light: RGB(41, 65, 102), Dark: RGB(41, 65, 102)},
		Tertiary:         ThemeColor{Light: RGB(56, 142, 60), Dark: RGB(139, 195, 74)},
		OnTertiary:       ThemeColor{Light: RGB(224, 224, 224), Dark: RGB(224, 224, 224)},
		TertiaryVariant:  ThemeColor{Light: RGB(51, 105, 30), Dark: RGB(99, 143, 55)},
		Surface:          ThemeColor{Light: RGB(224, 224, 224), Dark: RGB(32, 32, 32)},
		OnSurface:        ThemeColor{Light: RGB(32, 32, 32), Dark: RGB(224, 224, 224)},
		SurfaceAbove:     ThemeColor{Light: RGB(208, 208, 208), Dark: RGB(56, 56, 56)},
		SurfaceBelow:     ThemeColor{Light: RGB(240, 240, 240), Dark: RGB(16, 16, 16)},
		Error:            ThemeColor{Light: RGB(133, 20, 20), Dark: RGB(133, 20, 20)},
		OnError:          ThemeColor{Light: RGB(224, 224, 224), Dark: RGB(224, 224, 224)},
		Warning:          ThemeColor{Light: RGB(217, 76, 0), Dark: RGB(191, 67, 0)},
		OnWarning:        ThemeColor{Light: RGB(224, 224, 224), Dark: RGB(224, 224, 224)},
		Outline:          ThemeColor{Light: RGB(192, 192, 192), Dark: RGB(64, 64, 64)},
		OutlineVariant:   ThemeColor{Light: RGB(208, 208, 208), Dark: RGB(56, 56, 56)},
		Shadow:           ThemeColor{Light: RGB(0, 0, 0), Dark: RGB(0, 0, 0)},
	}
}

// GetColor returns the current color. Here to satisfy the ColorProvider interface.
func (t *ThemeColor) GetColor() Color {
	if IsDarkModeEnabled() {
		return t.Dark
	}
	return t.Light
}

// Paint returns a Paint for this ThemeColor. Here to satisfy the Ink interface.
func (t *ThemeColor) Paint(canvas *Canvas, rect Rect, style paintstyle.Enum) *Paint {
	return t.GetColor().Paint(canvas, rect, style)
}
