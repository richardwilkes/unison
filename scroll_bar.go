// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

// DefaultScrollBarTheme holds the default ScrollBarTheme values for ScrollBars. Modifying this data will not alter
// existing ScrollBars, but will alter any ScrollBars created in the future.
var DefaultScrollBarTheme = ScrollBarTheme{
	EdgeInk:          ScrollEdgeColor,
	ThumbInk:         ScrollColor,
	RolloverInk:      ScrollRolloverColor,
	MinimumThickness: 16,
	MinimumThumb:     16,
	ThumbIndent:      3,
	CornerRadius:     8,
}

// ScrollBarTheme holds theming data for a ScrollBar.
type ScrollBarTheme struct {
	EdgeInk          Ink
	ThumbInk         Ink
	RolloverInk      Ink
	MinimumThickness float32
	MinimumThumb     float32
	ThumbIndent      float32
	CornerRadius     float32
}

// ScrollBar holds the data necessary for tracking a scroll bar's state.
type ScrollBar struct {
	Panel
	ScrollBarTheme
	ChangedCallback func()
	value           float32
	extent          float32
	maximum         float32
	dragOffset      float32
	horizontal      bool
	overThumb       bool
	trackingThumb   bool
}

// NewScrollBar creates a new scroll bar.
func NewScrollBar(horizontal bool) *ScrollBar {
	s := &ScrollBar{
		ScrollBarTheme: DefaultScrollBarTheme,
		horizontal:     horizontal,
	}
	s.Self = s
	s.SetSizer(s.DefaultSizes)
	s.DrawCallback = s.DefaultDraw
	s.MouseDownCallback = s.DefaultMouseDown
	s.MouseMoveCallback = s.DefaultMouseMove
	s.MouseDragCallback = s.DefaultMouseDrag
	s.MouseUpCallback = s.DefaultMouseUp
	s.MouseEnterCallback = s.DefaultMouseEnter
	s.MouseExitCallback = s.DefaultMouseExit
	return s
}

// Horizontal returns true if this is a horizontal scroll bar.
func (s *ScrollBar) Horizontal() bool {
	return s.horizontal
}

// Vertical returns true if this is a vertical scroll bar.
func (s *ScrollBar) Vertical() bool {
	return !s.horizontal
}

// Value returns the current value.
func (s *ScrollBar) Value() float32 {
	return s.value
}

// MaxValue returns the maximum value that can be set without adjusting the extent or max.
func (s *ScrollBar) MaxValue() float32 {
	return max(s.maximum-s.extent, 0)
}

// Extent returns the amount of space representing the visible content area.
func (s *ScrollBar) Extent() float32 {
	return s.extent
}

// Max returns the amount of space representing the whole content area.
func (s *ScrollBar) Max() float32 {
	return s.maximum
}

// SetRange sets the value, extent and max values.
func (s *ScrollBar) SetRange(value, extent, maximum float32) {
	if value < 0 {
		value = 0
	}
	if maximum < 0 {
		maximum = 0
	}
	if extent > maximum {
		extent = maximum
	}
	if value+extent > maximum {
		value = maximum - extent
	}
	if value != s.value || extent != s.extent || maximum != s.maximum {
		s.value = value
		s.extent = extent
		s.maximum = maximum
		s.MarkForRedraw()
		if s.ChangedCallback != nil {
			s.ChangedCallback()
		}
	}
}

// Thumb returns the location of the thumb.
func (s *ScrollBar) Thumb() Rect {
	if s.maximum == 0 {
		return Rect{}
	}
	r := s.ContentRect(false)
	if s.horizontal {
		start := r.Width * (s.value / s.maximum)
		size := r.Width * (s.extent / s.maximum)
		if size < s.MinimumThumb {
			size = s.MinimumThumb
		}
		return NewRect(start, s.ThumbIndent, size, r.Height-2*s.ThumbIndent)
	}
	start := r.Height * (s.value / s.maximum)
	size := r.Height * (s.extent / s.maximum)
	if size < s.MinimumThumb {
		size = s.MinimumThumb
	}
	return NewRect(s.ThumbIndent, start, r.Width-2*s.ThumbIndent, size)
}

