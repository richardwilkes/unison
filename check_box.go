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

// DefaultCheckBoxTheme holds the default CheckBoxTheme values for CheckBoxes. Modifying this data will not alter
// existing CheckBoxes, but will alter any CheckBoxes created in the future.
var DefaultCheckBoxTheme = CheckBoxTheme{
	Font:               SystemFont,
	OnBackgroundInk:    OnBackgroundColor,
	EdgeInk:            ControlEdgeColor,
	SelectionInk:       SelectionColor,
	OnSelectionInk:     OnSelectionColor,
	ControlInk:         ControlColor,
	OnControlInk:       OnControlColor,
	Gap:                3,
	CornerRadius:       4,
	ClickAnimationTime: 100 * time.Millisecond,
	HAlign:             StartAlignment,
	VAlign:             MiddleAlignment,
	Side:               LeftSide,
}

// CheckBoxTheme holds theming data for a CheckBox.
type CheckBoxTheme struct {
	Font               FontProvider
	OnBackgroundInk    Ink
	EdgeInk            Ink
	SelectionInk       Ink
	OnSelectionInk     Ink
	ControlInk         Ink
	OnControlInk       Ink
	Gap                float32
	CornerRadius       float32
	ClickAnimationTime time.Duration
	HAlign             Alignment
	VAlign             Alignment
	Side               Side
}

// CheckBox represents a clickable checkbox with an optional label.
type CheckBox struct {
	Panel
	CheckBoxTheme
	ClickCallback func()
	Drawable      Drawable
	Text          string
	State         CheckState
	Pressed       bool
}

// NewCheckBox creates a new checkbox.
func NewCheckBox() *CheckBox {
	c := &CheckBox{CheckBoxTheme: DefaultCheckBoxTheme}
	c.Self = c
	c.SetFocusable(true)
	c.SetSizer(c.DefaultSizes)
	c.DrawCallback = c.DefaultDraw
	c.GainedFocusCallback = c.MarkForRedraw
	c.LostFocusCallback = c.MarkForRedraw
	c.MouseDownCallback = c.DefaultMouseDown
	c.MouseDragCallback = c.DefaultMouseDrag
	c.MouseUpCallback = c.DefaultMouseUp
	c.KeyDownCallback = c.DefaultKeyDown
	return c
}

// DefaultSizes provides the default sizing.
func (c *CheckBox) DefaultSizes(hint geom32.Size) (min, pref, max geom32.Size) {
	pref = c.boxAndLabelSize()
	if border := c.Border(); border != nil {
		pref.AddInsets(border.Insets())
	}
	pref.GrowToInteger()
	pref.ConstrainForHint(hint)
	return pref, pref, MaxSize(pref)
}

func (c *CheckBox) boxAndLabelSize() geom32.Size {
	boxSize := c.boxSize()
	if c.Drawable == nil && c.Text == "" {
		return geom32.Size{Width: boxSize, Height: boxSize}
	}
	size := LabelSize(c.Text, c.Font, c.Drawable, c.Side, c.Gap)
	size.Width += c.Gap + boxSize
	if size.Height < boxSize {
		size.Height = boxSize
	}
	return size
}

func (c *CheckBox) boxSize() float32 {
	return mathf32.Ceil(c.Font.ResolvedFont().Baseline())
}

