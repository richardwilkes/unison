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
	"strings"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/i18n"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/drag"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/enums/paintstyle"
	"github.com/richardwilkes/unison/enums/role"
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
	TabBorder:       NewEmptyBorder(geom.Insets{Top: 2, Left: 4, Bottom: 2, Right: 4}),
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
	LabelTheme      LabelTheme
	ButtonTheme     ButtonTheme
	Gap             float32
}

type dockTab struct {
	title    *Label
	button   *Button
	dockable Dockable
	Panel
	DockTabTheme
	pressed bool
}

func newDockTab(dockable Dockable) *dockTab {
	t := &dockTab{
		DockTabTheme: DefaultDockTabTheme,
		dockable:     dockable,
		title:        NewLabel(),
	}
	t.Self = t
	t.DrawCallback = t.draw
	t.SetBorder(t.TabBorder)
	flex := &FlexLayout{
		Columns:  1,
		HSpacing: t.Gap,
	}
	t.SetLayout(flex)
	t.title.LabelTheme = t.LabelTheme
	// The tab names itself from its dockable, so the label within it would only have an assistive technology say the
	// title a second time, marker and all.
	t.title.Accessibility.Role = role.None
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
			Size: geom.NewSize(fSize, fSize),
		}
		t.button.Accessibility.Name = i18n.Text("Close")
		t.button.SetLayoutData(&FlexLayoutData{HAlign: align.End, VAlign: align.Middle})
		t.AddChild(t.button)
		t.button.ClickCallback = func() { t.attemptClose() }
		flex.Columns++
	}
	t.MouseDownCallback = t.mouseDown
	t.MouseUpCallback = t.mouseUp
	t.MouseDragCallback = t.mouseDrag
	t.UpdateTooltipCallback = t.updateTooltip
	t.UpdateCursorCallback = t.updateCursor
	return t
}

