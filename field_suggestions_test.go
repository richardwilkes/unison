// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/behavior"
	"github.com/richardwilkes/unison/enums/mod"
)

// The words the field suggests that the tests below choose, named once so that they are not repeated through them.
const (
	suggestAlmond = "Almond"
	suggestAloe   = "Aloe"
)

// suggestionWords are what the fixture's field suggests from.
var suggestionWords = []string{
	suggestAlmond, suggestAloe, "Apple", "Apricot", "Ash", "Aspen", "Beech", "Birch", "Cedar",
}

// matchingWords returns the suggestionWords that start with text, ignoring case, or nil for no text.
func matchingWords(text string) []string {
	if text == "" {
		return nil
	}
	var words []string
	for _, word := range suggestionWords {
		if strings.HasPrefix(strings.ToLower(word), strings.ToLower(text)) {
			words = append(words, word)
		}
	}
	return words
}

// suggestionsFixture is a window holding a field with suggestions, a button after it and a plain panel to press
// outside the field. Its fields other than screen belong to the UI thread.
type suggestionsFixture struct {
	screen  *unison.HeadlessScreen
	wnd     *unison.Window
	column  *unison.Panel
	field   *unison.Field
	button  *unison.Button
	outside *unison.Panel
	sg      *unison.FieldSuggestions[string]
	chosen  []string
}

// startSuggestions starts a 400x300 session holding the fixture's window at rect, focused, with the focus in the field,
// which suggests the matchingWords of its text. Choosing a row records it and puts it in the field. With scrolled, the
// column sits in a ScrollPanel and the outside panel is tall enough to scroll.
func startSuggestions(t *testing.T, rect geom.Rect, scrolled bool) *suggestionsFixture {
	t.Helper()
	f := &suggestionsFixture{}
	f.screen = startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			f.field = unison.NewField()
			f.field.SetLayoutData(&unison.FlexLayoutData{HAlign: align.Fill, HGrab: true})
			f.sg = unison.NewFieldSuggestions[string](f.field)
			f.field.ModifiedCallback = func(_, after *unison.FieldState) { f.sg.SetRows(matchingWords(after.Text)) }
			f.sg.ChosenCallback = func(_ int, row string) {
				f.chosen = append(f.chosen, row)
				f.field.SetText(row)
			}
			f.button = unison.NewButton()
			f.button.SetTitle("Go")
			f.outside = unison.NewPanel()
			height := float32(40)
			if scrolled {
				height = 600
			}
			f.outside.SetLayoutData(&unison.FlexLayoutData{
				SizeHint: geom.NewSize(50, height),
				HAlign:   align.Fill,
				HGrab:    true,
			})
			f.column = axColumn(f.field, f.button, f.outside)
			var content unison.Paneler = f.column
			if scrolled {
				scroller := unison.NewScrollPanel()
				scroller.SetContent(f.column, behavior.Fill, behavior.Unmodified)
				content = scroller
			}
			if f.wnd = newHeadlessWindow(t, "suggestions", rect, content); f.wnd != nil {
				f.wnd.ToFront()
				f.wnd.SetFocus(f.field)
			}
		}))
	if f.wnd == nil {
		t.FailNow()
	}
	return f
}

// selected returns the highlighted row.
func (f *suggestionsFixture) selected() int {
	index := -2
	f.screen.Do(func() { index = f.sg.Selected() })
	return index
}

// showing returns whether the rows are showing.
func (f *suggestionsFixture) showing() bool {
	var showing bool
	f.screen.Do(func() { showing = f.sg.Showing() })
	return showing
}

// chosenRows returns the rows chosen so far.
func (f *suggestionsFixture) chosenRows() []string {
	var rows []string
	f.screen.Do(func() { rows = slices.Clone(f.chosen) })
	return rows
}

// rowCenter returns the center of a row of the list, in screen coordinates.
func (f *suggestionsFixture) rowCenter(row int) geom.Point {
	var list *unison.List[string]
	var offset geom.Point
	f.screen.Do(func() {
		list = f.sg.List()
		offset = list.RowRect(row).Center()
	})
	return f.screen.PanelPoint(list, offset)
}

