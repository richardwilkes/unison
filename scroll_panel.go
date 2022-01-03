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

var _ Layout = &ScrollPanel{}

// Possible ways to handle auto-sizing of the scroll content's preferred size.
const (
	UnmodifiedBehavior Behavior = iota
	FillWidthBehavior
	FillHeightBehavior
	FillBehavior
	FollowsWidthBehavior
	FollowsHeightBehavior
)

// Behavior controls how auto-sizing of the scroll content's preferred size is handled.
type Behavior uint8

// ScrollPanel provides a scrollable area.
type ScrollPanel struct {
	Panel
	BackgroundColor      Ink
	horizontalBar        *ScrollBar
	verticalBar          *ScrollBar
	columnHeader         *Panel
	rowHeader            *Panel
	view                 *Panel
	content              *Panel
	behavior             Behavior
	MouseWheelMultiplier float32
}

// NewScrollPanel creates a new scrollable area.
func NewScrollPanel() *ScrollPanel {
	s := &ScrollPanel{
		horizontalBar: NewScrollBar(true),
		verticalBar:   NewScrollBar(false),
		view:          NewPanel(),
	}
	s.Self = s
	s.AddChild(s.horizontalBar)
	s.AddChild(s.verticalBar)
	s.AddChild(s.view)
	s.SetLayout(s)
	s.horizontalBar.ChangedCallback = s.barChanged
	s.verticalBar.ChangedCallback = s.barChanged
	s.DrawCallback = s.DefaultDraw
	s.MouseWheelCallback = s.DefaultMouseWheel
	s.ScrollRectIntoViewCallback = s.DefaultScrollRectIntoView
	s.FrameChangeInChildHierarchyCallback = s.DefaultFrameChangeInChildHierarchy
	return s
}

// View returns the view port.
func (s *ScrollPanel) View() *Panel {
	return s.view
}

// Bar returns the specified scroll bar.
func (s *ScrollPanel) Bar(horizontal bool) *ScrollBar {
	if horizontal {
		return s.horizontalBar
	}
	return s.verticalBar
}

// ColumnHeader returns the current column header, if any.
func (s *ScrollPanel) ColumnHeader() *Panel {
	return s.columnHeader
}

// SetColumnHeader sets the current column header. May be nil.
func (s *ScrollPanel) SetColumnHeader(p *Panel) {
	if s.columnHeader != nil {
		s.columnHeader.RemoveFromParent()
	}
	s.columnHeader = p
	if p != nil {
		s.AddChild(p)
	}
	s.MarkForLayoutAndRedraw()
}

// RowHeader returns the current row header, if any.
func (s *ScrollPanel) RowHeader() *Panel {
	return s.rowHeader
}

// SetRowHeader sets the current row header. May be nil.
func (s *ScrollPanel) SetRowHeader(p *Panel) {
	if s.rowHeader != nil {
		s.rowHeader.RemoveFromParent()
	}
	s.rowHeader = p
	if p != nil {
		s.AddChild(p)
	}
	s.MarkForLayoutAndRedraw()
}

// Content returns the content panel.
func (s *ScrollPanel) Content() *Panel {
	return s.content
}

// SetContent sets the content panel.
func (s *ScrollPanel) SetContent(p Paneler, behave Behavior) {
	if s.content != nil {
		s.content.RemoveFromParent()
	}
	s.content = p.AsPanel()
	s.behavior = behave
	if p != nil {
		s.view.AddChild(p)
		s.barChanged()
	}
	s.MarkForLayoutAndRedraw()
}

// Position returns the current scroll position.
func (s *ScrollPanel) Position() (h, v float32) {
	return s.horizontalBar.Value(), s.verticalBar.Value()
}

// SetPosition sets the current scroll position.
func (s *ScrollPanel) SetPosition(h, v float32) {
	s.horizontalBar.SetRange(h, s.horizontalBar.Extent(), s.horizontalBar.Max())
	s.verticalBar.SetRange(v, s.verticalBar.Extent(), s.verticalBar.Max())
}

