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
	"slices"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/mod"
)

// suggestionsTestRows are rows for the tests that need some.
var suggestionsTestRows = []string{"Oak", "Elm"}

// startSuggestionsTest starts a 400x300 session holding an owner window, shown and focused at (20, 20, 200, 150), whose
// content holds a field with the focus and suggestions.
func startSuggestionsTest(t *testing.T) (
	screen *HeadlessScreen, owner *Window, field *Field, s *FieldSuggestions[string],
) {
	t.Helper()
	screen = startHeadlessTest(t, HeadlessConfig{Width: 400, Height: 300},
		StartupFinishedCallback(func() {
			if owner = newHeadlessTestWindow(t, ownerTitle, geom.NewRect(20, 20, 200, 150)); owner == nil {
				return
			}
			field = NewField()
			s = NewFieldSuggestions[string](field)
			owner.Content().SetLayout(&FlexLayout{Columns: 1})
			owner.Content().AddChild(field)
			owner.ToFront()
			owner.SetFocus(field)
		}))
	if owner == nil {
		t.FailNow()
	}
	return screen, owner, field, s
}

// varyingCellFactory makes cells 50 wide and 10 plus their row tall.
type varyingCellFactory struct{}

func (varyingCellFactory) CellHeight() float32 {
	return 0
}

func (varyingCellFactory) CreateCell(_ Paneler, _ any, row int, _, _ Ink, _, _ bool) Paneler {
	p := NewPanel()
	size := geom.NewSize(50, float32(10+row))
	p.SetSizer(func(_ geom.Size) (minSize, prefSize, maxSize geom.Size) { return size, size, size })
	return p
}

// fakeSuggestionsHost records the keys it is handed, and runs onHide, if set, when hidden while showing.
type fakeSuggestionsHost struct {
	onHide func()
	keys   []KeyCode
	shown  bool
}

func (h *fakeSuggestionsHost) showing() bool { return h.shown }

func (h *fakeSuggestionsHost) hide() {
	if h.shown {
		h.shown = false
		if h.onHide != nil {
			h.onHide()
		}
	}
}

func (h *fakeSuggestionsHost) keyDown(keyCode KeyCode, _ mod.Modifiers) bool {
	h.keys = append(h.keys, keyCode)
	return true
}

func (h *fakeSuggestionsHost) anchorContains(_ geom.Point) bool { return false }

func (h *fakeSuggestionsHost) anchorLost() bool { return false }

func (h *fakeSuggestionsHost) popup() (window *Window, content, list *Panel) { return nil, nil, nil }

func (h *fakeSuggestionsHost) describe(_ *accessibility.Node) {}

// TestFitSuggestionsRect verifies that rows that fit below the field stay there, that they are shifted onto the usable
// area sideways and narrowed to it, placed directly above the field when only there is room, and otherwise shortened to
// the larger side, below on a tie, and never less than one unit each way.
func TestFitSuggestionsRect(t *testing.T) {
	c := check.New(t)
	usable := geom.NewRect(0, 0, 400, 300)
	avoid := geom.NewRect(100, 100, 50, 20)
	c.Equal(geom.NewRect(100, 120, 80, 60), fitSuggestionsRect(geom.NewRect(100, 120, 80, 60), avoid, usable),
		"rows that fit below are placed as asked")
	c.Equal(geom.NewRect(320, 120, 80, 60), fitSuggestionsRect(geom.NewRect(380, 120, 80, 60), avoid, usable),
		"rows past the right edge are shifted left")
	c.Equal(geom.NewRect(50, 120, 80, 60),
		fitSuggestionsRect(geom.NewRect(20, 120, 80, 60), avoid, geom.NewRect(50, 0, 350, 300)),
		"rows past the left edge are shifted right")
	c.Equal(geom.NewRect(0, 120, 400, 60), fitSuggestionsRect(geom.NewRect(100, 120, 500, 60), avoid, usable),
		"rows wider than the usable area are narrowed to it")
	avoid = geom.NewRect(100, 250, 50, 20)
	c.Equal(geom.NewRect(100, 190, 80, 60), fitSuggestionsRect(geom.NewRect(100, 270, 80, 60), avoid, usable),
		"rows with no room below are placed directly above")
	usable = geom.NewRect(0, 0, 400, 100)
	c.Equal(geom.NewRect(100, 50, 80, 50),
		fitSuggestionsRect(geom.NewRect(100, 50, 80, 120), geom.NewRect(100, 30, 50, 20), usable),
		"rows with room on neither side are shortened below when it has more room")
	c.Equal(geom.NewRect(100, 60, 80, 40),
		fitSuggestionsRect(geom.NewRect(100, 60, 80, 120), geom.NewRect(100, 40, 50, 20), usable),
		"or the same room")
	c.Equal(geom.NewRect(100, 0, 80, 60),
		fitSuggestionsRect(geom.NewRect(100, 80, 80, 120), geom.NewRect(100, 60, 50, 20), usable),
		"and above when it has more")
	c.Equal(geom.NewRect(100, 120, 1, 1),
		fitSuggestionsRect(geom.NewRect(100, 120, 0, 0), geom.NewRect(100, 100, 50, 20), geom.NewRect(0, 0, 400, 300)),
		"at least one unit each way")
	c.Equal(geom.NewRect(100, 20, 80, 1),
		fitSuggestionsRect(geom.NewRect(100, 20, 80, 60), geom.NewRect(100, 0, 50, 20), geom.NewRect(0, 0, 400, 20)),
		"even with no room on either side")
}

