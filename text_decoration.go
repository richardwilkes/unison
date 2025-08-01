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
	"github.com/richardwilkes/toolbox/v2/xreflect"
	"github.com/richardwilkes/unison/enums/paintstyle"
)

// TextDecoration holds the decorations that can be applied to text when drawn.
type TextDecoration struct {
	Font            Font
	BackgroundInk   Ink
	OnBackgroundInk Ink
	BaselineOffset  float32
	Underline       bool
	StrikeThrough   bool
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
		d.BaselineOffset == other.BaselineOffset && d.OnBackgroundInk == other.OnBackgroundInk &&
		d.BackgroundInk == other.BackgroundInk && d.Font.Descriptor() == other.Font.Descriptor()
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
func (d *TextDecoration) DrawText(canvas *Canvas, text string, pt geom.Point, width float32) {
	r := geom.NewRect(pt.X, pt.Y-d.Font.Baseline(), width, d.Font.LineHeight())
	if !xreflect.IsNil(d.BackgroundInk) {
		canvas.DrawRect(r, d.BackgroundInk.Paint(canvas, r, paintstyle.Fill))
	}
	pt.Y += d.BaselineOffset
	paint := d.OnBackgroundInk.Paint(canvas, r, paintstyle.Fill)
	canvas.DrawSimpleString(text, pt, d.Font, paint)
	if d.Underline || d.StrikeThrough {
		pt.Y++
		if d.StrikeThrough {
			yy := pt.Y + 0.5 - d.Font.Baseline()/2
			paint.SetStrokeWidth(1)
			canvas.DrawLine(geom.NewPoint(pt.X, yy), geom.NewPoint(pt.X+width, yy), paint)
		}
		if d.Underline {
			paint.SetStrokeWidth(1)
			canvas.DrawLine(geom.NewPoint(pt.X, pt.Y+1), geom.NewPoint(pt.X+width, pt.Y+1), paint)
		}
	}
}
