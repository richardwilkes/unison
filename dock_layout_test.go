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
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/side"
)

// fakeDockNode is a minimal DockLayoutNode used to exercise the sizing and framing logic of DockLayout without needing
// a fully constructed DockContainer (which requires fonts and a graphics context). Note that fakeDockNode is
// intentionally NOT a *DockContainer or *DockLayout, so it is only suitable for code paths that operate purely through
// the DockLayoutNode interface (PreferredSize/FrameRect/SetFrameRect) and not the type switches used by the
// tree-management methods.
type fakeDockNode struct {
	pref    geom.Size
	minimum geom.Size
	frame   geom.Rect
}

func (f *fakeDockNode) PreferredSize() geom.Size { return f.pref }
func (f *fakeDockNode) minimumSize() geom.Size   { return f.minimum }
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
	// a was in slot 0; Right places b in slot 1, keeping a where it was. The two containers are deeply equal, so the
	// slots are compared by identity.
	c.True(layout.nodes[0] == DockLayoutNode(a))
	c.True(layout.nodes[1] == DockLayoutNode(b))
	c.True(layout.Full())
	c.True(layout.Horizontal)
	// Move a (currently in slot 0) to the right; it should swap into slot 1 with b moving to slot 0.
	layout.DockTo(a, layout, side.Right)
	c.True(layout.nodes[0] == DockLayoutNode(b))
	c.True(layout.nodes[1] == DockLayoutNode(a))
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

