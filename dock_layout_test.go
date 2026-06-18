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

// fakeDockNode is a minimal DockLayoutNode used to exercise the sizing and framing logic of DockLayout without needing
// a fully constructed DockContainer (which requires fonts and a graphics context). Note that fakeDockNode is
// intentionally NOT a *DockContainer or *DockLayout, so it is only suitable for code paths that operate purely through
// the DockLayoutNode interface (PreferredSize/FrameRect/SetFrameRect) and not the type switches used by the
// tree-management methods.
type fakeDockNode struct {
	pref  geom.Size
	frame geom.Rect
}

func (f *fakeDockNode) PreferredSize() geom.Size { return f.pref }
func (f *fakeDockNode) FrameRect() geom.Rect     { return f.frame }
func (f *fakeDockNode) SetFrameRect(r geom.Rect) { f.frame = r }

// newTestDock returns a Dock with the default theme and an empty root DockLayout, wired together the same way NewDock
// does, but without any of the callbacks or graphics setup.
func newTestDock() (*Dock, *DockLayout) {
	d := &Dock{DockTheme: DefaultDockTheme}
	layout := &DockLayout{dock: d, divider: -1}
	d.layout = layout
	return d, layout
}

func TestDockOrder(t *testing.T) {
	c := check.New(t)
	p1, p2 := dockOrder(side.Top)
	c.Equal(0, p1)
	c.Equal(1, p2)
	p1, p2 = dockOrder(side.Left)
	c.Equal(0, p1)
	c.Equal(1, p2)
	p1, p2 = dockOrder(side.Bottom)
	c.Equal(1, p1)
	c.Equal(0, p2)
	p1, p2 = dockOrder(side.Right)
	c.Equal(1, p1)
	c.Equal(0, p2)
}

func TestDockLayoutEmptyAndFull(t *testing.T) {
	c := check.New(t)
	d, layout := newTestDock()
	c.True(layout.Empty())
	c.False(layout.Full())

	a := &DockContainer{Dock: d}
	layout.nodes[0] = a
	c.False(layout.Empty())
	c.False(layout.Full())

	b := &DockContainer{Dock: d}
	layout.nodes[1] = b
	c.False(layout.Empty())
	c.True(layout.Full())

	layout.nodes[0] = nil
	c.False(layout.Empty())
	c.False(layout.Full())
}

func TestDockLayoutDockToEmpty(t *testing.T) {
	c := check.New(t)
	d, layout := newTestDock()
	a := &DockContainer{Dock: d}
	layout.DockTo(a, layout, side.Left)
	c.Equal(DockLayoutNode(a), layout.nodes[0])
	c.Nil(layout.nodes[1])
	c.True(layout.Horizontal) // Left is a horizontal split

	d, layout = newTestDock()
	a = &DockContainer{Dock: d}
	layout.DockTo(a, layout, side.Bottom)
	c.Equal(DockLayoutNode(a), layout.nodes[1]) // Bottom places into slot 1
	c.Nil(layout.nodes[0])
	c.False(layout.Horizontal) // Bottom is a vertical split
}

func TestDockLayoutDockWithinSecondNode(t *testing.T) {
	c := check.New(t)
	d, layout := newTestDock()
	a := &DockContainer{Dock: d}
	b := &DockContainer{Dock: d}
	layout.DockTo(a, layout, side.Left)
	layout.DockTo(b, layout, side.Right)
	// a was in slot 0; Right places b in slot 1, keeping a where it was.
	c.Equal(DockLayoutNode(a), layout.nodes[0])
	c.Equal(DockLayoutNode(b), layout.nodes[1])
	c.True(layout.Full())
	c.True(layout.Horizontal)
}

func TestDockLayoutDockWithinPushesDown(t *testing.T) {
	c := check.New(t)
	d, layout := newTestDock()
	a := &DockContainer{Dock: d}
	b := &DockContainer{Dock: d}
	cc := &DockContainer{Dock: d}
	layout.DockTo(a, layout, side.Left)
	layout.DockTo(b, layout, side.Right)
	// Both slots are full; docking a third on the left must push the existing pair down into a child layout.
	layout.DockTo(cc, layout, side.Left)
	c.Equal(DockLayoutNode(cc), layout.nodes[0])
	child, ok := layout.nodes[1].(*DockLayout)
	c.True(ok)
	c.Equal(layout, child.parent)
	c.Equal(DockLayoutNode(a), child.nodes[0])
	c.Equal(DockLayoutNode(b), child.nodes[1])
	c.Equal(float32(-1), layout.divider)
}

