// Copyright ©2021-2024 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"github.com/richardwilkes/toolbox"
	"github.com/richardwilkes/toolbox/xmath"
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
	Gap:    3,
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
	Panel
	LabelTheme
	Drawable Drawable
	Text     *Text
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
	if l.Text != nil {
		return l.Text.String()
	}
	return ""
}

// SetPlainText sets the text of the label to the specified plain text. The label theme's TextDecoration will be used,
// so any changes you want to make to it should be done before calling this method.
func (l *Label) SetPlainText(text string) {
	l.Text = NewText(text, &l.TextDecoration)
}

// DefaultSizes provides the default sizing.
func (l *Label) DefaultSizes(hint Size) (minSize, prefSize, maxSize Size) {
	prefSize, _ = l.contentSizes()
	if b := l.Border(); b != nil {
		prefSize = prefSize.Add(b.Insets().Size())
	}
	prefSize = prefSize.Ceil().ConstrainForHint(hint)
	return prefSize, prefSize, prefSize
}

func (l *Label) contentSizes() (size, txtSize Size) {
	noText := l.Text.Empty()
	if noText && l.Drawable == nil {
		txtSize.Height = l.Font.LineHeight()
		size = txtSize
	} else {
		if !noText {
			txtSize = l.Text.Extents()
			size = txtSize
		}
		if l.Drawable != nil {
			logicalSize := l.Drawable.LogicalSize()
			switch {
			case noText:
				size = logicalSize
			case l.Side.Horizontal():
				size.Width += logicalSize.Width + l.Gap
				size.Height = max(size.Height, logicalSize.Height)
			default:
				size.Height += logicalSize.Height + l.Gap
				size.Width = max(size.Width, logicalSize.Width)
			}
		}
	}
	return size.Ceil(), txtSize
}

// DefaultDraw provides the default drawing.
func (l *Label) DefaultDraw(canvas *Canvas, dirty Rect) {
	if !toolbox.IsNil(l.BackgroundInk) {
		canvas.DrawRect(dirty, l.BackgroundInk.Paint(canvas, dirty, paintstyle.Fill))
	}
	noText := l.Text.Empty()
	if l.Drawable == nil && noText {
		return
	}

	// Determine overall size of content
	size, txtSize := l.contentSizes()

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
	if !noText && l.Drawable != nil {
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
	if !noText {
		if l.Enabled() {
			l.Text.Draw(canvas, txtX, txtY+l.Text.Baseline())
		} else {
			savedDecorations := l.Text.AdjustDecorations(func(decoration *TextDecoration) {
				decoration.OnBackgroundInk = &ColorFilteredInk{
					OriginalInk: decoration.OnBackgroundInk,
					ColorFilter: Grayscale30Filter(),
				}
			})
			l.Text.Draw(canvas, txtX, txtY+l.Text.Baseline())
			l.Text.RestoreDecorations(savedDecorations)
		}
	}
	canvas.Restore()
}
