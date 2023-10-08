// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import "github.com/richardwilkes/unison/enums/paintstyle"

// TextDecoration holds the decorations that can be applied to text when drawn.
type TextDecoration struct {
	Font           Font
	Foreground     Ink
	Background     Ink
	BaselineOffset float32
	Underline      bool
	StrikeThrough  bool
}

// Equivalent returns true if this TextDecoration is equivalent to the other.
func (d *TextDecoration) Equivalent(other *TextDecoration) bool {
	if d == nil {
		return other == nil
	}
	if other == nil {
		return false
	}
	return d.Underline == other.Underline && d.StrikeThrough == other.StrikeThrough &&
		d.BaselineOffset == other.BaselineOffset && d.Foreground == other.Foreground &&
		d.Background == other.Background && d.Font.Descriptor() == other.Font.Descriptor()
}

// Clone the TextDecoration.
func (d *TextDecoration) Clone() *TextDecoration {
	if d == nil {
		return nil
	}
	other := *d
	return &other
}

// DrawText draws the given text using this TextDecoration.
func (d *TextDecoration) DrawText(canvas *Canvas, text string, x, y, width float32) {
	r := Rect{Point: Point{X: x, Y: y - d.Font.Baseline()}, Size: Size{Width: width, Height: d.Font.LineHeight()}}
	if d.Background != nil {
		canvas.DrawRect(r, d.Background.Paint(canvas, r, paintstyle.Fill))
	}
	y += d.BaselineOffset
	paint := d.Foreground.Paint(canvas, r, paintstyle.Fill)
	canvas.DrawSimpleString(text, x, y, d.Font, paint)
	if d.Underline || d.StrikeThrough {
		y++
		if d.StrikeThrough {
			yy := y + 0.5 - d.Font.Baseline()/2
			paint.SetStrokeWidth(1)
			canvas.DrawLine(x, yy, x+width, yy, paint)
		}
		if d.Underline {
			paint.SetStrokeWidth(1)
			canvas.DrawLine(x, y+1, x+width, y+1, paint)
		}
	}
}
