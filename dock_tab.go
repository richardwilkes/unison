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

// DefaultDockTabTheme holds the default DockTabTheme values for DockTabs. Modifying this data will not alter existing
// DockTabs, but will alter any DockTabs created in the future.
var DefaultDockTabTheme = DockTabTheme{
	BackgroundInk:   ControlColor,
	OnBackgroundInk: OnControlColor,
	EdgeInk:         ControlEdgeColor,
	TabFocusedInk:   TabFocusedColor,
	OnTabFocusedInk: OnTabFocusedColor,
	TabCurrentInk:   TabCurrentColor,
	OnTabCurrentInk: OnTabCurrentColor,
	TabBorder:       NewEmptyBorder(geom32.Insets{Top: 2, Left: 4, Bottom: 2, Right: 4}),
	Gap:             4,
	LabelTheme:      defaultLabelTheme(),
	ButtonTheme:     defaultButtonTheme(),
}

func defaultLabelTheme() LabelTheme {
	theme := DefaultLabelTheme
	theme.Font = SystemFont
	return theme
}

func defaultButtonTheme() ButtonTheme {
	theme := DefaultButtonTheme
	theme.HideBase = true
	return theme
}

// DockTabTheme holds theming data for a DockTab.
type DockTabTheme struct {
	BackgroundInk   Ink
	OnBackgroundInk Ink
	EdgeInk         Ink
	TabFocusedInk   Ink
	OnTabFocusedInk Ink
	TabCurrentInk   Ink
	OnTabCurrentInk Ink
	TabBorder       Border
	Gap             float32
	LabelTheme      LabelTheme
	ButtonTheme     ButtonTheme
}

type dockTab struct {
	Panel
	DockTabTheme
	dockable Dockable
	title    *Label
	button   *Button
}

func newDockTab(dockable Dockable) *dockTab {
	t := &dockTab{
		DockTabTheme: DefaultDockTabTheme,
		dockable:     dockable,
		title:        NewLabel(),
	}
	t.Self = t
	t.DrawCallback = t.draw
	t.SetBorder(t.DockTabTheme.TabBorder)
	flex := &FlexLayout{
		Columns:  1,
		HSpacing: t.Gap,
	}
	t.SetLayout(flex)
	t.title.LabelTheme = t.LabelTheme
	t.title.Text = t.fullTitle()
	t.title.Drawable = t.TitleIcon()
	t.title.SetLayoutData(&FlexLayoutData{HGrab: true, VAlign: MiddleAlignment})
	t.AddChild(t.title)
	if _, ok := t.dockable.(TabCloser); ok {
		t.button = NewButton()
		t.button.ButtonTheme = t.ButtonTheme
		t.button.SetFocusable(false)
		fSize := t.LabelTheme.Font.Baseline()
		t.button.Drawable = &DrawableSVG{
			SVG:  CircledXSVG(),
			Size: geom32.Size{Width: fSize, Height: fSize},
		}
		t.button.SetLayoutData(&FlexLayoutData{HAlign: EndAlignment, VAlign: MiddleAlignment})
		t.AddChild(t.button)
		t.button.ClickCallback = t.attemptClose
		flex.Columns++
	}
	t.MouseDownCallback = t.mouseDown
	t.MouseDragCallback = t.mouseDrag
	t.UpdateTooltipCallback = t.updateTooltip
	return t
}

func (t *dockTab) TitleIcon() Drawable {
	fSize := t.title.Font.Baseline()
	return t.dockable.TitleIcon(geom32.Size{Width: fSize, Height: fSize})
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
	drawable := t.TitleIcon()
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
			bg = t.TabFocusedInk
			fg = t.OnTabFocusedInk
		} else {
			bg = t.TabCurrentInk
			fg = t.OnTabCurrentInk
		}
	} else {
		bg = t.BackgroundInk
		fg = t.OnBackgroundInk
	}
	t.title.OnBackgroundInk = fg
	if t.button != nil {
		t.button.BackgroundInk = fg
	}
	r := t.ContentRect(true)
	p := NewPath()
	p.MoveTo(0, r.Height)
	p.LineTo(0, 6)
	p.CubicTo(0, 6, 0, 1, 6, 1)
	rightCornerStart := r.Width - 7
	p.LineTo(rightCornerStart, 1)
	right := r.Width - 1
	p.CubicTo(rightCornerStart, 1, right, 1, right, 7)
	p.LineTo(right, r.Height)
	p.Close()
	gc.DrawPath(p, bg.Paint(gc, r, Fill))
	gc.DrawPath(p, t.EdgeInk.Paint(gc, r, Stroke))
}

func (t *dockTab) attemptClose() {
	if dc := DockContainerFor(t.dockable); dc != nil {
		dc.AttemptClose(t.dockable)
	}
}

func (t *dockTab) updateTooltip(where geom32.Point, suggestedAvoidInRoot geom32.Rect) geom32.Rect {
	if tip := t.dockable.Tooltip(); tip != "" {
		t.Tooltip = NewTooltipWithText(t.dockable.Tooltip())
	} else {
		t.Tooltip = nil
	}
	return suggestedAvoidInRoot
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
			icon := t.TitleIcon()
			size := icon.LogicalSize()
			t.StartDataDrag(&DragData{
				Data:     map[string]interface{}{dc.Dock.DragKey: t.dockable},
				Drawable: icon,
				Ink:      t.title.OnBackgroundInk,
				Offset:   geom32.NewPoint(-size.Width/2, -size.Height/2),
			})
		}
	}
	return true
}
