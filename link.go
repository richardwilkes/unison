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
	"github.com/richardwilkes/toolbox"
	"github.com/richardwilkes/unison/enums/paintstyle"
)

// DefaultLinkTheme holds the default Link theme values.
var DefaultLinkTheme = LinkTheme{
	TextDecoration: TextDecoration{
		Font:       LabelFont,
		Foreground: ThemeFocus,
		Underline:  true,
	},
	OnPressedInk: ThemeOnFocus,
}

// LinkTheme holds theming data for a link.
type LinkTheme struct {
	TextDecoration
	OnPressedInk Ink
}

// NewLink creates a new RichLabel that can be used as a hyperlink.
func NewLink(title, tooltip, target string, theme LinkTheme, clickHandler func(Paneler, string)) *RichLabel {
	link := NewRichLabel()
	link.Text = NewText(title, &theme.TextDecoration)
	link.OnBackgroundInk = theme.Foreground
	if tooltip != "" {
		link.Tooltip = NewTooltipWithText(tooltip)
	}
	link.UpdateCursorCallback = func(_ Point) *Cursor {
		if link.Enabled() {
			return PointingCursor()
		}
		return ArrowCursor()
	}
	mouseDownIn := false
	link.MouseDownCallback = func(_ Point, _, _ int, _ Modifiers) bool {
		link.Text.AdjustDecorations(func(decoration *TextDecoration) {
			decoration.Foreground = theme.OnPressedInk
		})
		link.OnBackgroundInk = theme.OnPressedInk
		link.MarkForRedraw()
		mouseDownIn = true
		return true
	}
	link.MouseDragCallback = func(where Point, _ int, _ Modifiers) bool {
		now := where.In(link.ContentRect(true))
		if now != mouseDownIn {
			mouseDownIn = now
			link.Text.AdjustDecorations(func(decoration *TextDecoration) {
				if mouseDownIn {
					decoration.Foreground = theme.OnPressedInk
				} else {
					decoration.Foreground = theme.Foreground
				}
			})
			if mouseDownIn {
				link.OnBackgroundInk = theme.OnPressedInk
			} else {
				link.OnBackgroundInk = theme.Foreground
			}
			link.MarkForRedraw()
		}
		return true
	}
	link.MouseUpCallback = func(where Point, _ int, _ Modifiers) bool {
		link.Text.AdjustDecorations(func(decoration *TextDecoration) {
			decoration.Foreground = theme.Foreground
		})
		link.OnBackgroundInk = theme.Foreground
		link.MarkForRedraw()
		if where.In(link.ContentRect(true)) && clickHandler != nil {
			toolbox.Call(func() { clickHandler(link, target) })
		}
		mouseDownIn = false
		return true
	}
	link.DrawCallback = func(gc *Canvas, rect Rect) {
		if mouseDownIn {
			gc.DrawRect(rect, theme.Foreground.Paint(gc, rect, paintstyle.Fill))
		}
		link.DefaultDraw(gc, rect)
	}
	return link
}
