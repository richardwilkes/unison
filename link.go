// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/enums/paintstyle"
	"github.com/richardwilkes/unison/enums/role"
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

// NewLink creates a new Label that can be used as a hyperlink. You may pass nil for the theme to use the
// DefaultLinkTheme.
func NewLink(title, tooltip, target string, theme *LinkTheme, clickHandler func(Paneler, string)) *Label {
	link := NewLabel()
	if theme == nil {
		theme = &DefaultLinkTheme
	}
	link.LabelTheme = theme.LabelTheme
	link.SetTitle(title)
	if tooltip != "" {
		link.Tooltip = NewTooltipWithText(tooltip)
	}
	// A link is a link rather than static text, and where it leads is worth hearing: a person who cannot see that the
	// title is underlined has nothing else to tell them the two apart. Pressing it is already handled by the default
	// behavior for the Press action, which synthesizes the click the mouse callbacks below are waiting for.
	link.Accessibility.Role = role.Link
	// Where the link leads is carried as the link's own property rather than only as words to read out: an assistive
	// technology offers it as something to follow or to copy, and a document holding the link reports the same target
	// for the run of text it occupies.
	link.Accessibility.URL = target
	if target != "" && tooltip == "" {
		// Only when there is nothing better to say. A description is what an assistive technology reads after the name,
		// and the snapshot falls back to the tooltip's text for it while it is empty, so naming the target here for a
		// link that was also given a tooltip would replace words someone chose with a URL.
		link.Accessibility.Description = target
	}
	link.UpdateCursorCallback = func(_ geom.Point) *Cursor {
		if link.Enabled() {
			return PointingCursor()
		}
		return ArrowCursor()
	}
	mouseDown := false
	link.MouseDownCallback = func(_ geom.Point, button, _ int, _ mod.Modifiers) bool {
		if button != ButtonLeft {
			return false
		}
		mouseDown = true
		link.MarkForRedraw()
		return true
	}
	link.MouseDragCallback = func(where geom.Point, button int, _ mod.Modifiers) bool {
		if button != ButtonLeft {
			return false
		}
		now := where.In(link.ContentRect(true))
		if now != mouseDown {
			mouseDown = now
			link.MarkForRedraw()
		}
		return true
	}
	link.MouseUpCallback = func(where geom.Point, button int, _ mod.Modifiers) bool {
		if button != ButtonLeft {
			return false
		}
		link.MarkForRedraw()
		if where.In(link.ContentRect(true)) && clickHandler != nil {
			SafeCall(func() { clickHandler(link, target) })
		}
		mouseDown = false
		return true
	}
	link.DrawCallback = func(gc *Canvas, rect geom.Rect) {
		if mouseDown {
			defer link.Text.RestoreDecorations(link.Text.AdjustDecorations(func(decoration *TextDecoration) {
				decoration.OnBackgroundInk = theme.OnPressedInk
			}))
			paint := theme.PressedInk.Paint(gc, rect, paintstyle.Fill)
			gc.DrawRect(rect, paint)
		}
		link.DefaultDraw(gc, rect)
	}
	return link
}
