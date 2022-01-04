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
)

// DefaultButtonTheme holds the default ButtonTheme values for Buttons. Modifying this data will not alter existing
// Buttons, but will alter any Buttons created in the future.
var DefaultButtonTheme = ButtonTheme{
	Font:                SystemFont,
	BackgroundInk:       ControlColor,
	OnBackgroundInk:     OnControlColor,
	EdgeInk:             ControlEdgeColor,
	SelectionInk:        SelectionColor,
	OnSelectionInk:      OnSelectionColor,
	Gap:                 3,
	CornerRadius:        4,
	HMargin:             8,
	VMargin:             1,
	DrawableOnlyHMargin: 3,
	DrawableOnlyVMargin: 3,
	ClickAnimationTime:  100 * time.Millisecond,
	HAlign:              MiddleAlignment,
	VAlign:              MiddleAlignment,
	Side:                LeftSide,
	HideBase:            false,
}

// ButtonTheme holds theming data for a Button.
type ButtonTheme struct {
	Font                FontProvider
	BackgroundInk       Ink
	OnBackgroundInk     Ink
	EdgeInk             Ink
	SelectionInk        Ink
	OnSelectionInk      Ink
	Gap                 float32
	CornerRadius        float32
	HMargin             float32
	VMargin             float32
	DrawableOnlyHMargin float32
	DrawableOnlyVMargin float32
	ClickAnimationTime  time.Duration
	HAlign              Alignment
	VAlign              Alignment
	Side                Side
	HideBase            bool
	Sticky              bool
}

// Button represents a clickable button.
type Button struct {
	GroupPanel
	ButtonTheme
	ClickCallback func()
	Drawable      Drawable
	Text          string
	Pressed       bool
}

// NewButton creates a new button.
func NewButton() *Button {
	b := &Button{ButtonTheme: DefaultButtonTheme}
	b.Self = b
	b.SetFocusable(true)
	b.SetSizer(b.DefaultSizes)
	b.DrawCallback = b.DefaultDraw
	b.GainedFocusCallback = b.MarkForRedraw
	b.LostFocusCallback = b.MarkForRedraw
	b.MouseDownCallback = b.DefaultMouseDown
	b.MouseDragCallback = b.DefaultMouseDrag
	b.MouseUpCallback = b.DefaultMouseUp
	b.KeyDownCallback = b.DefaultKeyDown
	return b
}

// DefaultSizes provides the default sizing.
func (b *Button) DefaultSizes(hint geom32.Size) (min, pref, max geom32.Size) {
	text := b.Text
	if b.Drawable == nil && text == "" {
		text = "M"
	}
	pref = LabelSize(text, b.Font, b.Drawable, b.Side, b.Gap)
	if theBorder := b.Border(); theBorder != nil {
		pref.AddInsets(theBorder.Insets())
	}
	pref.Width += b.HorizontalMargin() * 2
	pref.Height += b.VerticalMargin() * 2
	if !b.HideBase {
		pref.Width += 2
		pref.Height += 2
	}
	pref.GrowToInteger()
	pref.ConstrainForHint(hint)
	return pref, pref, MaxSize(pref)
}

// HorizontalMargin returns the horizontal margin that will be used.
func (b *Button) HorizontalMargin() float32 {
	if b.Text == "" && b.Drawable != nil {
		return b.DrawableOnlyHMargin
	}
	return b.HMargin
}

// VerticalMargin returns the vertical margin that will be used.
func (b *Button) VerticalMargin() float32 {
	if b.Text == "" && b.Drawable != nil {
		return b.DrawableOnlyVMargin
	}
	return b.VMargin
}

// DefaultDraw provides the default drawing.
func (b *Button) DefaultDraw(canvas *Canvas, dirty geom32.Rect) {
	var fg, bg Ink
	switch {
	case b.Pressed || (b.Sticky && b.Selected()):
		bg = b.SelectionInk
		fg = b.OnSelectionInk
	default:
		bg = b.BackgroundInk
		fg = b.OnBackgroundInk
	}
	rect := b.ContentRect(false)
	if !b.HideBase || b.Focused() {
		thickness := float32(1)
		if b.Focused() {
			thickness++
		}
		DrawRoundedRectBase(canvas, rect, b.CornerRadius, thickness, bg, b.EdgeInk)
		rect.InsetUniform(thickness + 0.5)
	}
	rect.X += b.HorizontalMargin()
	rect.Y += b.VerticalMargin()
	rect.Width -= b.HorizontalMargin() * 2
	rect.Height -= b.VerticalMargin() * 2
	DrawLabel(canvas, rect, b.HAlign, b.VAlign, b.Text, b.Font, fg, b.Drawable, b.Side, b.Gap, !b.Enabled())
}

// Click makes the button behave as if a user clicked on it.
func (b *Button) Click() {
	b.SetSelected(true)
	pressed := b.Pressed
	b.Pressed = true
	b.MarkForRedraw()
	b.FlushDrawing()
	b.Pressed = pressed
	time.Sleep(b.ClickAnimationTime)
	b.MarkForRedraw()
	if b.ClickCallback != nil {
		b.ClickCallback()
	}
}

// DefaultMouseDown provides the default mouse down handling.
func (b *Button) DefaultMouseDown(where geom32.Point, button, clickCount int, mod Modifiers) bool {
	b.Pressed = true
	b.MarkForRedraw()
	return true
}

// DefaultMouseDrag provides the default mouse drag handling.
func (b *Button) DefaultMouseDrag(where geom32.Point, button int, mod Modifiers) bool {
	rect := b.ContentRect(false)
	pressed := rect.ContainsPoint(where)
	if b.Pressed != pressed {
		b.Pressed = pressed
		b.MarkForRedraw()
	}
	return true
}

// DefaultMouseUp provides the default mouse up handling.
func (b *Button) DefaultMouseUp(where geom32.Point, button int, mod Modifiers) bool {
	b.Pressed = false
	b.MarkForRedraw()
	rect := b.ContentRect(false)
	if rect.ContainsPoint(where) {
		b.SetSelected(true)
		if b.ClickCallback != nil {
			b.ClickCallback()
		}
	}
	return true
}

// DefaultKeyDown provides the default key down handling.
func (b *Button) DefaultKeyDown(keyCode KeyCode, mod Modifiers, repeat bool) bool {
	if IsControlAction(keyCode, mod) {
		b.Click()
		return true
	}
	return false
}
