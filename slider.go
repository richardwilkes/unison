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
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/unison/enums/paintstyle"
)

// DefaultSliderTheme holds the default SliderTheme values for Sliders. Modifying this data will not alter existing
// Sliders, but will alter any Sliders created in the future.
var DefaultSliderTheme = SliderTheme{
	FillInk:       ThemeSurface,
	EdgeInk:       ThemeSurfaceEdge,
	MarkerColor:   ThemeFocus,
	MarkerSize:    12,
	CornerRadius:  8,
	EdgeThickness: 1,
}

// SliderTheme holds theming data for a Slider.
type SliderTheme struct {
	FillInk       Ink
	EdgeInk       Ink
	MarkerColor   ColorProvider
	MarkerSize    float32
	CornerRadius  float32
	EdgeThickness float32
}

// Slider provides a control for setting a value in a range.
type Slider struct {
	SliderTheme
	ValueSnapCallback    func(value float32) float32
	ValueChangedCallback func()
	Panel
	minimum float32
	maximum float32
	value   float32
	pressed bool
}

// NewSlider creates a new Slider.
func NewSlider(minimum, maximum, value float32) *Slider {
	if minimum > maximum {
		minimum, maximum = maximum, minimum
	}
	if value < minimum {
		value = minimum
	} else if value > maximum {
		value = maximum
	}
	s := &Slider{
		SliderTheme: DefaultSliderTheme,
		minimum:     minimum,
		maximum:     maximum,
		value:       value,
	}
	s.Self = s
	s.SetSizer(s.DefaultSizes)
	s.DrawCallback = s.DefaultDraw
	s.MouseDownCallback = s.DefaultMouseDown
	s.MouseDragCallback = s.DefaultMouseDrag
	s.MouseUpCallback = s.DefaultMouseUp
	return s
}

// Value returns the current value of the slider.
func (s *Slider) Value() float32 {
	return s.value
}

// SetValue sets the current value.
func (s *Slider) SetValue(value float32) {
	if s.ValueSnapCallback != nil {
		value = s.ValueSnapCallback(value)
	}
	if value < s.minimum {
		value = s.minimum
	} else if value > s.maximum {
		value = s.maximum
	}
	if s.value != value {
		s.value = value
		if s.ValueChangedCallback != nil {
			xos.SafeCall(s.ValueChangedCallback, nil)
		}
		s.MarkForRedraw()
	}
}

// Minimum returns the minimum value of the slider.
func (s *Slider) Minimum() float32 {
	return s.minimum
}

// SetMinimum sets the minimum value.
func (s *Slider) SetMinimum(value float32) {
	if s.minimum != value {
		s.minimum = value
		if s.maximum < s.minimum {
			s.maximum = s.minimum
		}
		if s.value < s.minimum {
			s.value = s.minimum
		}
		s.MarkForRedraw()
	}
}

// Maximum returns the maximum value of the slider.
func (s *Slider) Maximum() float32 {
	return s.maximum
}

// SetMaximum sets the maximum value.
func (s *Slider) SetMaximum(value float32) {
	if s.maximum != value {
		s.maximum = value
		if s.minimum > s.maximum {
			s.minimum = s.maximum
		}
		if s.value > s.maximum {
			s.value = s.maximum
		}
		s.MarkForRedraw()
	}
}

// DefaultSizes provides the default sizing.
func (s *Slider) DefaultSizes(hint geom.Size) (minSize, prefSize, maxSize geom.Size) {
	minSize.Width = s.MarkerSize
	minSize.Height = s.MarkerSize
	prefSize.Width = s.MarkerSize + 100 + s.EdgeThickness*2
	prefSize.Height = s.MarkerSize
	maxSize.Width = DefaultMaxSize
	maxSize.Height = s.MarkerSize
	if border := s.Border(); border != nil {
		insets := border.Insets().Size()
		minSize = minSize.Add(insets)
		prefSize = prefSize.Add(insets)
		maxSize = maxSize.Add(insets)
	}
	return minSize.Ceil(), prefSize.Ceil().ConstrainForHint(hint), MaxSize(maxSize.Ceil())
}

