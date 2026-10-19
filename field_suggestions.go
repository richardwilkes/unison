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
	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xmath"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/behavior"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/enums/paintstyle"
	"github.com/richardwilkes/unison/enums/role"
)

// DefaultFieldSuggestionsTheme holds the default FieldSuggestionsTheme values for FieldSuggestions. Modifying this data
// will not alter existing FieldSuggestions, but will alter any FieldSuggestions created in the future.
var DefaultFieldSuggestionsTheme = FieldSuggestionsTheme{
	BackgroundInk: ThemeSurface,
	Border:        NewLineBorder(ThemeSurfaceEdge, geom.Size{}, geom.NewUniformInsets(1), false),
	Rows:          defaultFieldSuggestionsRowsTheme(),
}

// defaultFieldSuggestionsRowsTheme returns the list theme for the rows. The list never holds the keyboard focus, so its
// inactive selection inks are the active ones.
func defaultFieldSuggestionsRowsTheme() ListTheme {
	theme := DefaultListTheme
	theme.InactiveSelectionInk = theme.SelectionInk
	theme.OnInactiveSelectionInk = theme.OnSelectionInk
	return theme
}

// FieldSuggestionsTheme holds theming data for FieldSuggestions.
type FieldSuggestionsTheme struct {
	BackgroundInk Ink
	Border        Border
	// Rows is the theme given to the list when the FieldSuggestions is created.
	Rows ListTheme
}

// fieldSuggestionsHost is what a Field, its window, the accessibility snapshot and the headless screen use of a
// FieldSuggestions, none of which can name its row type.
type fieldSuggestionsHost interface {
	showing() bool
	hide()
	// keyDown handles a key pressed in the field while the suggestions are showing, returning true if it consumed it.
	keyDown(keyCode KeyCode, mods mod.Modifiers) bool
	// anchorContains returns true if where, in the root coordinates of the field's window, lies within the field.
	anchorContains(where geom.Point) bool
	// anchorLost returns true if the field has left its window or moved within it since the suggestions were placed, or
	// can no longer show them, being hidden or disabled.
	anchorLost() bool
	// popup returns the popup window, its content and the list, or nils until a window exists.
	popup() (window *Window, content, list *Panel)
	// describe adds the field's expanded state and the list it controls to the field's node.
	describe(node *accessibility.Node)
}

// FieldSuggestions shows a list of rows in a popup window below a Field (above it when there is no room below), such as
// the completions of what has been typed. The keyboard focus stays in the field: Down and Up move the highlight,
// PageDown and PageUp move it by a page, Return chooses the highlighted row and Escape hides the rows. Clicking a row
// chooses it. The rows hide when the field loses the focus, moves, is disabled or is hidden, when its window moves,
// resizes, hides, minimizes or loses the focus, on a press outside the field and when a menu opens.
type FieldSuggestions[T any] struct {
	// ChosenCallback is called when a row is chosen, after the rows have been hidden.
	ChosenCallback func(index int, row T)
	field          *Field
	list           *List[T]
	scroller       *ScrollPanel
	panel          *Panel
	wnd            *Window
	FieldSuggestionsTheme
	anchor geom.Rect
	// hoverHeldAt is where the pointer was, in the popup window's coordinates, when the rows were last shown; see
	// hoverIsHeld.
	hoverHeldAt geom.Point
	// MaxVisibleRows is the most rows shown at once; more are scrolled to. Defaults to 8.
	MaxVisibleRows int
	visible        bool
	inChosen       bool
	hoverHeld      bool
}

