// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package demo

import (
	"github.com/richardwilkes/toolbox/xmath/geom32"
	"github.com/richardwilkes/unison"
)

var (
	_ unison.Dockable  = &DockablePanel{}
	_ unison.TabCloser = &DockablePanel{}
)

// DockablePanel provides a sample dockable panel.
type DockablePanel struct {
	unison.Panel
	Text  string
	Tip   string
	Color unison.Ink
}

// NewDockablePanel creates a new sample dockable panel.
func NewDockablePanel(title, tip string, background unison.Ink) *DockablePanel {
	d := &DockablePanel{
		Text:  title,
		Tip:   tip,
		Color: background,
	}
	d.Self = d
	d.DrawCallback = d.draw
	d.SetSizer(func(_ geom32.Size) (min, pref, max geom32.Size) {
		pref.Width = 200
		pref.Height = 100
		return min, pref, unison.MaxSize(max)
	})
	return d
}

func (d *DockablePanel) draw(gc *unison.Canvas, rect geom32.Rect) {
	gc.DrawRect(rect, d.Color.Paint(gc, rect, unison.Fill))
}

// TitleIcon implements Dockable.
func (d *DockablePanel) TitleIcon(suggestedSize geom32.Size) unison.Drawable {
	return &unison.DrawableSVG{
		SVG:  unison.DocumentSVG(),
		Size: suggestedSize,
	}
}

// Title implements Dockable.
func (d *DockablePanel) Title() string {
	return d.Text
}

// Tooltip implements Dockable.
func (d *DockablePanel) Tooltip() string {
	return d.Tip
}

// Modified implements Dockable.
func (d *DockablePanel) Modified() bool {
	return false
}

// MayAttemptClose implements TabCloser.
func (d *DockablePanel) MayAttemptClose() bool {
	return true
}

// AttemptClose implements TabCloser.
func (d *DockablePanel) AttemptClose() {
	if dc := unison.DockContainerFor(d); dc != nil {
		dc.Close(d)
	}
}
