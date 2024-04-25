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

// Pre-defined theme colors.
var (
	AccentColor              = &ThemeColor{Light: RGB(0, 128, 128), Dark: RGB(100, 153, 153)}
	BackgroundColor          = &ThemeColor{Light: RGB(238, 238, 238), Dark: RGB(48, 48, 48)}
	BandingColor             = &ThemeColor{Light: RGB(235, 235, 220), Dark: RGB(42, 42, 42)}
	ContentColor             = &ThemeColor{Light: RGB(248, 248, 248), Dark: RGB(32, 32, 32)}
	ControlColor             = &ThemeColor{Light: RGB(248, 248, 255), Dark: RGB(64, 64, 64)}
	ControlEdgeColor         = &ThemeColor{Light: RGB(96, 96, 96), Dark: RGB(96, 96, 96)}
	ControlPressedColor      = &ThemeColor{Light: RGB(0, 96, 160), Dark: RGB(0, 96, 160)}
	DividerColor             = &ThemeColor{Light: RGB(192, 192, 192), Dark: RGB(102, 102, 102)}
	DropAreaColor            = &ThemeColor{Light: RGB(204, 0, 51), Dark: RGB(255, 0, 0)}
	EditableColor            = &ThemeColor{Light: RGB(255, 255, 255), Dark: RGB(16, 16, 16)}
	ErrorColor               = &ThemeColor{Light: RGB(192, 64, 64), Dark: RGB(115, 37, 37)}
	IconButtonColor          = &ThemeColor{Light: RGB(96, 96, 96), Dark: RGB(128, 128, 128)}
	IconButtonPressedColor   = &ThemeColor{Light: RGB(0, 96, 160), Dark: RGB(0, 96, 160)}
	IconButtonRolloverColor  = &ThemeColor{Light: RGB(0, 0, 0), Dark: RGB(192, 192, 192)}
	InactiveSelectionColor   = &ThemeColor{Light: RGB(0, 64, 128), Dark: RGB(0, 64, 128)}
	IndirectSelectionColor   = &ThemeColor{Light: RGB(230, 247, 255), Dark: RGB(0, 43, 64)}
	InteriorDividerColor     = &ThemeColor{Light: RGB(216, 216, 216), Dark: RGB(53, 53, 53)}
	LinkColor                = &ThemeColor{Light: RGB(115, 153, 37), Dark: RGB(0, 204, 102)}
	LinkPressedColor         = &ThemeColor{Light: RGB(0, 128, 255), Dark: RGB(0, 96, 160)}
	LinkRolloverColor        = &ThemeColor{Light: RGB(0, 192, 0), Dark: RGB(0, 179, 0)}
	OnBackgroundColor        = &ThemeColor{Light: RGB(0, 0, 0), Dark: RGB(221, 221, 221)}
	OnBandingColor           = &ThemeColor{Light: RGB(0, 0, 0), Dark: RGB(221, 221, 221)}
	OnContentColor           = &ThemeColor{Light: RGB(0, 0, 0), Dark: RGB(221, 221, 221)}
	OnControlColor           = &ThemeColor{Light: RGB(0, 0, 0), Dark: RGB(221, 221, 221)}
	OnControlPressedColor    = &ThemeColor{Light: RGB(255, 255, 255), Dark: RGB(255, 255, 255)}
	OnEditableColor          = &ThemeColor{Light: RGB(0, 0, 160), Dark: RGB(100, 153, 153)}
	OnErrorColor             = &ThemeColor{Light: RGB(255, 255, 255), Dark: RGB(221, 221, 221)}
	OnInactiveSelectionColor = &ThemeColor{Light: RGB(228, 228, 228), Dark: RGB(228, 228, 228)}
	OnIndirectSelectionColor = &ThemeColor{Light: RGB(0, 0, 0), Dark: RGB(228, 228, 228)}
	OnSelectionColor         = &ThemeColor{Light: RGB(255, 255, 255), Dark: RGB(255, 255, 255)}
	OnTabCurrentColor        = &ThemeColor{Light: RGB(0, 0, 0), Dark: RGB(221, 221, 221)}
	OnTabFocusedColor        = &ThemeColor{Light: RGB(0, 0, 0), Dark: RGB(221, 221, 221)}
	OnTooltipColor           = &ThemeColor{Light: RGB(0, 0, 0), Dark: RGB(0, 0, 0)}
	OnWarningColor           = &ThemeColor{Light: RGB(255, 255, 255), Dark: RGB(221, 221, 221)}
	ScrollColor              = &ThemeColor{Light: ARGB(0.5, 192, 192, 192), Dark: ARGB(0.5, 128, 128, 128)}
	ScrollEdgeColor          = &ThemeColor{Light: RGB(128, 128, 128), Dark: RGB(160, 160, 160)}
	ScrollRolloverColor      = &ThemeColor{Light: RGB(192, 192, 192), Dark: RGB(128, 128, 128)}
	SelectionColor           = &ThemeColor{Light: RGB(0, 96, 160), Dark: RGB(0, 96, 160)}
	TabCurrentColor          = &ThemeColor{Light: RGB(211, 207, 197), Dark: RGB(41, 61, 0)}
	TabFocusedColor          = &ThemeColor{Light: RGB(224, 212, 175), Dark: RGB(68, 102, 0)}
	TooltipColor             = &ThemeColor{Light: RGB(252, 252, 196), Dark: RGB(252, 252, 196)}
	WarningColor             = &ThemeColor{Light: RGB(224, 128, 0), Dark: RGB(192, 96, 0)}
)