func TestDockLayoutDockWithContainerSplit(t *testing.T) {
	c := check.New(t)
	d, layout := newTestDock()
	a := &DockContainer{Dock: d}
	b := &DockContainer{Dock: d}
	cc := &DockContainer{Dock: d}
	layout.DockTo(a, layout, side.Left)
	layout.DockTo(b, layout, side.Right)
	// Dock relative to the existing container b; since both slots are full, a new child layout must be created in b's
	// slot holding cc and b.
	layout.DockTo(cc, b, side.Bottom)
	c.Equal(DockLayoutNode(a), layout.nodes[0])
	child, ok := layout.nodes[1].(*DockLayout)
	c.True(ok)
	c.Equal(layout, child.parent)
	c.False(child.Horizontal) // Bottom is a vertical split
	// dockOrder(Bottom) => p1=1, p2=0, so cc lands in slot 1 and b in slot 0.
	c.Equal(DockLayoutNode(cc), child.nodes[1])
	c.Equal(DockLayoutNode(b), child.nodes[0])
}

func TestDockLayoutRepositionWithinSameLayout(t *testing.T) {
	c := check.New(t)
	d, layout := newTestDock()
	a := &DockContainer{Dock: d}
	b := &DockContainer{Dock: d}
	layout.DockTo(a, layout, side.Left)
	layout.DockTo(b, layout, side.Right)
	c.Equal(DockLayoutNode(a), layout.nodes[0])
	c.Equal(DockLayoutNode(b), layout.nodes[1])
	// Move a (currently in slot 0) to the right; it should swap into slot 1 with b moving to slot 0.
	layout.DockTo(a, layout, side.Right)
	c.Equal(DockLayoutNode(b), layout.nodes[0])
	c.Equal(DockLayoutNode(a), layout.nodes[1])
	c.True(layout.Full())
}

func TestDockLayoutRemoveDirect(t *testing.T) {
	c := check.New(t)
	d, layout := newTestDock()
	a := &DockContainer{Dock: d}
	b := &DockContainer{Dock: d}
	layout.nodes[0] = a
	layout.nodes[1] = b

	c.True(layout.Remove(a))
	// Removing slot 0 pulls slot 1's content up into slot 0 only when there is a parent; at the root the second node
	// is preserved and slot 0 is cleared.
	c.Nil(layout.nodes[0])
	c.Equal(DockLayoutNode(b), layout.nodes[1])

	// Removing something not present returns false.
	c.False(layout.Remove(a))
}

func TestDockLayoutRemoveCollapsesChild(t *testing.T) {
	c := check.New(t)
	d, root := newTestDock()
	a := &DockContainer{Dock: d}
	b := &DockContainer{Dock: d}
	other := &DockContainer{Dock: d}
	child := &DockLayout{dock: d, parent: root, divider: -1}
	child.nodes[0] = a
	child.nodes[1] = b
	root.nodes[0] = child
	root.nodes[1] = other

	// Removing a from deep in the tree should collapse the now single-child layout up into its parent's slot.
	c.True(root.Remove(a))
	c.Equal(DockLayoutNode(b), root.nodes[0])
	c.Equal(DockLayoutNode(other), root.nodes[1])
}

func TestDockLayoutRootAndFind(t *testing.T) {
	c := check.New(t)
	d, root := newTestDock()
	a := &DockContainer{Dock: d}
	b := &DockContainer{Dock: d}
	child := &DockLayout{dock: d, parent: root, divider: -1}
	child.nodes[0] = a
	root.nodes[0] = child
	root.nodes[1] = b

	c.Equal(root, root.RootLayout())
	c.Equal(root, child.RootLayout())

	// FindLayout always starts from the root, even when called on a sub-node.
	c.Equal(child, child.FindLayout(a))
	c.Equal(root, root.FindLayout(b))
	c.Nil(root.FindLayout(&DockContainer{Dock: d}))
}

