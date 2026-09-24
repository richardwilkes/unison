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
	"fmt"
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
	title    *dockTabTitle
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
	}
	t.Self = t
	t.title = newDockTabTitle(t)
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
		// A right press is left unclaimed, so that its release cannot close the dockable, unless the button already
		// holds a press: the window offers every press afresh, and a declined one would leave the release of the button
		// that began the gesture reaching no panel, with the close button drawn pressed for good. The right release is
		// ignored either way, so that it cannot click. See dockTab.mouseDown.
		buttonDown, buttonUp := t.button.MouseDownCallback, t.button.MouseUpCallback
		t.button.MouseDownCallback = func(where geom.Point, button, clickCount int, mods mod.Modifiers) bool {
			if button == ButtonRight {
				return t.button.Pressed
			}
			return buttonDown(where, button, clickCount, mods)
		}
		t.button.MouseUpCallback = func(where geom.Point, button int, mods mod.Modifiers) bool {
			if button == ButtonRight {
				return true
			}
			return buttonUp(where, button, mods)
		}
		flex.Columns++
	}
	t.MouseDownCallback = t.mouseDown
	t.MouseUpCallback = t.mouseUp
	t.MouseDragCallback = t.mouseDrag
	t.ContextMenuCallback = t.contextMenu
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

// mouseDown leaves a right press unclaimed, so that it never brings the dockable to the front or starts a drag, unless
// the tab already holds a press: the window offers every press afresh, and the release of the button that began the
// gesture would otherwise reach no panel, leaving the tab drawn pressed. mouseUp ignores the right release.
func (t *dockTab) mouseDown(_ geom.Point, button, _ int, _ mod.Modifiers) bool {
	if button == ButtonRight {
		return t.pressed
	}
	t.pressed = true
	t.MarkForRedraw()
	t.UpdateCursorNow()
	return true
}

// WithholdsContextMenu implements ContextMenuWithholder: the tab has no menu while its dockable is in no container or
// is alone in it.
func (t *dockTab) WithholdsContextMenu() bool {
	dc := Ancestor[*DockContainer](t.dockable)
	return dc == nil || len(dc.Dockables()) < 2
}

// dockTabTitle is a tab's title label. A right-click on a tab most often lands on the title, and only the panel under
// the pointer is asked for a contextual menu, so the title offers whatever menu the tab offers, through the tab's
// current ContextMenuCallback, and withholds it exactly when the tab offers none.
type dockTabTitle struct {
	tab *dockTab
	Label
}

func newDockTabTitle(tab *dockTab) *dockTabTitle {
	l := &dockTabTitle{tab: tab}
	l.LabelTheme = DefaultLabelTheme
	l.Self = l
	l.SetSizer(l.DefaultSizes)
	l.DrawCallback = l.DefaultDraw
	l.ContextMenuCallback = l.contextMenu
	return l
}

// contextMenu builds the tab's menu through the tab's current ContextMenuCallback, so that one an application replaced
// is the one a right-click on the title opens, with the position translated into the tab's coordinates.
func (l *dockTabTitle) contextMenu(where geom.Point) Menu {
	callback := l.tab.ContextMenuCallback
	if callback == nil {
		return nil
	}
	return callback(l.PointTo(where, l.tab.AsPanel()))
}

// WithholdsContextMenu implements ContextMenuWithholder: the title offers a menu exactly when the tab itself does.
func (l *dockTabTitle) WithholdsContextMenu() bool {
	return !l.tab.offersContextMenu()
}

// contextMenu builds the tab's contextual menu. It is not asked while the tab withholds its menu (see
// WithholdsContextMenu), so dc is never nil.
func (t *dockTab) contextMenu(_ geom.Point) Menu {
	dc := Ancestor[*DockContainer](t.dockable)
	f := DefaultMenuFactory()
	cm := f.NewMenu(PopupMenuTemporaryBaseID|ContextMenuIDFlag, "", nil)
	cm.InsertItem(-1, f.NewItem(-1, i18n.Text("Close Other Tabs"), KeyBinding{}, nil, func(MenuItem) {
		dc.AttemptCloseAllExcept(t.dockable)
	}))
	cm.InsertItem(-1, f.NewItem(-1, i18n.Text("Close All Tabs"), KeyBinding{}, nil, func(MenuItem) {
		dc.AttemptCloseAll()
	}))
	return cm
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
	if !t.pressed || button == ButtonRight {
		// A right press is only claimed on behalf of a press already held, whose own release ends it; see mouseDown.
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
		if node.Description == "" {
			node.Description = i18n.Text("Modified")
		} else {
			// A format string rather than a concatenation, so that a language needing the marker somewhere other than
			// after the description it is added to, or joined with something other than a comma, can say so.
			node.Description = fmt.Sprintf(i18n.Text("%s, Modified"), node.Description)
		}
	}
	node.Selectable = true
	dc := Ancestor[*DockContainer](t.dockable)
	if dc != nil {
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
