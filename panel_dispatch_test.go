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