// DefaultDraw provides the default drawing.
func (s *ScrollPanel) DefaultDraw(canvas *Canvas, dirty geom32.Rect) {
	r := s.ContentRect(true)
	canvas.DrawRect(r, ChooseInk(s.BackgroundColor, BackgroundColor).Paint(canvas, r, Fill))
}

func (s *ScrollPanel) barChanged() {
	if s.content != nil {
		r := s.content.ContentRect(true)
		r.X = -s.horizontalBar.Value()
		r.Y = -s.verticalBar.Value()
		s.content.SetFrameRect(r)
	}
}

// DefaultMouseWheel provides the default mouse wheel handling.
func (s *ScrollPanel) DefaultMouseWheel(where, delta geom32.Point, mod Modifiers) bool {
	if delta.Y != 0 {
		dy := delta.Y
		if s.MouseWheelMultiplier > 0 {
			dy *= s.MouseWheelMultiplier
		}
		s.verticalBar.SetRange(s.verticalBar.Value()-dy, s.verticalBar.Extent(), s.verticalBar.Max())
	}
	if delta.X != 0 {
		dx := delta.X
		if s.MouseWheelMultiplier > 0 {
			dx *= s.MouseWheelMultiplier
		}
		s.horizontalBar.SetRange(s.horizontalBar.Value()-dx, s.horizontalBar.Extent(), s.horizontalBar.Max())
	}
	return true
}

// DefaultScrollRectIntoView provides the default scroll rect into view handling.
func (s *ScrollPanel) DefaultScrollRectIntoView(rect geom32.Rect) bool {
	viewRect := s.view.ContentRect(false)
	hAdj := computeScrollAdj(rect.X, viewRect.X, rect.Y+rect.Width, viewRect.X+viewRect.Width)
	vAdj := computeScrollAdj(rect.Y, viewRect.Y, rect.Y+rect.Height, viewRect.Y+viewRect.Height)
	if hAdj != 0 || vAdj != 0 {
		if hAdj != 0 {
			s.verticalBar.SetRange(s.verticalBar.Value()+hAdj, s.verticalBar.Extent(), s.verticalBar.Max())
		}
		if vAdj != 0 {
			s.horizontalBar.SetRange(s.horizontalBar.Value()+vAdj, s.horizontalBar.Extent(), s.horizontalBar.Max())
		}
		return true
	}
	return false
}

func computeScrollAdj(upper1, upper2, lower1, lower2 float32) float32 {
	if upper1 < upper2 {
		return upper1 - upper2
	}
	if lower1 > lower2 {
		if lower1-upper1 <= lower2-upper2 {
			return lower1 - lower2
		}
		return upper1 - upper2
	}
	return 0
}

// DefaultFrameChangeInChildHierarchy provides the default frame change in child hierarchy handling.
func (s *ScrollPanel) DefaultFrameChangeInChildHierarchy(panel *Panel) {
	if s.content != nil {
		vs := s.view.ContentRect(false).Size
		rect := s.content.FrameRect()
		nl := rect.Point
		if rect.Y != 0 && vs.Height > rect.Y+rect.Height {
			nl.Y = mathf32.Min(vs.Height-rect.Height, 0)
		}
		if rect.X != 0 && vs.Width > rect.X+rect.Width {
			nl.X = mathf32.Min(vs.Width-rect.Width, 0)
		}
		if nl != rect.Point {
			rect.Point = nl
			s.content.SetFrameRect(rect)
		}
		s.MarkForLayoutAndRedraw()
	}
}