// DefaultDraw provides the default drawing.
func (c *CheckBox) DefaultDraw(canvas *Canvas, dirty geom32.Rect) {
	contentRect := c.ContentRect(false)
	rect := contentRect
	size := c.boxAndLabelSize()
	switch c.HAlign {
	case MiddleAlignment, FillAlignment:
		rect.X = mathf32.Floor(rect.X + (rect.Width-size.Width)/2)
	case EndAlignment:
		rect.X += rect.Width - size.Width
	default: // StartAlignment
	}
	switch c.VAlign {
	case MiddleAlignment, FillAlignment:
		rect.Y = mathf32.Floor(rect.Y + (rect.Height-size.Height)/2)
	case EndAlignment:
		rect.Y += rect.Height - size.Height
	default: // StartAlignment
	}
	rect.Size = size
	boxSize := c.boxSize()
	if c.Drawable != nil || c.Text != "" {
		r := rect
		r.X += boxSize + c.Gap
		r.Width -= boxSize + c.Gap
		DrawLabel(canvas, r, c.HAlign, c.VAlign, c.Text, c.Font, c.OnBackgroundInk, c.Drawable, c.Side, c.Gap,
			!c.Enabled())
	}
	if rect.Height > boxSize {
		rect.Y += mathf32.Floor((rect.Height - boxSize) / 2)
	}
	rect.Width = boxSize
	rect.Height = boxSize
	var fg, bg Ink
	switch {
	case c.Pressed:
		bg = c.SelectionInk
		fg = c.OnSelectionInk
	default:
		bg = c.ControlInk
		fg = c.OnControlInk
	}
	thickness := float32(1)
	if c.Focused() {
		thickness++
	}
	DrawRoundedRectBase(canvas, rect, c.CornerRadius, thickness, bg, c.EdgeInk)
	rect.InsetUniform(0.5)
	if c.State == OffCheckState {
		return
	}
	paint := fg.Paint(canvas, contentRect, Stroke)
	paint.SetStrokeWidth(2)
	if !c.Enabled() {
		paint.SetColorFilter(Grayscale30PercentFilter())
	}
	if c.State == OnCheckState {
		path := NewPath()
		path.MoveTo(rect.X+rect.Width*0.25, rect.Y+rect.Height*0.55)
		path.LineTo(rect.X+rect.Width*0.45, rect.Y+rect.Height*0.7)
		path.LineTo(rect.X+rect.Width*0.75, rect.Y+rect.Height*0.3)
		canvas.DrawPath(path, paint)
	} else {
		canvas.DrawLine(rect.X+rect.Width*0.25, rect.Y+rect.Height*0.5, rect.X+rect.Width*0.7, rect.Y+rect.Height*0.5,
			paint)
	}
}

// Click makes the checkbox behave as if a user clicked on it.
func (c *CheckBox) Click() {
	c.updateState()
	pressed := c.Pressed
	c.Pressed = true
	c.MarkForRedraw()
	c.FlushDrawing()
	c.Pressed = pressed
	time.Sleep(c.ClickAnimationTime)
	c.MarkForRedraw()
	if c.ClickCallback != nil {
		c.ClickCallback()
	}
}

func (c *CheckBox) updateState() {
	if c.State == OnCheckState {
		c.State = OffCheckState
	} else {
		c.State = OnCheckState
	}
}

// DefaultMouseDown provides the default mouse down handling.
func (c *CheckBox) DefaultMouseDown(where geom32.Point, button, clickCount int, mod Modifiers) bool {
	c.Pressed = true
	c.MarkForRedraw()
	return true
}

// DefaultMouseDrag provides the default mouse drag handling.
func (c *CheckBox) DefaultMouseDrag(where geom32.Point, button int, mod Modifiers) bool {
	rect := c.ContentRect(false)
	pressed := rect.ContainsPoint(where)
	if c.Pressed != pressed {
		c.Pressed = pressed
		c.MarkForRedraw()
	}
	return true
}

// DefaultMouseUp provides the default mouse up handling.
func (c *CheckBox) DefaultMouseUp(where geom32.Point, button int, mod Modifiers) bool {
	c.Pressed = false
	c.MarkForRedraw()
	rect := c.ContentRect(false)
	if rect.ContainsPoint(where) {
		c.updateState()
		if c.ClickCallback != nil {
			c.ClickCallback()
		}
	}
	return true
}

// DefaultKeyDown provides the default key down handling.
func (c *CheckBox) DefaultKeyDown(keyCode KeyCode, mod Modifiers, repeat bool) bool {
	if IsControlAction(keyCode, mod) {
		c.Click()
		return true
	}
	return false
}