func (t *dockTab) TitleIcon() Drawable {
	fSize := t.title.Font.Baseline()
	return t.dockable.TitleIcon(geom.NewSize(fSize, fSize))
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

func (t *dockTab) draw(gc *Canvas, _ geom.Rect) {
	var bg, fg Ink
	if t.pressed {
		bg = t.TabFocusedInk
		fg = t.OnTabFocusedInk
	} else if dc := Ancestor[*DockContainer](t.dockable); dc != nil && dc.CurrentDockable() == t.dockable {
		if dc == t.Window().CurrentFocus().Ancestor[*DockContainer]() {
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
	if t.title.OnBackgroundInk != fg {
		t.title.OnBackgroundInk = fg
		t.title.SetTitle(t.title.String())
	}
	if t.button != nil {
		t.button.OnBackgroundInk = fg
	}
	r := t.ContentRect(true)
	p := NewPath()
	p.MoveTo(geom.NewPoint(0, r.Height))
	p.LineTo(geom.NewPoint(0, 6))
	p.CubicTo(geom.NewPoint(0, 6), geom.NewPoint(0, 1), geom.NewPoint(6, 1))
	rightCornerStart := r.Width - 7
	p.LineTo(geom.NewPoint(rightCornerStart, 1))
	right := r.Width - 1
	p.CubicTo(geom.NewPoint(rightCornerStart, 1), geom.NewPoint(right, 1), geom.NewPoint(right, 7))
	p.LineTo(geom.NewPoint(right, r.Height))
	p.Close()
	bgPaint := bg.Paint(gc, r, paintstyle.Fill)
	gc.DrawPath(p, bgPaint)
	edgePaint := t.EdgeInk.Paint(gc, r, paintstyle.Stroke)
	gc.DrawPath(p, edgePaint)
}

func (t *dockTab) attemptClose() bool {
	if dc := Ancestor[*DockContainer](t.dockable); dc != nil {
		return dc.AttemptClose(t.dockable)
	}
	return false
}

func (t *dockTab) updateTooltip(_ geom.Point, suggestedAvoidInRoot geom.Rect) geom.Rect {
	if tip := t.dockable.Tooltip(); tip != "" {
		t.Tooltip = NewTooltipWithText(t.dockable.Tooltip())
	} else {
		t.Tooltip = nil
	}
	return suggestedAvoidInRoot
}

func (t *dockTab) updateCursor(_ geom.Point) *Cursor {
	if t.pressed {
		return ClosedHandCursor()
	}
	return OpenHandCursor()
}

func (t *dockTab) mouseDown(_ geom.Point, button, clickCount int, _ mod.Modifiers) bool {
	if button == ButtonRight && clickCount == 1 {
		// Claim the click so that the mouse up is delivered to this tab, where the context menu will be shown. The
		// menu cannot be popped up here, since it would swallow the mouse up event and leave the window convinced the
		// right button was still down, causing subsequent mouse moves to be treated as drags.
		return true
	}
	t.pressed = true
	t.MarkForRedraw()
	t.UpdateCursorNow()
	return true
}

func (t *dockTab) showContextMenu(where geom.Point) {
	if dc := Ancestor[*DockContainer](t.dockable); dc != nil && len(dc.Dockables()) > 1 {
		f := DefaultMenuFactory()
		cm := f.NewMenu(PopupMenuTemporaryBaseID|ContextMenuIDFlag, "", nil)
		cm.InsertItem(-1, f.NewItem(-1, i18n.Text("Close Other Tabs"), KeyBinding{}, nil, func(MenuItem) {
			dc.AttemptCloseAllExcept(t.dockable)
		}))
		cm.InsertItem(-1, f.NewItem(-1, i18n.Text("Close All Tabs"), KeyBinding{}, nil, func(MenuItem) {
			dc.AttemptCloseAll()
		}))
		where = t.PointToRoot(where)
		cm.Popup(geom.NewRect(where.X, where.Y, 1, 1), 0)
		cm.Dispose()
	}
}

func (t *dockTab) mouseDrag(where geom.Point, button int, _ mod.Modifiers) bool {
	if !t.pressed {
		return true
	}
	if dragDockable == nil && button == ButtonLeft && t.IsDragGesture(where) {
		if dc := Ancestor[*DockContainer](t.dockable); dc != nil {
			size := t.LogicalSize()
			img, err := NewImageFromDrawing(int(size.Width), int(size.Height), 144, func(c *Canvas) {
				t.Draw(c, geom.Rect{Size: size})
			})
			if err != nil {
				errs.Log(err)
				return true
			}
			dragDockable = t.dockable
			t.StartDrag(img, geom.Point{}, func() { dragDockable = nil }, drag.Move, drag.Data{
				Type: dockableDataType,
				Data: []byte{0},
			})
		}
	}
	return true
}

// LogicalSize is here to satisify the Drawable interface so that we can draw ourselves as we get dragged around.
func (t *dockTab) LogicalSize() geom.Size {
	return t.ContentRect(true).Size
}

// DrawInRect is here to satisify the Drawable interface so that we can draw ourselves as we get dragged around.
func (t *dockTab) DrawInRect(canvas *Canvas, rect geom.Rect, _ *SamplingOptions, _ *Paint) {
	t.Draw(canvas, rect)
}

func (t *dockTab) mouseUp(where geom.Point, button int, _ mod.Modifiers) bool {
	defer t.UpdateCursorNow()
	if button == ButtonRight {
		if where.In(t.ContentRect(true)) {
			t.showContextMenu(where)
		}
		return true
	}
	if !t.pressed {
		return true
	}
	if where.In(t.ContentRect(true)) {
		if dc := Ancestor[*DockContainer](t.dockable); dc != nil {
			switch {
			case dc.CurrentDockable() != t.dockable:
				dc.SetCurrentDockable(t.dockable)
			case dc != t.Window().CurrentFocus().Ancestor[*DockContainer]():
				dc.AcquireFocus()
			}
		}
	}
	t.pressed = false
	t.MarkForRedraw()
	return true
}

// ProvideAccessibility describes the tab to assistive technologies. The name is the dockable's title without the marker
// that says it has unsaved changes, since that marker is punctuation a screen reader would read out as part of the
// title; a tab for a modified dockable is still the tab for that dockable. What the marker says is said in words
// instead, as part of the description, since a person who cannot see the marker has no other way to learn that a
// dockable has changes that have not been saved.
func (t *dockTab) ProvideAccessibility(b *AccessibilityBuilder) {
	node := b.Node()
	if node.Role == role.Auto {
		node.Role = role.Tab
	}
	if node.Name == "" {
		node.Name = t.dockable.Title()
	}
	if node.Description == "" {
		node.Description = t.dockable.Tooltip()
	}
	if t.dockable.Modified() {
		modified := i18n.Text("Modified")
		if node.Description == "" {
			node.Description = modified
		} else {
			node.Description += ", " + modified
		}
	}
	node.Selectable = true
	if dc := Ancestor[*DockContainer](t.dockable); dc != nil {
		node.Selected = dc.content.axCurrent() == t.dockable
	}
	node.Actions = node.Actions.With(accessibility.Press, accessibility.Select)
}

// PerformAccessibilityAction carries out a request from an assistive technology. Pressing or selecting a tab brings its
// dockable to the front of the container, which is what clicking the tab does.
func (t *dockTab) PerformAccessibilityAction(req accessibility.ActionRequest) bool {
	switch req.Action {
	case accessibility.Press, accessibility.Select:
		dc := Ancestor[*DockContainer](t.dockable)
		if dc == nil {
			return false
		}
		dc.SetCurrentDockable(t.dockable)
		return true
	default:
		return false
	}
}
