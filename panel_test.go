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

func TestPanelAtReturnsSelfWhenNoChildContainsPoint(t *testing.T) {
	c := check.New(t)
	root := framedPanel(0, 0, 200, 200, geom.Point{})
	child := framedPanel(10, 10, 50, 50, geom.Point{})
	root.AddChild(child)
	// (5,5) is outside the child's frame, so the deepest hit is the root itself.
	c.Equal(root, root.PanelAt(geom.NewPoint(5, 5)))
}

func TestPanelAtDescendsIntoChild(t *testing.T) {
	c := check.New(t)
	root := framedPanel(0, 0, 200, 200, geom.Point{})
	child := framedPanel(10, 10, 50, 50, geom.Point{})
	root.AddChild(child)
	// (15,15) falls within the child's frame [10,60).
	c.Equal(child, root.PanelAt(geom.NewPoint(15, 15)))
}

func TestPanelAtSkipsHiddenChildren(t *testing.T) {
	c := check.New(t)
	root := framedPanel(0, 0, 200, 200, geom.Point{})
	child := framedPanel(10, 10, 50, 50, geom.Point{})
	root.AddChild(child)
	child.Hidden = true
	// The point is within the child's frame, but a hidden child is not a hit target.
	c.Equal(root, root.PanelAt(geom.NewPoint(15, 15)))
}

func TestPanelAtAppliesScaleWhenDescending(t *testing.T) {
	c := check.New(t)
	root := framedPanel(0, 0, 200, 200, geom.Point{})
	child := framedPanel(10, 10, 100, 100, geom.NewPoint(2, 2))
	// The grandchild's frame is expressed in the child's (scaled) coordinate system.
	grandchild := framedPanel(5, 5, 10, 10, geom.Point{})
	root.AddChild(child)
	child.AddChild(grandchild)
	// (20,20) is in the child; child-local is (20-10,20-10)/2 = (5,5), which is the grandchild's origin.
	c.Equal(grandchild, root.PanelAt(geom.NewPoint(20, 20)))
	// (8,8) is outside the child's frame entirely, so the root is the hit.
	c.Equal(root, root.PanelAt(geom.NewPoint(8, 8)))
}

func TestPanelAtPrefersLaterSibling(t *testing.T) {
	c := check.New(t)
	root := framedPanel(0, 0, 200, 200, geom.Point{})
	first := framedPanel(10, 10, 50, 50, geom.Point{})
	// Overlaps 'first' in the region [10,60).
	second := framedPanel(10, 10, 50, 50, geom.Point{})
	root.AddChild(first)
	root.AddChild(second)
	// PanelAt iterates children in order and returns the first whose frame contains the point.
	c.Equal(first, root.PanelAt(geom.NewPoint(15, 15)))
}

// Distinct command IDs used by the dispatch tests.
const (
	testCmdA = 9001
	testCmdB = 9002
)

func TestPerformCmdWalksToAncestor(t *testing.T) {
	c := check.New(t)
	parent := unison.NewPanel()
	child := unison.NewPanel()
	parent.AddChild(child)

	performed := 0
	parent.InstallCmdHandlers(testCmdA, unison.AlwaysEnabled, func(_ any) { performed++ })

	// The child has no handler of its own, so dispatch climbs to the parent.
	c.True(child.CanPerformCmd(nil, testCmdA))
	child.PerformCmd(nil, testCmdA)
	c.Equal(1, performed)
}

func TestPerformCmdPrefersNearestHandler(t *testing.T) {
	c := check.New(t)
	parent := unison.NewPanel()
	child := unison.NewPanel()
	parent.AddChild(child)

	parentPerformed := 0
	childPerformed := 0
	parent.InstallCmdHandlers(testCmdA, unison.AlwaysEnabled, func(_ any) { parentPerformed++ })
	child.InstallCmdHandlers(testCmdA, unison.AlwaysEnabled, func(_ any) { childPerformed++ })

	child.PerformCmd(nil, testCmdA)
	c.Equal(1, childPerformed)
	c.Equal(0, parentPerformed)
}

func TestPerformCmdStopsAtDisabledNearestHandler(t *testing.T) {
	c := check.New(t)
	parent := unison.NewPanel()
	child := unison.NewPanel()
	parent.AddChild(child)

	parentPerformed := 0
	// The child claims the command but reports it cannot currently perform it. Because the nearest handler wins, the
	// ancestor's handler must NOT run as a fallback.
	parent.InstallCmdHandlers(testCmdA, unison.AlwaysEnabled, func(_ any) { parentPerformed++ })
	child.InstallCmdHandlers(testCmdA, func(_ any) bool { return false }, func(_ any) {})

	c.False(child.CanPerformCmd(nil, testCmdA))
	child.PerformCmd(nil, testCmdA)
	c.Equal(0, parentPerformed)
}

