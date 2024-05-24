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
	"github.com/richardwilkes/toolbox/xmath"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/paintstyle"
	"github.com/richardwilkes/unison/enums/pathop"
	"github.com/richardwilkes/unison/enums/side"
)

// LabelSize returns the preferred size of a label. Provided as a standalone function so that other types of panels can
// make use of it.
func LabelSize(text *Text, drawable Drawable, drawableSide side.Enum, imgGap float32) Size {
	var size Size
	hasText := !text.Empty()
	if hasText {
		size = text.Extents().Ceil()
	}
	adjustLabelSizeForDrawable(hasText, drawable, drawableSide, imgGap, &size)
	return size.Ceil()
}

// DrawLabel draws a label. Provided as a standalone function so that other types of panels can make use of it.
func DrawLabel(canvas *Canvas, rect Rect, hAlign, vAlign align.Enum, text *Text, textInk Ink, drawable Drawable, drawableSide side.Enum, imgGap float32, applyDisabledFilter bool) {
	noText := text.Empty()
	if drawable == nil && noText {
		return
	}

	fg := textInk
	if applyDisabledFilter {
		fg = &ColorFilteredInk{
			OriginalInk: fg,
			ColorFilter: Grayscale30Filter(),
		}
	}
	paint := fg.Paint(canvas, rect, paintstyle.Fill)

	// Determine overall size of content
	var size, txtSize Size
	if !noText {
		text.AdjustDecorations(func(decoration *TextDecoration) { decoration.OnBackgroundInk = fg })
		txtSize = text.Extents()
		size = txtSize
	}
	adjustLabelSizeForDrawable(!noText, drawable, drawableSide, imgGap, &size)

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
	imgX := rect.X
	imgY := rect.Y
	txtX := rect.X
	txtY := rect.Y
	if !noText && drawable != nil {
		logicalSize := drawable.LogicalSize()
		switch drawableSide {
		case side.Top:
			txtY += logicalSize.Height + imgGap
			if logicalSize.Width > txtSize.Width {
				txtX = xmath.Floor(txtX + (logicalSize.Width-txtSize.Width)/2)
			} else {
				imgX = xmath.Floor(imgX + (txtSize.Width-logicalSize.Width)/2)
			}
		case side.Left:
			txtX += logicalSize.Width + imgGap
			if logicalSize.Height > txtSize.Height {
				txtY = xmath.Floor(txtY + (logicalSize.Height-txtSize.Height)/2)
			} else {
				imgY = xmath.Floor(imgY + (txtSize.Height-logicalSize.Height)/2)
			}
		case side.Bottom:
			imgY += rect.Height - logicalSize.Height
			txtY = imgY - (imgGap + txtSize.Height)
			if logicalSize.Width > txtSize.Width {
				txtX = xmath.Floor(txtX + (logicalSize.Width-txtSize.Width)/2)
			} else {
				imgX = xmath.Floor(imgX + (txtSize.Width-logicalSize.Width)/2)
			}
		case side.Right:
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
	canvas.ClipRect(rect, pathop.Intersect, false)
	if drawable != nil {
		rect.X = imgX
		rect.Y = imgY
		rect.Size = drawable.LogicalSize()
		drawable.DrawInRect(canvas, rect, nil, paint)
	}
	if !noText {
		text.Draw(canvas, txtX, txtY+text.Baseline())
	}
	canvas.Restore()
}

func adjustLabelSizeForDrawable(hasText bool, drawable Drawable, drawableSide side.Enum, imgGap float32, size *Size) {
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