// NewFieldSuggestions creates suggestions for field, replacing any it already has. A field with role.Auto is given
// role.ComboBox. With Field.InstallDropdown also installed, the dropdown sees a bare Down key first.
func NewFieldSuggestions[T any](field *Field) *FieldSuggestions[T] {
	if field.suggestions != nil {
		field.suggestions.hide()
	}
	s := &FieldSuggestions[T]{
		FieldSuggestionsTheme: DefaultFieldSuggestionsTheme,
		MaxVisibleRows:        8,
		field:                 field,
	}
	s.list = NewList[T]()
	s.list.ListTheme = s.Rows
	s.list.SetFocusable(false)
	s.list.SetAllowMultipleSelection(false)
	s.list.KeyDownCallback = nil
	s.list.MouseDownCallback = func(where geom.Point, _, _ int, _ mod.Modifiers) bool {
		s.highlightAt(where)
		return true
	}
	s.list.MouseDragCallback = func(where geom.Point, _ int, _ mod.Modifiers) bool {
		s.highlightAt(where)
		return true
	}
	s.list.MouseUpCallback = func(where geom.Point, button int, _ mod.Modifiers) bool {
		if button == ButtonLeft {
			if i := s.rowUnder(where); i >= 0 {
				s.choose(i)
			}
		}
		return true
	}
	hover := func(where geom.Point, _ mod.Modifiers) bool {
		if !s.hoverIsHeld() {
			s.highlightAt(where)
		}
		return false
	}
	s.list.MouseEnterCallback = hover
	s.list.MouseMoveCallback = hover
	// Never reached by a click, which chooses on the release; set so that rows offer Press to assistive technologies.
	s.list.DoubleClickCallback = func() { s.choose(s.Selected()) }
	s.scroller = NewScrollPanel()
	s.scroller.Accessibility.Role = role.None
	s.scroller.SetContent(s.list, behavior.Follow, behavior.Fill)
	s.scroller.SetLayoutData(&FlexLayoutData{
		HAlign: align.Fill,
		VAlign: align.Fill,
		HGrab:  true,
		VGrab:  true,
	})
	s.panel = NewPanel()
	s.panel.SetBorder(s.Border)
	s.panel.SetLayout(&FlexLayout{Columns: 1})
	s.panel.DrawCallback = func(canvas *Canvas, _ geom.Rect) {
		r := s.panel.ContentRect(true)
		canvas.DrawRect(r, s.BackgroundInk.Paint(canvas, r, paintstyle.Fill))
	}
	s.panel.Accessibility.LabeledBy = field
	s.panel.AddChild(s.scroller)
	if field.Accessibility.Role == role.Auto {
		field.Accessibility.Role = role.ComboBox
	}
	field.suggestions = s
	return s
}

// List returns the list that shows the rows.
func (s *FieldSuggestions[T]) List() *List[T] {
	return s.list
}

// SetRows replaces the rows, with none highlighted, and shows them. No rows hides them. Rows set from within
// ChosenCallback replace the rows without showing them.
func (s *FieldSuggestions[T]) SetRows(rows []T) {
	s.list.Clear()
	s.list.Append(rows...)
	if len(rows) == 0 {
		s.Hide()
		return
	}
	if !s.inChosen {
		s.Show()
	}
}

// Show shows the rows, if there are any, the field holds the keyboard focus, is enabled and can be seen, and no menu is
// open. Rows already showing are placed again.
func (s *FieldSuggestions[T]) Show() {
	// Not while a menu is open, as for a tooltip: the window would sit above the menu's and take the keys from it.
	if s.inChosen || s.list.Count() == 0 || !s.field.Focused() || !s.fieldShowable() || menusOpen() {
		return
	}
	owner := s.field.Window()
	if !owner.IsValid() || owner.focusLeaving == s.field.AsPanel() {
		// A field whose LostFocusCallback is running still holds the focus, but rows shown from it, as by a callback
		// that reformats the text before chaining to Field.DefaultFocusLost, would be hidden again at once.
		return
	}
	if s.wnd != nil && s.wnd.owner != owner {
		// The field has moved to another window.
		s.hide()
	}
	if other := owner.suggestions; other != nil && other != s {
		other.hide()
	}
	if s.wnd == nil {
		wnd, err := newPopupWindow(owner, "", WindowKindDropdown)
		if err != nil {
			errs.Log(err)
			return
		}
		wnd.WillCloseCallback = func() {
			if s.wnd == wnd {
				s.wnd = nil
				s.visible = false
			}
			if owner.suggestions == s {
				owner.suggestions = nil
				owner.axMarkForPublish()
				owner.apiSuggestionsChanged(false)
			}
		}
		// Not Window.SetContent, which would lay the panel out at the window's initial size.
		wnd.root.setContent(s.panel)
		s.wnd = wnd
	}
	if !s.visible {
		// A tooltip would sit over the rows.
		owner.ClearTooltip()
	}
	// Laid out first, so that the field is where the next draw will find it; see anchorLost.
	owner.root.ValidateLayout()
	s.anchor = s.field.RectToRoot(s.field.ContentRect(true))
	s.panel.SetBorder(s.Border)
	wnd := s.wnd
	wnd.SetContentRect(s.windowRect(owner))
	wnd.ValidateLayout()
	if i := s.Selected(); i >= 0 {
		s.list.ScrollRectIntoView(s.list.RowRect(i))
	} else {
		s.scroller.SetPosition(0, 0)
	}
	// Drawn before it is shown, as showTooltip does, so that the previous rows are never put on the screen.
	wnd.MarkForRedraw()
	wnd.FlushDrawing()
	wnd.Show()
	s.hoverHeldAt = wnd.MouseLocation()
	s.hoverHeld = true
	s.visible = true
	owner.suggestions = s
	owner.axMarkForPublish()
	owner.apiSuggestionsChanged(true)
}

