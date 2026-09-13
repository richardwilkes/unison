// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
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
	"github.com/richardwilkes/unison/enums/role"
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

// ProvideAccessibility describes the panel to assistive technologies. There is nothing in a drawable that says what it
// shows, so a panel that nothing has described is skipped rather than announced as an image of nothing. A tooltip is
// the usual way such a panel is described — markdown hangs an image's alt text there — so it becomes the name. That has
// to happen here rather than being left to the description the snapshot would otherwise take from the tooltip, since a
// node marked ignored is never reached to hear it.
func (d *DrawablePanel) ProvideAccessibility(b *AccessibilityBuilder) {
	node := b.Node()
	if node.Role == role.Auto {
		node.Role = role.Image
	}
	if node.Name == "" {
		node.Name = axTooltipText(d.AsPanel())
	}
	if node.Name == "" && xreflect.IsNil(d.Accessibility.LabeledBy) {
		node.Ignored = true
	}
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
