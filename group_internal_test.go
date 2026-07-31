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
)

type testGrouper struct {
	group *Group
	Panel
}

func newTestGrouper() *testGrouper {
	g := &testGrouper{}
	g.Self = g
	return g
}

func (g *testGrouper) Group() *Group {
	return g.group
}

func (g *testGrouper) SetGroup(group *Group) {
	g.group = group
}

// TestNewGroupAddsEachPanelOnce verifies that NewGroup inserts each panel into the group exactly once, so a
// subsequent Remove fully detaches the panel rather than leaving a stale duplicate entry behind.
func TestNewGroupAddsEachPanelOnce(t *testing.T) {
	c := check.New(t)
	g1 := newTestGrouper()
	g2 := newTestGrouper()
	sg := NewGroup(g1, g2)
	c.Equal(2, len(sg.panel))
	c.Equal(sg, g1.Group())
	c.Equal(sg, g2.Group())

	sg.Remove(g1)
	c.Equal(1, len(sg.panel))
	c.Nil(g1.Group())
	for _, one := range sg.panel {
		c.False(one.AsPanel().Is(g1), "removed panel must not remain in the group")
	}

	sg.Remove(g2)
	c.Equal(0, len(sg.panel))
	c.Nil(g2.Group())
}

// TestNewGroupMovesPanelsFromPriorGroup verifies that NewGroup removes its panels from any group they previously
// belonged to.
func TestNewGroupMovesPanelsFromPriorGroup(t *testing.T) {
	c := check.New(t)
	g1 := newTestGrouper()
	first := NewGroup(g1)
	c.Equal(1, len(first.panel))
	second := NewGroup(g1)
	c.Equal(0, len(first.panel))
	c.Equal(1, len(second.panel))
	c.Equal(second, g1.Group())
}
