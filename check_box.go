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

	"github.com/richardwilkes/toolbox/xmath"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/check"
	"github.com/richardwilkes/unison/enums/paintstyle"
	"github.com/richardwilkes/unison/enums/side"
)

// DefaultCheckBoxTheme holds the default CheckBoxTheme values for CheckBoxes. Modifying this data will not alter
// existing CheckBoxes, but will alter any CheckBoxes created in the future.
var DefaultCheckBoxTheme = CheckBoxTheme{
	Font:               SystemFont,
	OnBackgroundInk:    &PrimaryTheme.OnSurface,
	EdgeInk:            &PrimaryTheme.Outline,
	SelectionInk:       &PrimaryTheme.Primary,
	OnSelectionInk:     &PrimaryTheme.OnPrimary,
	ControlInk:         &PrimaryTheme.SurfaceAbove,
	OnControlInk:       &PrimaryTheme.OnSurface,
	Gap:                3,
	CornerRadius:       4,
	ClickAnimationTime: 100 * time.Millisecond,
	HAlign:             align.Start,
	VAlign:             align.Middle,
	Side:               side.Left,
}

// CheckBoxTheme holds theming data for a CheckBox.
type CheckBoxTheme struct {
	Font               Font
	OnBackgroundInk    Ink
	EdgeInk            Ink
	SelectionInk       Ink
	OnSelectionInk     Ink
	ControlInk         Ink
	OnControlInk       Ink
	Gap                float32
	CornerRadius       float32
	ClickAnimationTime time.Duration
	HAlign             align.Enum
	VAlign             align.Enum
	Side               side.Enum
}

// CheckBox represents a clickable checkbox with an optional label.
type CheckBox struct {
	Panel
	CheckBoxTheme
	ClickCallback func()
	Drawable      Drawable
	Text          string
	cache         TextCache
	State         check.Enum
	Pressed       bool
}

// NewCheckBox creates a new checkbox.
func NewCheckBox() *CheckBox {
	c := &CheckBox{CheckBoxTheme: DefaultCheckBoxTheme}
	c.Self = c
	c.SetFocusable(true)
	c.SetSizer(c.DefaultSizes)
	c.DrawCallback = c.DefaultDraw
	c.GainedFocusCallback = c.DefaultFocusGained
	c.LostFocusCallback = c.MarkForRedraw
	c.MouseDownCallback = c.DefaultMouseDown
	c.MouseDragCallback = c.DefaultMouseDrag
	c.MouseUpCallback = c.DefaultMouseUp
	c.KeyDownCallback = c.DefaultKeyDown
	c.UpdateCursorCallback = c.DefaultUpdateCursor
	return c
}

// DefaultFocusGained provides the default focus gained handling.
func (c *CheckBox) DefaultFocusGained() {
	c.ScrollIntoView()
	c.MarkForRedraw()
}

// DefaultSizes provides the default sizing.
func (c *CheckBox) DefaultSizes(hint Size) (minSize, prefSize, maxSize Size) {
	prefSize = c.boxAndLabelSize()
	if border := c.Border(); border != nil {
		prefSize = prefSize.Add(border.Insets().Size())
	}
	prefSize = prefSize.Ceil().ConstrainForHint(hint)
	return prefSize, prefSize, MaxSize(prefSize)
}

func (c *CheckBox) boxAndLabelSize() Size {
	boxSize := c.boxSize()
	if c.Drawable == nil && c.Text == "" {
		return Size{Width: boxSize, Height: boxSize}
	}
	size := LabelSize(c.cache.Text(c.Text, c.Font), c.Drawable, c.Side, c.Gap)
	size.Width += c.Gap + boxSize
	if size.Height < boxSize {
		size.Height = boxSize
	}
	return size
}

func (c *CheckBox) boxSize() float32 {
	return xmath.Ceil(c.Font.Baseline())
}

// DefaultDraw provides the default drawing.
func (c *CheckBox) DefaultDraw(canvas *Canvas, _ Rect) {
	contentRect := c.ContentRect(false)
	rect := contentRect
	size := c.boxAndLabelSize()
	switch c.HAlign {
	case align.Middle, align.Fill:
		rect.X = xmath.Floor(rect.X + (rect.Width-size.Width)/2)
	case align.End:
		rect.X += rect.Width - size.Width
	default: // align.Start
	}
	switch c.VAlign {
	case align.Middle, align.Fill:
		rect.Y = xmath.Floor(rect.Y + (rect.Height-size.Height)/2)
	case align.End:
		rect.Y += rect.Height - size.Height
	default: // align.Start
	}
	rect.Size = size
	boxSize := c.boxSize()
	if c.Drawable != nil || c.Text != "" {
		r := rect
		r.X += boxSize + c.Gap
		r.Width -= boxSize + c.Gap
		DrawLabel(canvas, r, c.HAlign, c.VAlign, c.cache.Text(c.Text, c.Font), c.OnBackgroundInk, c.Drawable,
			c.Side, c.Gap, !c.Enabled())
	}
	if rect.Height > boxSize {
		rect.Y += xmath.Floor((rect.Height - boxSize) / 2)
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
	edge := c.EdgeInk
	thickness := float32(1)
	if c.Focused() {
		thickness++
		edge = c.SelectionInk
	}
	DrawRoundedRectBase(canvas, rect, c.CornerRadius, thickness, bg, edge)
	rect = rect.Inset(NewUniformInsets(0.5))
	if c.State == check.Off {
		return
	}
	paint := fg.Paint(canvas, contentRect, paintstyle.Stroke)
	paint.SetStrokeWidth(2)
	if !c.Enabled() {
		paint.SetColorFilter(Grayscale30Filter())
	}
	if c.State == check.On {
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
	if c.State == check.On {
		c.State = check.Off
	} else {
		c.State = check.On
	}
}

// DefaultMouseDown provides the default mouse down handling.
func (c *CheckBox) DefaultMouseDown(_ Point, _, _ int, _ Modifiers) bool {
	c.Pressed = true
	c.MarkForRedraw()
	return true
}

// DefaultMouseDrag provides the default mouse drag handling.
func (c *CheckBox) DefaultMouseDrag(where Point, _ int, _ Modifiers) bool {
	if pressed := where.In(c.ContentRect(false)); pressed != c.Pressed {
		c.Pressed = pressed
		c.MarkForRedraw()
	}
	return true
}

// DefaultMouseUp provides the default mouse up handling.
func (c *CheckBox) DefaultMouseUp(where Point, _ int, _ Modifiers) bool {
	c.Pressed = false
	c.MarkForRedraw()
	if where.In(c.ContentRect(false)) {
		c.updateState()
		if c.ClickCallback != nil {
			c.ClickCallback()
		}
	}
	return true
}

// DefaultKeyDown provides the default key down handling.
func (c *CheckBox) DefaultKeyDown(keyCode KeyCode, mod Modifiers, _ bool) bool {
	if IsControlAction(keyCode, mod) {
		c.Click()
		return true
	}
	return false
}

// DefaultUpdateCursor provides the default cursor for check boxes.
func (c *CheckBox) DefaultUpdateCursor(_ Point) *Cursor {
	if !c.Enabled() {
		return ArrowCursor()
	}
	return PointingCursor()
}