func TestDockLayoutContains(t *testing.T) {
	c := check.New(t)
	d, root := newTestDock()
	a := &DockContainer{Dock: d}
	b := &DockContainer{Dock: d}
	child := &DockLayout{dock: d, parent: root, divider: -1}
	child.nodes[0] = a
	root.nodes[0] = child
	root.nodes[1] = b

	c.True(root.Contains(root))
	c.True(root.Contains(child))
	c.True(root.Contains(a))
	c.True(root.Contains(b))
	c.False(root.Contains(&DockContainer{Dock: d}))
	c.False(child.Contains(b)) // b is not within child
}

func TestDockLayoutForEachDockContainer(t *testing.T) {
	c := check.New(t)
	d, root := newTestDock()
	a := &DockContainer{Dock: d}
	b := &DockContainer{Dock: d}
	cc := &DockContainer{Dock: d}
	child := &DockLayout{dock: d, parent: root, divider: -1}
	child.nodes[0] = b
	child.nodes[1] = cc
	root.nodes[0] = a
	root.nodes[1] = child

	var visited []*DockContainer
	root.ForEachDockContainer(func(dc *DockContainer) bool {
		visited = append(visited, dc)
		return false
	})
	c.Equal(3, len(visited))
	c.Equal(a, visited[0])
	c.Equal(b, visited[1])
	c.Equal(cc, visited[2])

	// Returning true stops the iteration early.
	visited = nil
	root.ForEachDockContainer(func(dc *DockContainer) bool {
		visited = append(visited, dc)
		return true
	})
	c.Equal(1, len(visited))
	c.Equal(a, visited[0])
}

func TestDockLayoutDividerMaximum(t *testing.T) {
	c := check.New(t)
	d, layout := newTestDock()
	dividerSize := d.DockDividerSize()

	// Not full: always 0.
	c.Equal(float32(0), layout.DividerMaximum())

	layout.nodes[0] = &fakeDockNode{}
	layout.nodes[1] = &fakeDockNode{}
	layout.Horizontal = true
	layout.frame = geom.NewRect(0, 0, 200, 100)
	c.Equal(200-dividerSize, layout.DividerMaximum())

	layout.Horizontal = false
	c.Equal(100-dividerSize, layout.DividerMaximum())

	// Frame smaller than divider clamps to 0 rather than going negative.
	layout.frame = geom.NewRect(0, 0, 1, 1)
	c.Equal(float32(0), layout.DividerMaximum())
}

func TestDockLayoutDividerPosition(t *testing.T) {
	c := check.New(t)
	_, layout := newTestDock()

	// Not full: position is 0.
	c.Equal(float32(0), layout.DividerPosition())

	node0 := &fakeDockNode{frame: geom.NewRect(0, 0, 75, 40)}
	layout.nodes[0] = node0
	layout.nodes[1] = &fakeDockNode{}
	layout.frame = geom.NewRect(0, 0, 200, 100)

	// Default mode (divider < 0) reports the first node's current extent.
	layout.divider = -1
	layout.Horizontal = true
	c.Equal(float32(75), layout.DividerPosition())
	layout.Horizontal = false
	c.Equal(float32(40), layout.DividerPosition())

	// Explicit position is clamped to the maximum.
	layout.Horizontal = true
	layout.divider = 50
	c.Equal(float32(50), layout.DividerPosition())
	layout.divider = 10000
	c.Equal(layout.DividerMaximum(), layout.DividerPosition())

	c.Equal(float32(10000), layout.RawDividerPosition())
}

func TestDockLayoutSetDividerPosition(t *testing.T) {
	c := check.New(t)
	d, layout := newTestDock()
	layout.parent = &DockLayout{dock: d} // avoid the root ContentRect path during the triggered PerformLayout
	layout.nodes[0] = &fakeDockNode{}
	layout.nodes[1] = &fakeDockNode{}
	layout.frame = geom.NewRect(0, 0, 200, 100)

	layout.SetDividerPosition(60)
	c.Equal(float32(60), layout.RawDividerPosition())

	// A negative value resets to the default (-1) mode.
	layout.SetDividerPosition(-5)
	c.Equal(float32(-1), layout.RawDividerPosition())
}

