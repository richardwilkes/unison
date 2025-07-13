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
	"github.com/richardwilkes/unison/enums/paintstyle"
)

// DefaultScrollBarTheme holds the default ScrollBarTheme values for ScrollBars. Modifying this data will not alter
// existing ScrollBars, but will alter any ScrollBars created in the future.
var DefaultScrollBarTheme = ScrollBarTheme{
	EdgeInk:          ThemeSurfaceEdge,
	ThumbInk:         ThemeFocus,
	MinimumThickness: 11,
	MinimumThumb:     16,
	ThumbIndent:      3,
	CornerRadius:     4,
}

// ScrollBarTheme holds theming data for a ScrollBar.
type ScrollBarTheme struct {
	EdgeInk          Ink
	ThumbInk         Ink
	MinimumThickness float32
	MinimumThumb     float32
	ThumbIndent      float32
	CornerRadius     float32
}

// ScrollBar holds the data necessary for tracking a scroll bar's state.
type ScrollBar struct {
	ChangedCallback func()
	ScrollBarTheme
	Panel
	value         float32
	extent        float32
	maximum       float32
	dragOffset    float32
	horizontal    bool
	overThumb     bool
	trackingThumb bool
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
func (s *ScrollBar) Thumb() geom.Rect {
	if s.maximum == 0 {
		return geom.Rect{}
	}
	r := s.ContentRect(false)
	if s.horizontal {
		start := r.Width * (s.value / s.maximum)
		size := r.Width * (s.extent / s.maximum)
		if size < s.MinimumThumb {
			size = s.MinimumThumb
		}
		return geom.Rect{Point: geom.Point{X: start}, Size: geom.Size{Width: size, Height: r.Height - s.ThumbIndent}}
	}
	start := r.Height * (s.value / s.maximum)
	size := r.Height * (s.extent / s.maximum)
	if size < s.MinimumThumb {
		size = s.MinimumThumb
	}
	return geom.Rect{Point: geom.Point{Y: start}, Size: geom.Size{Width: r.Width - s.ThumbIndent, Height: size}}
}

// DefaultSizes provides the default sizing.
func (s *ScrollBar) DefaultSizes(_ geom.Size) (minSize, prefSize, maxSize geom.Size) {
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
func (s *ScrollBar) DefaultDraw(gc *Canvas, _ geom.Rect) {
	if thumb := s.Thumb(); thumb.Width > 0 && thumb.Height > 0 {
		p := s.ThumbInk.Paint(gc, thumb, paintstyle.Fill)
		if !s.overThumb {
			p.SetColorFilter(Alpha30Filter())
		}
		gc.DrawRoundedRect(thumb, s.CornerRadius, s.CornerRadius, p)
		gc.DrawRoundedRect(thumb, s.CornerRadius, s.CornerRadius, s.EdgeInk.Paint(gc, thumb, paintstyle.Stroke))
	}
}

// DefaultMouseDown provides the default mouse down handling.
func (s *ScrollBar) DefaultMouseDown(where geom.Point, _, _ int, _ Modifiers) bool {
	thumb := s.Thumb()
	if !where.In(thumb) {
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
func (s *ScrollBar) DefaultMouseUp(where geom.Point, _ int, _ Modifiers) bool {
	s.trackingThumb = false
	s.checkOverThumb(where)
	return true
}

// DefaultMouseEnter provides the default mouse enter handling.
func (s *ScrollBar) DefaultMouseEnter(where geom.Point, _ Modifiers) bool {
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
func (s *ScrollBar) DefaultMouseMove(where geom.Point, _ Modifiers) bool {
	s.checkOverThumb(where)
	return true
}

// DefaultMouseDrag provides the default mouse drag handling.
func (s *ScrollBar) DefaultMouseDrag(where geom.Point, _ int, _ Modifiers) bool {
	s.adjustValueForPoint(where)
	s.MarkForRedraw()
	return true
}

func (s *ScrollBar) adjustValueForPoint(pt geom.Point) {
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

func (s *ScrollBar) checkOverThumb(pt geom.Point) {
	was := s.overThumb
	s.overThumb = pt.In(s.Thumb())
	if was != s.overThumb {
		s.MarkForRedraw()
	}
}
