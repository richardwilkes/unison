// Copyright ©2021-2024 by Richard A. Wilkes. All rights reserved.
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

	"github.com/richardwilkes/toolbox/i18n"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/paintstyle"
)

// TabCloser defines the methods that must be implemented to cause the tabs to show a close button.
type TabCloser interface {
	// MayAttemptClose returns true if a call to AttemptClose() is permitted.
	MayAttemptClose() bool
	// AttemptClose attempts to close the tab. On success, returns true.
	AttemptClose() bool
}

// DefaultDockTabTheme holds the default DockTabTheme values for DockTabs. Modifying this data will not alter existing
// DockTabs, but will alter any DockTabs created in the future.
var DefaultDockTabTheme = DockTabTheme{
	BackgroundInk:   ThemeAboveSurface,
	OnBackgroundInk: ThemeOnAboveSurface,
	EdgeInk:         ThemeSurfaceEdge,
	TabFocusedInk:   ThemeFocus,
	OnTabFocusedInk: ThemeOnFocus,
	TabCurrentInk:   ThemeDeepestFocus,
	OnTabCurrentInk: ThemeOnDeepestFocus,
	TabBorder:       NewEmptyBorder(Insets{Top: 2, Left: 4, Bottom: 2, Right: 4}),
	Gap:             4,
	LabelTheme:      defaultDockLabelTheme(),
	ButtonTheme:     defaultDockButtonTheme(),
}

func defaultDockLabelTheme() LabelTheme {
	theme := DefaultLabelTheme
	theme.Font = SystemFont
	return theme
}

func defaultDockButtonTheme() ButtonTheme {
	theme := DefaultButtonTheme
	theme.HideBase = true
	theme.SelectionInk = ThemeWarning
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
	pressed  bool
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
	t.title.SetTitle(t.fullTitle())
	t.title.Drawable = t.TitleIcon()
	t.title.SetLayoutData(&FlexLayoutData{HGrab: true, VAlign: align.Middle})
	t.AddChild(t.title)
	if _, ok := t.dockable.(TabCloser); ok {
		t.button = NewButton()
		t.button.ButtonTheme = t.ButtonTheme
		t.button.SetFocusable(false)
		fSize := t.LabelTheme.Font.Baseline()
		t.button.Drawable = &DrawableSVG{
			SVG:  CircledXSVG,
			Size: Size{Width: fSize, Height: fSize},
		}
		t.button.SetLayoutData(&FlexLayoutData{HAlign: align.End, VAlign: align.Middle})
		t.AddChild(t.button)
		t.button.ClickCallback = func() { t.attemptClose() }
		flex.Columns++
	}
	t.MouseDownCallback = t.mouseDown
	t.MouseUpCallback = t.mouseUp
	t.MouseDragCallback = t.mouseDrag
	t.UpdateTooltipCallback = t.updateTooltip
	return t
}

func (t *dockTab) TitleIcon() Drawable {
	fSize := t.title.Font.Baseline()
	return t.dockable.TitleIcon(Size{Width: fSize, Height: fSize})
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
	if title != t.title.String() || t.title.Drawable != drawable {
		t.title.SetTitle(title)
		t.title.Drawable = drawable
		t.NeedsLayout = true
		t.title.NeedsLayout = true
		if p := t.Parent(); p != nil {
			p.NeedsLayout = true
		}
		t.MarkForRedraw()
	}
}

func (t *dockTab) draw(gc *Canvas, _ Rect) {
	var bg, fg Ink
	if t.pressed {
		bg = t.TabFocusedInk
		fg = t.OnTabFocusedInk
	} else if dc := Ancestor[*DockContainer](t.dockable); dc != nil && dc.CurrentDockable() == t.dockable {
		if dc == Ancestor[*DockContainer](t.Window().Focus()) {
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
		t.button.OnBackgroundInk = fg
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
	gc.DrawPath(p, bg.Paint(gc, r, paintstyle.Fill))
	gc.DrawPath(p, t.EdgeInk.Paint(gc, r, paintstyle.Stroke))
}

func (t *dockTab) attemptClose() bool {
	if dc := Ancestor[*DockContainer](t.dockable); dc != nil {
		return dc.AttemptClose(t.dockable)
	}
	return false
}

func (t *dockTab) updateTooltip(_ Point, suggestedAvoidInRoot Rect) Rect {
	if tip := t.dockable.Tooltip(); tip != "" {
		t.Tooltip = NewTooltipWithText(t.dockable.Tooltip())
	} else {
		t.Tooltip = nil
	}
	return suggestedAvoidInRoot
}

func (t *dockTab) mouseDown(where Point, button, clickCount int, _ Modifiers) bool {
	if button == ButtonRight && clickCount == 1 && !t.Window().InDrag() {
		if dc := Ancestor[*DockContainer](t.dockable); dc != nil {
			if len(dc.Dockables()) > 1 {
				f := DefaultMenuFactory()
				cm := f.NewMenu(PopupMenuTemporaryBaseID|ContextMenuIDFlag, "", nil)
				cm.InsertItem(-1, f.NewItem(-1, i18n.Text("Close Other Tabs"), KeyBinding{}, nil, func(MenuItem) {
					dc.AttemptCloseAllExcept(t.dockable)
				}))
				cm.InsertItem(-1, f.NewItem(-1, i18n.Text("Close All Tabs"), KeyBinding{}, nil, func(MenuItem) {
					dc.AttemptCloseAll()
				}))
				cm.Popup(Rect{
					Point: t.PointToRoot(where),
					Size: Size{
						Width:  1,
						Height: 1,
					},
				}, 0)
				cm.Dispose()
				return true
			}
		}
	}
	t.pressed = true
	t.MarkForRedraw()
	return true
}

func (t *dockTab) mouseDrag(where Point, _ int, _ Modifiers) bool {
	if !t.pressed {
		return true
	}
	if t.IsDragGesture(where) {
		if dc := Ancestor[*DockContainer](t.dockable); dc != nil {
			icon := t.TitleIcon()
			size := icon.LogicalSize()
			t.StartDataDrag(&DragData{
				Data:     map[string]any{dc.Dock.DragKey: t.dockable},
				Drawable: icon,
				Ink:      t.title.OnBackgroundInk,
				Offset:   Point{X: -size.Width / 2, Y: -size.Height / 2},
			})
		}
	}
	return true
}

func (t *dockTab) mouseUp(where Point, _ int, _ Modifiers) bool {
	if !t.pressed {
		return true
	}
	if where.In(t.ContentRect(true)) {
		if dc := Ancestor[*DockContainer](t.dockable); dc != nil {
			switch {
			case dc.CurrentDockable() != t.dockable:
				dc.SetCurrentDockable(t.dockable)
			case dc != Ancestor[*DockContainer](t.Window().Focus()):
				dc.AcquireFocus()
			}
		}
	}
	t.pressed = false
	t.MarkForRedraw()
	return true
}