// DefaultSizes provides the default sizing.
func (s *ScrollBar) DefaultSizes(_ Size) (minSize, prefSize, maxSize Size) {
	minSize.Width = s.MinimumThickness
	minSize.Height = s.MinimumThickness
	if s.horizontal {
		prefSize.Width = s.MinimumThickness * 2
		prefSize.Height = s.MinimumThickness
		maxSize.Width = DefaultMaxSize
		maxSize.Height = s.MinimumThickness
	} else {
		prefSize.Width = s.MinimumThickness
		prefSize.Height = s.MinimumThickness * 2
		maxSize.Width = s.MinimumThickness
		maxSize.Height = DefaultMaxSize
	}
	return minSize, prefSize, maxSize
}

// DefaultDraw provides the default drawing.
func (s *ScrollBar) DefaultDraw(gc *Canvas, _ Rect) {
	if thumb := s.Thumb(); thumb.Width > 0 && thumb.Height > 0 {
		var ink Ink
		if s.overThumb {
			ink = s.RolloverInk
		} else {
			ink = s.ThumbInk
		}
		gc.DrawRoundedRect(thumb, s.CornerRadius, s.CornerRadius, ink.Paint(gc, thumb, Fill))
		gc.DrawRoundedRect(thumb, s.CornerRadius, s.CornerRadius, s.EdgeInk.Paint(gc, thumb, Stroke))
	}
}

// DefaultMouseDown provides the default mouse down handling.
func (s *ScrollBar) DefaultMouseDown(where Point, _, _ int, _ Modifiers) bool {
	thumb := s.Thumb()
	if !thumb.ContainsPoint(where) {
		s.dragOffset = 0
		s.adjustValueForPoint(where)
		thumb = s.Thumb()
	}
	if s.horizontal {
		s.dragOffset = thumb.X - where.X
	} else {
		s.dragOffset = thumb.Y - where.Y
	}
	s.overThumb = true
	s.trackingThumb = true
	s.MarkForRedraw()
	return true
}

// DefaultMouseUp provides the default mouse up handling.
func (s *ScrollBar) DefaultMouseUp(where Point, _ int, _ Modifiers) bool {
	s.trackingThumb = false
	s.checkOverThumb(where)
	return true
}

// DefaultMouseEnter provides the default mouse enter handling.
func (s *ScrollBar) DefaultMouseEnter(where Point, _ Modifiers) bool {
	if !s.trackingThumb {
		s.checkOverThumb(where)
	}
	return true
}

// DefaultMouseExit provides the default mouse enter handling.
func (s *ScrollBar) DefaultMouseExit() bool {
	if !s.trackingThumb && s.overThumb {
		s.overThumb = false
		s.MarkForRedraw()
	}
	return true
}

// DefaultMouseMove provides the default mouse move handling.
func (s *ScrollBar) DefaultMouseMove(where Point, _ Modifiers) bool {
	s.checkOverThumb(where)
	return true
}

// DefaultMouseDrag provides the default mouse drag handling.
func (s *ScrollBar) DefaultMouseDrag(where Point, _ int, _ Modifiers) bool {
	s.adjustValueForPoint(where)
	s.MarkForRedraw()
	return true
}

func (s *ScrollBar) adjustValueForPoint(pt Point) {
	r := s.ContentRect(false)
	thumb := s.Thumb()
	var pos, maximum float32
	if s.horizontal {
		pos = pt.X
		maximum = r.Width - thumb.Width
	} else {
		pos = pt.Y
		maximum = r.Height - thumb.Height
	}
	if s.maximum <= s.extent {
		s.SetRange(0, s.extent, s.maximum)
	} else {
		s.SetRange((s.maximum-s.extent)*(pos+s.dragOffset)/maximum, s.extent, s.maximum)
	}
}

func (s *ScrollBar) checkOverThumb(pt Point) {
	was := s.overThumb
	s.overThumb = s.Thumb().ContainsPoint(pt)
	if was != s.overThumb {
		s.MarkForRedraw()
	}
}