// TestFieldSuggestionsShowBelowField verifies that typing into a focused field shows its suggestions in a popup window
// directly below it, at least as wide as the field and tall enough for the rows, with none of them highlighted.
func TestFieldSuggestionsShowBelowField(t *testing.T) {
	c := check.New(t)
	f := startSuggestions(t, geom.NewRect(20, 20, 240, 200), false)
	c.Nil(f.screen.SuggestionsWindow(f.wnd))
	f.screen.Type("a")
	popup := f.screen.SuggestionsWindow(f.wnd)
	c.NotNil(popup)
	var listed bool
	var rect, fieldRect geom.Rect
	var origin geom.Point
	var rowHeight float32
	var count int
	c.True(f.screen.Do(func() {
		listed = slices.Contains(unison.Windows(), popup)
		rect = popup.ContentRect()
		fieldRect = f.field.RectToRoot(f.field.ContentRect(true))
		origin = f.wnd.ContentRect().Point
		rowHeight = f.sg.List().RowRect(0).Height
		count = f.sg.List().Count()
	}))
	c.False(listed, "Windows() leaves the suggestions window out")
	c.Equal(6, count)
	c.Equal(fieldRect.X+origin.X, rect.X, "at the field's left edge")
	c.Equal(fieldRect.Bottom()+origin.Y, rect.Y, "directly below the field")
	c.True(rect.Width >= fieldRect.Width, "at least as wide as the field")
	c.Equal(float32(count)*rowHeight+2, rect.Height, "every row and the border")
	c.Equal(-1, f.selected(), "nothing is highlighted")
	c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
}

// TestFieldSuggestionsNeedTheFocus verifies that the rows of a field without the keyboard focus are not shown.
func TestFieldSuggestionsNeedTheFocus(t *testing.T) {
	c := check.New(t)
	f := startSuggestions(t, geom.NewRect(20, 20, 240, 200), false)
	var showing bool
	c.True(f.screen.Do(func() {
		f.wnd.SetFocus(f.button)
		f.sg.SetRows([]string{suggestAlmond})
		showing = f.sg.Showing()
	}))
	c.False(showing)
	c.Nil(f.screen.SuggestionsWindow(f.wnd))
	c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
}

// TestFieldSuggestionsNoRowsHides verifies that setting no rows hides them.
func TestFieldSuggestionsNoRowsHides(t *testing.T) {
	c := check.New(t)
	f := startSuggestions(t, geom.NewRect(20, 20, 240, 200), false)
	f.screen.Type("a")
	c.NotNil(f.screen.SuggestionsWindow(f.wnd))
	f.screen.Type("\b")
	c.Nil(f.screen.SuggestionsWindow(f.wnd), "emptying the field leaves nothing to suggest")
	f.screen.Type("b")
	c.NotNil(f.screen.SuggestionsWindow(f.wnd))
	c.True(f.screen.Do(func() { f.sg.SetRows(nil) }))
	c.Nil(f.screen.SuggestionsWindow(f.wnd))
	c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
}

