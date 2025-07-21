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
	"github.com/richardwilkes/unison/enums/filltype"
	"github.com/richardwilkes/unison/enums/paintstyle"
)

var _ Border = &LineBorder{}

// LineBorder private a lined border.
type LineBorder struct {
	ink          Ink
	insets       geom.Insets
	cornerRadius geom.Size
	noInset      bool
}

// NewLineBorder creates a new line border. The cornerRadius specifies the amount of rounding to use on the corners. The
// insets represent how thick the border will be drawn on that edge. If noInset is true, the Insets() method will return
// zeroes.
func NewLineBorder(ink Ink, cornerRadius geom.Size, insets geom.Insets, noInset bool) *LineBorder {
	return &LineBorder{
		insets:       insets,
		ink:          ink,
		cornerRadius: cornerRadius,
		noInset:      noInset,
	}
}

// Insets returns the insets describing the space the border occupies on each side.
func (b *LineBorder) Insets() geom.Insets {
	if b.noInset {
		return geom.Insets{}
	}
	return b.insets
}

// Draw the border into rect.
func (b *LineBorder) Draw(canvas *Canvas, rect geom.Rect) {
	clip := rect.Inset(b.insets)
	path := NewPath()
	path.SetFillType(filltype.EvenOdd)
	if b.cornerRadius.Width > 0 || b.cornerRadius.Height > 0 {
		path.RoundedRect(rect, b.cornerRadius)
		path.RoundedRect(clip, b.cornerRadius.Sub(geom.NewUniformSize((b.insets.Width()+b.insets.Height())/4)).
			Max(geom.NewUniformSize(1)))
	} else {
		path.Rect(rect)
		path.Rect(clip)
	}
	canvas.DrawPath(path, b.ink.Paint(canvas, rect, paintstyle.Fill))
}
