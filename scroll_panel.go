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
	"github.com/richardwilkes/toolbox/xmath"
)

var _ Layout = &ScrollPanel{}

// Possible ways to handle auto-sizing of the scroll content's preferred size.
const (
	UnmodifiedBehavior Behavior = iota
	FillBehavior                // If the content is smaller than the available space, expand it
	FollowBehavior              // Fix the content to the view size
)

// Behavior controls how auto-sizing of the scroll content's preferred size is handled.
type Behavior uint8

// DefaultScrollPanelTheme holds the default ScrollPanelTheme values for ScrollPanels. Modifying this data will not
// alter existing ScrollPanels, but will alter any ScrollPanels created in the future.
var DefaultScrollPanelTheme = ScrollPanelTheme{
	BackgroundInk:        BackgroundColor,
	MouseWheelMultiplier: 16,
}

// ScrollPanelTheme holds theming data for a ScrollPanel.
type ScrollPanelTheme struct {
	BackgroundInk        Ink
	MouseWheelMultiplier float32
}

// ScrollPanel provides a scrollable area.
type ScrollPanel struct {
	Panel
	ScrollPanelTheme
	horizontalBar    *ScrollBar
	verticalBar      *ScrollBar
	columnHeaderView *Panel
	columnHeader     Paneler
	rowHeaderView    *Panel
	rowHeader        Paneler
	contentView      *Panel
	content          Paneler
	widthBehavior    Behavior
	heightBehavior   Behavior
}

// NewScrollPanel creates a new scrollable area.
func NewScrollPanel() *ScrollPanel {
	s := &ScrollPanel{
		ScrollPanelTheme: DefaultScrollPanelTheme,
		horizontalBar:    NewScrollBar(true),
		verticalBar:      NewScrollBar(false),
		contentView:      NewPanel(),
	}
	s.Self = s
	s.AddChild(s.horizontalBar)
	s.AddChild(s.verticalBar)
	s.AddChild(s.contentView)
	s.SetLayout(s)
	s.horizontalBar.ChangedCallback = s.Sync
	s.verticalBar.ChangedCallback = s.Sync
	s.DrawCallback = s.DefaultDraw
	s.MouseWheelCallback = s.DefaultMouseWheel
	s.ScrollRectIntoViewCallback = s.DefaultScrollRectIntoView
	s.FrameChangeInChildHierarchyCallback = s.DefaultFrameChangeInChildHierarchy
	return s
}

// Bar returns the specified scroll bar.
func (s *ScrollPanel) Bar(horizontal bool) *ScrollBar {
	if horizontal {
		return s.horizontalBar
	}
	return s.verticalBar
}

// ColumnHeaderView returns the column header view port. May be nil, if there is no column header.
func (s *ScrollPanel) ColumnHeaderView() *Panel {
	return s.columnHeaderView
}

// ColumnHeader returns the current column header, if any.
func (s *ScrollPanel) ColumnHeader() Paneler {
	return s.columnHeader
}

// SetColumnHeader sets the current column header. May be nil.
func (s *ScrollPanel) SetColumnHeader(p Paneler) {
	if s.columnHeader != nil {
		s.columnHeader.AsPanel().RemoveFromParent()
	}
	s.columnHeader = p
	if p != nil {
		if s.columnHeaderView == nil {
			s.columnHeaderView = NewPanel()
			s.AddChild(s.columnHeaderView)
		}
		s.columnHeaderView.AddChild(p)
		s.Sync()
	} else if s.columnHeaderView != nil {
		s.columnHeaderView.RemoveFromParent()
		s.columnHeaderView = nil
	}
	s.MarkForLayoutAndRedraw()
}

// RowHeaderView returns the row header view port. May be nil, if there is no row header.
func (s *ScrollPanel) RowHeaderView() *Panel {
	return s.rowHeaderView
}

// RowHeader returns the current row header, if any.
func (s *ScrollPanel) RowHeader() Paneler {
	return s.rowHeader
}

// SetRowHeader sets the current row header. May be nil.
func (s *ScrollPanel) SetRowHeader(p Paneler) {
	if s.rowHeader != nil {
		s.rowHeader.AsPanel().RemoveFromParent()
	}
	s.rowHeader = p
	if p != nil {
		if s.rowHeaderView == nil {
			s.rowHeaderView = NewPanel()
			s.AddChild(s.rowHeaderView)
		}
		s.rowHeaderView.AddChild(p)
		s.Sync()
	} else if s.rowHeaderView != nil {
		s.rowHeaderView.RemoveFromParent()
		s.rowHeaderView = nil
	}
	s.MarkForLayoutAndRedraw()
}

// ContentView returns the content view port.
func (s *ScrollPanel) ContentView() *Panel {
	return s.contentView
}

// Content returns the content panel.
func (s *ScrollPanel) Content() Paneler {
	return s.content
}

