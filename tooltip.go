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
	"strings"
	"time"

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/enums/paintstyle"
	"github.com/richardwilkes/unison/enums/role"
)

// DefaultTooltipTheme holds the default TooltipTheme values for Tooltips. Modifying this data will not alter existing
// Tooltips, but will alter any Tooltips created in the future.
var DefaultTooltipTheme = TooltipTheme{
	BackgroundInk: ThemeTooltip,
	BaseBorder: NewCompoundBorder(
		NewLineBorder(ThemeTooltipEdge, geom.Size{}, geom.NewUniformInsets(1), false),
		NewEmptyBorder(StdInsets()),
	),
	Label:     defaultToolTipLabelTheme(),
	Delay:     1500 * time.Millisecond,
	Dismissal: 5 * time.Second,
}

func defaultToolTipLabelTheme() LabelTheme {
	theme := DefaultLabelTheme
	theme.Font = FieldFont
	theme.OnBackgroundInk = ThemeOnTooltip
	return theme
}

// TooltipTheme holds theming data for a Tooltip.
type TooltipTheme struct {
	// SecondaryTextFont is the font used for the secondary text of tooltips created via
	// NewTooltipWithSecondaryText(). When nil, a font one size smaller than the Label theme's font is used.
	SecondaryTextFont Font
	BackgroundInk     Ink
	BaseBorder        Border
	Label             LabelTheme
	Delay             time.Duration
	Dismissal         time.Duration
}

type tooltipSequencer struct {
	window   *Window
	avoid    geom.Rect
	sequence int
}

// NewTooltipBase returns the base for a tooltip.
func NewTooltipBase() *Panel {
	tip := NewPanel()
	tip.SetBorder(DefaultTooltipTheme.BaseBorder)
	// A tooltip panel becomes part of a window's description only while it is showing, and what it is then is a tooltip
	// rather than the anonymous group a bare panel would be taken for.
	tip.Accessibility.Role = role.Tooltip
	tip.DrawCallback = func(canvas *Canvas, _ geom.Rect) {
		r := tip.ContentRect(true)
		paint := DefaultTooltipTheme.BackgroundInk.Paint(canvas, r, paintstyle.Fill)
		canvas.DrawRect(r, paint)
	}
	return tip
}

// NewTooltipWithText creates a standard text tooltip panel.
func NewTooltipWithText(text string) *Panel {
	tip := NewTooltipBase()
	// Naming the tooltip panel rather than leaving its text to be gathered from the labels below means a panel's
	// accessible description is the text as it was written, complete with the line breaks a label per line would have
	// thrown away.
	tip.Accessibility.Name = text
	tip.SetLayout(&FlexLayout{
		Columns:  1,
		HSpacing: StdHSpacing,
		VSpacing: StdVSpacing,
	})
	for str := range strings.SplitSeq(text, "\n") {
		l := NewLabel()
		l.LabelTheme = DefaultTooltipTheme.Label
		// The tooltip has already said the whole of the text, so a label per line of it would only have an assistive
		// technology say the same thing again, a line at a time.
		l.Accessibility.Role = role.None
		l.SetTitle(str)
		tip.AddChild(l)
	}
	return tip
}

// NewTooltipWithSecondaryText creates a text tooltip panel containing a primary piece of text along with a secondary
// piece of text in a slightly smaller font.
func NewTooltipWithSecondaryText(primary, secondary string) *Panel {
	tip := NewTooltipWithText(primary)
	if secondary != "" {
		tip.Accessibility.Description = secondary
		font := DefaultTooltipTheme.SecondaryTextFont
		if font == nil {
			desc := DefaultTooltipTheme.Label.Font.Descriptor()
			desc.Size--
			font = desc.Font()
		}
		for str := range strings.SplitSeq(secondary, "\n") {
			l := NewLabel()
			l.LabelTheme = DefaultTooltipTheme.Label
			l.Font = font
			// The secondary text is already the tooltip's description, so its labels are hidden for the same reason the
			// primary text's are.
			l.Accessibility.Role = role.None
			l.SetTitle(str)
			tip.AddChild(l)
		}
	}
	return tip
}

func (ts *tooltipSequencer) show() {
	if ts.window.tooltipSequence == ts.sequence && ts.window.Focused() {
		tip := ts.window.lastTooltip
		_, pref, _ := tip.Sizes(geom.Size{})
		rect := geom.NewRect(ts.avoid.X, ts.avoid.Bottom()+1, pref.Width, pref.Height)
		if rect.X < 0 {
			rect.X = 0
		}
		if rect.Y < 0 {
			rect.Y = 0
		}
		viewSize := ts.window.root.ContentRect(true).Size
		if viewSize.Width < rect.Width {
			_, pref, _ = tip.Sizes(geom.NewSize(viewSize.Width, 0))
			if viewSize.Width < pref.Width {
				rect.X = 0
				rect.Width = viewSize.Width
			} else {
				rect.Width = pref.Width
			}
			rect.Height = pref.Height
		}
		if viewSize.Width < rect.X+rect.Width {
			rect.X = viewSize.Width - rect.Width
		}
		if viewSize.Height < rect.Y+rect.Height {
			rect.Y = ts.avoid.Y - (rect.Height + 1)
			if rect.Y < 0 {
				rect.Y = 0
			}
		}
		tip.SetFrameRect(rect)
		ts.window.root.setTooltip(tip)
		ts.window.lastTooltipShownAt = time.Now()
		if !tip.TooltipImmediate {
			InvokeTaskAfter(ts.close, DefaultTooltipTheme.Dismissal)
		}
	}
}

func (ts *tooltipSequencer) close() {
	if ts.window.tooltipSequence == ts.sequence {
		ts.window.root.setTooltip(nil)
	}
}
