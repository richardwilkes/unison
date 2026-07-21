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
	"github.com/richardwilkes/unison/enums/side"
)

type testDockable struct {
	title string
	Panel
}

func newTestDockable(title string) *testDockable {
	d := &testDockable{title: title}
	d.Self = d
	return d
}

func (d *testDockable) TitleIcon(_ geom.Size) Drawable { return nil }
func (d *testDockable) Title() string                  { return d.title }
func (d *testDockable) Tooltip() string                { return "" }
func (d *testDockable) Modified() bool                 { return false }

// newTestDockContainer returns a DockContainer within a real Dock holding the given dockables as tabs, in order, with
// the last one current. There is no Window, so no dockable ever has focus, exercising the focus-elsewhere paths.
func newTestDockContainer(dockables ...Dockable) (*Dock, *DockContainer) {
	dock := NewDock()
	dock.DockTo(dockables[0], nil, side.Left)
	dc := Ancestor[*DockContainer](dockables[0])
	for _, d := range dockables[1:] {
		dc.Stack(d, -1)
	}
	return dock, dc
}

// TestDockContainerCloseCurrentTabWithoutFocus verifies that closing the current (last) tab while focus is elsewhere
// promotes a neighboring tab instead of leaving currentIndex out of range, which used to blank the container and
// misreport CurrentDockable/CurrentDockableIndex until the next draw pass healed it.
func TestDockContainerCloseCurrentTabWithoutFocus(t *testing.T) {
	c := check.New(t)
	d1 := newTestDockable("one")
	d2 := newTestDockable("two")
	_, dc := newTestDockContainer(d1, d2)
	c.Equal(1, dc.CurrentDockableIndex())

	dc.Close(d2)
	c.Equal([]Dockable{d1}, dc.Dockables())
	c.Equal(0, dc.CurrentDockableIndex())
	c.Equal(Dockable(d1), dc.CurrentDockable())
	c.False(d1.Hidden)
}

// TestDockContainerCloseCurrentFirstTabWithoutFocus verifies that closing the current tab promotes the following tab
// and unhides it immediately, rather than leaving its Hidden flag stale until the next layout pass.
func TestDockContainerCloseCurrentFirstTabWithoutFocus(t *testing.T) {
	c := check.New(t)
	d1 := newTestDockable("one")
	d2 := newTestDockable("two")
	d3 := newTestDockable("three")
	_, dc := newTestDockContainer(d1, d2, d3)
	dc.SetCurrentDockable(d1)
	c.Equal(0, dc.CurrentDockableIndex())

	dc.Close(d1)
	c.Equal([]Dockable{d2, d3}, dc.Dockables())
	c.Equal(0, dc.CurrentDockableIndex())
	c.Equal(Dockable(d2), dc.CurrentDockable())
	c.False(d2.Hidden)
	c.True(d3.Hidden)
}

// TestDockContainerCloseNonCurrentTabWithoutFocus verifies that closing a tab before the current one keeps the same
// dockable current, adjusting currentIndex for the shift caused by the removal.
func TestDockContainerCloseNonCurrentTabWithoutFocus(t *testing.T) {
	c := check.New(t)
	d1 := newTestDockable("one")
	d2 := newTestDockable("two")
	d3 := newTestDockable("three")
	_, dc := newTestDockContainer(d1, d2, d3)
	c.Equal(2, dc.CurrentDockableIndex())

	dc.Close(d1)
	c.Equal([]Dockable{d2, d3}, dc.Dockables())
	c.Equal(1, dc.CurrentDockableIndex())
	c.Equal(Dockable(d3), dc.CurrentDockable())
	c.False(d3.Hidden)
	c.True(d2.Hidden)
}

// TestDockContainerCloseLastTabRemovesContainer verifies that closing the only tab removes the container from the Dock
// entirely.
func TestDockContainerCloseLastTabRemovesContainer(t *testing.T) {
	c := check.New(t)
	d1 := newTestDockable("one")
	dock, dc := newTestDockContainer(d1)

	dc.Close(d1)
	c.Nil(dc.Dock)
	c.True(dock.RootDockLayout().Empty())
	c.Equal(0, len(dock.Children()))
}