// SetContent sets the content panel.
func (s *ScrollPanel) SetContent(p Paneler, widthBehavior, heightBehavior Behavior) {
	if s.content != nil {
		s.content.AsPanel().RemoveFromParent()
	}
	s.content = p
	s.widthBehavior = widthBehavior
	s.heightBehavior = heightBehavior
	if p != nil {
		s.contentView.AddChild(p)
		s.Sync()
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
func (s *ScrollPanel) DefaultDraw(canvas *Canvas, dirty Rect) {
	r := s.ContentRect(true)
	canvas.DrawRect(r, s.BackgroundInk.Paint(canvas, r, Fill))
}

// Sync the headers and content with the current scroll state.
func (s *ScrollPanel) Sync() {
	if s.columnHeader != nil {
		r := s.columnHeader.AsPanel().FrameRect()
		r.X = -s.horizontalBar.Value()
		r.Y = 0
		s.columnHeader.AsPanel().SetFrameRect(r)
	}
	if s.rowHeader != nil {
		r := s.rowHeader.AsPanel().FrameRect()
		r.X = 0
		r.Y = -s.verticalBar.Value()
		s.rowHeader.AsPanel().SetFrameRect(r)
	}
	if s.content != nil {
		r := s.content.AsPanel().FrameRect()
		r.X = -s.horizontalBar.Value()
		r.Y = -s.verticalBar.Value()
		s.content.AsPanel().SetFrameRect(r)
	}
	s.MarkForLayoutAndRedraw()
}

// DefaultMouseWheel provides the default mouse wheel handling.
func (s *ScrollPanel) DefaultMouseWheel(where, delta Point, mod Modifiers) bool {
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

// DefaultScrollRectIntoView provides the default scroll rect into contentView handling.
func (s *ScrollPanel) DefaultScrollRectIntoView(rect Rect) bool {
	viewRect := s.contentView.FrameRect()
	viewRect.X = 0
	viewRect.Y = 0
	if s.columnHeaderView != nil {
		height := s.columnHeaderView.FrameRect().Height
		viewRect.Y += height
		viewRect.Height -= height
	}
	if s.rowHeaderView != nil {
		width := s.rowHeaderView.FrameRect().Width
		viewRect.X += width
		viewRect.Width -= width
	}
	hAdj := computeScrollAdj(rect.X, viewRect.X, rect.Right(), viewRect.Right())
	vAdj := computeScrollAdj(rect.Y, viewRect.Y, rect.Bottom(), viewRect.Bottom())
	if hAdj != 0 || vAdj != 0 {
		if hAdj != 0 {
			s.horizontalBar.SetRange(s.horizontalBar.Value()+hAdj, s.horizontalBar.Extent(), s.horizontalBar.Max())
		}
		if vAdj != 0 {
			s.verticalBar.SetRange(s.verticalBar.Value()+vAdj, s.verticalBar.Extent(), s.verticalBar.Max())
		}
		return true
	}
	return false
}

func computeScrollAdj(contentTopLeft, viewTopLeft, contentBottomRight, viewBottomRight float32) float32 {
	if contentTopLeft < viewTopLeft {
		return contentTopLeft - viewTopLeft
	}
	if contentBottomRight > viewBottomRight {
		if contentBottomRight-contentTopLeft <= viewBottomRight-viewTopLeft {
			return contentBottomRight - viewBottomRight
		}
		return contentTopLeft - viewTopLeft
	}
	return 0
}

// DefaultFrameChangeInChildHierarchy provides the default frame change in child hierarchy handling.
func (s *ScrollPanel) DefaultFrameChangeInChildHierarchy(panel *Panel) {
	// TODO: Need to adjust the headers, too?
	if s.content != nil {
		vs := s.contentView.FrameRect().Size
		r := s.content.AsPanel().FrameRect()
		nl := r.Point
		if r.Y != 0 && vs.Height > r.Bottom() {
			nl.Y = xmath.Min(vs.Height-r.Height, 0)
		}
		if r.X != 0 && vs.Width > r.Right() {
			nl.X = xmath.Min(vs.Width-r.Width, 0)
		}
		if nl != r.Point {
			r.Point = nl
			s.content.AsPanel().SetFrameRect(r)
			if s.columnHeaderView != nil {
				r = s.columnHeader.AsPanel().FrameRect()
				r.X = nl.X
				s.columnHeader.AsPanel().SetFrameRect(r)
			}
			if s.rowHeaderView != nil {
				r = s.rowHeader.AsPanel().FrameRect()
				r.Y = nl.Y
				s.rowHeader.AsPanel().SetFrameRect(r)
			}
		}
		s.MarkForLayoutAndRedraw()
	}
}

// LayoutSizes implements the Layout interface.
func (s *ScrollPanel) LayoutSizes(_ *Panel, hint Size) (min, pref, max Size) {
	if s.content != nil {
		_, pref, _ = s.content.AsPanel().Sizes(hint)
	}
	min.Width = s.verticalBar.MinimumThickness
	min.Height = s.horizontalBar.MinimumThickness
	if s.columnHeaderView != nil {
		_, p, _ := s.columnHeader.AsPanel().Sizes(Size{Width: hint.Width})
		min.Height += p.Height
		pref.Height += p.Height
		if border := s.columnHeaderView.Border(); border != nil {
			insets := border.Insets()
			min.Height += insets.Top + insets.Bottom
			pref.Height += insets.Top + insets.Bottom
		}
	}
	if s.rowHeaderView != nil {
		_, p, _ := s.rowHeader.AsPanel().Sizes(Size{Height: hint.Height})
		min.Width += p.Width
		pref.Width += p.Width
		if border := s.rowHeaderView.Border(); border != nil {
			insets := border.Insets()
			min.Width += insets.Left + insets.Right
			pref.Width += insets.Left + insets.Right
		}
	}
	if border := s.contentView.Border(); border != nil {
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
func (s *ScrollPanel) PerformLayout(_ *Panel) {
	r := s.FrameRect()
	r.X = 0
	r.Y = 0
	columnHeaderTop := r.Y
	if s.columnHeaderView != nil {
		_, p, _ := s.columnHeader.AsPanel().Sizes(Size{Width: r.Width})
		height := xmath.Min(r.Height, p.Height)
		if border := s.columnHeaderView.Border(); border != nil {
			insets := border.Insets()
			height += insets.Top + insets.Bottom
		}
		r.Y += height
		r.Height -= height
	}
	if s.rowHeaderView != nil {
		_, p, _ := s.rowHeader.AsPanel().Sizes(Size{Height: r.Height})
		row := NewRect(r.X, r.Y, 0, r.Height)
		row.Width = xmath.Min(r.Width, p.Width)
		if border := s.rowHeaderView.Border(); border != nil {
			insets := border.Insets()
			row.Width += insets.Left + insets.Right
		}
		s.rowHeaderView.AsPanel().SetFrameRect(row)
		r.X += row.Width
		r.Width -= row.Width
	}
	if s.columnHeaderView != nil {
		_, p, _ := s.columnHeader.AsPanel().Sizes(Size{Width: r.Width})
		col := NewRect(r.X, columnHeaderTop, r.Width, xmath.Min(r.Height, p.Height))
		if border := s.columnHeaderView.Border(); border != nil {
			insets := border.Insets()
			col.Height += insets.Top + insets.Bottom
		}
		s.columnHeaderView.AsPanel().SetFrameRect(col)
	}
	viewContent := r
	if border := s.contentView.Border(); border != nil {
		viewContent.Inset(border.Insets())
	}
	var contentSize Size
	if s.content != nil {
		var hint Size
		if s.widthBehavior == FollowBehavior {
			hint.Width = viewContent.Width
		}
		if s.heightBehavior == FollowBehavior {
			hint.Height = viewContent.Height
		}
		_, contentSize, _ = s.content.AsPanel().Sizes(hint)
		switch s.widthBehavior {
		case FillBehavior:
			if contentSize.Width < viewContent.Width {
				contentSize.Width = viewContent.Width
			}
		case FollowBehavior:
			contentSize.Width = viewContent.Width
		}
		switch s.heightBehavior {
		case FillBehavior:
			if contentSize.Height < viewContent.Height {
				contentSize.Height = viewContent.Height
			}
		case FollowBehavior:
			contentSize.Height = viewContent.Height
		}
		cr := s.content.AsPanel().FrameRect()
		cr.Size = contentSize
		s.content.AsPanel().SetFrameRect(cr)
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
		width -= s.verticalBar.MinimumThickness
		height -= s.horizontalBar.MinimumThickness
	}
	s.verticalBar.SetFrameRect(NewRect(viewContent.Right()-s.verticalBar.MinimumThickness, viewContent.Y, s.verticalBar.MinimumThickness, height))
	s.horizontalBar.SetFrameRect(NewRect(viewContent.X, viewContent.Bottom()-s.horizontalBar.MinimumThickness, width, s.horizontalBar.MinimumThickness))
	s.contentView.SetFrameRect(r)
	if s.columnHeaderView != nil {
		vr := s.columnHeaderView.FrameRect()
		r = s.columnHeader.AsPanel().FrameRect()
		r.Height = vr.Height
		r.Width = xmath.Max(vr.Width, contentSize.Width)
		s.columnHeader.AsPanel().SetFrameRect(r)
	}
	if s.rowHeaderView != nil {
		vr := s.rowHeaderView.FrameRect()
		r = s.rowHeader.AsPanel().FrameRect()
		r.Width = vr.Width
		r.Height = xmath.Max(vr.Height, contentSize.Height)
		s.rowHeader.AsPanel().SetFrameRect(r)
	}
}