// TestFieldSuggestionsKeyboard verifies that Down and Up move the highlight within the rows, entering them from the top
// and the bottom, that Escape hides them and leaves the text alone, that Return chooses the highlighted row once,
// leaving the focus in the field, and that Return with nothing highlighted hides them and goes on to the field.
func TestFieldSuggestionsKeyboard(t *testing.T) {
	c := check.New(t)
	f := startSuggestions(t, geom.NewRect(20, 20, 240, 200), false)
	f.screen.Type("a")
	f.screen.KeyPress(unison.KeyDown, mod.None)
	c.Equal(0, f.selected(), "Down enters the rows at the top")
	for range 8 {
		f.screen.KeyPress(unison.KeyDown, mod.None)
	}
	c.Equal(5, f.selected(), "and stops at the last row")
	f.screen.KeyPress(unison.KeyUp, mod.None)
	c.Equal(4, f.selected())
	for range 8 {
		f.screen.KeyPress(unison.KeyUp, mod.None)
	}
	c.Equal(0, f.selected(), "Up stops at the first row")

	f.screen.KeyPress(unison.KeyEscape, mod.None)
	c.False(f.showing(), "Escape hides the rows")
	var text string
	c.True(f.screen.Do(func() { text = f.field.Text() }))
	c.Equal("a", text, "and leaves the text alone")

	f.screen.Type("l")
	c.True(f.showing())
	c.Equal(-1, f.selected())
	f.screen.KeyPress(unison.KeyUp, mod.None)
	c.Equal(1, f.selected(), "Up enters the rows at the bottom")
	f.screen.KeyPress(unison.KeyReturn, mod.None)
	c.Equal([]string{suggestAloe}, f.chosenRows(), "Return chooses the highlighted row, once")
	c.False(f.showing())
	var focus *unison.Panel
	c.True(f.screen.Do(func() {
		text = f.field.Text()
		focus = f.wnd.CurrentFocus()
	}))
	c.Equal(suggestAloe, text)
	c.True(focus == f.field.AsPanel(), "the focus stays in the field")
	c.True(f.screen.FocusedWindow() == f.wnd)

	f.screen.Type("\b")
	c.True(f.showing())
	var undoID, afterID int64
	c.True(f.screen.Do(func() { undoID = f.field.CurrentUndoID() }))
	f.screen.KeyPress(unison.KeyReturn, mod.None)
	c.False(f.showing(), "Return with nothing highlighted hides the rows")
	c.True(f.screen.Do(func() { afterID = f.field.CurrentUndoID() }))
	c.NotEqual(undoID, afterID, "and goes on to the field")
	c.Equal([]string{suggestAloe}, f.chosenRows(), "without choosing anything")
	c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
}

// TestFieldSuggestionsPageKeys verifies that PageDown and PageUp move the highlight by the rows that fit in the view.
func TestFieldSuggestionsPageKeys(t *testing.T) {
	c := check.New(t)
	f := startSuggestions(t, geom.NewRect(20, 20, 240, 200), false)
	rows := make([]string, 20)
	for i := range rows {
		rows[i] = fmt.Sprintf("Row %02d", i)
	}
	c.True(f.screen.Do(func() { f.sg.SetRows(rows) }))
	c.True(f.showing())
	f.screen.KeyPress(unison.KeyPageDown, mod.None)
	c.Equal(7, f.selected(), "the last row of the first page")
	f.screen.KeyPress(unison.KeyPageDown, mod.None)
	c.Equal(15, f.selected())
	f.screen.KeyPress(unison.KeyPageDown, mod.None)
	c.Equal(19, f.selected(), "stops at the last row")
	f.screen.KeyPress(unison.KeyPageUp, mod.None)
	c.Equal(11, f.selected())
	f.screen.KeyPress(unison.KeyPageUp, mod.None)
	c.Equal(3, f.selected())
	f.screen.KeyPress(unison.KeyPageUp, mod.None)
	c.Equal(0, f.selected(), "stops at the first row")
	c.True(f.screen.Do(func() { f.sg.Select(-1) }))
	f.screen.KeyPress(unison.KeyPageUp, mod.None)
	c.Equal(12, f.selected(), "the first row of the last page")
	c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
}

// TestFieldSuggestionsModifiedArrowsGoToTheField verifies that an arrow key with a modifier is the field's.
func TestFieldSuggestionsModifiedArrowsGoToTheField(t *testing.T) {
	c := check.New(t)
	f := startSuggestions(t, geom.NewRect(20, 20, 240, 200), false)
	f.screen.Type("a")
	f.screen.KeyPress(unison.KeyDown, mod.None)
	c.Equal(0, f.selected())
	f.screen.KeyPress(unison.KeyDown, mod.Shift)
	c.Equal(0, f.selected(), "Shift+Down leaves the highlight alone")
	c.True(f.showing())
	c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
}

// TestFieldSuggestionsRefilterInPlace verifies that typing again replaces the rows in the same window, with nothing
// highlighted.
func TestFieldSuggestionsRefilterInPlace(t *testing.T) {
	c := check.New(t)
	f := startSuggestions(t, geom.NewRect(20, 20, 240, 200), false)
	f.screen.Type("a")
	first := f.screen.SuggestionsWindow(f.wnd)
	f.screen.KeyPress(unison.KeyDown, mod.None)
	c.Equal(0, f.selected())
	f.screen.Type("p")
	c.True(f.screen.SuggestionsWindow(f.wnd) == first, "the same window")
	c.Equal(-1, f.selected(), "with nothing highlighted")
	var count int
	c.True(f.screen.Do(func() { count = f.sg.List().Count() }))
	c.Equal(2, count)
	c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
}

