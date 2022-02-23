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
	"github.com/richardwilkes/toolbox/xmath/geom32"
	"github.com/richardwilkes/toolbox/xmath/mathf32"
)

var _ Border = &LineBorder{}

// LineBorder private a lined border.
type LineBorder struct {
	insets       geom32.Insets
	ink          Ink
	cornerRadius float32
	noInset      bool
}

// NewLineBorder creates a new line border. The cornerRadius specifies the amount of rounding to use on the corners. The
// insets represent how thick the border will be drawn on that edge. If noInset is true, the Insets() method will return
// zeroes.
func NewLineBorder(ink Ink, cornerRadius float32, insets geom32.Insets, noInset bool) *LineBorder {
	return &LineBorder{
		insets:       insets,
		ink:          ink,
		cornerRadius: cornerRadius,
		noInset:      noInset,
	}
}

// Insets returns the insets describing the space the border occupies on each side.
func (b *LineBorder) Insets() geom32.Insets {
	if b.noInset {
		return geom32.Insets{}
	}
	return b.insets
}

// Draw the border into rect.
func (b *LineBorder) Draw(canvas *Canvas, rect geom32.Rect) {
	clip := rect
	clip.Inset(b.insets)
	path := NewPath()
	path.SetFillType(EvenOdd)
	if b.cornerRadius > 0 {
		path.RoundedRect(rect, b.cornerRadius, b.cornerRadius)
		radius := mathf32.Max(b.cornerRadius-((b.insets.Top+b.insets.Left+b.insets.Bottom+b.insets.Right)/4), 1)
		path.RoundedRect(clip, radius, radius)
	} else {
		path.Rect(rect)
		path.Rect(clip)
	}
	canvas.DrawPath(path, b.ink.Paint(canvas, rect, Fill))
}
