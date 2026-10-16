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
	// A tooltip panel becomes part of a window's description only while it is showing, as the content of that window's
	// tooltip window, and what it is then is a tooltip rather than the anonymous group a bare panel would be taken for.
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
		// The secondary text is part of what the tooltip says, so it is part of the tooltip's name, exactly as the
		// second line of a two-line primary text would be. Keeping it apart from the name would put it somewhere
		// nothing could reach: the panel a tooltip belongs to takes its accessible description from the tooltip's name,
		// and the labels the text is drawn with say nothing of their own, so the secondary line would be heard only in
		// the moment the tooltip itself happened to be showing.
		if tip.Accessibility.Name == "" {
			tip.Accessibility.Name = secondary
		} else {
			tip.Accessibility.Name += "\n" + secondary
		}
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
			// The secondary text is already part of the tooltip's name, so its labels are hidden for the same reason
			// the primary text's are.
			l.Accessibility.Role = role.None
			l.SetTitle(str)
			tip.AddChild(l)
		}
	}
	return tip
}

func (ts *tooltipSequencer) show() {
	w := ts.window
	if w.tooltipSequence != ts.sequence {
		return
	}
	if !w.IsValid() || !w.Focused() || menusOpen() {
		// The tooltip asked for is still the current one, so one showing in its place has to go.
		w.hideTooltip()
		return
	}
	tip := w.lastTooltip
	w.showTooltip(tip, tooltipWindowRect(tip, ts.avoid, w))
	if !tip.TooltipImmediate {
		InvokeTaskAfter(ts.close, DefaultTooltipTheme.Dismissal)
	}
}

func (ts *tooltipSequencer) close() {
	if ts.window.tooltipSequence == ts.sequence {
		ts.window.hideTooltip()
	}
}

// tooltipWindowRect returns the rect, in the space window rects use, of the window showing tip for a panel of owner
// occupying avoid, in owner's root coordinates: just below the part of avoid within owner, or below avoid when none of
// it is, fitted onto the display as tooltipWindowRectOn does.
func tooltipWindowRect(tip *Panel, avoid geom.Rect, owner *Window) geom.Rect {
	if visible := avoid.Intersect(owner.root.ContentRect(true)); !visible.Empty() {
		avoid = visible
	}
	var where geom.Rect
	_, where.Size, _ = tip.Sizes(geom.Size{})
	where.Point = owner.apiPointToScreen(geom.NewPoint(avoid.X, avoid.Bottom()+1))
	avoid.Point = owner.apiPointToScreen(avoid.Point)
	d := tooltipDisplayAmong(AllDisplays(), where, avoid)
	if d == nil {
		d = owner.Display()
	}
	return tooltipWindowRectOn(tip, where, avoid, d)
}

// tooltipDisplayAmong returns the one of displays a tooltip placed at where, below a panel occupying avoid, belongs on:
// the one where overlaps most, as for a menu, or else the one avoid overlaps most, since a tooltip asked for just below
// the bottom of a display's usable area lies on none and belongs with its panel rather than on whichever display holds
// the owner window. Returns nil when neither overlaps any. The rects are in the space window rects use.
func tooltipDisplayAmong(displays []*Display, where, avoid geom.Rect) *Display {
	if d := popupDisplayAmong(displays, where); d != nil {
		return d
	}
	return popupDisplayAmong(displays, avoid)
}

// tooltipWindowRectOn returns where, the rect of the window showing tip below avoid, fitted onto d as fitTooltipRect
// does, with tip narrowed to d's width first when it is wider; or where as it is, but no smaller than fitTooltipRect
// makes it, when d is nil, which it is on macOS while every display is asleep. The rects are in the space window rects
// use, and the fitting is done in the display's own space, as for a menu; see menuWindowRect.
func tooltipWindowRectOn(tip *Panel, where, avoid geom.Rect, d *Display) geom.Rect {
	if d == nil {
		where.Width = max(where.Width, 1)
		where.Height = max(where.Height, 1)
		return where
	}
	maxWidth := d.apiRectToWindowUnits(geom.Rect{Size: geom.NewSize(d.Usable.Width, 0)}).Width
	if where.Width > maxWidth {
		_, where.Size, _ = tip.Sizes(geom.NewSize(maxWidth, 0))
	}
	gap := d.apiRectFromWindowUnits(geom.Rect{Size: geom.NewSize(0, 1)}).Height
	return d.apiRectToWindowUnits(fitTooltipRect(d.apiRectFromWindowUnits(where), d.apiRectFromWindowUnits(avoid),
		d.Usable, gap))
}

// fitTooltipRect returns where, a tooltip placed below avoid, moved so that it lies within usable: at least one unit
// each way, since a window cannot be empty, narrowed to its width, shifted left or right onto it, and, when its bottom
// would fall outside it, placed gap above avoid instead, but never above usable's top. The rects and gap are all in the
// same space.
func fitTooltipRect(where, avoid, usable geom.Rect, gap float32) geom.Rect {
	where.Width = max(where.Width, 1)
	where.Height = max(where.Height, 1)
	if where.Width > usable.Width {
		where.Width = usable.Width
	}
	if where.Right() > usable.Right() {
		where.X = usable.Right() - where.Width
	}
	if where.X < usable.X {
		where.X = usable.X
	}
	if where.Bottom() > usable.Bottom() {
		where.Y = avoid.Y - where.Height - gap
	}
	if where.Y < usable.Y {
		where.Y = usable.Y
	}
	return where
}
