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
		Foreground: &PrimaryTheme.Primary,
	},
	PressedInk: &PrimaryTheme.PrimaryVariant,
}

// LinkTheme holds theming data for a link.
type LinkTheme struct {
	TextDecoration
	PressedInk Ink
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
	link.MouseEnterCallback = func(_ Point, _ Modifiers) bool {
		link.Text.AdjustDecorations(func(decoration *TextDecoration) {
			decoration.Underline = true
		})
		link.MarkForRedraw()
		return true
	}
	link.MouseExitCallback = func() bool {
		link.Text.AdjustDecorations(func(decoration *TextDecoration) {
			decoration.Underline = false
		})
		link.MarkForRedraw()
		return true
	}
	in := false
	link.MouseDownCallback = func(_ Point, _, _ int, _ Modifiers) bool {
		link.Text.AdjustDecorations(func(decoration *TextDecoration) {
			decoration.Underline = true
			decoration.Foreground = theme.PressedInk
		})
		link.OnBackgroundInk = theme.PressedInk
		link.MarkForRedraw()
		in = true
		return true
	}
	link.MouseDragCallback = func(where Point, _ int, _ Modifiers) bool {
		now := where.In(link.ContentRect(true))
		if now != in {
			in = now
			link.Text.AdjustDecorations(func(decoration *TextDecoration) {
				if in {
					decoration.Underline = true
					decoration.Foreground = theme.PressedInk
				} else {
					decoration.Underline = false
					decoration.Foreground = theme.Foreground
				}
			})
			if in {
				link.OnBackgroundInk = theme.PressedInk
			} else {
				link.OnBackgroundInk = theme.Foreground
			}
			link.MarkForRedraw()
		}
		return true
	}
	link.MouseUpCallback = func(where Point, _ int, _ Modifiers) bool {
		inside := where.In(link.ContentRect(true))
		link.Text.AdjustDecorations(func(decoration *TextDecoration) {
			decoration.Underline = inside
			decoration.Foreground = theme.Foreground
		})
		link.OnBackgroundInk = theme.Foreground
		link.MarkForRedraw()
		if inside && clickHandler != nil {
			toolbox.Call(func() { clickHandler(link, target) })
		}
		return true
	}
	return link
}
