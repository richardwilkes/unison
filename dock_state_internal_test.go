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
)

// TestDockStateApplyRejectsExcessLayoutChildren verifies that applying a malformed (e.g. hand-edited) saved state
// whose layout node claims more than two children ignores the extras instead of panicking on the fixed-size nodes
// array.
func TestDockStateApplyRejectsExcessLayoutChildren(t *testing.T) {
	c := check.New(t)
	_, layout := newTestDock()
	state := &DockState{
		Type: LayoutType,
		Children: []*DockState{
			{Type: LayoutType},
			{Type: LayoutType},
			{Type: LayoutType},
		},
	}
	state.apply(layout, func(string) Dockable { return nil })
	c.NotNil(layout.nodes[0])
	c.NotNil(layout.nodes[1])
}

// TestDockStateApplyClearsMaximizedContainer verifies that applying a saved state while a container is maximized
// releases the maximized container first. A DockState records no maximized container, so leaving the stale pointer in
// place made DockLayout.PerformLayout's maximized branch hide every restored container (none of them being the removed
// one), leaving the dock blank with no visible way to recover.
func TestDockStateApplyClearsMaximizedContainer(t *testing.T) {
	c := check.New(t)
	dock, dc := newTestDockContainer(newTestDockable("one"))
	dock.SetFrameRect(geom.NewRect(0, 0, 400, 300))
	state := NewDockState(dock, func(d Dockable) string { return d.Title() })
	dock.Maximize(dc)
	c.Equal(dc, dock.MaximizedContainer)

	restored := newTestDockable("one")
	state.Apply(dock, func(string) Dockable { return restored })

	c.Nil(dock.MaximizedContainer)
	restoredContainer := Ancestor[*DockContainer](restored)
	c.NotNil(restoredContainer)
	c.NotEqual(dc, restoredContainer)
	c.False(restoredContainer.Hidden)
	c.False(restoredContainer.FrameRect().Empty())
}

// TestDockStateApplyPerformsLayout verifies that Apply actually performs layout rather than merely marking it needed,
// since code commonly inspects frame rects immediately after restoring a saved state. It used to call the Layout()
// getter, a no-op, instead of ValidateLayout().
func TestDockStateApplyPerformsLayout(t *testing.T) {
	c := check.New(t)
	source := newTestDockable("one")
	dock, _ := newTestDockContainer(source)
	state := NewDockState(dock, func(d Dockable) string { return d.Title() })

	restored := newTestDockable("one")
	target := NewDock()
	target.SetFrameRect(geom.NewRect(0, 0, 400, 300))
	state.Apply(target, func(string) Dockable { return restored })

	c.False(target.NeedsLayout)
	dc := Ancestor[*DockContainer](restored)
	c.NotNil(dc)
	c.False(dc.FrameRect().Empty())
}

// TestDockStateCurrentIndexSkipsUnkeyedDockables verifies that the recorded current tab survives a round trip when a
// dockable ahead of it has no key. Children omits unkeyed dockables and apply() resolves CurrentIndex against that
// filtered list, so recording the position within the full dockable list selected the wrong tab on restore (or fell
// through to the container's default when the index landed out of range).
func TestDockStateCurrentIndexSkipsUnkeyedDockables(t *testing.T) {
	c := check.New(t)
	unkeyed := newTestDockable("unkeyed")
	first := newTestDockable("first")
	current := newTestDockable("current")
	dock, dc := newTestDockContainer(unkeyed, first, current)
	dock.SetFrameRect(geom.NewRect(0, 0, 400, 300))
	c.Equal(2, dc.CurrentDockableIndex(), "newTestDockContainer leaves the last dockable current")

	keyOf := func(d Dockable) string {
		if d == Dockable(unkeyed) {
			return ""
		}
		return d.Title()
	}

	// The current dockable is the second of the two that get saved, so the one-based index is 2, not the 3 its
	// position in the unfiltered list would produce.
	containerState := collectDockState(dc, keyOf)
	c.Equal(ContainerType, containerState.Type)
	c.Equal(2, len(containerState.Children))
	c.Equal(2, containerState.CurrentIndex)

	restored := map[string]Dockable{"first": newTestDockable("first"), "current": newTestDockable("current")}
	target := NewDock()
	target.SetFrameRect(geom.NewRect(0, 0, 400, 300))
	NewDockState(dock, keyOf).Apply(target, func(key string) Dockable { return restored[key] })

	restoredContainer := Ancestor[*DockContainer](restored["current"])
	c.NotNil(restoredContainer)
	c.Equal(restored["current"], restoredContainer.CurrentDockable())
	c.Equal(1, restoredContainer.CurrentDockableIndex())
}

// TestDockStateCurrentIndexWithUnkeyedCurrent verifies that a container whose current dockable is itself unkeyed
// records no current index, since it has no entry in Children to point at, and restores without disturbing the
// container's own default selection.
func TestDockStateCurrentIndexWithUnkeyedCurrent(t *testing.T) {
	c := check.New(t)
	first := newTestDockable("first")
	unkeyed := newTestDockable("unkeyed")
	dock, dc := newTestDockContainer(first, unkeyed)
	dock.SetFrameRect(geom.NewRect(0, 0, 400, 300))
	c.Equal(1, dc.CurrentDockableIndex())

	keyOf := func(d Dockable) string {
		if d == Dockable(unkeyed) {
			return ""
		}
		return d.Title()
	}
	containerState := collectDockState(dc, keyOf)
	c.Equal(1, len(containerState.Children))
	c.Equal(0, containerState.CurrentIndex, "an unkeyed current dockable records no index")

	restoredFirst := newTestDockable("first")
	target := NewDock()
	target.SetFrameRect(geom.NewRect(0, 0, 400, 300))
	NewDockState(dock, keyOf).Apply(target, func(string) Dockable { return restoredFirst })

	restoredContainer := Ancestor[*DockContainer](restoredFirst)
	c.NotNil(restoredContainer)
	c.Equal(Dockable(restoredFirst), restoredContainer.CurrentDockable())
}
