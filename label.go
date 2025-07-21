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
	"github.com/richardwilkes/toolbox/v2/xmath"
	"github.com/richardwilkes/toolbox/v2/xreflect"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/paintstyle"
	"github.com/richardwilkes/unison/enums/pathop"
	"github.com/richardwilkes/unison/enums/side"
)

// DefaultLabelTheme holds the default LabelTheme values for Labels. Modifying this data will not alter existing Labels,
// but will alter any Labels created in the future.
var DefaultLabelTheme = LabelTheme{
	TextDecoration: TextDecoration{
		Font:            LabelFont,
		OnBackgroundInk: ThemeOnSurface,
	},
	Gap:    StdIconGap,
	HAlign: align.Start,
	VAlign: align.Middle,
	Side:   side.Left,
}

// LabelTheme holds theming data for a Label.
type LabelTheme struct {
	TextDecoration
	Gap    float32
	HAlign align.Enum
	VAlign align.Enum
	Side   side.Enum
}

// Label represents non-interactive text and/or a Drawable.
type Label struct {
	Drawable Drawable
	Text     *Text
	LabelTheme
	Panel
}

// NewLabel creates a new, empty label.
func NewLabel() *Label {
	l := &Label{LabelTheme: DefaultLabelTheme}
	l.Self = l
	l.SetSizer(l.DefaultSizes)
	l.DrawCallback = l.DefaultDraw
	return l
}

func (l *Label) String() string {
	if l.Text == nil {
		return ""
	}
	return l.Text.String()
}

// SetTitle sets the text of the label to the specified text. The theme's TextDecoration will be used, so any
// changes you want to make to it should be done before calling this method. Alternatively, you can directly set the
// .Text field.
func (l *Label) SetTitle(text string) {
	l.Text = NewText(text, &l.TextDecoration)
}

// DefaultSizes provides the default sizing.
func (l *Label) DefaultSizes(hint geom.Size) (minSize, prefSize, maxSize geom.Size) {
	prefSize, _ = LabelContentSizes(l.Text, l.Drawable, l.Font, l.Side, l.Gap)
	if b := l.Border(); b != nil {
		prefSize = prefSize.Add(b.Insets().Size())
	}
	prefSize = prefSize.Ceil().ConstrainForHint(hint)
	return prefSize, prefSize, prefSize
}

// DefaultDraw provides the default drawing.
func (l *Label) DefaultDraw(canvas *Canvas, _ geom.Rect) {
	DrawLabel(canvas, l.ContentRect(false), l.HAlign, l.VAlign, l.Font, l.Text, l.OnBackgroundInk, l.BackgroundInk,
		l.Drawable, l.Side, l.Gap, !l.Enabled())
}

// LabelContentSizes returns the preferred size of a label, as well as the preferred size of the text within the label.
// When no drawable is present, the two values will be the same. Provided as a standalone function so that other types
// of panels can make use of it.
func LabelContentSizes(text *Text, drawable Drawable, font Font, drawableSide side.Enum, gap float32) (size, txtSize geom.Size) {
	empty := text.Empty()
	if empty && drawable == nil {
		txtSize.Height = font.LineHeight()
		size = txtSize
	} else {
		if !empty {
			txtSize = text.Extents()
			size = txtSize
		}
		if drawable != nil {
			logicalSize := drawable.LogicalSize()
			switch {
			case empty:
				size = logicalSize
			case drawableSide.Horizontal():
				size.Width += logicalSize.Width + gap
				size.Height = max(size.Height, logicalSize.Height)
			default:
				size.Height += logicalSize.Height + gap
				size.Width = max(size.Width, logicalSize.Width)
			}
		}
	}
	return size.Ceil(), txtSize
}

