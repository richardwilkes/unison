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
	"strings"

	"github.com/richardwilkes/toolbox/xmath/geom32"
)

// TabCloser defines the methods that must be implemented to cause the tabs to show a close button.
type TabCloser interface {
	MayAttemptClose() bool
	AttemptClose()
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
	flex := &FlexLayout{
		Columns:  1,
		HSpacing: 4,
	}
	t.SetLayout(flex)
	t.title.Font = SystemFont
	t.title.Text = t.fullTitle()
	t.title.Drawable = t.dockable.TitleIcon()
	t.title.SetLayoutData(&FlexLayoutData{HGrab: true, VAlign: MiddleAlignment})
	t.AddChild(t.title)
	if _, ok := t.dockable.(TabCloser); ok {
		t.button = NewButton()
		t.button.SetFocusable(false)
		fSize := ChooseFont(t.title.Font, SystemFont).Baseline()
		t.button.Drawable = &DrawableSVG{
			SVG:  CircledXSVG(),
			Size: geom32.Size{Width: fSize, Height: fSize},
		}
		t.button.SetLayoutData(&FlexLayoutData{HAlign: EndAlignment, VAlign: MiddleAlignment})
		t.button.HideBase = true
		t.AddChild(t.button)
		t.button.ClickCallback = t.attemptClose
		flex.Columns++
	}
	t.MouseDownCallback = t.mouseDown
	t.MouseDragCallback = t.mouseDrag
	t.UpdateTooltipCallback = t.updateTooltip
	return t
}

func (t *dockTab) fullTitle() string {
	var buffer strings.Builder
	if t.dockable.Modified() {
		buffer.WriteByte('*')
	}
	buffer.WriteString(t.dockable.Title())
	return buffer.String()
}

func (t *dockTab) updateTitle() {
	drawable := t.dockable.TitleIcon()
	title := t.fullTitle()
	if title != t.title.Text || t.title.Drawable != drawable {
		t.title.Text = title
		t.title.Drawable = drawable
		t.NeedsLayout = true
		t.title.NeedsLayout = true
		if p := t.Parent(); p != nil {
			p.NeedsLayout = true
		}
		t.MarkForRedraw()
	}
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
		t.button.BackgroundColor = fg
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

func (t *dockTab) updateTooltip(where geom32.Point, suggestedAvoid geom32.Rect) geom32.Rect {
	if tip := t.dockable.Tooltip(); tip != "" {
		t.Tooltip = NewTooltipWithText(t.dockable.Tooltip())
	} else {
		t.Tooltip = nil
	}
	return suggestedAvoid
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

func (t *dockTab) mouseDrag(where geom32.Point, button int, mod Modifiers) bool {
	if t.IsDragGesture(where) {
		if dc := DockContainerFor(t.dockable); dc != nil {
			icon := t.dockable.TitleIcon()
			size := icon.LogicalSize()
			t.StartDataDrag(&DragData{
				Data:     map[string]interface{}{dc.Dock.DragKey: t.dockable},
				Drawable: icon,
				Ink:      t.title.Ink,
				Offset:   geom32.NewPoint(-size.Width/2, -size.Height/2),
			})
		}
	}
	return true
}