func TestDockLayoutLayoutSizes(t *testing.T) {
	c := check.New(t)
	d, layout := newTestDock()
	dividerSize := d.DockDividerSize()

	// Empty layout has a zero preferred size.
	_, pref, _ := layout.LayoutSizes(nil, geom.Size{})
	c.Equal(geom.Size{}, pref)

	// Single node mirrors that node's preferred size.
	layout.nodes[0] = &fakeDockNode{pref: geom.NewSize(100, 50)}
	_, pref, _ = layout.LayoutSizes(nil, geom.Size{})
	c.Equal(geom.NewSize(100, 50), pref)

	// Two nodes, vertical: width is the max, height is doubled plus the divider.
	layout.nodes[1] = &fakeDockNode{pref: geom.NewSize(80, 60)}
	layout.Horizontal = false
	_, pref, _ = layout.LayoutSizes(nil, geom.Size{})
	c.Equal(geom.NewSize(100, 60*2+dividerSize), pref)

	// Two nodes, horizontal: height is the max, width is doubled plus the divider.
	layout.Horizontal = true
	_, pref, _ = layout.LayoutSizes(nil, geom.Size{})
	c.Equal(geom.NewSize(100*2+dividerSize, 60), pref)
}

func TestDockLayoutPerformLayoutSingleNode(t *testing.T) {
	c := check.New(t)
	d, layout := newTestDock()
	layout.parent = &DockLayout{dock: d} // skip the root ContentRect path
	node := &fakeDockNode{}
	layout.nodes[0] = node
	layout.frame = geom.NewRect(5, 7, 200, 100)
	layout.PerformLayout(nil)
	c.Equal(geom.NewRect(5, 7, 200, 100), node.frame)
}

func TestDockLayoutPerformLayoutVertical(t *testing.T) {
	c := check.New(t)
	d, layout := newTestDock()
	dividerSize := d.DockDividerSize()
	layout.parent = &DockLayout{dock: d}
	node0 := &fakeDockNode{}
	node1 := &fakeDockNode{}
	layout.nodes[0] = node0
	layout.nodes[1] = node1
	layout.Horizontal = false
	layout.frame = geom.NewRect(0, 0, 200, 108)

	// Default divider splits the available (frame minus divider) space evenly.
	layout.divider = -1
	layout.PerformLayout(nil)
	available := float32(108) - dividerSize
	primary := available / 2
	c.Equal(geom.NewRect(0, 0, 200, primary), node0.frame)
	c.Equal(geom.NewRect(0, primary+dividerSize, 200, available-primary), node1.frame)

	// Explicit divider position.
	layout.divider = 30
	layout.PerformLayout(nil)
	c.Equal(geom.NewRect(0, 0, 200, 30), node0.frame)
	c.Equal(geom.NewRect(0, 30+dividerSize, 200, available-30), node1.frame)
}

func TestDockLayoutPerformLayoutHorizontal(t *testing.T) {
	c := check.New(t)
	d, layout := newTestDock()
	dividerSize := d.DockDividerSize()
	layout.parent = &DockLayout{dock: d}
	node0 := &fakeDockNode{}
	node1 := &fakeDockNode{}
	layout.nodes[0] = node0
	layout.nodes[1] = node1
	layout.Horizontal = true
	layout.frame = geom.NewRect(0, 0, 208, 100)

	layout.divider = 40
	layout.PerformLayout(nil)
	available := float32(208) - dividerSize
	c.Equal(geom.NewRect(0, 0, 40, 100), node0.frame)
	c.Equal(geom.NewRect(40+dividerSize, 0, available-40, 100), node1.frame)
}

func TestDockLayoutPerformLayoutClampsDivider(t *testing.T) {
	c := check.New(t)
	d, layout := newTestDock()
	dividerSize := d.DockDividerSize()
	layout.parent = &DockLayout{dock: d}
	node0 := &fakeDockNode{}
	node1 := &fakeDockNode{}
	layout.nodes[0] = node0
	layout.nodes[1] = node1
	layout.Horizontal = false
	layout.frame = geom.NewRect(0, 0, 200, 108)

	// A divider larger than the available space is clamped down to the available space.
	layout.divider = 10000
	layout.PerformLayout(nil)
	available := float32(108) - dividerSize
	c.Equal(available, layout.divider)
	c.Equal(geom.NewRect(0, 0, 200, available), node0.frame)
	c.Equal(geom.NewRect(0, available+dividerSize, 200, 0), node1.frame)
}