// TestDockLayoutPerformLayoutVertical checks the frames PerformLayout gives a lone node, and two nodes split vertically
// or horizontally by a default, explicit or clamped divider, with and without minimum sizes to keep them at.
func TestDockLayoutPerformLayoutVertical(t *testing.T) {
	dividerSize := DefaultDockTheme.DockDividerSize()
	vAvailable := 108 - dividerSize
	hAvailable := 208 - dividerSize
	for _, tc := range []struct {
		name       string
		frame      geom.Rect
		want0      geom.Rect
		want1      geom.Rect
		min0       geom.Size
		min1       geom.Size
		setWithin  geom.Rect // The frame the divider is set by hand within, if it is
		divider    float32
		single     bool
		horizontal bool
	}{
		{
			name:    "single node fills the frame",
			frame:   geom.NewRect(5, 7, 200, 100),
			want0:   geom.NewRect(5, 7, 200, 100),
			divider: -1,
			single:  true,
		},
		{
			name:    "default divider splits the available space evenly",
			frame:   geom.NewRect(0, 0, 200, 108),
			want0:   geom.NewRect(0, 0, 200, vAvailable/2),
			want1:   geom.NewRect(0, vAvailable/2+dividerSize, 200, vAvailable-vAvailable/2),
			divider: -1,
		},
		{
			name:    "explicit divider",
			frame:   geom.NewRect(0, 0, 200, 108),
			want0:   geom.NewRect(0, 0, 200, 30),
			want1:   geom.NewRect(0, 30+dividerSize, 200, vAvailable-30),
			divider: 30,
		},
		{
			name:    "divider clamped to the available space",
			frame:   geom.NewRect(0, 0, 200, 108),
			want0:   geom.NewRect(0, 0, 200, vAvailable),
			want1:   geom.NewRect(0, vAvailable+dividerSize, 200, 0),
			divider: 10000,
		},
		{
			name:       "horizontal",
			frame:      geom.NewRect(0, 0, 208, 100),
			want0:      geom.NewRect(0, 0, 40, 100),
			want1:      geom.NewRect(40+dividerSize, 0, hAvailable-40, 100),
			divider:    40,
			horizontal: true,
		},
		{
			name:      "divider held back to leave the second node its minimum once there is less to share",
			frame:     geom.NewRect(0, 0, 200, 108),
			want0:     geom.NewRect(0, 0, 200, vAvailable-30),
			want1:     geom.NewRect(0, vAvailable-30+dividerSize, 200, 30),
			min1:      geom.NewSize(0, 30),
			divider:   90,
			setWithin: geom.NewRect(0, 0, 200, 208),
		},
		{
			name:    "divider of unknown origin held back to leave the second node its minimum",
			frame:   geom.NewRect(0, 0, 200, 108),
			want0:   geom.NewRect(0, 0, 200, vAvailable-30),
			want1:   geom.NewRect(0, vAvailable-30+dividerSize, 200, 30),
			min1:    geom.NewSize(0, 30),
			divider: 10000,
		},
		{
			name:      "divider set against the far edge stays there",
			frame:     geom.NewRect(0, 0, 200, 108),
			want0:     geom.NewRect(0, 0, 200, vAvailable),
			want1:     geom.NewRect(0, vAvailable+dividerSize, 200, 0),
			min1:      geom.NewSize(0, 30),
			divider:   vAvailable,
			setWithin: geom.NewRect(0, 0, 200, 108),
		},
		{
			name:      "divider set against the near edge stays there",
			frame:     geom.NewRect(0, 0, 200, 108),
			want0:     geom.NewRect(0, 0, 200, 0),
			want1:     geom.NewRect(0, dividerSize, 200, vAvailable),
			min0:      geom.NewSize(0, 30),
			divider:   0,
			setWithin: geom.NewRect(0, 0, 200, 108),
		},
		{
			name:    "default divider gives way to a minimum",
			frame:   geom.NewRect(0, 0, 200, 108),
			want0:   geom.NewRect(0, 0, 200, 70),
			want1:   geom.NewRect(0, 70+dividerSize, 200, vAvailable-70),
			min0:    geom.NewSize(0, 70),
			divider: -1,
		},
		{
			name:      "minimums that do not fit share the space in proportion",
			frame:     geom.NewRect(0, 0, 200, 108),
			want0:     geom.NewRect(0, 0, 200, vAvailable*3/4),
			want1:     geom.NewRect(0, vAvailable*3/4+dividerSize, 200, vAvailable/4),
			min0:      geom.NewSize(0, 150),
			min1:      geom.NewSize(0, 50),
			divider:   300,
			setWithin: geom.NewRect(0, 0, 200, 408),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			d, layout := newTestDock()
			layout.parent = &DockLayout{dock: d} // skip the root ContentRect path
			node0 := &fakeDockNode{minimum: tc.min0}
			node1 := &fakeDockNode{minimum: tc.min1}
			layout.nodes[0] = node0
			if !tc.single {
				layout.nodes[1] = node1
			}
			layout.Horizontal = tc.horizontal
			if tc.setWithin.Empty() {
				layout.divider = tc.divider
			} else {
				layout.frame = tc.setWithin
				layout.SetDividerPosition(tc.divider)
			}
			layout.frame = tc.frame
			layout.PerformLayout(nil)
			c.Equal(tc.want0, node0.frame)
			c.Equal(tc.want1, node1.frame)
			c.Equal(tc.divider, layout.divider, "laying out must not clamp the raw divider")
		})
	}
}

