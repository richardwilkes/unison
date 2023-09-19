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
)

// DefaultLinkTheme holds the default Link theme values.
var DefaultLinkTheme = LinkTheme{
	TextDecoration: TextDecoration{
		Font:       LabelFont,
		Foreground: LinkColor,
		Underline:  true,
	},
	RolloverInk: LinkRolloverColor,
	PressedInk:  LinkPressedColor,
}

// LinkTheme holds theming data for a link.
type LinkTheme struct {
	TextDecoration
	RolloverInk Ink
	PressedInk  Ink
}

// NewLink creates a new RichLabel that can be used as a hyperlink.
func NewLink(title, tooltip, target string, theme LinkTheme, clickHandler func(Paneler, string)) *RichLabel {
	link := NewRichLabel()
	link.Text = NewText(title, &theme.TextDecoration)
	if tooltip != "" {
		link.Tooltip = NewTooltipWithText(tooltip)
	}
	link.UpdateCursorCallback = func(where Point) *Cursor {
		return PointingCursor()
	}
	link.MouseEnterCallback = func(where Point, mod Modifiers) bool {
		link.Text.AdjustDecorations(func(decoration *TextDecoration) {
			decoration.Foreground = theme.RolloverInk
		})
		link.MarkForRedraw()
		return true
	}
	link.MouseExitCallback = func() bool {
		link.Text.AdjustDecorations(func(decoration *TextDecoration) {
			decoration.Foreground = theme.Foreground
		})
		link.MarkForRedraw()
		return true
	}
	in := false
	link.MouseDownCallback = func(where Point, button, clickCount int, mod Modifiers) bool {
		link.Text.AdjustDecorations(func(decoration *TextDecoration) {
			decoration.Foreground = theme.PressedInk
		})
		link.MarkForRedraw()
		in = true
		return true
	}
	link.MouseDragCallback = func(where Point, button int, mod Modifiers) bool {
		now := where.In(link.ContentRect(true))
		if now != in {
			in = now
			link.Text.AdjustDecorations(func(decoration *TextDecoration) {
				if in {
					decoration.Foreground = theme.PressedInk
				} else {
					decoration.Foreground = theme.Foreground
				}
			})
			link.MarkForRedraw()
		}
		return true
	}
	link.MouseUpCallback = func(where Point, button int, mod Modifiers) bool {
		ink := theme.Foreground
		inside := where.In(link.ContentRect(true))
		if inside {
			ink = theme.RolloverInk
		}
		link.Text.AdjustDecorations(func(decoration *TextDecoration) {
			decoration.Foreground = ink
		})
		link.MarkForRedraw()
		if inside && clickHandler != nil {
			toolbox.Call(func() { clickHandler(link, target) })
		}
		return true
	}
	return link
}
