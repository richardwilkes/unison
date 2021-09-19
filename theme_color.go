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
	"github.com/richardwilkes/toolbox/xmath/geom32"
)

var (
	_ ColorProvider = &ThemeColor{}
	_ Ink           = &ThemeColor{}
)

// Pre-defined theme colors.
var (
	BackgroundColor          = &ThemeColor{Light: RGB(238, 238, 238), Dark: RGB(50, 50, 50)}
	OnBackgroundColor        = &ThemeColor{Light: Black, Dark: RGB(221, 221, 221)}
	DividerColor             = &ThemeColor{Light: RGB(192, 192, 192), Dark: RGB(80, 80, 80)}
	ControlColor             = &ThemeColor{Light: RGB(248, 248, 255), Dark: RGB(64, 64, 64)}
	OnControlColor           = &ThemeColor{Light: Black, Dark: RGB(221, 221, 221)}
	ControlEdgeColor         = &ThemeColor{Light: RGB(96, 96, 96), Dark: RGB(96, 96, 96)}
	ControlPressedColor      = &ThemeColor{Light: RGB(0, 80, 136), Dark: RGB(0, 80, 136)}
	OnControlPressedColor    = &ThemeColor{Light: White, Dark: White}
	ControlDisabledColor     = &ThemeColor{Light: RGB(232, 232, 240), Dark: RGB(56, 56, 56)}
	OnControlDisabledColor   = &ThemeColor{Light: RGB(184, 184, 184), Dark: RGB(128, 128, 128)}
	SelectionColor           = &ThemeColor{Light: RGB(0, 96, 160), Dark: RGB(0, 96, 160)}
	OnSelectionColor         = &ThemeColor{Light: White, Dark: White}
	InactiveSelectionColor   = &ThemeColor{Light: RGB(0, 64, 148), Dark: RGB(0, 64, 148)}
	OnInactiveSelectionColor = &ThemeColor{Light: RGB(228, 228, 228), Dark: RGB(228, 228, 228)}
	ListColor                = &ThemeColor{Light: RGB(235, 235, 220), Dark: RGB(42, 42, 42)}
	OnListColor              = &ThemeColor{Light: Black, Dark: RGB(221, 221, 221)}
	ListAltColor             = &ThemeColor{Light: White, Dark: RGB(50, 50, 50)}
	OnListAltColor           = &ThemeColor{Light: Black, Dark: RGB(221, 221, 221)}
	EditableColor            = &ThemeColor{Light: White, Dark: RGB(24, 24, 24)}
	OnEditableColor          = &ThemeColor{Light: Black, Dark: RGB(221, 221, 221)}
	EditableErrorColor       = &ThemeColor{Light: RGB(192, 64, 64), Dark: RGB(115, 37, 37)}
	OnEditableErrorColor     = &ThemeColor{Light: White, Dark: RGB(221, 221, 221)}
	TooltipColor             = &ThemeColor{Light: RGB(252, 252, 196), Dark: RGB(192, 192, 130)}
	OnTooltipColor           = &ThemeColor{Light: Black, Dark: RGB(32, 32, 32)}
	ScrollColor              = &ThemeColor{Light: ARGB(0.5, 192, 192, 192), Dark: ARGB(0.5, 128, 128, 128)}
	ScrollRolloverColor      = &ThemeColor{Light: RGB(192, 192, 192), Dark: RGB(128, 128, 128)}
	ScrollEdgeColor          = &ThemeColor{Light: RGB(102, 102, 102), Dark: RGB(153, 153, 153)}
	BlackWhenDarkColor       = &ThemeColor{Light: White, Dark: Black}
	WhiteWhenDarkColor       = &ThemeColor{Light: Black, Dark: White}
)

// ThemeColor holds a pair of colors, one for light mode and one for dark mode.
type ThemeColor struct {
	Light Color
	Dark  Color
}

// GetColor returns the current color. Here to satisfy the ColorProvider interface.
func (t *ThemeColor) GetColor() Color {
	if IsDarkModeEnabled() {
		return t.Dark
	}
	return t.Light
}

// Paint returns a Paint for this ThemeColor. Here to satisfy the Ink interface.
func (t *ThemeColor) Paint(canvas *Canvas, rect geom32.Rect, style PaintStyle) *Paint {
	return t.GetColor().Paint(canvas, rect, style)
}
