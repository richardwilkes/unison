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
	"time"

	"github.com/richardwilkes/toolbox/xmath/geom32"
	"github.com/richardwilkes/toolbox/xmath/mathf32"
)

// RadioButton represents a clickable radio button with an optional label.
type RadioButton struct {
	GroupPanel
	ClickCallback      func()
	Font               FontProvider
	ControlColor       Ink
	OnControlColor     Ink
	OnBackgroundColor  Ink
	EdgeColor          Ink
	PressedColor       Ink
	OnPressedColor     Ink
	Drawable           Drawable
	Text               string
	Gap                float32
	CornerRadius       float32
	ClickAnimationTime time.Duration
	HAlign             Alignment
	VAlign             Alignment
	Side               Side
	Pressed            bool
}

// NewRadioButton creates a new radio button.
func NewRadioButton() *RadioButton {
	r := &RadioButton{
		Gap:                3,
		CornerRadius:       4,
		ClickAnimationTime: 100 * time.Millisecond,
		HAlign:             MiddleAlignment,
		VAlign:             MiddleAlignment,
		Side:               LeftSide,
	}
	r.Self = r
	r.SetFocusable(true)
	r.SetSizer(r.DefaultSizes)
	r.DrawCallback = r.DefaultDraw
	r.GainedFocusCallback = r.MarkForRedraw
	r.LostFocusCallback = r.MarkForRedraw
	r.MouseDownCallback = r.DefaultMouseDown
	r.MouseDragCallback = r.DefaultMouseDrag
	r.MouseUpCallback = r.DefaultMouseUp
	r.KeyDownCallback = r.DefaultKeyDown
	return r
}

// DefaultSizes provides the default sizing.
func (r *RadioButton) DefaultSizes(hint geom32.Size) (min, pref, max geom32.Size) {
	pref = r.circleAndLabelSize()
	if border := r.Border(); border != nil {
		pref.AddInsets(border.Insets())
	}
	pref.GrowToInteger()
	pref.ConstrainForHint(hint)
	return pref, pref, MaxSize(pref)
}

func (r *RadioButton) circleAndLabelSize() geom32.Size {
	circleSize := r.circleSize()
	if r.Drawable == nil && r.Text == "" {
		return geom32.Size{Width: circleSize, Height: circleSize}
	}
	size := LabelSize(r.Text, ChooseFont(r.Font, SystemFont), r.Drawable, r.Side, r.Gap)
	size.Width += r.Gap + circleSize
	if size.Height < circleSize {
		size.Height = circleSize
	}
	return size
}

func (r *RadioButton) circleSize() float32 {
	return mathf32.Ceil(ChooseFont(r.Font, SystemFont).Baseline())
}

// DefaultDraw provides the default drawing.
func (r *RadioButton) DefaultDraw(canvas *Canvas, dirty geom32.Rect) {
	rect := r.ContentRect(false)
	size := r.circleAndLabelSize()
	switch r.HAlign {
	case MiddleAlignment, FillAlignment:
		rect.X = mathf32.Floor(rect.X + (rect.Width-size.Width)/2)
	case EndAlignment:
		rect.X += rect.Width - size.Width
	default: // StartAlignment
	}
	switch r.VAlign {
	case MiddleAlignment, FillAlignment:
		rect.Y = mathf32.Floor(rect.Y + (rect.Height-size.Height)/2)
	case EndAlignment:
		rect.Y += rect.Height - size.Height
	default: // StartAlignment
	}
	var fg, bg Ink
	switch {
	case r.Pressed:
		bg = ChooseInk(r.PressedColor, SelectionColor)
		fg = ChooseInk(r.OnPressedColor, OnSelectionColor)
	default:
		bg = ChooseInk(r.ControlColor, ControlColor)
		fg = ChooseInk(r.OnControlColor, OnControlColor)
	}
	thickness := float32(1)
	if r.Focused() {
		thickness++
	}
	rect.Size = size
	circleSize := r.circleSize()
	if r.Drawable != nil || r.Text != "" {
		rct := rect
		rct.X += circleSize + r.Gap
		rct.Width -= circleSize + r.Gap
		DrawLabel(canvas, rct, r.HAlign, r.VAlign, r.Text, ChooseFont(r.Font, SystemFont),
			ChooseInk(r.OnBackgroundColor, OnBackgroundColor), r.Drawable, r.Side, r.Gap, !r.Enabled())
	}
	if rect.Height > circleSize {
		rect.Y += mathf32.Floor((rect.Height - circleSize) / 2)
	}
	rect.Width = circleSize
	rect.Height = circleSize
	DrawEllipseBase(canvas, rect, thickness, bg, ChooseInk(r.EdgeColor, ControlEdgeColor))
	if r.Selected() {
		rect.InsetUniform(0.5 + 0.2*circleSize)
		paint := fg.Paint(canvas, rect, Fill)
		if !r.Enabled() {
			paint.SetColorFilter(Grayscale30PercentFilter())
		}
		canvas.DrawOval(rect, paint)
	}
}

// Click makes the radio button behave as if a user clicked on it.
func (r *RadioButton) Click() {
	r.SetSelected(true)
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
func (r *RadioButton) DefaultMouseDown(where geom32.Point, button, clickCount int, mod Modifiers) bool {
	r.Pressed = true
	r.MarkForRedraw()
	return true
}

// DefaultMouseDrag provides the default mouse drag handling.
func (r *RadioButton) DefaultMouseDrag(where geom32.Point, button int, mod Modifiers) bool {
	rect := r.ContentRect(false)
	pressed := rect.ContainsPoint(where)
	if r.Pressed != pressed {
		r.Pressed = pressed
		r.MarkForRedraw()
	}
	return true
}

// DefaultMouseUp provides the default mouse up handling.
func (r *RadioButton) DefaultMouseUp(where geom32.Point, button int, mod Modifiers) bool {
	r.Pressed = false
	r.MarkForRedraw()
	rect := r.ContentRect(false)
	if rect.ContainsPoint(where) {
		r.SetSelected(true)
		if r.ClickCallback != nil {
			r.ClickCallback()
		}
	}
	return true
}

// DefaultKeyDown provides the default key down handling.
func (r *RadioButton) DefaultKeyDown(keyCode KeyCode, mod Modifiers, repeat bool) bool {
	if IsControlAction(keyCode, mod) {
		r.Click()
		return true
	}
	return false
}