var (
	ColorSurface          = &ThemeColor{Light: RGB(238, 239, 227), Dark: RGB(30, 32, 26)}
	ColorOnSurface        = &ThemeColor{Light: RGB(26, 28, 22), Dark: RGB(226, 227, 216)}
	ColorSurfaceVariant   = &ThemeColor{Light: RGB(249, 250, 239), Dark: RGB(18, 20, 14)}
	ColorOnSurfaceVariant = &ThemeColor{Light: RGB(68, 72, 61), Dark: RGB(197, 200, 186)}
	ColorPrimary          = &ThemeColor{Light: RGB(76, 102, 43), Dark: RGB(177, 209, 138)}
	ColorOnPrimary        = &ThemeColor{Light: White, Dark: RGB(42, 51, 30)}
	ColorSecondary        = &ThemeColor{Light: RGB(88, 98, 73), Dark: RGB(191, 203, 173)}
	ColorOnSecondary      = &ThemeColor{Light: White, Dark: RGB(42, 51, 30)}
	ColorTertiary         = &ThemeColor{Light: RGB(56, 102, 99), Dark: RGB(160, 208, 203)}
	ColorOnTertiary       = &ThemeColor{Light: White, Dark: RGB(0, 55, 53)}
	ColorTertiaryFixed    = &ThemeColor{Light: RGB(188, 236, 231), Dark: RGB(188, 236, 231)}
	ColorOnTertiaryFixed  = &ThemeColor{Light: RGB(0, 32, 30), Dark: RGB(0, 32, 30)}
	ColorHeader           = &ThemeColor{Light: RGB(66, 67, 57), Dark: RGB(49, 52, 45)}
	ColorOnHeader         = &ThemeColor{Light: RGB(222, 223, 211), Dark: RGB(128, 129, 119)}
	ColorEditable         = &ThemeColor{Light: White, Dark: Black}
	ColorOnEditable       = &ThemeColor{Light: Black, Dark: White}
	ColorWarning          = &ThemeColor{Light: RGB(224, 128, 0), Dark: RGB(192, 96, 0)}
	ColorOnWarning        = &ThemeColor{Light: White, Dark: RGB(221, 221, 221)}
	ColorError            = &ThemeColor{Light: RGB(186, 26, 26), Dark: RGB(255, 180, 171)}
	ColorOnError          = &ThemeColor{Light: White, Dark: RGB(105, 0, 5)}
	ColorBackground       = &ThemeColor{Light: RGB(249, 250, 239), Dark: RGB(26, 28, 22)}
	ColorShadow           = &ThemeColor{Light: Black, Dark: Black}
	ColorOutline          = &ThemeColor{Light: RGB(117, 121, 108), Dark: RGB(143, 146, 133)}
	ColorOutlineVariant   = &ThemeColor{Light: RGB(197, 200, 186), Dark: RGB(68, 72, 61)}
)

func newThemeColor(light, dark string) *ThemeColor {
	return &ThemeColor{Light: MustColorDecode(light), Dark: MustColorDecode(dark)}
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
