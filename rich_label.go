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
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/paintstyle"
	"github.com/richardwilkes/unison/enums/pathop"
	"github.com/richardwilkes/unison/enums/side"
)

// DefaultRichLabelTheme holds the default RichLabelTheme values for RichLabels. Modifying this data will not alter
// existing RichLabels, but will alter any RichLabels created in the future.
var DefaultRichLabelTheme = RichLabelTheme{
	OnBackgroundInk: ThemeOnSurface,
	Gap:             3,
	HAlign:          align.Start,
	VAlign:          align.Middle,
	Side:            side.Left,
}

// RichLabelTheme holds theming data for a RichLabel.
type RichLabelTheme struct {
	OnBackgroundInk Ink
	Gap             float32
	HAlign          align.Enum
	VAlign          align.Enum
	Side            side.Enum
}

// RichLabel represents non-interactive text and/or a Drawable.
type RichLabel struct {
	Panel
	RichLabelTheme
	Drawable Drawable
	Text     *Text
}

// NewRichLabel creates a new, empty rich label.
func NewRichLabel() *RichLabel {
	l := &RichLabel{RichLabelTheme: DefaultRichLabelTheme}
	l.Self = l
	l.SetSizer(l.DefaultSizes)
	l.DrawCallback = l.DefaultDraw
	return l
}

// DefaultSizes provides the default sizing.
func (l *RichLabel) DefaultSizes(hint Size) (minSize, prefSize, maxSize Size) {
	if l.Text == nil && l.Drawable == nil {
		prefSize.Height = DefaultLabelTheme.Font.LineHeight()
		prefSize = prefSize.Ceil()
	} else {
		prefSize = LabelSize(l.Text, l.Drawable, l.Side, l.Gap)
	}
	if b := l.Border(); b != nil {
		prefSize = prefSize.Add(b.Insets().Size())
	}
	prefSize = prefSize.Ceil().ConstrainForHint(hint)
	return prefSize, prefSize, prefSize
}

// DefaultDraw provides the default drawing.
func (l *RichLabel) DefaultDraw(canvas *Canvas, _ Rect) {
	if l.Drawable == nil && l.Text == nil {
		return
	}

	// Determine overall size of content
	var size, txtSize Size
	if l.Text != nil {
		txtSize = l.Text.Extents()
		size = txtSize
	}
	adjustLabelSizeForDrawable(l.Text != nil, l.Drawable, l.Side, l.Gap, &size)

	// Adjust the working area for the content size
	rect := l.ContentRect(false)
	switch l.HAlign {
	case align.Middle, align.Fill:
		rect.X = xmath.Floor(rect.X + (rect.Width-size.Width)/2)
	case align.End:
		rect.X += rect.Width - size.Width
	default: // align.Start
	}
	switch l.VAlign {
	case align.Middle, align.Fill:
		rect.Y = xmath.Floor(rect.Y + (rect.Height-size.Height)/2)
	case align.End:
		rect.Y += rect.Height - size.Height
	default: // align.Start
	}
	rect.Size = size

	// Determine drawable and text areas
	imgX := rect.X
	imgY := rect.Y
	txtX := rect.X
	txtY := rect.Y
	if l.Text != nil && l.Drawable != nil {
		logicalSize := l.Drawable.LogicalSize()
		switch l.Side {
		case side.Top:
			txtY += logicalSize.Height + l.Gap
			if logicalSize.Width > txtSize.Width {
				txtX = xmath.Floor(txtX + (logicalSize.Width-txtSize.Width)/2)
			} else {
				imgX = xmath.Floor(imgX + (txtSize.Width-logicalSize.Width)/2)
			}
		case side.Left:
			txtX += logicalSize.Width + l.Gap
			if logicalSize.Height > txtSize.Height {
				txtY = xmath.Floor(txtY + (logicalSize.Height-txtSize.Height)/2)
			} else {
				imgY = xmath.Floor(imgY + (txtSize.Height-logicalSize.Height)/2)
			}
		case side.Bottom:
			imgY += rect.Height - logicalSize.Height
			txtY = imgY - (l.Gap + txtSize.Height)
			if logicalSize.Width > txtSize.Width {
				txtX = xmath.Floor(txtX + (logicalSize.Width-txtSize.Width)/2)
			} else {
				imgX = xmath.Floor(imgX + (txtSize.Width-logicalSize.Width)/2)
			}
		case side.Right:
			imgX += rect.Width - logicalSize.Width
			txtX = imgX - (l.Gap + txtSize.Width)
			if logicalSize.Height > txtSize.Height {
				txtY = xmath.Floor(txtY + (logicalSize.Height-txtSize.Height)/2)
			} else {
				imgY = xmath.Floor(imgY + (txtSize.Height-logicalSize.Height)/2)
			}
		}
	}

	canvas.Save()
	canvas.ClipRect(rect, pathop.Intersect, false)
	if l.Drawable != nil {
		rect.X = imgX
		rect.Y = imgY
		rect.Size = l.Drawable.LogicalSize()
		fg := l.OnBackgroundInk
		if !l.Enabled() {
			fg = &ColorFilteredInk{
				OriginalInk: fg,
				ColorFilter: Grayscale30Filter(),
			}
		}
		l.Drawable.DrawInRect(canvas, rect, nil, fg.Paint(canvas, rect, paintstyle.Fill))
	}
	if l.Text != nil {
		if l.Enabled() {
			l.Text.Draw(canvas, txtX, txtY+l.Text.Baseline())
		} else {
			m := make(map[*TextDecoration]Ink)
			l.Text.AdjustDecorations(func(decoration *TextDecoration) {
				m[decoration] = decoration.Foreground
				decoration.Foreground = &ColorFilteredInk{
					OriginalInk: decoration.Foreground,
					ColorFilter: Grayscale30Filter(),
				}
			})
			l.Text.Draw(canvas, txtX, txtY+l.Text.Baseline())
			l.Text.AdjustDecorations(func(decoration *TextDecoration) {
				decoration.Foreground = m[decoration]
			})
		}
	}
	canvas.Restore()
}