// TestDockLayoutDividersAfterRearranging checks a Dock's layout after it is rearranged, resized or has its divider set:
// starting from two containers side by side, split 3:1 by a divider that was dragged there, each case changes the Dock
// and gives the frames that the dockables' containers must then have.
func TestDockLayoutDividersAfterRearranging(t *testing.T) {
	containerOf := func(d Dockable) *DockContainer { return Ancestor[*DockContainer](d) }
	for _, tc := range []struct {
		rearrange func(dock *Dock, a, b, c Dockable)
		name      string
		want      [3]geom.Rect
		closed    [3]bool // Whether the dockable is no longer in the Dock
	}{
		{
			name:      "docking beside the pair keeps its proportions",
			rearrange: func(dock *Dock, _, _, c Dockable) { dock.DockTo(c, nil, side.Right) },
			want: [3]geom.Rect{
				geom.NewRect(0, 0, 378, 800),
				geom.NewRect(386, 0, 126, 800),
				geom.NewRect(520, 0, 512, 800),
			},
		},
		{
			name: "removing what was docked beside the pair restores it",
			rearrange: func(dock *Dock, _, _, c Dockable) {
				dock.DockTo(c, nil, side.Right)
				dock.ValidateLayout()
				containerOf(c).Close(c)
			},
			want:   [3]geom.Rect{geom.NewRect(0, 0, 768, 800), geom.NewRect(776, 0, 256, 800)},
			closed: [3]bool{false, false, true},
		},
		{
			name:      "splitting the first container leaves the divider beside it alone",
			rearrange: func(dock *Dock, a, _, c Dockable) { dock.DockTo(c, containerOf(a), side.Bottom) },
			want: [3]geom.Rect{
				geom.NewRect(0, 0, 768, 396),
				geom.NewRect(776, 0, 256, 800),
				geom.NewRect(0, 404, 768, 396),
			},
		},
		{
			name: "a container closed up by hand stays closed",
			rearrange: func(dock *Dock, _, _, c Dockable) {
				dock.RootDockLayout().SetDividerPosition(1024)
				dock.DockTo(c, nil, side.Right)
			},
			want: [3]geom.Rect{
				geom.NewRect(0, 0, 504, 800),
				geom.NewRect(512, 0, 0, 800),
				geom.NewRect(520, 0, 512, 800),
			},
		},
		{
			name:      "a divider does not outlive a change of orientation",
			rearrange: func(dock *Dock, a, b, _ Dockable) { dock.DockTo(b, containerOf(a), side.Bottom) },
			want:      [3]geom.Rect{geom.NewRect(0, 0, 1032, 396), geom.NewRect(0, 404, 1032, 396)},
			closed:    [3]bool{false, false, true},
		},
		{
			name: "a divider does not outlive the container it closed up",
			rearrange: func(dock *Dock, _, b, c Dockable) {
				dock.RootDockLayout().SetDividerPosition(1024)
				containerOf(b).Close(b)
				dock.DockTo(c, nil, side.Right)
			},
			want:   [3]geom.Rect{geom.NewRect(0, 0, 512, 800), {}, geom.NewRect(520, 0, 512, 800)},
			closed: [3]bool{false, true, false},
		},
		{
			name: "a divider squeezed while the Dock was too small returns once it has the room",
			rearrange: func(dock *Dock, a, _, c Dockable) {
				dock.SetFrameRect(geom.NewRect(0, 0, 160, 800))
				dock.ValidateLayout()
				dock.DockTo(c, containerOf(a), side.Bottom)
				dock.ValidateLayout()
				dock.SetFrameRect(geom.NewRect(0, 0, 1032, 800))
			},
			want: [3]geom.Rect{
				geom.NewRect(0, 0, 768, 396),
				geom.NewRect(776, 0, 256, 800),
				geom.NewRect(0, 404, 768, 396),
			},
		},
		{
			name: "splitting a container after setting the divider leaves room for both halves",
			rearrange: func(dock *Dock, a, _, c Dockable) {
				dock.RootDockLayout().SetDividerPosition(150)
				dock.DockTo(c, containerOf(a), side.Right)
			},
			want: [3]geom.Rect{
				geom.NewRect(0, 0, 100, 800),
				geom.NewRect(216, 0, 816, 800),
				geom.NewRect(108, 0, 100, 800),
			},
		},
		{
			name: "a divider set while a resize awaits layout is kept as given",
			rearrange: func(dock *Dock, _, _, _ Dockable) {
				dock.SetFrameRect(geom.NewRect(0, 0, 508, 800))
				dock.RootDockLayout().SetDividerPosition(500)
			},
			want:   [3]geom.Rect{geom.NewRect(0, 0, 500, 800), geom.NewRect(508, 0, 0, 800)},
			closed: [3]bool{false, false, true},
		},
		{
			name: "a divider set while a panel above the Dock awaits layout is kept as given",
			rearrange: func(dock *Dock, _, _, _ Dockable) {
				parent := NewPanel()
				parent.SetLayout(&FlexLayout{Columns: 1})
				dock.SetLayoutData(&FlexLayoutData{HAlign: align.Fill, VAlign: align.Fill, HGrab: true, VGrab: true})
				parent.AddChild(dock)
				parent.SetFrameRect(geom.NewRect(0, 0, 508, 800))
				dock.ValidateLayout()
				dock.RootDockLayout().SetDividerPosition(500)
				// Laying out the Dock alone, ahead of the panel above it, must leave the divider waiting.
				dock.ValidateLayout()
				parent.ValidateLayout()
			},
			want:   [3]geom.Rect{geom.NewRect(0, 0, 500, 800), geom.NewRect(508, 0, 0, 800)},
			closed: [3]bool{false, false, true},
		},
		{
			name: "a divider set while a panel above the Dock awaits layout is held back when the Dock later shrinks",
			rearrange: func(dock *Dock, _, _, _ Dockable) {
				parent := NewPanel()
				parent.AddChild(dock)
				dock.RootDockLayout().SetDividerPosition(500)
				// Laying out the Dock alone first must still have the Dock laid out again after the panel above it.
				dock.ValidateLayout()
				parent.ValidateLayout()
				dock.SetFrameRect(geom.NewRect(0, 0, 508, 800))
			},
			want:   [3]geom.Rect{geom.NewRect(0, 0, 400, 800), geom.NewRect(408, 0, 100, 800)},
			closed: [3]bool{false, false, true},
		},
		{
			name: "dividers set in two layouts while a panel above the Dock awaits layout are each kept as given",
			rearrange: func(dock *Dock, a, _, c Dockable) {
				dock.DockTo(c, containerOf(a), side.Bottom)
				parent := NewPanel()
				parent.SetLayout(&FlexLayout{Columns: 1})
				dock.SetLayoutData(&FlexLayoutData{HAlign: align.Fill, VAlign: align.Fill, HGrab: true, VGrab: true})
				parent.AddChild(dock)
				parent.SetFrameRect(geom.NewRect(0, 0, 508, 800))
				dock.ValidateLayout()
				dock.RootDockLayout().SetDividerPosition(500)
				dock.RootDockLayout().FindLayout(containerOf(a)).SetDividerPosition(296)
				parent.ValidateLayout()
			},
			want: [3]geom.Rect{
				geom.NewRect(0, 0, 500, 296),
				geom.NewRect(508, 0, 0, 800),
				geom.NewRect(0, 304, 500, 496),
			},
		},
		{
			name: "setting a nested divider leaves a maximized container filling the Dock",
			rearrange: func(dock *Dock, a, b, c Dockable) {
				dock.DockTo(c, containerOf(a), side.Bottom)
				dock.ValidateLayout()
				dock.Maximize(containerOf(b))
				dock.ValidateLayout()
				dock.RootDockLayout().FindLayout(containerOf(a)).SetDividerPosition(396)
			},
			want: [3]geom.Rect{
				geom.NewRect(0, 0, 768, 396),
				geom.NewRect(0, 0, 1032, 800),
				geom.NewRect(0, 404, 768, 396),
			},
		},
		{
			name: "a divider set while the Dock has no room is kept as given once it has",
			rearrange: func(dock *Dock, _, _, _ Dockable) {
				dock.SetFrameRect(geom.Rect{})
				dock.ValidateLayout()
				dock.RootDockLayout().SetDividerPosition(10000)
				dock.SetFrameRect(geom.NewRect(0, 0, 1032, 800))
			},
			want:   [3]geom.Rect{geom.NewRect(0, 0, 1024, 800), geom.NewRect(1032, 0, 0, 800)},
			closed: [3]bool{false, false, true},
		},
		{
			name: "a divider set while a layout is pending keeps its proportions when docking beside the pair",
			rearrange: func(dock *Dock, _, _, c Dockable) {
				dock.MarkForLayoutAndRedraw()
				dock.RootDockLayout().SetDividerPosition(768)
				dock.DockTo(c, nil, side.Right)
			},
			want: [3]geom.Rect{
				geom.NewRect(0, 0, 378, 800),
				geom.NewRect(386, 0, 126, 800),
				geom.NewRect(520, 0, 512, 800),
			},
		},
		{
			name: "a divider set while a panel above the Dock awaits layout keeps its proportions through docking",
			rearrange: func(dock *Dock, _, _, c Dockable) {
				parent := NewPanel()
				parent.AddChild(dock)
				dock.RootDockLayout().SetDividerPosition(768)
				dock.DockTo(c, nil, side.Right)
			},
			want: [3]geom.Rect{
				geom.NewRect(0, 0, 378, 800),
				geom.NewRect(386, 0, 126, 800),
				geom.NewRect(520, 0, 512, 800),
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			dock := NewDock()
			dock.SetFrameRect(geom.NewRect(0, 0, 1032, 800))
			dockables := [3]Dockable{newTestDockable("a"), newTestDockable("b"), newTestDockable("c")}
			dock.DockTo(dockables[0], nil, side.Left)
			dock.DockTo(dockables[1], nil, side.Right)
			dock.ValidateLayout()
			dock.RootDockLayout().SetDividerPosition(768)
			tc.rearrange(dock, dockables[0], dockables[1], dockables[2])
			dock.ValidateLayout()
			c.False(dock.NeedsLayout, "the Dock must not be left awaiting layout")
			for i, d := range dockables {
				dc := containerOf(d)
				c.Equal(tc.closed[i], dc == nil, "dockable %s", d.Title())
				if dc != nil {
					c.Equal(tc.want[i], dc.FrameRect(), "dockable %s", d.Title())
				}
			}
		})
	}
}

