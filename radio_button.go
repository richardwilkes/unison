// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
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
	"github.com/richardwilkes/toolbox/v2/xmath"
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
	Gap:                StdIconGap,
	CornerRadius:       4,
	ClickAnimationTime: 100 * time.Millisecond,
	HAlign:             align.Middle,
	VAlign:             align.Middle,
	Side:               side.Left,
}

// RadioButtonTheme holds theming data for a RadioButton.
type RadioButtonTheme struct {
	EdgeInk        Ink
	SelectionInk   Ink
	OnSelectionInk Ink
	TextDecoration
	Gap                float32
	CornerRadius       float32
	ClickAnimationTime time.Duration
	HAlign             align.Enum
	VAlign             align.Enum
	Side               side.Enum
}

// RadioButton represents a clickable radio button with an optional label.
type RadioButton struct {
	ClickCallback func()
	Drawable      Drawable
	Text          *Text
	group         *Group
	RadioButtonTheme
	Panel
	Pressed bool
}

// NewRadioButton creates a new radio button.
func NewRadioButton() *RadioButton {
	r := &RadioButton{RadioButtonTheme: DefaultRadioButtonTheme}
	r.Self = r
	r.SetFocusable(true)
	r.SetSizer(r.DefaultSizes)
	r.DrawCallback = r.DefaultDraw
	r.GainedFocusCallback = r.DefaultFocusGained
	r.LostFocusCallback = r.MarkForRedraw
	r.MouseDownCallback = r.DefaultMouseDown
	r.MouseDragCallback = r.DefaultMouseDrag
	r.MouseUpCallback = r.DefaultMouseUp
	r.KeyDownCallback = r.DefaultKeyDown
	r.UpdateCursorCallback = r.DefaultUpdateCursor
	return r
}

// SetTitle sets the text of the radio button to the specified text. The theme's TextDecoration will be used, so any
// changes you want to make to it should be done before calling this method. Alternatively, you can directly set the
// .Text field.
func (r *RadioButton) SetTitle(text string) {
	r.Text = NewText(text, &r.TextDecoration)
}

// Group returns the group that this button is a part of.
func (r *RadioButton) Group() *Group {
	return r.group
}

// SetGroup sets the group that this button is a part of. Should only be called by the Group.
func (r *RadioButton) SetGroup(group *Group) {
	r.group = group
}

// DefaultSizes provides the default sizing.
func (r *RadioButton) DefaultSizes(hint geom.Size) (minSize, prefSize, maxSize geom.Size) {
	prefSize = r.circleAndLabelSize()
	if border := r.Border(); border != nil {
		prefSize = prefSize.Add(border.Insets().Size())
	}
	prefSize = prefSize.Ceil().ConstrainForHint(hint)
	return prefSize, prefSize, MaxSize(prefSize)
}

func (r *RadioButton) circleAndLabelSize() geom.Size {
	circleSize := r.circleSize()
	if r.Drawable == nil && r.Text.Empty() {
		return geom.Size{Width: circleSize, Height: circleSize}
	}
	size, _ := LabelContentSizes(r.Text, r.Drawable, r.Font, r.Side, r.Gap)
	size.Width += r.Gap + circleSize
	if size.Height < circleSize {
		size.Height = circleSize
	}
	return size
}

func (r *RadioButton) circleSize() float32 {
	return xmath.Ceil(r.Font.Baseline())
}

// DefaultFocusGained provides the default focus gained handling.
func (r *RadioButton) DefaultFocusGained() {
	r.ScrollIntoView()
	r.MarkForRedraw()
}