// TestFieldSuggestionsMouse verifies that the pointer moving over a row or pressing it highlights it and releasing it
// chooses it, leaving the window and the field with the focus.
func TestFieldSuggestionsMouse(t *testing.T) {
	c := check.New(t)
	f := startSuggestions(t, geom.NewRect(20, 20, 240, 200), false)
	f.screen.Type("a")
	f.screen.MouseMove(f.rowCenter(2), mod.None)
	c.Equal(2, f.selected(), "moving over a row highlights it")
	pt := f.rowCenter(1)
	f.screen.MouseDown(pt, unison.ButtonLeft, mod.None)
	c.Equal(1, f.selected(), "pressing a row highlights it")
	c.True(f.showing())
	c.Equal(0, len(f.chosenRows()))
	f.screen.MouseUp(pt, unison.ButtonLeft, mod.None)
	c.Equal([]string{suggestAloe}, f.chosenRows(), "releasing it chooses it")
	c.False(f.showing())
	var focus *unison.Panel
	c.True(f.screen.Do(func() { focus = f.wnd.CurrentFocus() }))
	c.True(f.screen.FocusedWindow() == f.wnd, "the window keeps the focus")
	c.True(focus == f.field.AsPanel(), "and so does the field")
	c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
}

// TestFieldSuggestionsHide verifies that the rows hide when the focus leaves the field, on a press outside the field
// but not on one inside it, when another window comes to the front, when a contextual or other menu opens, when the
// field is scrolled and when it is moved by a layout.
func TestFieldSuggestionsHide(t *testing.T) {
	rect := geom.NewRect(20, 20, 240, 200)
	t.Run("tab", func(t *testing.T) {
		c := check.New(t)
		f := startSuggestions(t, rect, false)
		f.screen.Type("a")
		f.screen.KeyPress(unison.KeyTab, mod.None)
		c.Nil(f.screen.SuggestionsWindow(f.wnd))
		var focus *unison.Panel
		c.True(f.screen.Do(func() { focus = f.wnd.CurrentFocus() }))
		c.True(focus == f.button.AsPanel())
		c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
	})
	t.Run("press", func(t *testing.T) {
		c := check.New(t)
		f := startSuggestions(t, rect, false)
		f.screen.Type("b")
		f.screen.Click(f.screen.PanelCenter(f.field))
		c.NotNil(f.screen.SuggestionsWindow(f.wnd), "a press on the field keeps the rows")
		// Near the bottom of the window, well below the two rows.
		f.screen.Click(geom.NewPoint(rect.X+10, rect.Bottom()-5))
		c.Nil(f.screen.SuggestionsWindow(f.wnd), "a press outside it hides them")
		c.Equal(0, len(f.chosenRows()), "without choosing anything")
		c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
	})
	t.Run("other window", func(t *testing.T) {
		c := check.New(t)
		f := startSuggestions(t, rect, false)
		f.screen.Type("a")
		c.True(f.screen.Do(func() {
			other := newHeadlessWindow(t, "other", geom.NewRect(280, 20, 100, 100), unison.NewPanel())
			if other != nil {
				other.ToFront()
			}
		}))
		c.Nil(f.screen.SuggestionsWindow(f.wnd))
		c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
	})
	t.Run("context menu", func(t *testing.T) {
		c := check.New(t)
		f := startSuggestions(t, rect, false)
		f.screen.Type("a")
		f.screen.ClickWith(f.screen.PanelCenter(f.field), unison.ButtonRight, mod.None)
		c.Equal(1, len(f.screen.MenuWindows(f.wnd)), "the field's contextual menu opened")
		c.Nil(f.screen.SuggestionsWindow(f.wnd))
		f.screen.KeyPress(unison.KeyEscape, mod.None)
		c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
	})
	t.Run("menu", func(t *testing.T) {
		c := check.New(t)
		f := startSuggestions(t, rect, false)
		f.screen.Type("b")
		c.True(f.screen.Do(func() {
			factory := unison.NewInWindowMenuFactory()
			m := factory.NewMenu(unison.PopupMenuTemporaryBaseID, "", nil)
			m.InsertItem(-1, factory.NewItem(unison.PopupMenuTemporaryBaseID+1, "Item", unison.KeyBinding{}, nil, nil))
			m.Popup(geom.NewRect(10, 150, 1, 1), 0)
		}))
		c.Equal(1, len(f.screen.MenuWindows(f.wnd)), "the menu opened")
		c.Nil(f.screen.SuggestionsWindow(f.wnd))
		f.screen.KeyPress(unison.KeyEscape, mod.None)
		c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
	})
	t.Run("scrolled", func(t *testing.T) {
		c := check.New(t)
		f := startSuggestions(t, rect, true)
		f.screen.Type("b")
		c.NotNil(f.screen.SuggestionsWindow(f.wnd))
		f.screen.Wheel(geom.NewPoint(100, 200), geom.NewPoint(0, -1), mod.None)
		c.Nil(f.screen.SuggestionsWindow(f.wnd))
		c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
	})
	t.Run("layout", func(t *testing.T) {
		c := check.New(t)
		f := startSuggestions(t, rect, false)
		f.screen.Type("b")
		c.True(f.screen.Do(func() {
			label := unison.NewLabel()
			label.SetTitle("Above")
			f.column.AddChildAtIndex(label, 0)
			f.column.MarkForLayoutAndRedraw()
		}))
		c.Nil(f.screen.SuggestionsWindow(f.wnd))
		c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
	})
}