// TestSuggestionsWindowRectOn verifies that the rows are left where they were asked to go when there is no display, as
// on macOS while every display is asleep, but are never empty, that they are fitted onto a display, and that only a
// fitting that shortened them says so, which is decided in the display's own space, so that the trip through it and
// back cannot report a height it moved by a float32 ulp as shortened.
func TestSuggestionsWindowRectOn(t *testing.T) {
	c := check.New(t)
	avoid := geom.NewRect(500, 250, 40, 20)
	rect, shortened := suggestionsWindowRectOn(geom.NewRect(500, 270, 100, 50), avoid, nil)
	c.Equal(geom.NewRect(500, 270, 100, 50), rect)
	c.False(shortened)
	rect, shortened = suggestionsWindowRectOn(geom.NewRect(500, 270, 0, 0), avoid, nil)
	c.Equal(geom.NewRect(500, 270, 1, 1), rect)
	c.False(shortened)
	d := &Display{Usable: geom.NewRect(0, 0, 400, 300), headless: true}
	rect, shortened = suggestionsWindowRectOn(geom.NewRect(500, 270, 100, 50), avoid, d)
	c.Equal(geom.NewRect(300, 200, 100, 50), rect)
	c.False(shortened, "moved above, not shortened")
	avoid = geom.NewRect(100, 100, 40, 20)
	rect, shortened = suggestionsWindowRectOn(geom.NewRect(100, 120, 100, 250), avoid, d)
	c.Equal(geom.NewRect(100, 120, 100, 180), rect)
	c.True(shortened, "shortened to the room below")
}