// Hide hides the rows, keeping them.
func (s *FieldSuggestions[T]) Hide() {
	s.hide()
}

// Showing returns true while the rows are showing.
func (s *FieldSuggestions[T]) Showing() bool {
	return s.visible
}

// Select highlights the row at index, scrolling it into view when the rows are showing. An index out of range removes
// the highlight.
func (s *FieldSuggestions[T]) Select(index int) {
	if index < 0 || index >= s.list.Count() {
		s.list.Select(false)
		return
	}
	s.highlight(index, true)
}

// Selected returns the index of the highlighted row, or -1 if there is none.
func (s *FieldSuggestions[T]) Selected() int {
	if i := s.list.Selection.FirstSet(); i >= 0 && i < s.list.Count() {
		return i
	}
	return -1
}

// highlight highlights the row at index, which must be in range, and scrolls it into view when asked to and the rows
// are showing.
func (s *FieldSuggestions[T]) highlight(index int, scroll bool) {
	s.list.Select(false, index)
	if scroll && s.visible {
		s.list.ScrollRectIntoView(s.list.RowRect(index))
	}
}

// hoverIsHeld returns true if the pointer has not moved since the rows were shown, releasing the hold once it has. The
// crossing a platform reports when the popup window appears under a resting pointer, as X11 and Windows do, must not
// highlight the row there, since nothing is highlighted until the pointer moves; see rootPanel.holdMenuHover.
func (s *FieldSuggestions[T]) hoverIsHeld() bool {
	if s.hoverHeld && (s.wnd == nil || s.wnd.MouseLocation() != s.hoverHeldAt) {
		s.hoverHeld = false
	}
	return s.hoverHeld
}

// highlightAt highlights the row at where, in the list's coordinates, if there is one.
func (s *FieldSuggestions[T]) highlightAt(where geom.Point) {
	if i := s.rowUnder(where); i >= 0 && i != s.Selected() {
		s.highlight(i, false)
	}
}

// rowUnder returns the row at where, in the list's coordinates, or -1 if where is outside the part of the list that can
// be seen.
func (s *FieldSuggestions[T]) rowUnder(where geom.Point) int {
	view := s.scroller.ContentView()
	if !s.list.PointTo(where, view).In(view.ContentRect(false)) {
		return -1
	}
	row, _ := s.list.rowAt(where.Y)
	return row
}

// choose hides the rows and reports the one at index as chosen. The field's undo id is moved on first, so that what
// the callback does to the field is an edit of its own.
func (s *FieldSuggestions[T]) choose(index int) {
	if index < 0 || index >= s.list.Count() {
		return
	}
	row := s.list.DataAtIndex(index)
	s.Hide()
	s.field.undoID = NextUndoID()
	if s.ChosenCallback != nil {
		s.inChosen = true
		SafeCall(func() { s.ChosenCallback(index, row) })
		s.inChosen = false
	}
}