// TestFieldSuggestionsChosenCallbackSetsRows verifies that rows set from within ChosenCallback, as the field's
// ModifiedCallback does when the callback puts the chosen row in the field, replace the rows without showing them.
func TestFieldSuggestionsChosenCallbackSetsRows(t *testing.T) {
	c := check.New(t)
	f := startSuggestions(t, geom.NewRect(20, 20, 240, 200), false)
	f.screen.Type("a")
	f.screen.KeyPress(unison.KeyDown, mod.None)
	f.screen.KeyPress(unison.KeyReturn, mod.None)
	c.Equal([]string{suggestAlmond}, f.chosenRows())
	var count int
	c.True(f.screen.Do(func() { count = f.sg.List().Count() }))
	c.Equal(1, count, "the rows were replaced")
	c.False(f.showing(), "but not shown")
	c.Nil(f.screen.SuggestionsWindow(f.wnd))
	c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
}

// TestFieldSuggestionsPlacedAbove verifies that rows with no room below the field are placed directly above it.
func TestFieldSuggestionsPlacedAbove(t *testing.T) {
	c := check.New(t)
	f := startSuggestions(t, geom.NewRect(20, 220, 240, 70), false)
	f.screen.Type("a")
	popup := f.screen.SuggestionsWindow(f.wnd)
	c.NotNil(popup)
	var rect, fieldRect geom.Rect
	var origin geom.Point
	c.True(f.screen.Do(func() {
		rect = popup.ContentRect()
		fieldRect = f.field.RectToRoot(f.field.ContentRect(true))
		origin = f.wnd.ContentRect().Point
	}))
	c.Equal(fieldRect.X+origin.X, rect.X)
	c.Equal(fieldRect.Y+origin.Y, rect.Bottom(), "directly above the field")
	c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
}

// TestFieldSuggestionsFollowTheField verifies that a field moved to another window shows its rows in a new popup window
// belonging to that window, the old one being disposed of.
func TestFieldSuggestionsFollowTheField(t *testing.T) {
	c := check.New(t)
	f := startSuggestions(t, geom.NewRect(20, 20, 240, 200), false)
	f.screen.Type("a")
	first := f.screen.SuggestionsWindow(f.wnd)
	c.NotNil(first)
	var other *unison.Window
	c.True(f.screen.Do(func() {
		f.field.RemoveFromParent()
		if other = newHeadlessWindow(t, "other", geom.NewRect(20, 20, 240, 200), axColumn(f.field)); other != nil {
			other.ToFront()
			other.SetFocus(f.field)
		}
	}))
	c.NotNil(other)
	c.Nil(f.screen.SuggestionsWindow(f.wnd))
	f.screen.Type("b")
	second := f.screen.SuggestionsWindow(other)
	c.NotNil(second)
	c.True(second != first, "a new popup window")
	var valid bool
	c.True(f.screen.Do(func() { valid = first.IsValid() }))
	c.False(valid, "the old one is disposed of")
	c.Nil(f.screen.SuggestionsWindow(f.wnd))
	c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
}

