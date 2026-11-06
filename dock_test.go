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

// testSizedDockable is a Dockable that asks for a minimum size of its own.
type testSizedDockable struct {
	*testDockable
	minimum geom.Size
}

func (d *testSizedDockable) DockMinimumSize() geom.Size { return d.minimum }

// TestDockLaysOutAgainWhenAContainerNeedsMoreRoom checks that a change to what a container holds has the Dock holding
// it laid out again, so that the container gets the room it now needs: here the container of a Dock nested beside
// another container, in an outer Dock with just enough room for both once the change is made.
func TestDockLaysOutAgainWhenAContainerNeedsMoreRoom(t *testing.T) {
	for _, tc := range []struct {
		change func(container *DockContainer, nested *testNestedDock)
		name   string
	}{
		{
			name: "the nested Dock gains a container",
			change: func(_ *DockContainer, nested *testNestedDock) {
				nested.DockTo(newTestDockable("c"), nil, side.Right)
			},
		},
		{
			name: "a Dockable that needs more room is stacked into the container",
			change: func(container *DockContainer, _ *testNestedDock) {
				sized := &testSizedDockable{testDockable: newTestDockable("sized"), minimum: geom.NewSize(316, 0)}
				sized.Self = sized
				container.Stack(sized, -1)
			},
		},
		{
			name: "a Dockable within the nested Dock comes to need more room",
			change: func(container *DockContainer, nested *testNestedDock) {
				sized := &testSizedDockable{testDockable: newTestDockable("sized")}
				sized.Self = sized
				Ancestor[*DockContainer](nested.NextDockableFor(nil)).Stack(sized, -1)
				container.Dock.ValidateLayout()
				sized.minimum = geom.NewSize(208, 0)
				sized.MarkForLayoutRecursivelyUpward()
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			outer := NewDock()
			outer.SetFrameRect(geom.NewRect(0, 0, 424, 800))
			nested := &testNestedDock{Dock: NewDock()}
			nested.Self = nested
			outer.DockTo(newTestDockable("beside"), nil, side.Left)
			outer.DockTo(nested, nil, side.Right)
			nested.DockTo(newTestDockable("a"), nil, side.Left)
			nested.DockTo(newTestDockable("b"), nil, side.Right)
			outer.ValidateLayout()
			container := Ancestor[*DockContainer](nested)
			c.Equal(float32(208), container.FrameRect().Width, "before the change")

			tc.change(container, nested)
			outer.ValidateLayout()
			c.Equal(float32(316), container.FrameRect().Width, "after the change")
		})
	}
}