// visibleRows returns how many whole rows fit in the view, at least one. Rows of their own heights are counted from
// the first row wholly within the view, as scrolled, since a page elsewhere in the list may hold a different number.
func (s *FieldSuggestions[T]) visibleRows() int {
	view := s.scroller.ContentView()
	viewRect := view.ContentRect(false)
	count := s.list.Count()
	if cellHeight := xmath.Ceil(s.list.Factory.CellHeight()); cellHeight >= 1 {
		return max(min(int(viewRect.Height/cellHeight), count), 1)
	}
	viewTop := view.PointTo(viewRect.Point, s.list.AsPanel()).Y
	row, top := s.list.rowAt(viewTop)
	if row < 0 {
		return 1
	}
	total := top - viewTop
	if total < 0 {
		// The row at the top is cut off, so the count starts at the one below it.
		_, pref, _ := s.list.cell(row).Sizes(geom.Size{})
		total += pref.Ceil().Height
		row++
	}
	n := 0
	for ; row+n < count; n++ {
		_, pref, _ := s.list.cell(row + n).Sizes(geom.Size{})
		if total += pref.Ceil().Height; total > viewRect.Height {
			break
		}
	}
	return max(n, 1)
}

// windowRect returns the rect of the popup window in the space window rects use: at most MaxVisibleRows rows tall, as
// wide as the field or as the rows and the vertical scroll bar beside them need, whichever is more, below the part of
// the field visible within the owner, as a tooltip is placed, or above it when there is no room below, and fitted onto
// the display.
func (s *FieldSuggestions[T]) windowRect(owner *Window) geom.Rect {
	count := s.list.Count()
	n := min(count, max(s.MaxVisibleRows, 1))
	_, panelPref, _ := s.panel.Sizes(geom.Size{})
	_, listPref, _ := s.list.Sizes(geom.Size{})
	avoid := s.anchor
	if visible := avoid.Intersect(owner.root.ContentRect(true)); !visible.Empty() {
		avoid = visible
	}
	var where geom.Rect
	// The panel's border and the scroll panel wrap the first n rows just the same.
	where.Height = panelPref.Height - listPref.Height
	if n > 0 {
		where.Height += s.list.RowRect(n-1).Bottom() - s.list.RowRect(0).Y
	}
	// The vertical scroll bar is drawn over the rows rather than beside them, so the rows need its width on top of
	// their own only when they scroll, and a field wider than both needs nothing added.
	bar := s.scroller.Bar(false).MinimumThickness
	rowsWidth := panelPref.Width
	if count > n {
		rowsWidth += bar
	}
	where.Width = max(rowsWidth, avoid.Width)
	where.Point = owner.apiPointToScreen(geom.NewPoint(avoid.X, avoid.Bottom()))
	avoid.Point = owner.apiPointToScreen(avoid.Point)
	d := tooltipDisplayAmong(AllDisplays(), where, avoid)
	if d == nil {
		d = owner.Display()
	}
	fitted := suggestionsWindowRectOn(where, avoid, d)
	if count <= n && fitted.Height < where.Height {
		// Shortened to the room on the display, so the rows scroll after all.
		where.Width = max(rowsWidth+bar, avoid.Width)
		fitted = suggestionsWindowRectOn(where, avoid, d)
	}
	return fitted
}

// suggestionsWindowRectOn returns where, the rect of the window showing the rows below avoid, fitted onto d as
// fitSuggestionsRect does, or where as it is, but at least one unit each way, when d is nil, which it is on macOS while
// every display is asleep. The rects are in the space window rects use; the fitting is done in the display's own space.
func suggestionsWindowRectOn(where, avoid geom.Rect, d *Display) geom.Rect {
	if d == nil {
		where.Width = max(where.Width, 1)
		where.Height = max(where.Height, 1)
		return where
	}
	return d.apiRectToWindowUnits(fitSuggestionsRect(d.apiRectFromWindowUnits(where), d.apiRectFromWindowUnits(avoid),
		d.Usable))
}

// fitSuggestionsRect returns where, rows placed below avoid, moved so that it lies within usable: at least one unit
// each way, narrowed to usable's width and shifted onto it sideways, then directly below avoid when it fits there, else
// directly above it when it fits there, else on the side with more room, shortened to that room. The rects are all in
// the same space.
func fitSuggestionsRect(where, avoid, usable geom.Rect) geom.Rect {
	where.Width = max(where.Width, 1)
	where.Height = max(where.Height, 1)
	where.Width = min(where.Width, usable.Width)
	if where.Right() > usable.Right() {
		where.X = usable.Right() - where.Width
	}
	if where.X < usable.X {
		where.X = usable.X
	}
	below := usable.Bottom() - avoid.Bottom()
	above := avoid.Y - usable.Y
	switch {
	case where.Height <= below:
		where.Y = avoid.Bottom()
	case where.Height <= above:
		where.Y = avoid.Y - where.Height
	case below >= above:
		where.Y = avoid.Bottom()
		where.Height = max(below, 1)
	default:
		where.Height = max(above, 1)
		where.Y = avoid.Y - where.Height
	}
	return where
}

