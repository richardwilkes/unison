// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import "github.com/richardwilkes/unison/enums/paintstyle"

var _ ColorProvider = &ThemeColor{}

// PrimaryTheme is the default theme.
var PrimaryTheme = NewDefaultTheme()

// Theme holds a set of colors for a theme.
type Theme struct {
	Primary        ThemeColor `json:"primary"`
	OnPrimary      ThemeColor `json:"on_primary"`
	Secondary      ThemeColor `json:"secondary"`
	OnSecondary    ThemeColor `json:"on_secondary"`
	Tertiary       ThemeColor `json:"tertiary"`
	OnTertiary     ThemeColor `json:"on_tertiary"`
	Surface        ThemeColor `json:"surface"`
	OnSurface      ThemeColor `json:"on_surface"`
	SurfaceAbove   ThemeColor `json:"surface_above"`
	OnSurfaceAbove ThemeColor `json:"on_surface_above"`
	SurfaceBelow   ThemeColor `json:"surface_below"`
	OnSurfaceBelow ThemeColor `json:"on_surface_below"`
	Error          ThemeColor `json:"error"`
	OnError        ThemeColor `json:"on_error"`
	Warning        ThemeColor `json:"warning"`
	OnWarning      ThemeColor `json:"on_warning"`
	Outline        ThemeColor `json:"outline"`
	OutlineVariant ThemeColor `json:"outline_variant"`
	Shadow         ThemeColor `json:"shadow"`
}

// ThemeColor holds a pair of colors, one for light mode and one for dark mode.
type ThemeColor struct {
	Light Color `json:"light"`
	Dark  Color `json:"dark"`
}

// NewDefaultTheme returns a new Theme with default colors.
func NewDefaultTheme() Theme {
	return NewThemeFromPalettes(BluePalette, IndigoPalette, GreenPalette)
}

// NewThemeFromPalettes returns a new Theme created with the specified palettes.
func NewThemeFromPalettes(primary, secondary, tertiary Palette) Theme {
	darkSurface := RGB(48, 48, 48)
	t := Theme{
		Primary:        ThemeColor{Light: primary[7], Dark: primary[7]},
		Secondary:      ThemeColor{Light: secondary[7], Dark: secondary[7]},
		Tertiary:       ThemeColor{Light: tertiary[7], Dark: tertiary[7]},
		Surface:        ThemeColor{Light: GreyPalette[3], Dark: darkSurface},
		SurfaceAbove:   ThemeColor{Light: GreyPalette[4], Dark: GreyPalette[8]},
		SurfaceBelow:   ThemeColor{Light: GreyPalette[2], Dark: GreyPalette[9]},
		Error:          ThemeColor{Light: RedPalette[7], Dark: RedPalette[9]},
		Warning:        ThemeColor{Light: OrangePalette[8], Dark: OrangePalette[9]},
		Outline:        ThemeColor{Light: GreyPalette[5], Dark: GreyPalette[7]},
		OutlineVariant: ThemeColor{Light: GreyPalette[4], Dark: GreyPalette[8]},
		Shadow:         ThemeColor{Light: Black, Dark: Black},
	}
	t.OnPrimary = newOn(t.Primary)
	t.OnSecondary = newOn(t.Secondary)
	t.OnTertiary = newOn(t.Tertiary)
	t.OnSurface = newOn(t.Surface)
	t.OnSurfaceAbove = newOn(t.SurfaceAbove)
	t.OnSurfaceBelow = newOn(t.SurfaceBelow)
	t.OnError = newOn(t.Error)
	t.OnWarning = newOn(t.Warning)
	return t
}

func newOn(color ThemeColor) ThemeColor {
	return ThemeColor{Light: OnColor(color.Light), Dark: OnColor(color.Dark)}
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

// Pre-defined theme colors.
//
// Deprecated: Use PrimaryTheme instead. These variables were deprecated on April 28, 2024 and will be removed on or
// after January 1, 2025.
var (
	AccentColor              = &PrimaryTheme.Tertiary
	BackgroundColor          = &PrimaryTheme.Surface
	BandingColor             = &PrimaryTheme.SurfaceBelow
	ContentColor             = &PrimaryTheme.Surface
	ControlColor             = &PrimaryTheme.SurfaceAbove
	ControlEdgeColor         = &PrimaryTheme.Outline
	ControlPressedColor      = &PrimaryTheme.Primary
	DividerColor             = &PrimaryTheme.Outline
	DropAreaColor            = &PrimaryTheme.Warning
	EditableColor            = &PrimaryTheme.SurfaceBelow
	ErrorColor               = &PrimaryTheme.Error
	IconButtonColor          = &PrimaryTheme.SurfaceAbove
	IconButtonPressedColor   = &PrimaryTheme.Primary
	IconButtonRolloverColor  = &PrimaryTheme.SurfaceAbove
	InactiveSelectionColor   = &PrimaryTheme.Secondary
	IndirectSelectionColor   = &PrimaryTheme.Secondary
	InteriorDividerColor     = &PrimaryTheme.OutlineVariant
	LinkColor                = &PrimaryTheme.SurfaceAbove
	LinkPressedColor         = &PrimaryTheme.Secondary
	LinkRolloverColor        = &PrimaryTheme.Secondary
	OnBackgroundColor        = &PrimaryTheme.OnSurface
	OnBandingColor           = &PrimaryTheme.OnSurfaceBelow
	OnContentColor           = &PrimaryTheme.OnSurface
	OnControlColor           = &PrimaryTheme.OnSurfaceAbove
	OnControlPressedColor    = &PrimaryTheme.OnPrimary
	OnEditableColor          = &PrimaryTheme.OnSurfaceBelow
	OnErrorColor             = &PrimaryTheme.OnError
	OnInactiveSelectionColor = &PrimaryTheme.OnSecondary
	OnIndirectSelectionColor = &PrimaryTheme.OnSecondary
	OnSelectionColor         = &PrimaryTheme.OnPrimary
	OnTabCurrentColor        = &PrimaryTheme.OnSecondary
	OnTabFocusedColor        = &PrimaryTheme.OnPrimary
	OnTooltipColor           = &PrimaryTheme.OnSurfaceAbove
	OnWarningColor           = &PrimaryTheme.OnWarning
	ScrollColor              = &PrimaryTheme.Primary
	ScrollEdgeColor          = &PrimaryTheme.Outline
	ScrollRolloverColor      = &PrimaryTheme.Primary
	SelectionColor           = &PrimaryTheme.Primary
	TabCurrentColor          = &PrimaryTheme.Secondary
	TabFocusedColor          = &PrimaryTheme.Primary
	TooltipColor             = &PrimaryTheme.SurfaceAbove
	WarningColor             = &PrimaryTheme.Warning
)
