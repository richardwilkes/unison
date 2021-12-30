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
	"strings"

	"github.com/richardwilkes/toolbox/xmath/geom32"
)

type Saver interface {
	Modified() bool
	AddDataModifiedListener(func())
}

type dockTab struct {
	Panel
	dockable Dockable
	title    *Label
	button   *Button
}

func newDockTab(dockable Dockable) *dockTab {
	t := &dockTab{
		dockable: dockable,
		title:    NewLabel(),
	}
	t.Self = t
	t.DrawCallback = t.draw
	t.SetBorder(NewEmptyBorder(geom32.Insets{
		Top:    2,
		Left:   4,
		Bottom: 2,
		Right:  4,
	}))
	t.SetLayout(&FlexLayout{
		Columns:  1,
		HSpacing: 4,
		VSpacing: 4,
		HAlign:   StartAlignment,
		VAlign:   StartAlignment,
	})
	t.title.Text = t.fullTitle()
	t.title.Drawable = t.dockable.TitleIcon()
	t.title.SetLayoutData(&FlexLayoutData{HGrab: true})
	t.AddChild(t.title)
	if _, ok := t.dockable.(TabCloser); ok {
		t.button = NewButton()
		fSize := ChooseFont(t.title.Font, LabelFont).Baseline()
		t.button.Drawable = &DrawableSVG{
			SVG:  CircledXSVG(),
			Size: geom32.Size{Width: fSize, Height: fSize},
		}
		t.button.SetLayoutData(&FlexLayoutData{HAlign: EndAlignment})
		t.AddChild(t.button)
		t.button.ClickCallback = t.attemptClose
	}
	if saver, ok := t.dockable.(Saver); ok {
		saver.AddDataModifiedListener(t.updateDataModified)
	}
	t.Tooltip = NewTooltipWithText(t.dockable.Tooltip())
	t.MouseDownCallback = t.mouseDown
	return t
}

func (t *dockTab) fullTitle() string {
	var buffer strings.Builder
	if saver, ok := t.dockable.(Saver); ok && saver.Modified() {
		buffer.WriteByte('*')
	}
	buffer.WriteString(t.dockable.Title())
	return buffer.String()
}

func (t *dockTab) updateDataModified() {
	title := t.fullTitle()
	if t.title.Text != title {
		t.title.Text = title
		t.title.MarkForLayoutAndRedraw()
	}
}

func (t *dockTab) updateTitle() {
	t.updateDataModified()
	drawable := t.dockable.TitleIcon()
	if t.title.Drawable != drawable {
		t.title.Drawable = drawable
		t.title.MarkForLayoutAndRedraw()
	}
	t.Tooltip = NewTooltipWithText(t.dockable.Tooltip())
}

func (t *dockTab) draw(gc *Canvas, rect geom32.Rect) {
	var bg, fg Ink
	if dc := DockContainerFor(t.dockable); dc != nil && dc.CurrentDockable() == t.dockable {
		if dc == FocusedDockContainerFor(t.Window()) {
			bg = TabFocusedColor
			fg = OnTabFocusedColor
		} else {
			bg = TabCurrentColor
			fg = OnTabCurrentColor
		}
	} else {
		bg = ControlColor
		fg = OnControlColor
	}
	t.title.Ink = fg
	if t.button != nil {
		t.button.EnabledColor = fg
	}
	r := t.ContentRect(true)
	p := NewPath()
	p.MoveTo(0, r.Height)
	p.LineTo(0, 6)
	p.CubicToPt(geom32.NewPoint(0, 6), geom32.NewPoint(0, 1), geom32.NewPoint(6, 1))
	p.LineTo(r.Width-7, 1)
	p.CubicToPt(geom32.NewPoint(r.Width-7, 1), geom32.NewPoint(r.Width-1, 1), geom32.NewPoint(r.Width-1, 7))
	p.LineTo(r.Width-1, r.Height)
	p.Close()
	gc.DrawPath(p, bg.Paint(gc, r, Fill))
	gc.DrawPath(p, DividerColor.Paint(gc, r, Stroke))
}

func (t *dockTab) attemptClose() {
	if dc := DockContainerFor(t.dockable); dc != nil {
		dc.AttemptClose(t.dockable)
	}
}

func (t *dockTab) mouseDown(where geom32.Point, button, clickCount int, mod Modifiers) bool {
	if dc := DockContainerFor(t.dockable); dc != nil {
		switch {
		case dc.CurrentDockable() != t.dockable:
			dc.SetCurrentDockable(t.dockable)
		case dc != FocusedDockContainerFor(t.Window()):
			dc.AcquireFocus()
		}
	}
	return true
}
