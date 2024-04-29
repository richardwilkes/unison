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

	"github.com/richardwilkes/toolbox"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/side"
)

// DefaultButtonTheme holds the default ButtonTheme values for Buttons. Modifying this data will not alter existing
// Buttons, but will alter any Buttons created in the future.
var DefaultButtonTheme = ButtonTheme{
	Font:                SystemFont,
	BackgroundInk:       &PrimaryTheme.SurfaceAbove,
	OnBackgroundInk:     &PrimaryTheme.OnSurfaceAbove,
	EdgeInk:             &PrimaryTheme.Outline,
	SelectionInk:        &PrimaryTheme.Primary,
	OnSelectionInk:      &PrimaryTheme.OnPrimary,
	Gap:                 3,
	CornerRadius:        4,
	HMargin:             8,
	VMargin:             1,
	DrawableOnlyHMargin: 3,
	DrawableOnlyVMargin: 3,
	ClickAnimationTime:  100 * time.Millisecond,
	HAlign:              align.Middle,
	VAlign:              align.Middle,
	Side:                side.Left,
	HideBase:            false,
}

// ButtonTheme holds theming data for a Button.
type ButtonTheme struct {
	Font                Font
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
	HAlign              align.Enum
	VAlign              align.Enum
	Side                side.Enum
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
	cache         TextCache
	Pressed       bool
}

// NewButton creates a new button.
func NewButton() *Button {
	b := &Button{ButtonTheme: DefaultButtonTheme}
	b.Self = b
	b.SetFocusable(true)
	b.SetSizer(b.DefaultSizes)
	b.DrawCallback = b.DefaultDraw
	b.GainedFocusCallback = b.DefaultFocusGained
	b.LostFocusCallback = b.MarkForRedraw
	b.MouseDownCallback = b.DefaultMouseDown
	b.MouseDragCallback = b.DefaultMouseDrag
	b.MouseUpCallback = b.DefaultMouseUp
	b.KeyDownCallback = b.DefaultKeyDown
	b.UpdateCursorCallback = b.DefaultUpdateCursor
	return b
}

// NewSVGButton creates an SVG icon button with a size equal to the default button theme's font baseline.
func NewSVGButton(svg *SVG) *Button {
	b := NewButton()
	b.HideBase = true
	baseline := b.Font.Baseline()
	b.Drawable = &DrawableSVG{
		SVG:  svg,
		Size: NewSize(baseline, baseline).Ceil(),
	}
	return b
}

// DefaultSizes provides the default sizing.
func (b *Button) DefaultSizes(hint Size) (minSize, prefSize, maxSize Size) {
	prefSize = LabelSize(b.cache.Text(b.Text, b.Font), b.Drawable, b.Side, b.Gap)
	if b.Text == "" && toolbox.IsNil(b.Drawable) {
		prefSize.Height = b.Font.LineHeight()
	}
	if border := b.Border(); border != nil {
		prefSize = prefSize.Add(border.Insets().Size())
	}
	prefSize.Width += b.HorizontalMargin() * 2
	prefSize.Height += b.VerticalMargin() * 2
	if !b.HideBase {
		prefSize.Width += 2
		prefSize.Height += 2
	}
	prefSize = prefSize.Ceil().ConstrainForHint(hint)
	return prefSize, prefSize, MaxSize(prefSize)
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

// DefaultFocusGained provides the default focus gained handling.
func (b *Button) DefaultFocusGained() {
	b.ScrollIntoView()
	b.MarkForRedraw()
}

// DefaultDraw provides the default drawing.
func (b *Button) DefaultDraw(canvas *Canvas, _ Rect) {
	var fg, bg Ink
	switch {
	case b.Pressed || (b.Sticky && b.Selected()):
		if b.HideBase {
			bg = Transparent
			fg = b.SelectionInk
		} else {
			bg = b.SelectionInk
			fg = b.OnSelectionInk
		}
	default:
		if b.HideBase {
			bg = Transparent
		} else {
			bg = b.BackgroundInk
		}
		fg = b.OnBackgroundInk
	}
	r := b.ContentRect(false)
	if !b.HideBase || b.Focused() {
		thickness := float32(1)
		edge := b.EdgeInk
		if b.Focused() {
			thickness++
			edge = b.SelectionInk
		}
		DrawRoundedRectBase(canvas, r, b.CornerRadius, thickness, bg, edge)
		r = r.Inset(NewUniformInsets(thickness + 0.5))
	}
	r = r.Inset(NewSymmetricInsets(b.HorizontalMargin(), b.VerticalMargin()))
	DrawLabel(canvas, r, b.HAlign, b.VAlign, b.cache.Text(b.Text, b.Font), fg, b.Drawable, b.Side, b.Gap, !b.Enabled())
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
func (b *Button) DefaultMouseDown(_ Point, _, _ int, _ Modifiers) bool {
	b.Pressed = true
	b.MarkForRedraw()
	return true
}

// DefaultMouseDrag provides the default mouse drag handling.
func (b *Button) DefaultMouseDrag(where Point, _ int, _ Modifiers) bool {
	if pressed := where.In(b.ContentRect(false)); pressed != b.Pressed {
		b.Pressed = pressed
		b.MarkForRedraw()
	}
	return true
}

// DefaultMouseUp provides the default mouse up handling.
func (b *Button) DefaultMouseUp(where Point, _ int, _ Modifiers) bool {
	b.Pressed = false
	b.MarkForRedraw()
	if where.In(b.ContentRect(false)) {
		b.SetSelected(true)
		if b.ClickCallback != nil {
			b.ClickCallback()
		}
	}
	return true
}

// DefaultKeyDown provides the default key down handling.
func (b *Button) DefaultKeyDown(keyCode KeyCode, mod Modifiers, _ bool) bool {
	if IsControlAction(keyCode, mod) {
		b.Click()
		return true
	}
	return false
}

// DefaultUpdateCursor provides the default cursor for buttons.
func (b *Button) DefaultUpdateCursor(_ Point) *Cursor {
	if !b.Enabled() {
		return ArrowCursor()
	}
	return PointingCursor()
}
