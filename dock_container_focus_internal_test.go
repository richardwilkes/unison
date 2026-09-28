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
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/enums/side"
)

// A headless session owns most of the package's mutable globals for as long as it runs, so none of these tests may
// call t.Parallel.

// newFocusTestDockable returns a dockable holding a single field, so that the focus can be placed within it.
func newFocusTestDockable(title string) (*testDockable, *Field) {
	d := newTestDockable(title)
	d.SetLayout(&FlexLayout{Columns: 1})
	field := NewField()
	d.AddChild(field)
	return d, field
}

// TestDockContainerCloseLastFocusedTabReleasesFocus verifies that closing the only tab of a Dock while the focus is
// within it leaves the window with nothing focused -- the tab is told it lost the focus rather than the window going
// on naming a panel that is no longer in it -- and that Tab then brings the focus back into the window, at its first
// tab stop, rather than being ignored because nothing holds the focus.
func TestDockContainerCloseLastFocusedTabReleasesFocus(t *testing.T) {
	c := check.New(t)
	var wnd *Window
	var other, inner *Field
	var dock *Dock
	var d *testDockable
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			other = NewField()
			d, inner = newFocusTestDockable("one")
			dock = NewDock()
			dock.DockTo(d, nil, side.Left)
			column := NewPanel()
			column.SetLayout(&FlexLayout{Columns: 1})
			column.AddChild(other)
			column.AddChild(dock)
			wnd = axNewTestWindow(t, "close last tab", geom.NewRect(10, 10, 500, 500), column)
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()
	focus := func() *Panel {
		var p *Panel
		screen.Do(func() { p = wnd.CurrentFocus() })
		return p
	}

	var lost int
	var empty bool
	screen.Do(func() {
		wnd.SetFocus(inner)
		lostFocus := inner.LostFocusCallback
		inner.LostFocusCallback = func() {
			lost++
			if lostFocus != nil {
				lostFocus()
			}
		}
	})
	c.True(focus().Is(inner), "the field in the tab holds the focus")

	screen.Do(func() {
		Ancestor[*DockContainer](d).Close(d)
		empty = dock.RootDockLayout().Empty()
	})
	c.True(empty, "closing the only tab empties the dock")
	c.Nil(focus(), "with no other tab to take the focus, nothing holds it")
	c.Equal(1, lost, "and the field was told it lost the focus")

	screen.KeyPress(KeyTab, mod.None)
	c.True(focus().Is(other), "Tab seeds the focus at the window's first tab stop")
	c.Equal(1, lost, "which does not tell the field again")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestDockContainerCloseFocusedTabHandsFocusToNeighbor verifies that closing a tab that holds the focus while another
// container in the Dock still has a tab moves the focus into that tab, as it always has, so that the release of the
// focus when the last tab closes is confined to that case.
func TestDockContainerCloseFocusedTabHandsFocusToNeighbor(t *testing.T) {
	c := check.New(t)
	var wnd *Window
	var first, second *Field
	var d1, d2 *testDockable
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			d1, first = newFocusTestDockable("one")
			d2, second = newFocusTestDockable("two")
			dock := NewDock()
			dock.DockTo(d1, nil, side.Left)
			dock.DockTo(d2, Ancestor[*DockContainer](d1), side.Right)
			wnd = axNewTestWindow(t, "close neighbor tab", geom.NewRect(10, 10, 500, 500), dock)
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()
	focus := func() *Panel {
		var p *Panel
		screen.Do(func() { p = wnd.CurrentFocus() })
		return p
	}

	screen.Do(func() { wnd.SetFocus(first) })
	c.True(focus().Is(first), "the field in the first tab holds the focus")
	screen.Do(func() { Ancestor[*DockContainer](d1).Close(d1) })
	c.True(focus().Is(second), "closing that tab hands the focus to the tab in the other container")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTabSeedsTheFocusWhenNothingHoldsIt verifies that Tab and shift-Tab place the focus when the window has nothing
// focused, at the first and last tab stop respectively, rather than doing nothing until the mouse puts the focus
// somewhere.
func TestTabSeedsTheFocusWhenNothingHoldsIt(t *testing.T) {
	c := check.New(t)
	var wnd *Window
	var first, last *Field
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			first = NewField()
			last = NewField()
			column := NewPanel()
			column.SetLayout(&FlexLayout{Columns: 1})
			column.AddChild(first)
			column.AddChild(last)
			wnd = axNewTestWindow(t, "seed focus", geom.NewRect(10, 10, 500, 500), column)
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()
	focus := func() *Panel {
		var p *Panel
		screen.Do(func() { p = wnd.CurrentFocus() })
		return p
	}

	screen.Do(func() { wnd.SetFocus(nil) })
	c.Nil(focus(), "nothing holds the focus")
	screen.KeyPress(KeyTab, mod.None)
	c.True(focus().Is(first), "Tab seeds the focus at the first tab stop")

	screen.Do(func() { wnd.SetFocus(nil) })
	c.Nil(focus(), "nothing holds the focus")
	screen.KeyPress(KeyTab, mod.Shift)
	c.True(focus().Is(last), "shift-Tab seeds the focus at the last tab stop")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// newTwoFieldDockable returns a dockable holding two fields, so that the focus can be placed on a panel within it other
// than the first that can take the focus, which is where DockContainer.AcquireFocus would put it.
func newTwoFieldDockable(title string) (d *testDockable, first, second *Field) {
	d = newTestDockable(title)
	d.SetLayout(&FlexLayout{Columns: 1})
	first = NewField()
	second = NewField()
	d.AddChild(first)
	d.AddChild(second)
	return d, first, second
}

// countFocusChanges wraps the focus callbacks of the fields so that a test can tell whether a move disturbed the focus,
// returning a function that reports how many times any of them has gained or lost the focus since.
func countFocusChanges(fields ...*Field) func() int {
	var n int
	for _, f := range fields {
		gained, lost := f.GainedFocusCallback, f.LostFocusCallback
		f.GainedFocusCallback = func() {
			n++
			if gained != nil {
				gained()
			}
		}
		f.LostFocusCallback = func() {
			n++
			if lost != nil {
				lost()
			}
		}
	}
	return func() int { return n }
}

// TestDockToMoveOfSoleDockableKeepsFocus verifies that moving the only Dockable in a Dock to another side of it with
// Dock.DockTo, while the focus is on a panel within it other than its first, leaves the focus on that panel with
// nothing told it lost or gained the focus: the removal the move begins with is not a close, and there is no gap in
// which anything could notice the panel outside the window.
func TestDockToMoveOfSoleDockableKeepsFocus(t *testing.T) {
	c := check.New(t)
	var wnd *Window
	var first, second *Field
	var dock *Dock
	var d *testDockable
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			d, first, second = newTwoFieldDockable("one")
			dock = NewDock()
			dock.DockTo(d, nil, side.Left)
			wnd = axNewTestWindow(t, "move sole tab", geom.NewRect(10, 10, 500, 500), dock)
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()
	focus := func() *Panel {
		var p *Panel
		screen.Do(func() { p = wnd.CurrentFocus() })
		return p
	}

	var changes func() int
	screen.Do(func() {
		wnd.SetFocus(second)
		changes = countFocusChanges(first, second)
	})
	c.True(focus().Is(second), "the second field in the tab holds the focus")

	var after *DockContainer
	screen.Do(func() {
		dock.DockTo(d, nil, side.Right)
		after = Ancestor[*DockContainer](d)
	})
	c.NotNil(after, "the tab is in a container again")
	c.True(focus().Is(second), "the second field still holds the focus")
	c.Equal(0, changes(), "and nothing was told it lost or gained the focus")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestStackMoveOfFocusedDockableKeepsFocus verifies that moving a Dockable into another container with Stack, and then
// to another position within that container, keeps the focus on the panel within it that held it, rather than putting
// it on the tab's first focusable panel by way of a neighbor, as a close would.
func TestStackMoveOfFocusedDockableKeepsFocus(t *testing.T) {
	c := check.New(t)
	var wnd *Window
	var first, second *Field
	var d1, d2 *testDockable
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			d1, first, second = newTwoFieldDockable("one")
			d2, _, _ = newTwoFieldDockable("two")
			dock := NewDock()
			dock.DockTo(d1, nil, side.Left)
			dock.DockTo(d2, Ancestor[*DockContainer](d1), side.Right)
			wnd = axNewTestWindow(t, "stack focused tab", geom.NewRect(10, 10, 500, 500), dock)
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()
	focus := func() *Panel {
		var p *Panel
		screen.Do(func() { p = wnd.CurrentFocus() })
		return p
	}

	var changes func() int
	screen.Do(func() {
		wnd.SetFocus(second)
		changes = countFocusChanges(first, second)
	})
	c.True(focus().Is(second), "the second field in the first tab holds the focus")

	var target *DockContainer
	var dockables []Dockable
	screen.Do(func() {
		target = Ancestor[*DockContainer](d2)
		target.Stack(d1, -1)
		dockables = target.Dockables()
	})
	c.Equal([]Dockable{d2, d1}, dockables, "the tab was stacked onto the other container")
	c.True(focus().Is(second), "and the second field still holds the focus")
	c.Equal(0, changes(), "with nothing told it lost or gained the focus")

	screen.Do(func() {
		target.Stack(d1, 0)
		dockables = target.Dockables()
	})
	c.Equal([]Dockable{d1, d2}, dockables, "the tab was moved to the front of its container")
	c.True(focus().Is(second), "which does not disturb the focus either")
	c.Equal(0, changes())
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestDockToMoveToAnotherWindowReleasesFocusInTheOldOne verifies that a Dockable moved into a Dock in another window
// does not leave the focus of the window it left naming a panel that is now in the other window: that window is told
// nothing holds the focus, the panel is told it lost it, and the Dock that received the tab puts the focus within it.
func TestDockToMoveToAnotherWindowReleasesFocusInTheOldOne(t *testing.T) {
	c := check.New(t)
	var from, to *Window
	var first, second *Field
	var d *testDockable
	var dockTo *Dock
	screen := startHeadlessTest(t, HeadlessConfig{Width: 1200, Height: 600},
		StartupFinishedCallback(func() {
			dockTo = NewDock()
			to = axNewTestWindow(t, "to", geom.NewRect(610, 10, 500, 500), dockTo)
			d, first, second = newTwoFieldDockable("one")
			dockFrom := NewDock()
			dockFrom.DockTo(d, nil, side.Left)
			from = axNewTestWindow(t, "from", geom.NewRect(10, 10, 500, 500), dockFrom)
			if from != nil {
				from.ToFront()
			}
		}))
	c.NotNil(from)
	c.NotNil(to)
	screen.Sync()
	focusOf := func(wnd *Window) *Panel {
		var p *Panel
		screen.Do(func() { p = wnd.CurrentFocus() })
		return p
	}

	var lost int
	screen.Do(func() {
		from.SetFocus(second)
		lostFocus := second.LostFocusCallback
		second.LostFocusCallback = func() {
			lost++
			if lostFocus != nil {
				lostFocus()
			}
		}
	})
	c.True(focusOf(from).Is(second), "the second field in the tab holds the focus")

	screen.Do(func() { dockTo.DockTo(d, nil, side.Left) })
	c.Nil(focusOf(from), "the window the tab left has nothing focused")
	c.Equal(1, lost, "and the field was told it lost the focus")
	c.True(focusOf(to).Is(first), "while the Dock that received the tab put the focus on its first field")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
