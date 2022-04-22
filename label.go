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

// DefaultLabelTheme holds the default LabelTheme values for Labels. Modifying this data will not alter existing Labels,
// but will alter any Labels created in the future.
var DefaultLabelTheme = LabelTheme{
	Font:            LabelFont,
	OnBackgroundInk: OnBackgroundColor,
	Gap:             3,
	HAlign:          StartAlignment,
	VAlign:          MiddleAlignment,
	Side:            LeftSide,
}

// LabelTheme holds theming data for a Label.
type LabelTheme struct {
	Font            Font
	OnBackgroundInk Ink
	Gap             float32
	HAlign          Alignment
	VAlign          Alignment
	Side            Side
	Underline       bool
	StrikeThrough   bool
}

// Label represents non-interactive text and/or a Drawable.
type Label struct {
	Panel
	LabelTheme
	Drawable  Drawable
	Text      string
	textCache TextCache
}

// NewLabel creates a new, empty label.
func NewLabel() *Label {
	l := &Label{LabelTheme: DefaultLabelTheme}
	l.Self = l
	l.SetSizer(l.DefaultSizes)
	l.DrawCallback = l.DefaultDraw
	return l
}

// DefaultSizes provides the default sizing.
func (l *Label) DefaultSizes(hint Size) (min, pref, max Size) {
	text := l.textCache.Text(l.Text, l.Font)
	if text == nil && l.Drawable == nil {
		pref.Height = l.Font.LineHeight()
		pref.GrowToInteger()
	} else {
		pref = LabelSize(text, l.Drawable, l.Side, l.Gap)
	}
	if b := l.Border(); b != nil {
		pref.AddInsets(b.Insets())
	}
	pref.GrowToInteger()
	pref.ConstrainForHint(hint)
	return pref, pref, pref
}

// DefaultDraw provides the default drawing.
func (l *Label) DefaultDraw(canvas *Canvas, dirty Rect) {
	txt := l.textCache.Text(l.Text, l.Font)
	if l.Underline {
		txt.ReplaceUnderline(l.Underline)
	}
	if l.StrikeThrough {
		txt.ReplaceStrikeThrough(l.StrikeThrough)
	}
	DrawLabel(canvas, l.ContentRect(false), l.HAlign, l.VAlign, txt, l.OnBackgroundInk, l.Drawable, l.Side, l.Gap,
		!l.Enabled())
}

// LabelSize returns the preferred size of a label. Provided as a standalone function so that other types of panels can
// make use of it.
func LabelSize(text *Text, drawable Drawable, drawableSide Side, imgGap float32) Size {
	var size Size
	if text != nil {
		size = text.Extents()
		size.GrowToInteger()
	}
	adjustLabelSizeForDrawable(text != nil, drawable, drawableSide, imgGap, &size)
	size.GrowToInteger()
	return size
}

// DrawLabel draws a label. Provided as a standalone function so that other types of panels can make use of it.
func DrawLabel(canvas *Canvas, rect Rect, hAlign, vAlign Alignment, text *Text, textInk Ink, drawable Drawable,
	drawableSide Side, imgGap float32, applyDisabledFilter bool,
) {
	if drawable == nil && text == nil {
		return
	}

	paint := textInk.Paint(canvas, rect, Fill)
	if applyDisabledFilter {
		paint.SetColorFilter(Grayscale30PercentFilter())
	}

	// Determine overall size of content
	var size, txtSize Size
	if text != nil {
		text.ReplacePaint(paint)
		txtSize = text.Extents()
		size = txtSize
	}
	adjustLabelSizeForDrawable(text != nil, drawable, drawableSide, imgGap, &size)

	// Adjust the working area for the content size
	switch hAlign {
	case MiddleAlignment, FillAlignment:
		rect.X = xmath.Floor(rect.X + (rect.Width-size.Width)/2)
	case EndAlignment:
		rect.X += rect.Width - size.Width
	default: // StartAlignment
	}
	switch vAlign {
	case MiddleAlignment, FillAlignment:
		rect.Y = xmath.Floor(rect.Y + (rect.Height-size.Height)/2)
	case EndAlignment:
		rect.Y += rect.Height - size.Height
	default: // StartAlignment
	}
	rect.Size = size

	// Determine drawable and text areas
	imgX := rect.X
	imgY := rect.Y
	txtX := rect.X //nolint:ifshort // Variable cannot be collapsed into the if, despite what the linter claims
	txtY := rect.Y
	if text != nil && drawable != nil {
		logicalSize := drawable.LogicalSize()
		switch drawableSide {
		case TopSide:
			txtY += logicalSize.Height + imgGap
			if logicalSize.Width > txtSize.Width {
				txtX = xmath.Floor(txtX + (logicalSize.Width-txtSize.Width)/2)
			} else {
				imgX = xmath.Floor(imgX + (txtSize.Width-logicalSize.Width)/2)
			}
		case LeftSide:
			txtX += logicalSize.Width + imgGap
			if logicalSize.Height > txtSize.Height {
				txtY = xmath.Floor(txtY + (logicalSize.Height-txtSize.Height)/2)
			} else {
				imgY = xmath.Floor(imgY + (txtSize.Height-logicalSize.Height)/2)
			}
		case BottomSide:
			imgY += rect.Height - logicalSize.Height
			txtY = imgY - (imgGap + txtSize.Height)
			if logicalSize.Width > txtSize.Width {
				txtX = xmath.Floor(txtX + (logicalSize.Width-txtSize.Width)/2)
			} else {
				imgX = xmath.Floor(imgX + (txtSize.Width-logicalSize.Width)/2)
			}
		case RightSide:
			imgX += rect.Width - logicalSize.Width
			txtX = imgX - (imgGap + txtSize.Width)
			if logicalSize.Height > txtSize.Height {
				txtY = xmath.Floor(txtY + (logicalSize.Height-txtSize.Height)/2)
			} else {
				imgY = xmath.Floor(imgY + (txtSize.Height-logicalSize.Height)/2)
			}
		}
	}

	canvas.Save()
	canvas.ClipRect(rect, IntersectClipOp, false)
	if drawable != nil {
		rect.X = imgX
		rect.Y = imgY
		rect.Size = drawable.LogicalSize()
		drawable.DrawInRect(canvas, rect, nil, paint)
	}
	if text != nil {
		text.Draw(canvas, txtX, txtY+text.Baseline())
	}
	canvas.Restore()
}

func adjustLabelSizeForDrawable(hasText bool, drawable Drawable, drawableSide Side, imgGap float32, size *Size) {
	if drawable != nil {
		logicalSize := drawable.LogicalSize()
		switch {
		case !hasText:
			*size = logicalSize
		case drawableSide.Horizontal():
			size.Width += logicalSize.Width + imgGap
			if size.Height < logicalSize.Height {
				size.Height = logicalSize.Height
			}
		default:
			size.Height += logicalSize.Height + imgGap
			if size.Width < logicalSize.Width {
				size.Width = logicalSize.Width
			}
		}
	}
}
