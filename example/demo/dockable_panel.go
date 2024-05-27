// Copyright (c) 2021-2024 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package demo

import (
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/enums/paintstyle"
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
	Color unison.Color
}

// NewDockablePanel creates a new sample dockable panel.
func NewDockablePanel(title, tip string, background unison.Color) *DockablePanel {
	d := &DockablePanel{
		Text:  title,
		Tip:   tip,
		Color: background,
	}
	d.Self = d
	d.DrawCallback = d.draw
	d.GainedFocusCallback = d.MarkForRedraw
	d.LostFocusCallback = d.MarkForRedraw
	d.MouseDownCallback = d.mouseDown
	d.SetFocusable(true)
	d.SetSizer(func(_ unison.Size) (minSize, prefSize, maxSize unison.Size) {
		prefSize.Width = 200
		prefSize.Height = 100
		return minSize, prefSize, unison.MaxSize(maxSize)
	})
	return d
}

func (d *DockablePanel) draw(gc *unison.Canvas, rect unison.Rect) {
	gc.DrawRect(rect, d.Color.Paint(gc, rect, paintstyle.Fill))
	if d.Focused() {
		txt := unison.NewText("Focused", &unison.TextDecoration{
			Font:            unison.EmphasizedSystemFont,
			OnBackgroundInk: d.Color.On(),
		})
		r := d.ContentRect(false)
		size := txt.Extents()
		txt.Draw(gc, r.X+(r.Width-size.Width)/2, r.Y+(r.Height-size.Height)/2+txt.Baseline())
	}
}

func (d *DockablePanel) mouseDown(_ unison.Point, _, _ int, _ unison.Modifiers) bool {
	if !d.Focused() {
		d.RequestFocus()
		d.MarkForRedraw()
	}
	return true
}

// TitleIcon implements Dockable.
func (d *DockablePanel) TitleIcon(suggestedSize unison.Size) unison.Drawable {
	return &unison.DrawableSVG{
		SVG:  unison.DocumentSVG,
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
func (d *DockablePanel) AttemptClose() bool {
	if dc := unison.Ancestor[*unison.DockContainer](d); dc != nil {
		dc.Close(d)
		return true
	}
	return false
}
