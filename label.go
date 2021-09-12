// Copyright ©2021 by Richard A. Wilkes. All rights reserved.
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

// Label represents non-interactive text and/or an image.
type Label struct {
	Panel
	Font   FontProvider
	Ink    Ink
	Image  *Image
	Text   string
	Gap    float32
	HAlign Alignment
	VAlign Alignment
	Side   Side
}

// NewLabel creates a new, empty label.
func NewLabel() *Label {
	l := &Label{
		Gap:    3,
		VAlign: MiddleAlignment,
		Side:   LeftSide,
	}
	l.Self = l
	l.SetSizer(l.DefaultSizes)
	l.DrawCallback = l.DefaultDraw
	return l
}

// DefaultSizes provides the default sizing.
func (l *Label) DefaultSizes(hint geom32.Size) (min, pref, max geom32.Size) {
	pref = LabelSize(l.Text, ChooseFont(l.Font, LabelFont), l.Image, l.Side, l.Gap)
	if b := l.Border(); b != nil {
		pref.AddInsets(b.Insets())
	}
	pref.GrowToInteger()
	pref.ConstrainForHint(hint)
	return pref, pref, pref
}

// DefaultDraw provides the default drawing.
func (l *Label) DefaultDraw(canvas *Canvas, dirty geom32.Rect) {
	DrawLabel(canvas, l.ContentRect(false), l.HAlign, l.VAlign, l.Text, ChooseFont(l.Font, LabelFont),
		ChooseInk(l.Ink, OnBackgroundColor), l.Image, l.Side, l.Gap, !l.Enabled())
}

// LabelSize returns the preferred size of a label. Provided as a standalone
// function so that other types of panels can make use of it.
func LabelSize(text string, font *Font, image *Image, imgSide Side, imgGap float32) geom32.Size {
	var size geom32.Size
	if text != "" {
		size = font.Extents(text)
		size.GrowToInteger()
	}
	adjustLabelSizeForImage(text, image, imgSide, imgGap, &size)
	size.GrowToInteger()
	return size
}

func adjustLabelSizeForImage(text string, image *Image, imgSide Side, imgGap float32, size *geom32.Size) {
	if image != nil {
		logicalSize := image.LogicalSize()
		switch {
		case text == "":
			*size = logicalSize
		case imgSide.Horizontal():
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

// DrawLabel draws a label. Provided as a standalone function so that other types of panels can make use of it.
func DrawLabel(canvas *Canvas, rect geom32.Rect, hAlign, vAlign Alignment, text string, font *Font, textInk Ink, image *Image, imgSide Side, imgGap float32, applyDisabledImageFilter bool) {
	origRect := rect
	// Determine overall size of content
	var size, txtSize geom32.Size
	if text != "" {
		txtSize = font.Extents(text)
		size = txtSize
	}
	adjustLabelSizeForImage(text, image, imgSide, imgGap, &size)

	// Adjust the working area for the content size
	switch hAlign {
	case MiddleAlignment, FillAlignment:
		rect.X = mathf32.Floor(rect.X + (rect.Width-size.Width)/2)
	case EndAlignment:
		rect.X += rect.Width - size.Width
	default: // StartAlignment
	}
	switch vAlign {
	case MiddleAlignment, FillAlignment:
		rect.Y = mathf32.Floor(rect.Y + (rect.Height-size.Height)/2)
	case EndAlignment:
		rect.Y += rect.Height - size.Height
	default: // StartAlignment
	}
	rect.Size = size

	// Determine image and text areas
	imgX := rect.X
	imgY := rect.Y
	txtX := rect.X
	txtY := rect.Y
	if text != "" && image != nil {
		logicalSize := image.LogicalSize()
		switch imgSide {
		case TopSide:
			txtY += logicalSize.Height + imgGap
			if logicalSize.Width > txtSize.Width {
				txtX = mathf32.Floor(txtX + (logicalSize.Width-txtSize.Width)/2)
			} else {
				imgX = mathf32.Floor(imgX + (txtSize.Width-logicalSize.Width)/2)
			}
		case LeftSide:
			txtX += logicalSize.Width + imgGap
			if logicalSize.Height > txtSize.Height {
				txtY = mathf32.Floor(txtY + (logicalSize.Height-txtSize.Height)/2)
			} else {
				imgY = mathf32.Floor(imgY + (txtSize.Height-logicalSize.Height)/2)
			}
		case BottomSide:
			imgY += rect.Height - logicalSize.Height
			txtY = imgY - (imgGap + txtSize.Height)
			if logicalSize.Width > txtSize.Width {
				txtX = mathf32.Floor(txtX + (logicalSize.Width-txtSize.Width)/2)
			} else {
				imgX = mathf32.Floor(imgX + (txtSize.Width-logicalSize.Width)/2)
			}
		case RightSide:
			imgX += rect.Width - logicalSize.Width
			txtX = imgX - (imgGap + txtSize.Width)
			if logicalSize.Height > txtSize.Height {
				txtY = mathf32.Floor(txtY + (logicalSize.Height-txtSize.Height)/2)
			} else {
				imgY = mathf32.Floor(imgY + (txtSize.Height-logicalSize.Height)/2)
			}
		}
	}

	canvas.Save()
	canvas.ClipRect(rect, IntersectClipOp, false)

	// Draw the image
	if image != nil {
		rect.X = imgX
		rect.Y = imgY
		rect.Size = image.LogicalSize()
		canvas.DrawImageInRect(image, rect, nil, imagePaint(applyDisabledImageFilter))
	}

	// Draw the text
	if text != "" {
		canvas.DrawSimpleText(text, txtX, txtY+font.Baseline(), font, textInk.Paint(canvas, origRect, Fill))
	}
	canvas.Restore()
}

var disabledImagePaint *Paint

func imagePaint(disabledImageFilter bool) *Paint {
	if disabledImageFilter {
		if disabledImagePaint == nil {
			disabledImagePaint = NewPaint()
			disabledImagePaint.SetColorFilter(NewMatrixColorFilter([]float32{
				0.2126, 0.7152, 0.0722, 0, 0,
				0.2126, 0.7152, 0.0722, 0, 0,
				0.2126, 0.7152, 0.0722, 0, 0,
				0, 0, 0, 1, -0.67,
			}))
		}
		return disabledImagePaint
	}
	return nil
}
