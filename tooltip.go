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
	"strings"
	"time"

	"github.com/richardwilkes/toolbox/xmath/geom32"
)

var (
	// TooltipDelay holds the delay before a tooltip will be shown.
	TooltipDelay = 1500 * time.Millisecond
	// TooltipDismissal holds the delay before a tooltip will be dismissed.
	TooltipDismissal = 3 * time.Second
)

type tooltipSequencer struct {
	window   *Window
	avoid    geom32.Rect
	sequence int
}

// NewTooltipBase returns the base for a tooltip.
func NewTooltipBase() *Panel {
	tip := NewPanel()
	tip.SetBorder(NewCompoundBorder(NewLineBorder(ControlEdgeColor, 0,
		geom32.NewUniformInsets(1), false), NewEmptyBorder(geom32.Insets{Top: 2, Left: 4, Bottom: 2, Right: 4})))
	tip.DrawCallback = func(canvas *Canvas, dirty geom32.Rect) {
		r := tip.ContentRect(true)
		canvas.DrawRect(r, TooltipColor.Paint(canvas, r, Fill))
	}
	return tip
}

// NewTooltipWithText creates a standard text tooltip panel.
func NewTooltipWithText(text string) *Panel {
	tip := NewTooltipBase()
	tip.SetLayout(&FlexLayout{
		Columns:  1,
		HSpacing: StdHSpacing,
		VSpacing: StdVSpacing,
	})
	for _, str := range strings.Split(text, "\n") {
		l := NewLabel()
		l.Text = str
		l.Font = SystemFont
		l.Ink = OnTooltipColor
		tip.AddChild(l)
	}
	return tip
}

// NewTooltipWithSecondaryText creates a text tooltip panel containing a primary piece of text along with a secondary
// piece of text in a slightly smaller font.
func NewTooltipWithSecondaryText(primary, secondary string) *Panel {
	tip := NewTooltipWithText(primary)
	if secondary != "" {
		for _, str := range strings.Split(secondary, "\n") {
			l := NewLabel()
			l.Text = str
			l.Font = SmallSystemFont
			l.Ink = OnTooltipColor
			tip.AddChild(l)
		}
	}
	return tip
}

func (ts *tooltipSequencer) show() {
	if ts.window.tooltipSequence == ts.sequence && ts.window.Focused() {
		tip := ts.window.lastTooltip
		_, pref, _ := tip.Sizes(geom32.Size{})
		rect := geom32.Rect{Point: geom32.Point{X: ts.avoid.X, Y: ts.avoid.Y + ts.avoid.Height + 1}, Size: pref}
		if rect.X < 0 {
			rect.X = 0
		}
		if rect.Y < 0 {
			rect.Y = 0
		}
		viewSize := ts.window.root.ContentRect(true).Size
		if viewSize.Width < rect.Width {
			_, pref, _ = tip.Sizes(geom32.Size{Width: viewSize.Width})
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
		InvokeTaskAfter(ts.close, TooltipDismissal)
	}
}

func (ts *tooltipSequencer) close() {
	if ts.window.tooltipSequence == ts.sequence {
		ts.window.root.setTooltip(nil)
	}
}