// DrawLabel draws a label. Provided as a standalone function so that other types of panels can make use of it.
func DrawLabel(canvas *Canvas, rect geom.Rect, hAlign, vAlign align.Enum, font Font, text *Text, onBackgroundInk, backgroundInk Ink, drawable Drawable, drawableSide side.Enum, imgGap float32, applyDisabledFilter bool) {
	if !xreflect.IsNil(backgroundInk) {
		canvas.DrawRect(rect, backgroundInk.Paint(canvas, rect, paintstyle.Fill))
	}
	empty := text.Empty()
	if drawable == nil && empty {
		return
	}

	// Determine overall size of content
	size, txtSize := LabelContentSizes(text, drawable, font, drawableSide, imgGap)

	// Adjust the working area for the content size
	switch hAlign {
	case align.Middle, align.Fill:
		rect.X = xmath.Floor(rect.X + (rect.Width-size.Width)/2)
	case align.End:
		rect.X += rect.Width - size.Width
	default: // align.Start
	}
	switch vAlign {
	case align.Middle, align.Fill:
		rect.Y = xmath.Floor(rect.Y + (rect.Height-size.Height)/2)
	case align.End:
		rect.Y += rect.Height - size.Height
	default: // align.Start
	}
	rect.Size = size

	// Determine drawable and text areas
	imgPt := rect.Point
	txtPt := rect.Point
	if !empty && drawable != nil {
		logicalSize := drawable.LogicalSize()
		switch drawableSide {
		case side.Top:
			txtPt.Y += logicalSize.Height + imgGap
			if logicalSize.Width > txtSize.Width {
				txtPt.X = xmath.Floor(txtPt.X + (logicalSize.Width-txtSize.Width)/2)
			} else {
				imgPt.X = xmath.Floor(imgPt.X + (txtSize.Width-logicalSize.Width)/2)
			}
		case side.Left:
			txtPt.X += logicalSize.Width + imgGap
			if logicalSize.Height > txtSize.Height {
				txtPt.Y = xmath.Floor(txtPt.Y + (logicalSize.Height-txtSize.Height)/2)
			} else {
				imgPt.Y = xmath.Floor(imgPt.Y + (txtSize.Height-logicalSize.Height)/2)
			}
		case side.Bottom:
			imgPt.Y += rect.Height - logicalSize.Height
			txtPt.Y = imgPt.Y - (imgGap + txtSize.Height)
			if logicalSize.Width > txtSize.Width {
				txtPt.X = xmath.Floor(txtPt.X + (logicalSize.Width-txtSize.Width)/2)
			} else {
				imgPt.X = xmath.Floor(imgPt.X + (txtSize.Width-logicalSize.Width)/2)
			}
		case side.Right:
			imgPt.X += rect.Width - logicalSize.Width
			txtPt.X = imgPt.X - (imgGap + txtSize.Width)
			if logicalSize.Height > txtSize.Height {
				txtPt.Y = xmath.Floor(txtPt.Y + (logicalSize.Height-txtSize.Height)/2)
			} else {
				imgPt.Y = xmath.Floor(imgPt.Y + (txtSize.Height-logicalSize.Height)/2)
			}
		}
	}

	canvas.Save()
	canvas.ClipRect(rect, pathop.Intersect, false)
	if drawable != nil {
		rect.Point = imgPt
		rect.Size = drawable.LogicalSize()
		fg := onBackgroundInk
		if applyDisabledFilter {
			fg = &ColorFilteredInk{
				OriginalInk: fg,
				ColorFilter: Grayscale30Filter(),
			}
		}
		drawable.DrawInRect(canvas, rect, nil, fg.Paint(canvas, rect, paintstyle.Fill))
	}
	if !empty {
		if applyDisabledFilter {
			defer text.RestoreDecorations(text.AdjustDecorations(func(decoration *TextDecoration) {
				decoration.OnBackgroundInk = &ColorFilteredInk{
					OriginalInk: decoration.OnBackgroundInk,
					ColorFilter: Grayscale30Filter(),
				}
			}))
		}
		txtPt.Y += text.Baseline()
		text.Draw(canvas, txtPt)
	}
	canvas.Restore()
}
