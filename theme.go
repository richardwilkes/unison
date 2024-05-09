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

var (
	_ ColorProvider = &ThemeColor{}
	_ ColorProvider = &DerivedThemeColor{}
)

// The theme colors used throughout Unison.
var (
	ThemeSurface = DefaultThemeSurface()
	ThemeFocus   = DefaultThemeFocus()
	ThemeTooltip = DefaultThemeTooltip()
	ThemeError   = DefaultThemeError()
	ThemeWarning = DefaultThemeWarning()
)

// The colors derived from the theme colors.
var (
	ThemeOnSurface      = ThemeSurface.DeriveOn()
	ThemeAboveSurface   = ThemeSurface.DeriveLightness(-0.05, 0.1)
	ThemeOnAboveSurface = ThemeAboveSurface.DeriveOn()
	ThemeBelowSurface   = ThemeSurface.DeriveLightness(0.05, -0.1)
	ThemeOnBelowSurface = ThemeBelowSurface.DeriveOn()
	ThemeSurfaceEdge    = ThemeSurface.DeriveLightness(-0.1, 0.15)
	ThemeOnFocus        = ThemeFocus.DeriveOn()
	ThemeDeepFocus      = ThemeFocus.DeriveLightness(-0.05, -0.1)
	ThemeOnDeepFocus    = ThemeDeepFocus.DeriveOn()
	ThemeDeeperFocus    = ThemeFocus.DeriveLightness(-0.1, -0.15)
	ThemeOnDeeperFocus  = ThemeDeeperFocus.DeriveOn()
	ThemeDeepestFocus   = ThemeFocus.DeriveLightness(-0.15, -0.2)
	ThemeOnDeepestFocus = ThemeDeepestFocus.DeriveOn()
	ThemeOnTooltip      = ThemeTooltip.DeriveOn()
	ThemeTooltipEdge    = ThemeTooltip.DeriveLightness(-0.2, -0.2)
	ThemeOnError        = ThemeError.DeriveOn()
	ThemeOnWarning      = ThemeWarning.DeriveOn()
)

// DefaultThemeSurface returns the default surface color.
func DefaultThemeSurface() *ThemeColor {
	return &ThemeColor{Light: RGB(224, 224, 224), Dark: RGB(32, 32, 32)}
}

// DefaultThemeFocus returns the default focus color.
func DefaultThemeFocus() *ThemeColor {
	return &ThemeColor{Light: RGB(0, 97, 153), Dark: RGB(0, 128, 204)}
}

// DefaultThemeTooltip returns the default tooltip color.
func DefaultThemeTooltip() *ThemeColor {
	return &ThemeColor{Light: RGB(255, 244, 198), Dark: RGB(255, 242, 153)}
}

// DefaultThemeError returns the default error color.
func DefaultThemeError() *ThemeColor {
	return &ThemeColor{Light: RGB(133, 20, 20), Dark: RGB(133, 20, 20)}
}

// DefaultThemeWarning returns the default warning color.
func DefaultThemeWarning() *ThemeColor {
	return &ThemeColor{Light: RGB(217, 76, 0), Dark: RGB(191, 67, 0)}
}

// ThemeColor holds a pair of colors, one for light mode and one for dark mode.
type ThemeColor struct {
	Light Color `json:"light"`
	Dark  Color `json:"dark"`
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

// Derive returns a new DerivedThemeColor that is derived from this ThemeColor.
func (t *ThemeColor) Derive(deriver func(ThemeColor) ThemeColor) *DerivedThemeColor {
	return &DerivedThemeColor{
		derived:      deriver(*t),
		deriveFunc:   deriver,
		lastSeen:     *t,
		lastSeenFunc: func() ThemeColor { return *t },
	}
}

// DeriveOn returns a new DerivedThemeColor that is the On color for this ThemeColor.
func (t *ThemeColor) DeriveOn() *DerivedThemeColor {
	return t.Derive(DeriveOn)
}

// DeriveOn returns a new ThemeColor that is the On color for the passed in ThemeColor.
func DeriveOn(basedOn ThemeColor) ThemeColor {
	return ThemeColor{
		Light: basedOn.Light.On(),
		Dark:  basedOn.Dark.On(),
	}
}

// DeriveLightness returns a new DerivedThemeColor that has its lightness adjusted by the given amount.
func (t *ThemeColor) DeriveLightness(light, dark float32) *DerivedThemeColor {
	return t.Derive(CreateDeriveLightnessFunc(light, dark))
}

// CreateDeriveLightnessFunc returns a function that will adjust the lightness of a ThemeColor by the given amount.
func CreateDeriveLightnessFunc(light, dark float32) func(ThemeColor) ThemeColor {
	return func(basedOn ThemeColor) ThemeColor {
		return ThemeColor{
			Light: basedOn.Light.AdjustPerceivedLightness(light),
			Dark:  basedOn.Dark.AdjustPerceivedLightness(dark),
		}
	}
}

// DerivedThemeColor holds a ThemeColor that is derived from another ThemeColor.
type DerivedThemeColor struct {
	derived      ThemeColor
	deriveFunc   func(ThemeColor) ThemeColor
	lastSeen     ThemeColor
	lastSeenFunc func() ThemeColor
}

// GetColor returns the current color. Here to satisfy the ColorProvider interface.
func (t *DerivedThemeColor) GetColor() Color {
	lastSeen := t.lastSeenFunc()
	if t.lastSeen != lastSeen {
		t.lastSeen = lastSeen
		t.derived = t.deriveFunc(lastSeen)
	}
	return t.derived.GetColor()
}

// Paint returns a Paint for this ThemeColor. Here to satisfy the Ink interface.
func (t *DerivedThemeColor) Paint(canvas *Canvas, rect Rect, style paintstyle.Enum) *Paint {
	return t.GetColor().Paint(canvas, rect, style)
}

// Derive returns a new DerivedThemeColor that is derived from this DerivedThemeColor.
func (t *DerivedThemeColor) Derive(deriver func(ThemeColor) ThemeColor) *DerivedThemeColor {
	t.GetColor() // Ensure we have the latest colors
	return &DerivedThemeColor{
		derived:    deriver(t.derived),
		deriveFunc: deriver,
		lastSeen:   t.derived,
		lastSeenFunc: func() ThemeColor {
			t.GetColor() // Ensure we have the latest colors
			return t.derived
		},
	}
}

// DeriveOn returns a new DerivedThemeColor that is the On color for this DerivedThemeColor.
func (t *DerivedThemeColor) DeriveOn() *DerivedThemeColor {
	return t.Derive(DeriveOn)
}

// DeriveLightness returns a new DerivedThemeColor that has its lightness adjusted by the given amount.
func (t *DerivedThemeColor) DeriveLightness(light, dark float32) *DerivedThemeColor {
	return t.Derive(CreateDeriveLightnessFunc(light, dark))
}