// TestFieldSuggestionsWindowClosedOnItsOwn verifies that a popup window closed by something other than the toolkit is
// forgotten, and that the next rows are shown in a new one.
func TestFieldSuggestionsWindowClosedOnItsOwn(t *testing.T) {
	c := check.New(t)
	f := startSuggestions(t, geom.NewRect(20, 20, 240, 200), false)
	f.screen.Type("a")
	first := f.screen.SuggestionsWindow(f.wnd)
	c.NotNil(first)
	var closed bool
	c.True(f.screen.Do(func() { closed = first.AttemptClose() }))
	c.True(closed)
	c.False(f.showing())
	c.Nil(f.screen.SuggestionsWindow(f.wnd))
	f.screen.Type("l")
	second := f.screen.SuggestionsWindow(f.wnd)
	c.NotNil(second)
	c.True(second != first)
	c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
}

// TestFieldSuggestionsSelectAndMaxVisibleRows verifies Select, Selected and List, and that MaxVisibleRows bounds the
// height, the vertical scroll bar not widening a window the field already makes wide enough for it.
func TestFieldSuggestionsSelectAndMaxVisibleRows(t *testing.T) {
	c := check.New(t)
	f := startSuggestions(t, geom.NewRect(20, 20, 240, 200), false)
	f.screen.Type("a")
	popup := f.screen.SuggestionsWindow(f.wnd)
	c.NotNil(popup)
	var selected []int
	var count int
	var firstRow string
	c.True(f.screen.Do(func() {
		f.sg.Select(2)
		selected = append(selected, f.sg.Selected())
		f.sg.Select(-1)
		selected = append(selected, f.sg.Selected())
		f.sg.Select(3)
		f.sg.Select(6)
		selected = append(selected, f.sg.Selected())
		count = f.sg.List().Count()
		firstRow = f.sg.List().DataAtIndex(0)
	}))
	c.Equal([]int{2, -1, -1}, selected)
	c.Equal(6, count)
	c.Equal(suggestAlmond, firstRow)
	var before, after, fieldRect geom.Rect
	var rowHeight float32
	c.True(f.screen.Do(func() {
		before = popup.ContentRect()
		rowHeight = f.sg.List().RowRect(0).Height
		fieldRect = f.field.RectToRoot(f.field.ContentRect(true))
		f.sg.MaxVisibleRows = 3
		f.sg.Show()
		after = popup.ContentRect()
	}))
	c.True(f.screen.SuggestionsWindow(f.wnd) == popup, "placed again in the same window")
	c.Equal(3*rowHeight+2, after.Height)
	c.Equal(fieldRect.Width, before.Width, "as wide as the field")
	c.Equal(before.Width, after.Width, "which has room for the rows and the scroll bar")
	c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
}

// TestFieldSuggestionsHideDuringPress verifies that hiding the rows while a press on one of them is in progress
// disposes of their window, so that no window holding a press is left hidden, that the release then chooses nothing,
// and that the field's contextual menu still opens afterwards.
func TestFieldSuggestionsHideDuringPress(t *testing.T) {
	c := check.New(t)
	f := startSuggestions(t, geom.NewRect(20, 20, 240, 200), false)
	f.screen.Type("a")
	popup := f.screen.SuggestionsWindow(f.wnd)
	c.NotNil(popup)
	pt := f.rowCenter(1)
	f.screen.MouseDown(pt, unison.ButtonLeft, mod.None)
	c.Equal(1, f.selected())
	f.screen.KeyPress(unison.KeyEscape, mod.None)
	c.False(f.showing())
	var valid bool
	c.True(f.screen.Do(func() { valid = popup.IsValid() }))
	c.False(valid, "the window holding the press is disposed of")
	f.screen.MouseUp(pt, unison.ButtonLeft, mod.None)
	c.Equal(0, len(f.chosenRows()), "the release chooses nothing")
	var text string
	c.True(f.screen.Do(func() { text = f.field.Text() }))
	c.Equal("a", text)
	f.screen.ClickWith(f.screen.PanelCenter(f.field), unison.ButtonRight, mod.None)
	c.Equal(1, len(f.screen.MenuWindows(f.wnd)), "the field's contextual menu opens")
	f.screen.KeyPress(unison.KeyEscape, mod.None)
	c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
}