// showing implements fieldSuggestionsHost.
func (s *FieldSuggestions[T]) showing() bool {
	return s.visible
}

// hide implements fieldSuggestionsHost. Whether the rows are showing is decided by the toolkit's own state rather than
// by the window's visibility, since Windows hides an owned window along with its minimized owner before the owner
// learns of the minimize. The window is disposed of rather than kept hidden, as a menu's is: nothing would know to
// dispose of it when the field is discarded, a press in progress in it would be left in a hidden window, and Windows
// would show it again, as an owned window hidden with its minimized owner, when the owner is restored.
func (s *FieldSuggestions[T]) hide() {
	if !s.visible {
		return
	}
	s.visible = false
	wnd := s.wnd
	s.wnd = nil
	if owner := wnd.owner; owner != nil {
		if owner.suggestions == s {
			owner.suggestions = nil
		}
		// The rows are described as part of the owner, which does not repaint for their going.
		owner.axMarkForPublish()
		owner.apiSuggestionsChanged(false)
	}
	wnd.Dispose()
}

// keyDown implements fieldSuggestionsHost.
func (s *FieldSuggestions[T]) keyDown(keyCode KeyCode, mods mod.Modifiers) bool {
	count := s.list.Count()
	sel := s.Selected()
	var target int
	switch keyCode {
	case KeyDown, KeyUp, KeyPageDown, KeyPageUp:
		if mods&mod.NonSticky != 0 || count == 0 {
			return false
		}
		// With nothing highlighted, the rows are entered from above going down and from below going up.
		step := 1
		if keyCode == KeyPageDown || keyCode == KeyPageUp {
			step = s.visibleRows()
		}
		if keyCode == KeyDown || keyCode == KeyPageDown {
			target = min(sel+step, count-1)
			if sel < 0 {
				target = min(step-1, count-1)
			}
		} else {
			target = max(sel-step, 0)
			if sel < 0 {
				target = max(count-step, 0)
			}
		}
	case KeyReturn, KeyNumPadEnter:
		if sel >= 0 {
			s.choose(sel)
			return true
		}
		s.Hide()
		return false
	case KeyEscape:
		s.Hide()
		return true
	default:
		return false
	}
	s.highlight(target, true)
	return true
}

// anchorContains implements fieldSuggestionsHost.
func (s *FieldSuggestions[T]) anchorContains(where geom.Point) bool {
	return where.In(s.field.RectToRoot(s.field.ContentRect(true)))
}

// anchorLost implements fieldSuggestionsHost.
func (s *FieldSuggestions[T]) anchorLost() bool {
	if s.wnd == nil || s.field.Window() != s.wnd.owner || !s.fieldShowable() {
		return true
	}
	return s.field.RectToRoot(s.field.ContentRect(true)) != s.anchor
}

// fieldShowable returns true if the field is enabled and neither it nor any ancestor is hidden. A disabled field gets
// no keys to drive the rows with, and a hidden one has nothing on the screen for them to belong to.
func (s *FieldSuggestions[T]) fieldShowable() bool {
	if !s.field.Enabled() {
		return false
	}
	for p := s.field.Parent(); p != nil; p = p.Parent() {
		if p.Hidden {
			return false
		}
	}
	return true
}

// popup implements fieldSuggestionsHost.
func (s *FieldSuggestions[T]) popup() (window *Window, content, list *Panel) {
	if s.wnd == nil {
		return nil, nil, nil
	}
	return s.wnd, s.panel, s.list.AsPanel()
}

// describe implements fieldSuggestionsHost. The highlighted row is reported as the list's selected row rather than as
// the focus, since the focus stays in the field and there is no active-descendant concept to report it with.
func (s *FieldSuggestions[T]) describe(node *accessibility.Node) {
	node.Expandable = true
	node.Expanded = s.visible
	if s.visible {
		node.Actions = node.Actions.With(accessibility.Collapse)
		node.Controls = append(node.Controls, axIDFor(s.list.AsPanel()))
	}
}