// TestFieldSuggestionsWindowRect verifies the size of the popup window: MaxVisibleRows rows at most plus the border, at
// least as wide as the field, and wider than the rows by the vertical scroll bar only when there are rows to scroll to
// and the field is not already wide enough for both, for rows of one height and of their own heights.
func TestFieldSuggestionsWindowRect(t *testing.T) {
	c := check.New(t)
	screen, owner, _, s := startSuggestionsTest(t)
	rows := func(n int) []string {
		list := make([]string, n)
		for i := range list {
			list[i] = "row"
		}
		return list
	}
	bar := DefaultScrollBarTheme.MinimumThickness
	var few, many, wide, nearlyWide, varying, varyingFew geom.Rect
	var prefWidth float32
	c.True(screen.Do(func() {
		// Show records the field's rect as the anchor, so a narrower one is set after each SetRows.
		s.List().Factory = &DefaultCellFactory{Height: 20}
		s.SetRows(rows(5))
		s.anchor = geom.NewRect(0, 0, 10, 20)
		_, pref, _ := s.panel.Sizes(geom.Size{})
		prefWidth = pref.Width
		few = s.windowRect(owner)
		s.SetRows(rows(12))
		s.anchor = geom.NewRect(0, 0, 10, 20)
		many = s.windowRect(owner)
		s.anchor = geom.NewRect(0, 0, 150, 20)
		wide = s.windowRect(owner)
		s.anchor = geom.NewRect(0, 0, prefWidth+bar-1, 20)
		nearlyWide = s.windowRect(owner)
		s.List().Factory = varyingCellFactory{}
		s.MaxVisibleRows = 3
		s.SetRows(rows(5))
		s.anchor = geom.NewRect(0, 0, 10, 20)
		varying = s.windowRect(owner)
		s.SetRows(rows(3))
		s.anchor = geom.NewRect(0, 0, 10, 20)
		varyingFew = s.windowRect(owner)
	}))
	c.Equal(float32(5*20+2), few.Height, "every row and the border")
	c.Equal(prefWidth, few.Width, "as wide as the rows want")
	c.Equal(float32(8*20+2), many.Height, "MaxVisibleRows rows")
	c.Equal(prefWidth+bar, many.Width, "with room for the scroll bar")
	c.Equal(float32(150), wide.Width, "as wide as the field, which has room for the rows and the scroll bar")
	c.Equal(prefWidth+bar, nearlyWide.Width, "but no narrower than the rows and the scroll bar")
	c.Equal(float32(10+11+12+2), varying.Height, "the first rows' own heights")
	c.Equal(float32(50+2)+bar, varying.Width)
	c.Equal(float32(10+11+12+2), varyingFew.Height)
	c.Equal(float32(50+2), varyingFew.Width, "no scroll bar with nothing to scroll to")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestFieldSuggestionsVisibleRows verifies that a page is the rows that fit in the view, for rows of one height and of
// their own heights, counted from the row at the top of the view once scrolled, and never less than one.
func TestFieldSuggestionsVisibleRows(t *testing.T) {
	c := check.New(t)
	screen, _, _, s := startSuggestionsTest(t)
	var fixed, varying, scrolled, before int
	c.True(screen.Do(func() {
		before = s.visibleRows()
		s.List().Factory = &DefaultCellFactory{Height: 20}
		s.SetRows(make([]string, 12))
		fixed = s.visibleRows()
		s.List().Factory = varyingCellFactory{}
		s.MaxVisibleRows = 3
		s.SetRows(make([]string, 25))
		varying = s.visibleRows()
		s.Select(20)
		scrolled = s.visibleRows()
	}))
	c.Equal(1, before, "at least one, even before anything is shown")
	c.Equal(8, fixed)
	c.Equal(3, varying)
	c.Equal(1, scrolled, "only one of the taller rows at the top of the scrolled view fits")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestFieldSuggestionsPopupWindow verifies that the rows are shown in a popup window of the dropdown kind belonging to
// the field's window, rendered on the CPU and never taking the focus, which is not a tooltip window.
func TestFieldSuggestionsPopupWindow(t *testing.T) {
	c := check.New(t)
	screen, owner, _, s := startSuggestionsTest(t)
	var wnd *Window
	var kind WindowKind
	var showing, transient, cpuOnly, owned, popup, tooltip bool
	c.True(screen.Do(func() {
		s.SetRows(suggestionsTestRows)
		wnd = s.wnd
		showing = owner.suggestions == fieldSuggestionsHost(s)
		if wnd != nil {
			kind = wnd.kind
			transient = wnd.transient
			cpuOnly = wnd.cpuOnly
			owned = wnd.owner == owner
			popup = wnd.isPopup()
			tooltip = wnd.isTooltipWindow()
		}
	}))
	c.NotNil(wnd)
	c.True(showing)
	c.Equal(WindowKindDropdown, kind)
	c.True(transient)
	c.True(cpuOnly)
	c.True(owned)
	c.True(popup)
	c.False(tooltip)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestFieldSuggestionsThemeInks verifies that the rows draw their highlight with the active selection inks, since the
// list never holds the focus.
func TestFieldSuggestionsThemeInks(t *testing.T) {
	c := check.New(t)
	rows := DefaultFieldSuggestionsTheme.Rows
	c.True(rows.InactiveSelectionInk == rows.SelectionInk)
	c.True(rows.OnInactiveSelectionInk == rows.OnSelectionInk)
}

// TestFieldSuggestionsHideDisposesOfTheWindow verifies that hiding the rows disposes of their popup window, as closing
// a menu does, so that a FieldSuggestions that is replaced or discarded leaves no hidden window behind, and that the
// next rows are shown in a new one.
func TestFieldSuggestionsHideDisposesOfTheWindow(t *testing.T) {
	c := check.New(t)
	screen, owner, field, s := startSuggestionsTest(t)
	var first, second, third, afterHide *Window
	var firstValid, firstListed, secondValid, secondListed, thirdShowing bool
	c.True(screen.Do(func() {
		s.SetRows(suggestionsTestRows)
		first = s.wnd
		s.Hide()
		firstValid = first.IsValid()
		firstListed = slices.Contains(windowList, first)
		afterHide = s.wnd
		s.SetRows(suggestionsTestRows)
		second = s.wnd
		replacement := NewFieldSuggestions[string](field)
		secondValid = second.IsValid()
		secondListed = slices.Contains(windowList, second)
		replacement.SetRows(suggestionsTestRows)
		third = replacement.wnd
		thirdShowing = owner.suggestions == fieldSuggestionsHost(replacement)
	}))
	c.NotNil(first)
	c.False(firstValid, "hiding disposes of the window")
	c.False(firstListed)
	c.Nil(afterHide)
	c.NotNil(second)
	c.True(second != first, "the next rows get a new window")
	c.False(secondValid, "replacing the suggestions disposes of their window")
	c.False(secondListed)
	c.NotNil(third)
	c.True(third != second)
	c.True(thirdShowing)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestFieldSuggestionsHoverHeldUntilThePointerMoves verifies that the crossing a platform reports when the rows appear
// under a resting pointer, as X11 and Windows do, highlights nothing, and that the pointer then moving highlights the
// row it moves over.
func TestFieldSuggestionsHoverHeldUntilThePointerMoves(t *testing.T) {
	c := check.New(t)
	screen, owner, field, s := startSuggestionsTest(t)
	var resting geom.Point
	c.True(screen.Do(func() {
		r := field.RectToRoot(field.ContentRect(true))
		resting = owner.ContentRect().Point.Add(geom.NewPoint(r.X+10, r.Bottom()+5))
	}))
	screen.MouseMove(resting, mod.None)
	var entered, moved int
	c.True(screen.Do(func() {
		s.SetRows(suggestionsTestRows)
		// The crossing the platform reports as the window maps under the pointer.
		s.list.MouseEnterCallback(s.list.PointFromRoot(s.wnd.MouseLocation()), mod.None)
		entered = s.Selected()
	}))
	c.Equal(-1, entered, "the row under the resting pointer is not highlighted")
	screen.MouseMove(resting.Add(geom.NewPoint(0, 3)), mod.None)
	c.True(screen.Do(func() { moved = s.Selected() }))
	c.Equal(0, moved, "moving the pointer highlights the row under it")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestFieldSuggestionsNotShownWhileLosingTheFocus verifies that rows set from the field's LostFocusCallback, as one
// that reformats the text before chaining to Field.DefaultFocusLost sets them, are not shown, since the field is on its
// way to losing the focus and the rows would be hidden again at once.
func TestFieldSuggestionsNotShownWhileLosingTheFocus(t *testing.T) {
	c := check.New(t)
	screen, owner, field, s := startSuggestionsTest(t)
	var other *Field
	var shown []bool
	c.True(screen.Do(func() {
		other = NewField()
		owner.Content().AddChild(other)
		field.LostFocusCallback = func() {
			field.SetText(field.Text() + "!")
			field.DefaultFocusLost()
		}
		field.ModifiedCallback = func(_, _ *FieldState) {
			s.SetRows(suggestionsTestRows)
			shown = append(shown, s.Showing())
		}
		owner.SetFocus(other)
		owner.SetFocus(field)
		owner.SetFocus(nil)
	}))
	c.Equal([]bool{false, false}, shown, "never shown while the field is losing the focus")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestFieldSuggestionsWindowRectShortened verifies that rows shortened to the room on the display, so that they scroll
// after all, are widened for the vertical scroll bar just as more rows than MaxVisibleRows are.
func TestFieldSuggestionsWindowRectShortened(t *testing.T) {
	c := check.New(t)
	var owner *Window
	var s *FieldSuggestions[string]
	screen := startHeadlessTest(t, HeadlessConfig{Width: 400, Height: 130},
		StartupFinishedCallback(func() {
			if owner = newHeadlessTestWindow(t, ownerTitle, geom.NewRect(20, 20, 200, 100)); owner == nil {
				return
			}
			field := NewField()
			s = NewFieldSuggestions[string](field)
			owner.Content().SetLayout(&FlexLayout{Columns: 1})
			owner.Content().AddChild(field)
			owner.ToFront()
			owner.SetFocus(field)
		}))
	if owner == nil {
		t.FailNow()
	}
	bar := DefaultScrollBarTheme.MinimumThickness
	var rect geom.Rect
	var prefWidth float32
	c.True(screen.Do(func() {
		s.List().Factory = &DefaultCellFactory{Height: 20}
		s.SetRows(make([]string, 5))
		_, pref, _ := s.panel.Sizes(geom.Size{})
		prefWidth = pref.Width
		// Room for neither 102 units below nor above: 30 below, 80 above.
		s.anchor = geom.NewRect(0, 60, 10, 20)
		rect = s.windowRect(owner)
	}))
	c.Equal(float32(80), rect.Height, "shortened to the room above")
	c.Equal(prefWidth+bar, rect.Width, "with room for the scroll bar")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestFieldSuggestionsReplacedShowNothing verifies that suggestions a field has since been given others in place of
// never show again, so that a callback still holding them cannot put a popup the field no longer drives under it.
func TestFieldSuggestionsReplacedShowNothing(t *testing.T) {
	c := check.New(t)
	screen, owner, field, s := startSuggestionsTest(t)
	var showing bool
	var host fieldSuggestionsHost
	var wnd *Window
	c.True(screen.Do(func() {
		s.SetRows(suggestionsTestRows)
		NewFieldSuggestions[string](field)
		s.SetRows(suggestionsTestRows)
		s.Show()
		showing = s.Showing()
		host = owner.suggestions
		wnd = s.wnd
	}))
	c.False(showing, "replaced suggestions never show")
	c.Nil(host)
	c.Nil(wnd)
	c.Nil(screen.SuggestionsWindow(owner))
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestFieldSuggestionsEmptiedThroughTheListHide verifies that rows emptied through the list hide: at once when they are
// asked to show, and at the owner's next draw, which the emptying itself asks for, when the last row is removed or the
// list is cleared. The field's text is selected, so that no caret blink draws the owner on its own.
func TestFieldSuggestionsEmptiedThroughTheListHide(t *testing.T) {
	c := check.New(t)
	screen, owner, field, s := startSuggestionsTest(t)
	var showing bool
	c.True(screen.Do(func() {
		field.SetText("abc")
		field.SelectAll()
		s.SetRows(suggestionsTestRows)
		s.List().Clear()
		s.Show()
		showing = s.Showing()
	}))
	c.False(showing, "showing no rows hides them")
	c.Nil(screen.SuggestionsWindow(owner))
	c.True(screen.Do(func() {
		s.SetRows(suggestionsTestRows)
		for s.List().Count() > 0 {
			s.List().Remove(0)
		}
	}))
	c.True(screen.Do(func() { showing = s.Showing() }))
	c.False(showing, "removing the last row hides them at the owner's next draw")
	c.Nil(screen.SuggestionsWindow(owner))
	c.True(screen.Do(func() {
		s.SetRows(suggestionsTestRows)
		s.List().Clear()
	}))
	c.True(screen.Do(func() { showing = s.Showing() }))
	c.False(showing, "clearing the list hides them at the owner's next draw")
	c.Nil(screen.SuggestionsWindow(owner))
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestFieldSuggestionsShowFromChosenCallback verifies that a ChosenCallback that sets rows and asks for them to be
// shown, as one opening the next level of a path would, shows them, while rows it merely sets stay hidden.
func TestFieldSuggestionsShowFromChosenCallback(t *testing.T) {
	c := check.New(t)
	screen, owner, _, s := startSuggestionsTest(t)
	var showing bool
	var count int
	c.True(screen.Do(func() {
		s.ChosenCallback = func(_ int, row string) {
			s.SetRows([]string{row + "/Leaf", row + "/Bark"})
			s.Show()
		}
		s.SetRows(suggestionsTestRows)
		s.Select(0)
		s.keyDown(KeyReturn, 0)
		showing = s.Showing()
		count = s.List().Count()
	}))
	c.True(showing, "shown as the callback asked")
	c.Equal(2, count)
	c.NotNil(screen.SuggestionsWindow(owner))
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
