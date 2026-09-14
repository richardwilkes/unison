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
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/check"
	"github.com/richardwilkes/unison/enums/paintstyle"
	"github.com/richardwilkes/unison/enums/role"
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

// ProvideAccessibility describes the radio button to assistive technologies. A radio button is checked when it is the
// one selected within its group, so a button that belongs to no group is never checked, however often it is clicked.
//
// A button that has a group to be the selection of offers to be made that selection, which is what a sticky Button
// reported under this same role offers, so that the two describe themselves alike. One with no group has no selection
// to be and offers only the press.
func (r *RadioButton) ProvideAccessibility(b *AccessibilityBuilder) {
	node := b.Node()
	if node.Role == role.Auto {
		node.Role = role.RadioButton
	}
	node.HasCheck = true
	if r.group.Selected(r) {
		node.Checked = check.On
	} else {
		node.Checked = check.Off
	}
	node.Name = r.axName(node.Name)
	node.Actions = node.Actions.With(accessibility.Press)
	if r.group != nil {
		node.Actions = node.Actions.With(accessibility.Select)
	}
}

// PerformAccessibilityAction carries out a request from an assistive technology. Selecting a radio button makes it the
// selection of its group, without the click animation and without the callback, exactly as selecting the sticky Button
// reported under the same role does; a button that belongs to no group has no selection to be. Everything else is left
// to the shared handling, which clicks the button.
func (r *RadioButton) PerformAccessibilityAction(req accessibility.ActionRequest) bool {
	if req.Action == accessibility.Select {
		if r.group == nil {
			return false
		}
		r.group.Select(r)
		return true
	}
	return r.checkRadioBase.PerformAccessibilityAction(req)
}

func (r *RadioButton) drawRadio(canvas *Canvas, rect geom.Rect, thickness float32, fg, bg, edge Ink) {
	DrawEllipseBase(canvas, rect, thickness, bg, edge)
	if r.group.Selected(r) {
		rect = rect.Inset(geom.NewUniformInsets(0.5 + 0.2*rect.Width))
		paint := fg.Paint(canvas, rect, paintstyle.Fill)
		if !r.Enabled() {
			paint.SetColorFilter(Grayscale30Filter())
		}
		canvas.DrawOval(rect, paint)
	}
}