// DefaultDraw provides the default drawing.
func (r *RadioButton) DefaultDraw(canvas *Canvas, _ geom.Rect) {
	rect := r.ContentRect(false)
	size := r.circleAndLabelSize()
	switch r.HAlign {
	case align.Middle, align.Fill:
		rect.X = xmath.Floor(rect.X + (rect.Width-size.Width)/2)
	case align.End:
		rect.X += rect.Width - size.Width
	default: // align.Start
	}
	switch r.VAlign {
	case align.Middle, align.Fill:
		rect.Y = xmath.Floor(rect.Y + (rect.Height-size.Height)/2)
	case align.End:
		rect.Y += rect.Height - size.Height
	default: // align.Start
	}
	var fg, bg Ink
	switch {
	case r.Pressed:
		bg = r.SelectionInk
		fg = r.OnSelectionInk
	default:
		bg = r.BackgroundInk
		fg = r.OnBackgroundInk
	}
	edge := r.EdgeInk
	thickness := float32(1)
	if r.Focused() {
		thickness++
		edge = r.SelectionInk
	}
	rect.Size = size
	circleSize := r.circleSize()
	if r.Drawable != nil || !r.Text.Empty() {
		rct := rect
		rct.X += circleSize + r.Gap
		rct.Width -= circleSize + r.Gap
		defer r.Text.RestoreDecorations(r.Text.AdjustDecorations(func(d *TextDecoration) {
			d.BackgroundInk = nil
			d.OnBackgroundInk = fg
		}))
		DrawLabel(canvas, rct, r.HAlign, r.VAlign, r.Font, r.Text, fg, nil, r.Drawable, r.Side, r.Gap, !r.Enabled())
	}
	if rect.Height > circleSize {
		rect.Y += xmath.Floor((rect.Height - circleSize) / 2)
	}
	rect.Width = circleSize
	rect.Height = circleSize
	DrawEllipseBase(canvas, rect, thickness, bg, edge)
	if r.group.Selected(r) {
		rect = rect.Inset(geom.NewUniformInsets(0.5 + 0.2*circleSize))
		paint := fg.Paint(canvas, rect, paintstyle.Fill)
		if !r.Enabled() {
			paint.SetColorFilter(Grayscale30Filter())
		}
		canvas.DrawOval(rect, paint)
	}
}

// Click makes the radio button behave as if a user clicked on it.
func (r *RadioButton) Click() {
	r.group.Select(r)
	pressed := r.Pressed
	r.Pressed = true
	r.MarkForRedraw()
	r.FlushDrawing()
	r.Pressed = pressed
	time.Sleep(r.ClickAnimationTime)
	r.MarkForRedraw()
	if r.ClickCallback != nil {
		r.ClickCallback()
	}
}

// DefaultMouseDown provides the default mouse down handling.
func (r *RadioButton) DefaultMouseDown(_ geom.Point, _, _ int, _ Modifiers) bool {
	r.Pressed = true
	r.MarkForRedraw()
	return true
}

// DefaultMouseDrag provides the default mouse drag handling.
func (r *RadioButton) DefaultMouseDrag(where geom.Point, _ int, _ Modifiers) bool {
	if pressed := where.In(r.ContentRect(false)); pressed != r.Pressed {
		r.Pressed = pressed
		r.MarkForRedraw()
	}
	return true
}

// DefaultMouseUp provides the default mouse up handling.
func (r *RadioButton) DefaultMouseUp(where geom.Point, _ int, _ Modifiers) bool {
	r.Pressed = false
	r.MarkForRedraw()
	if where.In(r.ContentRect(false)) {
		r.group.Select(r)
		if r.ClickCallback != nil {
			r.ClickCallback()
		}
	}
	return true
}

// DefaultKeyDown provides the default key down handling.
func (r *RadioButton) DefaultKeyDown(keyCode KeyCode, mod Modifiers, _ bool) bool {
	if IsControlAction(keyCode, mod) {
		r.Click()
		return true
	}
	return false
}

// DefaultUpdateCursor provides the default cursor for radio buttons.
func (r *RadioButton) DefaultUpdateCursor(_ geom.Point) *Cursor {
	if !r.Enabled() {
		return ArrowCursor()
	}
	return PointingCursor()
}
