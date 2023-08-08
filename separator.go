// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

// DefaultSeparatorTheme holds the default SeparatorTheme values for Separators. Modifying this data will not alter
// existing Separators, but will alter any Separators created in the future.
var DefaultSeparatorTheme = SeparatorTheme{
	LineInk:  DividerColor,
	Vertical: false,
}

// SeparatorTheme holds theming data for a Separator.
type SeparatorTheme struct {
	LineInk  Ink
	Vertical bool
}

// Separator provides a simple vertical or horizontal separator line.
type Separator struct {
	Panel
	SeparatorTheme
}

// NewSeparator creates a new separator line.
func NewSeparator() *Separator {
	s := &Separator{SeparatorTheme: DefaultSeparatorTheme}
	s.Self = s
	s.SetSizer(s.DefaultSizes)
	s.DrawCallback = s.DefaultDraw
	return s
}

// DefaultSizes provides the default sizing.
func (s *Separator) DefaultSizes(hint Size) (minSize, prefSize, maxSize Size) {
	if s.Vertical {
		if hint.Height < 1 {
			prefSize.Height = 1
		} else {
			prefSize.Height = hint.Height
		}
		minSize.Height = 1
		maxSize.Height = DefaultMaxSize
		minSize.Width = 1
		prefSize.Width = 1
		maxSize.Width = 1
	} else {
		if hint.Width < 1 {
			prefSize.Width = 1
		} else {
			prefSize.Width = hint.Width
		}
		minSize.Width = 1
		maxSize.Width = DefaultMaxSize
		minSize.Height = 1
		prefSize.Height = 1
		maxSize.Height = 1
	}
	if border := s.Border(); border != nil {
		insets := border.Insets()
		minSize.AddInsets(insets)
		prefSize.AddInsets(insets)
		maxSize.AddInsets(insets)
	}
	return minSize, prefSize, maxSize
}

// DefaultDraw provides the default drawing.
func (s *Separator) DefaultDraw(canvas *Canvas, _ Rect) {
	rect := s.ContentRect(false)
	if s.Vertical {
		if rect.Width > 1 {
			rect.X += (rect.Width - 1) / 2
			rect.Width = 1
		}
	} else if rect.Height > 1 {
		rect.Y += (rect.Height - 1) / 2
		rect.Height = 1
	}
	canvas.DrawRect(rect, s.LineInk.Paint(canvas, rect, Fill))
}
