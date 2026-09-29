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

// linkFocusRingRoom is the room NewLink leaves to either side of a link's text for its focus outline.
const linkFocusRingRoom = 2

// NewLink creates a new Label that can be used as a hyperlink. You may pass nil for the theme to use the
// DefaultLinkTheme. The link is a tab stop, outlines itself while it holds the keyboard focus, and is followed by
// Return, the keypad's Enter and the space bar as well as by a click. Return pressed on a link in a dialog therefore
// follows the link rather than pressing the dialog's default button. The link is given a border that leaves room for
// the outline to either side of its text.
//
// Call SetFocusable(false) on the result for one that should be followed by a click alone, and SetBorder(nil) as well
// for one that should take up no more room than its text. A link that is, or is inside, a cell of a Table, List or
// TableHeader never takes the focus.
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
	follow := func() {
		if clickHandler != nil {
			SafeCall(func() { clickHandler(link, target) })
		}
	}
	link.SetFocusable(true)
	link.noFocusInCells = true
	link.SetBorder(NewEmptyBorder(geom.Insets{Left: linkFocusRingRoom, Right: linkFocusRingRoom}))
	link.GainedFocusCallback = func() {
		link.ScrollIntoView()
		link.MarkForRedraw()
	}
	link.LostFocusCallback = link.MarkForRedraw
	link.KeyDownCallback = func(keyCode KeyCode, mods mod.Modifiers, _ bool) bool {
		if IsControlAction(keyCode, mods) ||
			((keyCode == KeyReturn || keyCode == KeyNumPadEnter) && mods&mod.NonSticky == 0) {
			follow()
			return true
		}
		return false
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
		// Cleared before the link is followed, since following it may run an event loop of its own, during which the
		// link would be drawn pressed.
		mouseDown = false
		if where.In(link.ContentRect(true)) {
			link.axFocusOnClick()
			follow()
		}
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
		if link.Focused() {
			r := link.ContentRect(true).Inset(geom.NewUniformInsets(0.5))
			paint := theme.PressedInk.Paint(gc, r, paintstyle.Stroke)
			paint.SetStrokeWidth(1)
			gc.DrawRoundedRect(r, geom.NewSize(2, 2), paint)
		}
	}
	return link
}
