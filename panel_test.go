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
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison"
)

// TestAncestorOrSelfNil makes sure that the Ancestor and AncestorOrSelf methods accept a nil panel and return the
// zero value instead of a panic. UndoManagerFor sends a possibly-nil Paneler into this path, so this must not crash.
// The deprecated function forms must also accept a nil Paneler.
func TestAncestorOrSelfNil(t *testing.T) {
	c := check.New(t)
	c.Nil((*unison.Panel)(nil).Ancestor[*unison.Panel]())
	c.Nil((*unison.Panel)(nil).AncestorOrSelf[*unison.Panel]())
	c.Nil(unison.Ancestor[*unison.Panel](nil))       //nolint:staticcheck // checks the deprecated form
	c.Nil(unison.AncestorOrSelf[*unison.Panel](nil)) //nolint:staticcheck // checks the deprecated form
	c.Nil(unison.UndoManagerFor(nil))
}

// TestAncestorOrSelfResolution makes sure that AncestorOrSelf finds both the panel itself and its ancestors.
func TestAncestorOrSelfResolution(t *testing.T) {
	c := check.New(t)
	parent := unison.NewPanel()
	child := unison.NewPanel()
	parent.AddChild(child)
	c.True(child.AncestorOrSelf[*unison.Panel]() == child)
	c.True(child.Ancestor[*unison.Panel]() == parent)
}

// framedPanel returns a panel with the given frame and optional scale. A zero scale leaves the panel at its default
// scale of 1.
func framedPanel(x, y, width, height float32, scale geom.Point) *unison.Panel {
	p := unison.NewPanel()
	if scale != (geom.Point{}) {
		p.SetScale(scale)
	}
	p.SetFrameRect(geom.NewRect(x, y, width, height))
	return p
}

// TestPanelAtDescendsIntoChild verifies that PanelAt returns the deepest visible panel containing the point, mapping
// the point into each child's frame and scale on the way down, and the panel itself when no child contains it. Of two
// overlapping siblings, the earlier one wins.
func TestPanelAtDescendsIntoChild(t *testing.T) {
	// scaledTree returns a root holding a child scaled by 2, which holds a grandchild whose frame is expressed in the
	// child's scaled coordinates, along with that grandchild.
	scaledTree := func() (root, grandchild *unison.Panel) {
		root = framedPanel(0, 0, 200, 200, geom.Point{})
		child := framedPanel(10, 10, 100, 100, geom.NewPoint(2, 2))
		grandchild = framedPanel(5, 5, 10, 10, geom.Point{})
		root.AddChild(child)
		child.AddChild(grandchild)
		return root, grandchild
	}
	for _, tc := range []struct {
		build func() (root, want *unison.Panel)
		name  string
		pt    geom.Point
	}{
		{
			name: "a child containing the point",
			pt:   geom.NewPoint(15, 15), // Within the child's frame [10,60).
			build: func() (root, want *unison.Panel) {
				root = framedPanel(0, 0, 200, 200, geom.Point{})
				child := framedPanel(10, 10, 50, 50, geom.Point{})
				root.AddChild(child)
				return root, child
			},
		},
		{
			name: "a hidden child is not a hit target",
			pt:   geom.NewPoint(15, 15),
			build: func() (root, want *unison.Panel) {
				root = framedPanel(0, 0, 200, 200, geom.Point{})
				child := framedPanel(10, 10, 50, 50, geom.Point{})
				child.Hidden = true
				root.AddChild(child)
				return root, root
			},
		},
		{
			name: "a scaled child maps the point into its own coordinates",
			// (20,20) is in the child; child-local is (20-10,20-10)/2 = (5,5), which is the grandchild's origin.
			pt:    geom.NewPoint(20, 20),
			build: scaledTree,
		},
		{
			name: "no child contains the point",
			pt:   geom.NewPoint(8, 8), // Outside the child's frame entirely.
			build: func() (root, want *unison.Panel) {
				root, _ = scaledTree()
				return root, root
			},
		},
		{
			name: "the earlier of two overlapping siblings",
			pt:   geom.NewPoint(15, 15),
			build: func() (root, want *unison.Panel) {
				root = framedPanel(0, 0, 200, 200, geom.Point{})
				first := framedPanel(10, 10, 50, 50, geom.Point{})
				second := framedPanel(10, 10, 50, 50, geom.Point{})
				root.AddChild(first)
				root.AddChild(second)
				return root, first
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, want := tc.build()
			// Compared by identity, since overlapping siblings with the same frame are deeply equal.
			check.New(t).True(root.PanelAt(tc.pt) == want, "PanelAt returned the wrong panel")
		})
	}
}

// Distinct command IDs used by the dispatch tests.
const (
	testCmdA = 9001
	testCmdB = 9002
)