// TestFieldSuggestionsNotShownWhileMenusOpen verifies that rows set while a menu is open, as a deferred lookup may set
// them, are not shown, since their window would sit above the menu's and take the keys from it.
func TestFieldSuggestionsNotShownWhileMenusOpen(t *testing.T) {
	c := check.New(t)
	f := startSuggestions(t, geom.NewRect(20, 20, 240, 200), false)
	var showing bool
	c.True(f.screen.Do(func() {
		factory := unison.NewInWindowMenuFactory()
		m := factory.NewMenu(unison.PopupMenuTemporaryBaseID, "", nil)
		m.InsertItem(-1, factory.NewItem(unison.PopupMenuTemporaryBaseID+1, "Item", unison.KeyBinding{}, nil, nil))
		m.Popup(geom.NewRect(10, 150, 1, 1), 0)
		f.sg.SetRows([]string{suggestAlmond})
		showing = f.sg.Showing()
	}))
	c.Equal(1, len(f.screen.MenuWindows(f.wnd)), "the menu opened")
	c.False(showing)
	c.Nil(f.screen.SuggestionsWindow(f.wnd))
	f.screen.KeyPress(unison.KeyEscape, mod.None)
	c.Equal(0, len(f.screen.MenuWindows(f.wnd)))
	c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
}

// TestFieldSuggestionsDisabledField verifies that the rows of a disabled field are not shown, and that disabling the
// field hides rows already showing, since a disabled field gets no keys to drive them with.
func TestFieldSuggestionsDisabledField(t *testing.T) {
	c := check.New(t)
	f := startSuggestions(t, geom.NewRect(20, 20, 240, 200), false)
	var showing bool
	c.True(f.screen.Do(func() {
		f.field.SetEnabled(false)
		f.sg.SetRows([]string{suggestAlmond})
		showing = f.sg.Showing()
	}))
	c.False(showing, "not shown for a disabled field")
	c.Nil(f.screen.SuggestionsWindow(f.wnd))
	c.True(f.screen.Do(func() {
		f.field.SetEnabled(true)
		f.sg.Show()
	}))
	c.NotNil(f.screen.SuggestionsWindow(f.wnd))
	c.True(f.screen.Do(func() { f.field.SetEnabled(false) }))
	c.Nil(f.screen.SuggestionsWindow(f.wnd), "disabling the field hides them")
	c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
}

// TestFieldSuggestionsHiddenField verifies that the rows of a field that cannot be seen are not shown, and that hiding
// the field, or an ancestor of it, hides rows already showing.
func TestFieldSuggestionsHiddenField(t *testing.T) {
	c := check.New(t)
	f := startSuggestions(t, geom.NewRect(20, 20, 240, 200), false)
	var showing bool
	c.True(f.screen.Do(func() {
		f.field.Hidden = true
		f.sg.SetRows([]string{suggestAlmond})
		showing = f.sg.Showing()
	}))
	c.False(showing, "not shown for a hidden field")
	c.Nil(f.screen.SuggestionsWindow(f.wnd))
	c.True(f.screen.Do(func() {
		f.field.Hidden = false
		f.sg.Show()
	}))
	c.NotNil(f.screen.SuggestionsWindow(f.wnd))
	c.True(f.screen.Do(func() {
		f.column.Hidden = true
		f.column.MarkForRedraw()
	}))
	c.Nil(f.screen.SuggestionsWindow(f.wnd), "hiding an ancestor of the field hides them")
	c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
}