// DefaultDrawBackground provides the default background drawing.
func (s *Slider) DefaultDrawBackground(canvas *Canvas, bounds geom.Rect) {
	canvas.DrawRoundedRect(bounds, s.CornerRadius, s.CornerRadius, s.FillInk.Paint(canvas, bounds, paintstyle.Fill))
}

// DefaultDraw provides the default drawing.
func (s *Slider) DefaultDraw(canvas *Canvas, _ geom.Rect) {
	bounds := s.ContentRect(false)
	minSize := s.MarkerSize + s.EdgeThickness*2
	if bounds.Width >= bounds.Height {
		if bounds.Width < minSize {
			return
		}
		bounds.X += s.EdgeThickness
		bounds.Width -= s.EdgeThickness * 2
	} else {
		if bounds.Height < minSize {
			return
		}
		bounds.Y += s.EdgeThickness
		bounds.Height -= s.EdgeThickness * 2
	}
	canvas.DrawRoundedRect(bounds, s.CornerRadius, s.CornerRadius, s.FillInk.Paint(canvas, bounds, paintstyle.Fill))
	edgePaint := s.EdgeInk.Paint(canvas, bounds, paintstyle.Stroke)
	edgePaint.SetStrokeWidth(s.EdgeThickness)
	canvas.DrawRoundedRect(bounds, s.CornerRadius, s.CornerRadius, edgePaint)
	center := bounds.Center()
	var multiplier float32
	if meterRange := s.maximum - s.minimum; meterRange > 0 {
		multiplier = (s.value - s.minimum) / meterRange
	}
	if bounds.Width >= bounds.Height {
		center.X = bounds.X + s.MarkerSize/2 + (bounds.Width-s.MarkerSize)*multiplier
	} else {
		center.Y = bounds.Y + s.MarkerSize/2 + (bounds.Height-s.MarkerSize)*multiplier
	}
	var inner, outer *Paint
	if s.pressed {
		outer = s.MarkerColor.GetColor().On().Paint(canvas, bounds, paintstyle.Stroke)
		inner = s.MarkerColor.Paint(canvas, bounds, paintstyle.Fill)
	} else {
		outer = s.MarkerColor.GetColor().On().Paint(canvas, bounds, paintstyle.Stroke)
		outer.SetStrokeWidth(2)
		inner = s.MarkerColor.Paint(canvas, bounds, paintstyle.Stroke)
		inner.SetStrokeWidth(1)
	}
	canvas.DrawCircle(center.X, center.Y, s.MarkerSize/2-2, outer)
	canvas.DrawCircle(center.X, center.Y, s.MarkerSize/2-2, inner)
}

// DefaultMouseDown provides the default mouse down handling.
func (s *Slider) DefaultMouseDown(where geom.Point, button, _ int, mod Modifiers) bool {
	s.pressed = true
	s.DefaultMouseDrag(where, button, mod)
	s.MarkForRedraw()
	return true
}

// DefaultMouseDrag provides the default mouse drag handling.
func (s *Slider) DefaultMouseDrag(where geom.Point, _ int, _ Modifiers) bool {
	bounds := s.ContentRect(false)
	var minimum, maximum, pos float32
	inset := s.EdgeThickness + s.MarkerSize/2
	if bounds.Width >= bounds.Height {
		minimum = bounds.X + inset
		maximum = bounds.Right() - inset
		pos = where.X
	} else {
		minimum = bounds.Y + inset
		maximum = bounds.Bottom() - inset
		pos = where.Y
	}
	if maximum < minimum {
		return true
	}
	if minimum == maximum {
		s.SetValue(s.minimum)
		return true
	}
	ratio := (min(max(pos, minimum), maximum) - minimum) / (maximum - minimum)
	s.SetValue(ratio * (s.maximum - s.minimum))
	return true
}

// DefaultMouseUp provides the default mouse up handling.
func (s *Slider) DefaultMouseUp(where geom.Point, button int, mod Modifiers) bool {
	s.DefaultMouseDrag(where, button, mod)
	s.pressed = false
	s.MarkForRedraw()
	return true
}