// TestPerformCmdPrefersNearestHandler verifies that a command sent to a child panel goes to the nearest panel up the
// chain with a handler for it, which alone decides whether it can be performed, so a disabled nearest handler does not
// fall back to an ancestor's. With no handler for the command, or a nil panel, nothing is performed and nothing panics.
func TestPerformCmdPrefersNearestHandler(t *testing.T) {
	disabled := func(_ any) bool { return false }
	for _, tc := range []struct {
		parentCan  func(any) bool // Nil leaves the parent without a handler for testCmdA.
		childCan   func(any) bool // Nil leaves the child without a handler for testCmdA.
		name       string
		id         int
		wantParent int
		wantChild  int
		nilPanel   bool // Sends the command to a nil panel instead of the child.
		wantCan    bool
	}{
		{
			name:      "the nearest handler",
			parentCan: unison.AlwaysEnabled,
			childCan:  unison.AlwaysEnabled,
			id:        testCmdA,
			wantCan:   true,
			wantChild: 1,
		},
		{
			name:       "a handler only on an ancestor",
			parentCan:  unison.AlwaysEnabled,
			id:         testCmdA,
			wantCan:    true,
			wantParent: 1,
		},
		{
			name:      "a disabled nearest handler",
			parentCan: unison.AlwaysEnabled,
			childCan:  disabled,
			id:        testCmdA,
		},
		{
			name:     "no handler for the command",
			childCan: unison.AlwaysEnabled,
			id:       testCmdB,
		},
		{
			name:     "a nil panel",
			id:       testCmdA,
			nilPanel: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			parent := unison.NewPanel()
			child := unison.NewPanel()
			parent.AddChild(child)
			parentPerformed := 0
			childPerformed := 0
			if tc.parentCan != nil {
				parent.InstallCmdHandlers(testCmdA, tc.parentCan, func(_ any) { parentPerformed++ })
			}
			if tc.childCan != nil {
				child.InstallCmdHandlers(testCmdA, tc.childCan, func(_ any) { childPerformed++ })
			}
			target := child
			if tc.nilPanel {
				target = nil
			}
			c.Equal(tc.wantCan, target.CanPerformCmd(nil, tc.id))
			c.NotPanics(func() { target.PerformCmd(nil, tc.id) })
			c.Equal(tc.wantParent, parentPerformed)
			c.Equal(tc.wantChild, childPerformed)
		})
	}
}

func TestInstallCmdHandlersReturnsFormerHandlers(t *testing.T) {
	c := check.New(t)
	p := unison.NewPanel()

	firstPerformed := 0
	p.InstallCmdHandlers(testCmdA, unison.AlwaysEnabled, func(_ any) { firstPerformed++ })

	secondPerformed := 0
	formerCan, formerDo := p.InstallCmdHandlers(testCmdA, unison.AlwaysEnabled, func(_ any) { secondPerformed++ })
	c.NotNil(formerCan)
	c.NotNil(formerDo)

	// The newly installed handler is the active one.
	p.PerformCmd(nil, testCmdA)
	c.Equal(0, firstPerformed)
	c.Equal(1, secondPerformed)

	// The returned former handler still refers to the original behavior.
	formerDo(nil)
	c.Equal(1, firstPerformed)
}

func TestRemoveCmdHandler(t *testing.T) {
	c := check.New(t)
	p := unison.NewPanel()
	p.InstallCmdHandlers(testCmdA, unison.AlwaysEnabled, func(_ any) {})
	c.True(p.CanPerformCmd(nil, testCmdA))
	p.RemoveCmdHandler(testCmdA)
	c.False(p.CanPerformCmd(nil, testCmdA))
}

// transformPanel returns a panel positioned at (x, y) in its parent's coordinate system with the given scale. Only the
// frame origin and scale matter to the coordinate transforms, so the frame size is left at zero. A zero scale leaves
// the panel at its default scale of 1.
func transformPanel(x, y float32, scale geom.Point) *unison.Panel {
	p := unison.NewPanel()
	if scale != (geom.Point{}) {
		p.SetScale(scale)
	}
	p.SetFrameRect(geom.NewRect(x, y, 0, 0))
	return p
}