// TestFieldSuggestionsRowContextMenu verifies that the contextual menu of the rows' list, when the application gives it
// one and an assistive technology asks for it on a row, opens beneath the row where it is on the screen, rather than
// at the row's position within the popup window taken as a position within the field's window, and hides the rows.
func TestFieldSuggestionsRowContextMenu(t *testing.T) {
	c := check.New(t)
	f := startSuggestions(t, geom.NewRect(20, 20, 240, 200), false)
	c.True(f.screen.Do(func() {
		f.sg.List().ContextMenuCallback = func(_ geom.Point) unison.Menu {
			factory := unison.NewInWindowMenuFactory()
			m := factory.NewMenu(unison.PopupMenuTemporaryBaseID, "", nil)
			m.InsertItem(-1, factory.NewItem(unison.PopupMenuTemporaryBaseID+1, "Item", unison.KeyBinding{}, nil, nil))
			return m
		}
	}))
	f.screen.Type("a")
	c.NotNil(f.screen.SuggestionsWindow(f.wnd))
	var list *unison.List[string]
	var rowRect geom.Rect
	c.True(f.screen.Do(func() {
		list = f.sg.List()
		rowRect = list.RowRect(3)
	}))
	expected := f.screen.PanelPoint(list, geom.NewPoint(rowRect.X, rowRect.Bottom()))
	tree := f.screen.AccessibilityTree(f.wnd)
	c.NotNil(tree)
	listNode := axMustNode(c, f.screen.AccessibilityNodeFor(list))
	var row *accessibility.Node
	for _, child := range axChildNodes(tree, listNode) {
		if child.RowIndex == 3 {
			row = child
		}
	}
	row = axMustNode(c, row, "the row is described")
	c.True(row.Actions.Has(accessibility.ShowContextMenu))
	c.True(f.screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   row.ID,
		Action: accessibility.ShowContextMenu,
	}))
	menus := f.screen.MenuWindows(f.wnd)
	c.Equal(1, len(menus), "the row's contextual menu opened")
	if len(menus) != 1 {
		c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
		return
	}
	var rect geom.Rect
	c.True(f.screen.Do(func() { rect = menus[0].ContentRect() }))
	c.Equal(expected, rect.Point, "the menu opens beneath the row")
	c.Nil(f.screen.SuggestionsWindow(f.wnd), "the rows are hidden for the menu")
	f.screen.KeyPress(unison.KeyEscape, mod.None)
	c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
}

// TestFieldSuggestionsClippedField verifies that the rows of a field only partly visible within its window, as one in a
// ScrollPanel may be, are placed below the visible part of the field rather than below its hidden bottom.
func TestFieldSuggestionsClippedField(t *testing.T) {
	c := check.New(t)
	var field *unison.Field
	var sg *unison.FieldSuggestions[string]
	var wnd *unison.Window
	rect := geom.NewRect(20, 20, 240, 150)
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			field = unison.NewMultiLineField()
			field.SetLayoutData(&unison.FlexLayoutData{SizeHint: geom.NewSize(100, 400), HAlign: align.Fill, HGrab: true})
			sg = unison.NewFieldSuggestions[string](field)
			scroller := unison.NewScrollPanel()
			scroller.SetContent(axColumn(field), behavior.Fill, behavior.Unmodified)
			if wnd = newHeadlessWindow(t, "clipped", rect, scroller); wnd != nil {
				wnd.ToFront()
				wnd.SetFocus(field)
			}
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() { sg.SetRows([]string{suggestAlmond}) }))
	popup := screen.SuggestionsWindow(wnd)
	c.NotNil(popup)
	if popup == nil {
		return
	}
	var popupRect, ownerRect geom.Rect
	c.True(screen.Do(func() {
		popupRect = popup.ContentRect()
		ownerRect = wnd.ContentRect()
	}))
	c.Equal(ownerRect.Bottom(), popupRect.Y, "directly below the visible part of the field")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestFieldSuggestionsHideOnPressOutsideWindows verifies that a press in none of the application's windows, which
// moves the focus nowhere, hides the rows, as the platforms' event monitors and pointer grabs arrange for menus.
func TestFieldSuggestionsHideOnPressOutsideWindows(t *testing.T) {
	c := check.New(t)
	f := startSuggestions(t, geom.NewRect(20, 20, 240, 200), false)
	f.screen.Type("a")
	c.NotNil(f.screen.SuggestionsWindow(f.wnd))
	outside := geom.NewPoint(395, 295)
	c.Nil(f.screen.WindowAt(outside))
	f.screen.Click(outside)
	c.Nil(f.screen.SuggestionsWindow(f.wnd), "a press outside every window hides the rows")
	c.True(f.screen.FocusedWindow() == f.wnd, "and moves the focus nowhere")
	c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
}