// testNestedDock is a Dock that can be placed within another, as a Dockable.
type testNestedDock struct {
	*Dock
}

func (d *testNestedDock) TitleIcon(_ geom.Size) Drawable { return nil }
func (d *testNestedDock) Title() string                  { return "nested" }
func (d *testNestedDock) Tooltip() string                { return "" }
func (d *testNestedDock) Modified() bool                 { return false }

// TestDockLayoutKeepsContainersAtMinimumSize checks that a Dock which shrinks leaves each container the minimum size
// its theme gives it, and the container of a nested Dock the minimum of every container side by side within that Dock,
// while a divider dragged to the far edge still closes that container up.
func TestDockLayoutKeepsContainersAtMinimumSize(t *testing.T) {
	c := check.New(t)
	outer := NewDock()
	outer.SetFrameRect(geom.NewRect(0, 0, 1032, 800))
	nested := &testNestedDock{Dock: NewDock()}
	nested.Self = nested
	beside := newTestDockable("beside")
	a := newTestDockable("a")
	b := newTestDockable("b")
	outer.DockTo(beside, nil, side.Left)
	outer.DockTo(nested, nil, side.Right)
	nested.DockTo(a, nil, side.Left)
	nested.DockTo(b, nil, side.Right)
	outer.ValidateLayout()
	outer.RootDockLayout().SetDividerPosition(600)
	dividerSize := outer.DockDividerSize()

	outer.SetFrameRect(geom.NewRect(0, 0, 700+dividerSize, 800))
	outer.ValidateLayout()
	c.Equal(200+dividerSize, Ancestor[*DockContainer](nested).FrameRect().Width)
	c.Equal(float32(100), Ancestor[*DockContainer](a).FrameRect().Width)
	c.Equal(float32(100), Ancestor[*DockContainer](b).FrameRect().Width)

	outer.RootDockLayout().SetDividerPosition(700)
	outer.ValidateLayout()
	c.Equal(float32(0), Ancestor[*DockContainer](nested).FrameRect().Width)
}
