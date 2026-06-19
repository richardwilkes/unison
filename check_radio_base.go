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
	"github.com/richardwilkes/toolbox/v2/xmath"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/mod"
)

type checkRadioBase struct {
	ClickCallback func()
	baseTheme     *RadioButtonTheme
	updateState   func()
	drawMark      func(canvas *Canvas, rect geom.Rect, thickness float32, fg, bg, edge Ink)
	Drawable      Drawable
	Text          *Text
	Panel
	Pressed bool
}

func (c *checkRadioBase) commonInit() {
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
}

// SetTitle sets the title to the specified text. The theme's TextDecoration will be used, so any changes you want to
// make to it should be done before calling this method. Alternatively, you can directly set the .Text field.
func (c *checkRadioBase) SetTitle(text string) {
	c.Text = NewText(text, &c.baseTheme.TextDecoration)
}

// DefaultFocusGained provides the default focus gained handling.
func (c *checkRadioBase) DefaultFocusGained() {
	c.ScrollIntoView()
	c.MarkForRedraw()
}

// DefaultSizes provides the default sizing.
func (c *checkRadioBase) DefaultSizes(hint geom.Size) (minSize, prefSize, maxSize geom.Size) {
	prefSize = c.iconAndLabelSize()
	if b := c.Border(); b != nil {
		prefSize = prefSize.Add(b.Insets().Size())
	}
	prefSize = prefSize.Ceil().ConstrainForHint(hint)
	return prefSize, prefSize, MaxSize(prefSize)
}

func (c *checkRadioBase) iconAndLabelSize() geom.Size {
	mark := c.markSize()
	if c.Drawable == nil && c.Text.Empty() {
		return geom.NewSize(mark, mark)
	}
	size, _ := LabelContentSizes(c.Text, c.Drawable, c.baseTheme.Font, c.baseTheme.Side, c.baseTheme.Gap)
	size.Width += c.baseTheme.Gap + mark
	if size.Height < mark {
		size.Height = mark
	}
	return size
}

func (c *checkRadioBase) markSize() float32 {
	return xmath.Ceil(c.baseTheme.Font.Baseline())
}

// Click makes the checkbox behave as if a user clicked on it.
func (c *checkRadioBase) Click() {
	c.updateState()
	wasPressed := c.Pressed
	c.Pressed = true
	c.MarkForRedraw()
	c.FlushDrawing()
	c.Pressed = wasPressed
	time.Sleep(c.baseTheme.ClickAnimationTime)
	c.MarkForRedraw()
	SafeCall(c.ClickCallback)
}

// DefaultMouseDown provides the default mouse down handling.
func (c *checkRadioBase) DefaultMouseDown(_ geom.Point, _, _ int, _ mod.Modifiers) bool {
	c.Pressed = true
	c.MarkForRedraw()
	return true
}

// DefaultMouseDrag provides the default mouse drag handling.
func (c *checkRadioBase) DefaultMouseDrag(where geom.Point, _ int, _ mod.Modifiers) bool {
	if now := where.In(c.ContentRect(false)); now != c.Pressed {
		c.Pressed = now
		c.MarkForRedraw()
	}
	return true
}

// DefaultMouseUp provides the default mouse up handling.
func (c *checkRadioBase) DefaultMouseUp(where geom.Point, _ int, _ mod.Modifiers) bool {
	c.Pressed = false
	c.MarkForRedraw()
	if where.In(c.ContentRect(false)) {
		c.updateState()
		SafeCall(c.ClickCallback)
	}
	return true
}

// DefaultKeyDown provides the default key down handling.
func (c *checkRadioBase) DefaultKeyDown(keyCode KeyCode, mods mod.Modifiers, _repeat bool) bool {
	if IsControlAction(keyCode, mods) {
		c.Click()
		return true
	}
	return false
}

// DefaultUpdateCursor provides the default cursor for check boxes.
func (c *checkRadioBase) DefaultUpdateCursor(_ geom.Point) *Cursor {
	if !c.Enabled() {
		return ArrowCursor()
	}
	return PointingCursor()
}

// DefaultDraw provides the default drawing.
func (c *checkRadioBase) DefaultDraw(canvas *Canvas, _ geom.Rect) {
	rect := c.ContentRect(false)
	size := c.iconAndLabelSize()
	switch c.baseTheme.HAlign {
	case align.Middle, align.Fill:
		rect.X = xmath.Floor(rect.X + (rect.Width-size.Width)/2)
	case align.End:
		rect.X += rect.Width - size.Width
	}
	switch c.baseTheme.VAlign {
	case align.Middle, align.Fill:
		rect.Y = xmath.Floor(rect.Y + (rect.Height-size.Height)/2)
	case align.End:
		rect.Y += rect.Height - size.Height
	}
	rect.Size = size
	markSize := c.markSize()
	if c.Drawable != nil || !c.Text.Empty() {
		r := rect
		r.X += markSize + c.baseTheme.Gap
		r.Width -= markSize + c.baseTheme.Gap
		defer c.Text.RestoreDecorations(c.Text.AdjustDecorations(func(d *TextDecoration) {
			d.BackgroundInk = nil
			d.OnBackgroundInk = c.baseTheme.OnBackgroundInk
		}))
		DrawLabel(canvas, r, c.baseTheme.HAlign, c.baseTheme.VAlign, c.baseTheme.Font, c.Text, c.baseTheme.OnBackgroundInk, nil, c.Drawable, c.baseTheme.Side, c.baseTheme.Gap,
			!c.Enabled())
	}
	if rect.Height > markSize {
		rect.Y += xmath.Floor((rect.Height - markSize) / 2)
	}
	rect.Width = markSize
	rect.Height = markSize
	var fg, bg Ink
	switch {
	case c.Pressed:
		bg = c.baseTheme.SelectionInk
		fg = c.baseTheme.OnSelectionInk
	default:
		bg = c.baseTheme.BackgroundInk
		fg = c.baseTheme.OnBackgroundInk
	}
	edge := c.baseTheme.EdgeInk
	thickness := float32(1)
	if c.Focused() {
		thickness++
		edge = c.baseTheme.SelectionInk
	}
	c.drawMark(canvas, rect, thickness, fg, bg, edge)
}
