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
	"github.com/richardwilkes/toolbox/xmath/geom32"
	"github.com/richardwilkes/toolbox/xmath/mathf32"
)

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
	max             float32
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
	return mathf32.Max(s.max-s.extent, 0)
}

// Extent returns the amount of space representing the visible content area.
func (s *ScrollBar) Extent() float32 {
	return s.extent
}

// Max returns the amount of space representing the whole content area.
func (s *ScrollBar) Max() float32 {
	return s.max
}

// SetRange sets the value, extent and max values.
func (s *ScrollBar) SetRange(value, extent, max float32) {
	if value < 0 {
		value = 0
	}
	if max < 0 {
		max = 0
	}
	if extent > max {
		extent = max
	}
	if value+extent > max {
		value = max - extent
	}
	if value != s.value || extent != s.extent || max != s.max {
		s.value = value
		s.extent = extent
		s.max = max
		s.MarkForRedraw()
		if s.ChangedCallback != nil {
			s.ChangedCallback()
		}
	}
}

// Thumb returns the location of the thumb.
func (s *ScrollBar) Thumb() geom32.Rect {
	if s.max == 0 {
		return geom32.Rect{}
	}
	r := s.ContentRect(false)
	if s.horizontal {
		start := r.Width * (s.value / s.max)
		size := r.Width * (s.extent / s.max)
		if size < s.MinimumThumb {
			size = s.MinimumThumb
		}
		return geom32.NewRect(start, s.ThumbIndent, size, r.Height-2*s.ThumbIndent)
	}
	start := r.Height * (s.value / s.max)
	size := r.Height * (s.extent / s.max)
	if size < s.MinimumThumb {
		size = s.MinimumThumb
	}
	return geom32.NewRect(s.ThumbIndent, start, r.Width-2*s.ThumbIndent, size)
}

// DefaultSizes provides the default sizing.
func (s *ScrollBar) DefaultSizes(hint geom32.Size) (min, pref, max geom32.Size) {
	min.Width = s.MinimumThickness
	min.Height = s.MinimumThickness
	if s.horizontal {
		pref.Width = s.MinimumThickness * 2
		pref.Height = s.MinimumThickness
		max.Width = DefaultMaxSize
		max.Height = s.MinimumThickness
	} else {
		pref.Width = s.MinimumThickness
		pref.Height = s.MinimumThickness * 2
		max.Width = s.MinimumThickness
		max.Height = DefaultMaxSize
	}
	return min, pref, max
}

// DefaultDraw provides the default drawing.
func (s *ScrollBar) DefaultDraw(gc *Canvas, rect geom32.Rect) {
	if thumb := s.Thumb(); thumb.Width > 0 && thumb.Height > 0 {
		var ink Ink
		if s.overThumb {
			ink = s.RolloverInk
		} else {
			ink = s.ThumbInk
		}
		gc.DrawRoundedRect(thumb, s.CornerRadius, ink.Paint(gc, thumb, Fill))
		gc.DrawRoundedRect(thumb, s.CornerRadius, s.EdgeInk.Paint(gc, thumb, Stroke))
	}
}

// DefaultMouseDown provides the default mouse down handling.
func (s *ScrollBar) DefaultMouseDown(where geom32.Point, button, clickCount int, mod Modifiers) bool {
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
func (s *ScrollBar) DefaultMouseUp(where geom32.Point, button int, mod Modifiers) bool {
	s.trackingThumb = false
	s.checkOverThumb(where)
	return true
}

// DefaultMouseEnter provides the default mouse enter handling.
func (s *ScrollBar) DefaultMouseEnter(where geom32.Point, mod Modifiers) bool {
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
func (s *ScrollBar) DefaultMouseMove(where geom32.Point, mod Modifiers) bool {
	s.checkOverThumb(where)
	return true
}

// DefaultMouseDrag provides the default mouse drag handling.
func (s *ScrollBar) DefaultMouseDrag(where geom32.Point, button int, mod Modifiers) bool {
	s.adjustValueForPoint(where)
	s.MarkForRedraw()
	return true
}

func (s *ScrollBar) adjustValueForPoint(pt geom32.Point) {
	r := s.ContentRect(false)
	thumb := s.Thumb()
	var pos, max float32
	if s.horizontal {
		pos = pt.X
		max = r.Width - thumb.Width
	} else {
		pos = pt.Y
		max = r.Height - thumb.Height
	}
	if s.max <= s.extent {
		s.SetRange(0, s.extent, s.max)
	} else {
		s.SetRange((s.max-s.extent)*(pos+s.dragOffset)/max, s.extent, s.max)
	}
}

func (s *ScrollBar) checkOverThumb(pt geom32.Point) {
	was := s.overThumb //nolint:ifshort // Can't move this into the if statement
	s.overThumb = s.Thumb().ContainsPoint(pt)
	if was != s.overThumb {
		s.MarkForRedraw()
	}
}
