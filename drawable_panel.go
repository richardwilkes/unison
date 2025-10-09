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

// DrawablePanel provides a Panel that displays a Drawable.
type DrawablePanel struct {
	Drawable Drawable
	Ink      Ink
	Panel
}

// NewDrawablePanel creates a new DrawablePanel.
func NewDrawablePanel() *DrawablePanel {
	d := &DrawablePanel{}
	d.Self = d
	d.SetSizer(d.DefaultSizes)
	d.DrawCallback = d.DefaultDraw
	return d
}

// DefaultSizes provides the default sizing.
func (d *DrawablePanel) DefaultSizes(hint geom.Size) (minSize, prefSize, maxSize geom.Size) {
	var border geom.Size
	if b := d.Border(); b != nil {
		border = b.Insets().Size()
	}
	prefSize = d.Drawable.LogicalSize().Add(border).ConstrainForHint(hint)
	return prefSize, prefSize, prefSize
}

// DefaultDraw provides the default drawing.
func (d *DrawablePanel) DefaultDraw(canvas *Canvas, _ geom.Rect) {
	var paint *Paint
	r := d.ContentRect(false)
	if !xreflect.IsNil(d.Ink) {
		paint = d.Ink.Paint(canvas, r, paintstyle.Fill)
	}
	d.Drawable.DrawInRect(canvas, r, nil, paint)
}