// LayoutSizes implements the Layout interface.
func (s *ScrollPanel) LayoutSizes(_ Layoutable, hint geom32.Size) (min, pref, max geom32.Size) {
	if s.content != nil {
		_, pref, _ = s.content.Sizes(hint)
	}
	min.Width = MinimumScrollBarSize
	min.Height = MinimumScrollBarSize
	if s.columnHeader != nil {
		_, p, _ := s.columnHeader.Sizes(geom32.Size{Width: hint.Width})
		min.Height += p.Height
		pref.Height += p.Height
	}
	if s.rowHeader != nil {
		_, p, _ := s.rowHeader.Sizes(geom32.Size{Height: hint.Height})
		min.Width += p.Width
		pref.Width += p.Width
	}
	if border := s.view.Border(); border != nil {
		insets := border.Insets()
		min.AddInsets(insets)
		pref.AddInsets(insets)
	}
	if border := s.Border(); border != nil {
		insets := border.Insets()
		min.AddInsets(insets)
		pref.AddInsets(insets)
	}
	return min, pref, MaxSize(pref)
}

// PerformLayout implements the Layout interface.
func (s *ScrollPanel) PerformLayout(_ Layoutable) {
	r := s.ContentRect(false)
	col := geom32.NewRect(0, r.Y, 0, 0)
	if s.columnHeader != nil {
		_, p, _ := s.columnHeader.Sizes(geom32.Size{Width: r.Width})
		col.Height = mathf32.Min(r.Height, p.Height)
		r.Y += col.Height
		r.Height -= col.Height
	}
	row := geom32.NewRect(r.X, r.Y, 0, r.Height)
	if s.rowHeader != nil {
		_, p, _ := s.rowHeader.Sizes(geom32.Size{Height: r.Height})
		row.Width = mathf32.Min(r.Width, p.Width)
		s.rowHeader.SetFrameRect(row)
		r.X += row.Width
		r.Width -= row.Width
	}
	if s.columnHeader != nil {
		col.Width = r.Width
		col.X = r.X
		s.columnHeader.SetFrameRect(col)
	}
	viewContent := r
	if border := s.view.Border(); border != nil {
		viewContent.Inset(border.Insets())
	}
	var contentSize geom32.Size
	if s.content != nil {
		var hint geom32.Size
		switch s.behavior {
		case FollowsWidthBehavior:
			hint.Width = viewContent.Width
		case FollowsHeightBehavior:
			hint.Height = viewContent.Height
		}
		_, contentSize, _ = s.content.Sizes(hint)
		switch s.behavior {
		case FillWidthBehavior:
			if viewContent.Width > contentSize.Width {
				contentSize.Width = viewContent.Width
			}
		case FillHeightBehavior:
			if viewContent.Height > contentSize.Height {
				contentSize.Height = viewContent.Height
			}
		case FillBehavior:
			if viewContent.Width > contentSize.Width {
				contentSize.Width = viewContent.Width
			}
			if viewContent.Height > contentSize.Height {
				contentSize.Height = viewContent.Height
			}
		case FollowsWidthBehavior:
			contentSize.Width = viewContent.Width
		case FollowsHeightBehavior:
			contentSize.Height = viewContent.Height
		}
		cr := s.content.FrameRect()
		cr.Size = contentSize
		s.content.SetFrameRect(cr)
	}
	vBarNeeded := viewContent.Height < contentSize.Height
	hBarNeeded := viewContent.Width < contentSize.Width
	bothNeeded := vBarNeeded && hBarNeeded
	var height, width float32
	if vBarNeeded {
		height = viewContent.Height
	}
	if hBarNeeded {
		width = viewContent.Width
	}
	s.verticalBar.SetRange(s.verticalBar.Value(), viewContent.Height, contentSize.Height)
	s.horizontalBar.SetRange(s.horizontalBar.Value(), viewContent.Width, contentSize.Width)
	if bothNeeded {
		width -= MinimumScrollBarSize
		height -= MinimumScrollBarSize
	}
	s.verticalBar.SetFrameRect(geom32.NewRect(viewContent.Right()-MinimumScrollBarSize, viewContent.Y, MinimumScrollBarSize, height))
	s.horizontalBar.SetFrameRect(geom32.NewRect(viewContent.X, viewContent.Bottom()-MinimumScrollBarSize, width, MinimumScrollBarSize))
	s.view.SetFrameRect(r)
}