func TestPerformCmdUnknownIDIsNoOp(t *testing.T) {
	c := check.New(t)
	p := unison.NewPanel()
	p.InstallCmdHandlers(testCmdA, unison.AlwaysEnabled, func(_ any) {})
	c.False(p.CanPerformCmd(nil, testCmdB))
	p.PerformCmd(nil, testCmdB) // Must not panic.
}

func TestCmdHandlersOnNilPanel(t *testing.T) {
	c := check.New(t)
	var p *unison.Panel
	// Documented to be safe on a nil receiver.
	c.False(p.CanPerformCmd(nil, testCmdA))
	p.PerformCmd(nil, testCmdA) // Must not panic.
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

func TestPointToRootSinglePanel(t *testing.T) {
	c := check.New(t)
	// An unparented panel at the origin with no scale is the identity transform.
	p := transformPanel(0, 0, geom.Point{})
	c.Equal(geom.NewPoint(3, 4), p.PointToRoot(geom.NewPoint(3, 4)))

	// An unparented panel offset from the origin adds its frame position.
	p = transformPanel(10, 20, geom.Point{})
	c.Equal(geom.NewPoint(13, 24), p.PointToRoot(geom.NewPoint(3, 4)))
}

func TestPointToRootHierarchy(t *testing.T) {
	c := check.New(t)
	root := transformPanel(0, 0, geom.Point{})
	child := transformPanel(10, 20, geom.Point{})
	grandchild := transformPanel(5, 5, geom.Point{})
	root.AddChild(child)
	child.AddChild(grandchild)
	// (1,2) -> +grandchild(5,5) = (6,7) -> +child(10,20) = (16,27) -> +root(0,0) = (16,27).
	c.Equal(geom.NewPoint(16, 27), grandchild.PointToRoot(geom.NewPoint(1, 2)))
}

func TestPointToRootWithScale(t *testing.T) {
	c := check.New(t)
	root := transformPanel(0, 0, geom.Point{})
	child := transformPanel(10, 20, geom.NewPoint(2, 2))
	grandchild := transformPanel(5, 5, geom.Point{})
	root.AddChild(child)
	child.AddChild(grandchild)
	// (1,2) -> +grandchild(5,5) = (6,7) -> *child scale(2) +child(10,20) = (22,34) -> +root = (22,34).
	c.Equal(geom.NewPoint(22, 34), grandchild.PointToRoot(geom.NewPoint(1, 2)))
}

func TestPointFromRootRoundTrip(t *testing.T) {
	c := check.New(t)
	root := transformPanel(0, 0, geom.Point{})
	child := transformPanel(10, 20, geom.NewPoint(2, 2))
	grandchild := transformPanel(5, 5, geom.Point{})
	root.AddChild(child)
	child.AddChild(grandchild)
	local := geom.NewPoint(1, 2)
	c.Equal(local, grandchild.PointFromRoot(grandchild.PointToRoot(local)))
}

func TestPointFromRootValues(t *testing.T) {
	c := check.New(t)
	root := transformPanel(0, 0, geom.Point{})
	child := transformPanel(10, 20, geom.NewPoint(2, 2))
	root.AddChild(child)
	// (22,34) -> root identity -> child: (22-10, 34-20)/2 = (6,7).
	c.Equal(geom.NewPoint(6, 7), child.PointFromRoot(geom.NewPoint(22, 34)))
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

func TestPointToSelfIsIdentity(t *testing.T) {
	c := check.New(t)
	parent := transformPanel(0, 0, geom.Point{})
	child := transformPanel(10, 20, geom.NewPoint(2, 2))
	parent.AddChild(child)
	pt := geom.NewPoint(3, 4)
	c.Equal(pt, child.PointTo(pt, child))
}

func TestRectToRootNoScale(t *testing.T) {
	c := check.New(t)
	parent := transformPanel(10, 20, geom.Point{})
	child := transformPanel(5, 5, geom.Point{})
	parent.AddChild(child)
	// Without scale the rect is simply translated by the accumulated offsets (5,5)+(10,20) = (15,25).
	c.Equal(geom.NewRect(16, 27, 3, 4), child.RectToRoot(geom.NewRect(1, 2, 3, 4)))
}

func TestRectToRootWithScale(t *testing.T) {
	c := check.New(t)
	parent := transformPanel(10, 20, geom.Point{})
	child := transformPanel(5, 5, geom.NewPoint(2, 2))
	parent.AddChild(child)
	// Origin (1,2) -> *2 +(5,5) +(10,20) = (17,29); size (3,4) scales to (6,8).
	c.Equal(geom.NewRect(17, 29, 6, 8), child.RectToRoot(geom.NewRect(1, 2, 3, 4)))
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
