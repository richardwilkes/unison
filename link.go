// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"github.com/richardwilkes/toolbox"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/paintstyle"
	"github.com/richardwilkes/unison/enums/side"
)

// DefaultLinkTheme holds the default Link theme values.
var DefaultLinkTheme = LinkTheme{
	LabelTheme: LabelTheme{
		TextDecoration: TextDecoration{
			Font:            LabelFont,
			OnBackgroundInk: ThemeFocus,
			Underline:       true,
		},
		Gap:    StdIconGap,
		HAlign: align.Start,
		VAlign: align.Middle,
		Side:   side.Left,
	},
	PressedInk:   ThemeFocus,
	OnPressedInk: ThemeOnFocus,
}

// LinkTheme holds theming data for a link.
type LinkTheme struct {
	PressedInk   Ink
	OnPressedInk Ink
	LabelTheme
}

// NewLink creates a new RichLabel that can be used as a hyperlink.
func NewLink(title, tooltip, target string, theme LinkTheme, clickHandler func(Paneler, string)) *Label { //nolint:gocritic // OK for theme to be passed by value here
	link := NewLabel()
	link.LabelTheme = theme.LabelTheme
	link.SetTitle(title)
	if tooltip != "" {
		link.Tooltip = NewTooltipWithText(tooltip)
	}
	link.UpdateCursorCallback = func(_ Point) *Cursor {
		if link.Enabled() {
			return PointingCursor()
		}
		return ArrowCursor()
	}
	mouseDown := false
	link.MouseDownCallback = func(_ Point, _, _ int, _ Modifiers) bool {
		mouseDown = true
		link.MarkForRedraw()
		return true
	}
	link.MouseDragCallback = func(where Point, _ int, _ Modifiers) bool {
		now := where.In(link.ContentRect(true))
		if now != mouseDown {
			mouseDown = now
			link.MarkForRedraw()
		}
		return true
	}
	link.MouseUpCallback = func(where Point, _ int, _ Modifiers) bool {
		link.MarkForRedraw()
		if where.In(link.ContentRect(true)) && clickHandler != nil {
			toolbox.Call(func() { clickHandler(link, target) })
		}
		mouseDown = false
		return true
	}
	link.DrawCallback = func(gc *Canvas, rect Rect) {
		if mouseDown {
			defer link.Text.RestoreDecorations(link.Text.AdjustDecorations(func(decoration *TextDecoration) {
				decoration.OnBackgroundInk = theme.OnPressedInk
			}))
			gc.DrawRect(rect, theme.PressedInk.Paint(gc, rect, paintstyle.Fill))
		}
		link.DefaultDraw(gc, rect)
	}
	return link
}
