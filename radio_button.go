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
	"time"

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/paintstyle"
	"github.com/richardwilkes/unison/enums/side"
)

var _ Grouper = &RadioButton{}

// DefaultRadioButtonTheme holds the default RadioButtonTheme values for RadioButtons. Modifying this data will not
// alter existing RadioButtons, but will alter any RadioButtons created in the future.
var DefaultRadioButtonTheme = RadioButtonTheme{
	TextDecoration: TextDecoration{
		Font:            SystemFont,
		BackgroundInk:   ThemeAboveSurface,
		OnBackgroundInk: ThemeOnAboveSurface,
	},
	EdgeInk:            ThemeSurfaceEdge,
	SelectionInk:       ThemeFocus,
	OnSelectionInk:     ThemeOnFocus,
	ClickAnimationTime: 100 * time.Millisecond,
	Gap:                StdIconGap,
	HAlign:             align.Start,
	VAlign:             align.Middle,
	Side:               side.Left,
}

// RadioButtonTheme holds theming data for a RadioButton.
type RadioButtonTheme struct {
	EdgeInk        Ink
	SelectionInk   Ink
	OnSelectionInk Ink
	TextDecoration
	ClickAnimationTime time.Duration
	Gap                float32
	HAlign             align.Enum
	VAlign             align.Enum
	Side               side.Enum
}

// RadioButton represents a clickable radio button with an optional label.
type RadioButton struct {
	group *Group
	RadioButtonTheme
	checkRadioBase
}

// NewRadioButton creates a new radio button.
func NewRadioButton() *RadioButton {
	var r RadioButton
	r.Self = &r
	r.RadioButtonTheme = DefaultRadioButtonTheme
	r.baseTheme = &r.RadioButtonTheme
	r.commonInit()
	r.updateState = func() { r.group.Select(&r) }
	r.drawMark = r.drawRadio
	return &r
}

// Group returns the group that this button is a part of.
func (r *RadioButton) Group() *Group {
	return r.group
}

// SetGroup sets the group that this button is a part of. Should only be called by the Group.
func (r *RadioButton) SetGroup(group *Group) {
	r.group = group
}

func (r *RadioButton) drawRadio(canvas *Canvas, rect geom.Rect, thickness float32, fg, bg, edge Ink) {
	DrawEllipseBase(canvas, rect, thickness, bg, edge)
	if r.group.Selected(r) {
		rect = rect.Inset(geom.NewUniformInsets(0.5 + 0.2*rect.Width))
		paint := fg.Paint(canvas, rect, paintstyle.Fill)
		defer paint.Dispose()
		if !r.Enabled() {
			paint.SetColorFilter(Grayscale30Filter())
		}
		canvas.DrawOval(rect, paint)
	}
}