// TestPointToRootWithScale verifies that PointToRoot scales the point by each panel's scale and then adds its frame
// origin, on the way up to the root.
func TestPointToRootWithScale(t *testing.T) {
	// grandchild returns a panel at (5,5) within a child at (10,20) with the given scale, within a root at the origin.
	grandchild := func(childScale geom.Point) *unison.Panel {
		root := transformPanel(0, 0, geom.Point{})
		child := transformPanel(10, 20, childScale)
		p := transformPanel(5, 5, geom.Point{})
		root.AddChild(child)
		child.AddChild(p)
		return p
	}
	for _, tc := range []struct {
		panel *unison.Panel
		name  string
		pt    geom.Point
		want  geom.Point
	}{
		{
			name:  "an unparented panel at the origin",
			panel: transformPanel(0, 0, geom.Point{}),
			pt:    geom.NewPoint(3, 4),
			want:  geom.NewPoint(3, 4),
		},
		{
			name:  "an unparented panel away from the origin",
			panel: transformPanel(10, 20, geom.Point{}),
			pt:    geom.NewPoint(3, 4),
			want:  geom.NewPoint(13, 24),
		},
		{
			// (1,2) -> +grandchild(5,5) = (6,7) -> +child(10,20) = (16,27) -> +root(0,0) = (16,27).
			name:  "an unscaled chain",
			panel: grandchild(geom.Point{}),
			pt:    geom.NewPoint(1, 2),
			want:  geom.NewPoint(16, 27),
		},
		{
			// (1,2) -> +grandchild(5,5) = (6,7) -> *child scale(2) +child(10,20) = (22,34) -> +root = (22,34).
			name:  "a scaled chain",
			panel: grandchild(geom.NewPoint(2, 2)),
			pt:    geom.NewPoint(1, 2),
			want:  geom.NewPoint(22, 34),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			check.New(t).Equal(tc.want, tc.panel.PointToRoot(tc.pt))
		})
	}
}

// TestPointFromRootValues verifies that PointFromRoot undoes each panel's frame origin and scale on the way down from
// the root, so that it inverts PointToRoot through a scaled panel that is not at the origin.
func TestPointFromRootValues(t *testing.T) {
	c := check.New(t)
	root := transformPanel(0, 0, geom.Point{})
	child := transformPanel(10, 20, geom.NewPoint(2, 2))
	grandchild := transformPanel(5, 5, geom.Point{})
	root.AddChild(child)
	child.AddChild(grandchild)
	// (22,34) -> root identity -> child: (22-10, 34-20)/2 = (6,7).
	c.Equal(geom.NewPoint(6, 7), child.PointFromRoot(geom.NewPoint(22, 34)))
	local := geom.NewPoint(1, 2)
	c.Equal(local, grandchild.PointFromRoot(grandchild.PointToRoot(local)))
}

func TestPointTo(t *testing.T) {
	c := check.New(t)
	parent := transformPanel(0, 0, geom.Point{})
	childA := transformPanel(10, 10, geom.Point{})
	childB := transformPanel(50, 5, geom.Point{})
	parent.AddChild(childA)
	parent.AddChild(childB)
	// (1,1) in childA is (11,11) in parent; relative to childB at (50,5) that is (-39,6).
	c.Equal(geom.NewPoint(-39, 6), childA.PointTo(geom.NewPoint(1, 1), childB))
}

// TestRectToRootWithScale verifies that RectToRoot scales a rect's origin and size by each panel's scale and
// translates it by that panel's frame origin, on the way up to the root.
func TestRectToRootWithScale(t *testing.T) {
	for _, tc := range []struct {
		name  string
		scale geom.Point
		want  geom.Rect
	}{
		// Without scale the rect is simply translated by the accumulated offsets (5,5)+(10,20) = (15,25).
		{name: "unscaled", want: geom.NewRect(16, 27, 3, 4)},
		// Origin (1,2) -> *2 +(5,5) +(10,20) = (17,29); size (3,4) scales to (6,8).
		{name: "scaled", scale: geom.NewPoint(2, 2), want: geom.NewRect(17, 29, 6, 8)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parent := transformPanel(10, 20, geom.Point{})
			child := transformPanel(5, 5, tc.scale)
			parent.AddChild(child)
			check.New(t).Equal(tc.want, child.RectToRoot(geom.NewRect(1, 2, 3, 4)))
		})
	}
}

func TestRectFromRootRoundTrip(t *testing.T) {
	c := check.New(t)
	parent := transformPanel(10, 20, geom.Point{})
	child := transformPanel(5, 5, geom.NewPoint(2, 2))
	parent.AddChild(child)
	local := geom.NewRect(1, 2, 3, 4)
	c.Equal(local, child.RectFromRoot(child.RectToRoot(local)))
}

func TestRectTo(t *testing.T) {
	c := check.New(t)
	parent := transformPanel(0, 0, geom.Point{})
	childA := transformPanel(10, 10, geom.Point{})
	childB := transformPanel(50, 5, geom.Point{})
	parent.AddChild(childA)
	parent.AddChild(childB)
	// (1,1,2,2) in childA is (11,11,2,2) in parent; relative to childB at (50,5) the origin is (-39,6).
	c.Equal(geom.NewRect(-39, 6, 2, 2), childA.RectTo(geom.NewRect(1, 1, 2, 2), childB))
}
