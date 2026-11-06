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
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/enums/side"
)

var (
	_ Layout         = &DockLayout{}
	_ DockLayoutNode = &DockLayout{}
)

// DockLayoutNode defines the methods for nodes within a DockLayout.
type DockLayoutNode interface {
	PreferredSize() geom.Size
	FrameRect() geom.Rect
	SetFrameRect(r geom.Rect)
}

// DockLayout provides layout of DockContainers and other DockLayouts within a Dock.
type DockLayout struct {
	dock    *Dock
	parent  *DockLayout
	nodes   [2]DockLayoutNode
	frame   geom.Rect
	divider float32
	// dividerExtent is the extent the two nodes had to share when the divider was positioned: 0 if that is unknown, or
	// less than 0 if the divider was set by hand and a layout has yet to give it.
	dividerExtent float32
	// rescaleExtent, when not 0, is the extent the two nodes had to share when the Dock was rearranged, for the next
	// layout to scale the divider from it to the extent they have by then.
	rescaleExtent float32
	Horizontal    bool
	// belowMinimum records which nodes the divider was set to leave less than their minimum.
	belowMinimum [2]bool
}

// ForEachDockContainer iterates through all DockContainers in this DockLayout's hierarchy and calls the given function
// with them. The function should return true to stop further processing.
func (d *DockLayout) ForEachDockContainer(f func(*DockContainer) bool) {
	d.forEachDockContainer(f)
}

func (d *DockLayout) forEachDockContainer(f func(*DockContainer) bool) bool {
	for _, node := range d.nodes {
		switch c := node.(type) {
		case *DockContainer:
			stop := false
			SafeCall(func() { stop = f(c) })
			if stop {
				return true
			}
		case *DockLayout:
			if c.forEachDockContainer(f) {
				return true
			}
		}
	}
	return false
}

// RootLayout returns the topmost parent DockLayout.
func (d *DockLayout) RootLayout() *DockLayout {
	root := d
	for root.parent != nil {
		root = root.parent
	}
	return root
}

// FindLayout returns the DockLayout that contains the specified DockContainer, or nil if it is not present. Note that
// this method will always start at the root and work its way down, even if called on a sub-node.
func (d *DockLayout) FindLayout(dc *DockContainer) *DockLayout {
	return d.RootLayout().findLayout(dc)
}

func (d *DockLayout) findLayout(dc *DockContainer) *DockLayout {
	for _, node := range d.nodes {
		switch c := node.(type) {
		case *DockContainer:
			if c == dc {
				return d
			}
		case *DockLayout:
			if dl := c.findLayout(dc); dl != nil {
				return dl
			}
		}
	}
	return nil
}

// Contains returns true if the node is this DockLayout or one of its descendants.
func (d *DockLayout) Contains(node DockLayoutNode) bool {
	if d == node {
		return true
	}
	for _, n := range d.nodes {
		switch c := n.(type) {
		case *DockContainer:
			if c == node {
				return true
			}
		case *DockLayout:
			if c.Contains(node) {
				return true
			}
		}
	}
	return false
}

// DockTo docks a DockContainer within this DockLayout. If the DockContainer already exists in this DockLayout, it will
// be moved to the new location.
func (d *DockLayout) DockTo(dc *DockContainer, target DockLayoutNode, s side.Enum) {
	d.RootLayout().prepareToRescale()
	// Does the container already exist in our hierarchy?
	if existingLayout := d.FindLayout(dc); existingLayout != nil {
		// Yes. Is it the same layout?
		var targetLayout *DockLayout
		switch c := target.(type) {
		case *DockLayout:
			targetLayout = c
		case *DockContainer:
			targetLayout = d.FindLayout(c)
		default:
			targetLayout = nil
		}
		if targetLayout == existingLayout {
			// Yes. Reposition the target within this layout.
			p1, p2 := dockOrder(s)
			if targetLayout.nodes[p1] != dc {
				targetLayout.nodes[p2] = targetLayout.nodes[p1]
				targetLayout.nodes[p1] = dc
			}
			targetLayout.setHorizontal(s.Horizontal())
			return
		}
		// Not in the same layout. Remove the container from the hierarchy so we can re-add it.
		existingLayout.Remove(dc)
	}
	switch c := target.(type) {
	case *DockLayout:
		c.dockWithin(dc, s)
	case *DockContainer:
		d.FindLayout(c).dockWithContainer(dc, target, s)
	}
}

func (d *DockLayout) dockWithin(dc *DockContainer, s side.Enum) {
	p1, p2 := dockOrder(s)
	if d.nodes[p1] != nil {
		if d.nodes[p2] == nil {
			d.nodes[p2] = d.nodes[p1]
		} else {
			d.nodes[p2] = d.pushDown()
			d.resetDivider()
		}
	}
	d.nodes[p1] = dc
	d.setHorizontal(s.Horizontal())
}

func (d *DockLayout) pushDown() *DockLayout {
	layout := &DockLayout{
		dock:          d.dock,
		parent:        d,
		divider:       d.divider,
		dividerExtent: d.dividerExtent,
		rescaleExtent: d.rescaleExtent,
		Horizontal:    d.Horizontal,
		belowMinimum:  d.belowMinimum,
	}
	for i, n := range d.nodes {
		if dl, ok := n.(*DockLayout); ok {
			dl.parent = layout
		}
		layout.nodes[i] = n
	}
	return layout
}

func (d *DockLayout) dockWithContainer(dc *DockContainer, target DockLayoutNode, s side.Enum) {
	p1, p2 := dockOrder(s)
	if d.nodes[p1] != nil {
		if d.nodes[p2] == nil {
			d.nodes[p2] = d.nodes[p1]
			d.nodes[p1] = dc
			d.setHorizontal(s.Horizontal())
		} else {
			layout := &DockLayout{
				dock:       d.dock,
				parent:     d,
				divider:    -1,
				Horizontal: s.Horizontal(),
			}
			layout.nodes[p1] = dc
			which := p1
			if target != d.nodes[p1] {
				which = p2
			}
			layout.nodes[p2] = d.nodes[which]
			d.nodes[which] = layout
		}
	} else {
		d.nodes[p1] = dc
		d.setHorizontal(s.Horizontal())
	}
}

// setHorizontal sets the orientation, dropping a divider position that was set for the other one.
func (d *DockLayout) setHorizontal(horizontal bool) {
	if d.Horizontal != horizontal {
		d.Horizontal = horizontal
		d.resetDivider()
	}
}

func (d *DockLayout) resetDivider() {
	d.divider = -1
	d.dividerExtent = 0
	d.rescaleExtent = 0
	d.belowMinimum = [2]bool{}
}

// prepareToRescale has each layout from this one down note the extent its nodes share, ahead of a rearrangement of the
// Dock. A layout that then gives them another extent scales the divider to it, which keeps the proportions of the two
// nodes rather than a position that may no longer leave room for both.
func (d *DockLayout) prepareToRescale() {
	if d.divider >= 0 && d.rescaleExtent == 0 {
		available := d.DividerMaximum()
		if d.dividerExtent < 0 && available > 0 {
			// A divider set by hand that awaits layout is kept as given within the extent its nodes share now.
			d.belowMinimum = d.belowMinimumWithin(available)
			d.dividerExtent = available
		}
		if d.dividerExtent > 0 {
			d.rescaleExtent = available
		}
	}
	for _, node := range d.nodes {
		if dl, ok := node.(*DockLayout); ok {
			dl.prepareToRescale()
		}
	}
}

func dockOrder(s side.Enum) (p1, p2 int) {
	if s == side.Top || s == side.Left {
		return 0, 1
	}
	return 1, 0
}

// Remove a node. Returns true if the node was found and removed.
func (d *DockLayout) Remove(node DockLayoutNode) bool {
	d.RootLayout().prepareToRescale()
	return d.remove(node)
}

func (d *DockLayout) remove(node DockLayoutNode) bool {
	switch node {
	case d.nodes[0]:
		d.nodes[0] = nil
		d.resetDivider()
		d.pullUp(d.nodes[1])
		return true
	case d.nodes[1]:
		d.nodes[1] = nil
		d.resetDivider()
		d.pullUp(d.nodes[0])
		return true
	default:
		for _, n := range d.nodes {
			if dl, ok := n.(*DockLayout); ok {
				if dl.remove(node) {
					return true
				}
			}
		}
		return false
	}
}

func (d *DockLayout) pullUp(node DockLayoutNode) {
	if d.parent != nil {
		for i, n := range d.parent.nodes {
			if n == d {
				d.parent.nodes[i] = node
				break
			}
		}
		if node == nil {
			if d.parent.Empty() {
				d.parent.pullUp(nil)
			}
		} else if dl, ok := node.(*DockLayout); ok {
			dl.parent = d.parent
		}
	}
}

// Empty returns true if this DockLayout has no children.
func (d *DockLayout) Empty() bool {
	return d.nodes[0] == nil && d.nodes[1] == nil
}

// Full returns true if both child nodes of this DockLayout are occupied.
func (d *DockLayout) Full() bool {
	return d.nodes[0] != nil && d.nodes[1] != nil
}

// DividerMaximum returns the maximum value the divider can be set to. Will always return 0 if Full() returns false.
func (d *DockLayout) DividerMaximum() float32 {
	if d.Full() {
		return max(d.extent(d.frame.Size)-d.dock.DockDividerSize(), 0)
	}
	return 0
}

// extent returns the part of the size that runs across the divider.
func (d *DockLayout) extent(size geom.Size) float32 {
	if d.Horizontal {
		return size.Width
	}
	return size.Height
}

// RawDividerPosition returns the divider position, unadjusted for the current content.
func (d *DockLayout) RawDividerPosition() float32 {
	return d.divider
}

// DividerPosition returns the current divider position, which is where the layout puts the divider when that differs
// from where it was set.
func (d *DockLayout) DividerPosition() float32 {
	if !d.Full() {
		return 0
	}
	if d.divider < 0 {
		return d.extent(d.nodes[0].FrameRect().Size)
	}
	return d.dividerPositionFor(d.DividerMaximum())
}

// dividerPositionFor returns where the divider goes when the two nodes have the given extent to share. That is where
// it was positioned, moved as far as it takes to leave each node its minimum. A node that the divider was set to leave
// less than its minimum keeps the share it was left as its minimum instead, so that a divider can be dragged up against
// another, which a layout never does while it has the room not to. Without the room, the nodes share the extent in
// proportion to their minimums.
func (d *DockLayout) dividerPositionFor(available float32) float32 {
	floor0 := d.extent(dockNodeMinimumSize(d.nodes[0]))
	floor1 := d.extent(dockNodeMinimumSize(d.nodes[1]))
	pos := available / 2
	if d.divider >= 0 {
		pos = d.divider
		extent, below := d.dividerExtent, d.belowMinimum
		if extent <= 0 {
			extent, below = available, d.belowMinimumWithin(available)
		}
		if below[0] {
			floor0 = min(floor0, pos)
		}
		if below[1] {
			floor1 = min(floor1, max(extent-pos, 0))
		}
	}
	if total := floor0 + floor1; total > available {
		return available * floor0 / total
	}
	return min(max(pos, floor0), available-floor1)
}

// belowMinimumWithin returns which nodes the divider leaves less than their minimum when they share the given extent.
// The second node's share is not known for a divider of unknown origin, so it is never counted as below its minimum.
func (d *DockLayout) belowMinimumWithin(extent float32) [2]bool {
	return [2]bool{
		d.divider < d.extent(dockNodeMinimumSize(d.nodes[0])),
		d.dividerExtent < 0 && max(extent-d.divider, 0) < d.extent(dockNodeMinimumSize(d.nodes[1])),
	}
}

// minimumSize returns the smallest size at which each node within this layout still gets its own minimum.
func (d *DockLayout) minimumSize() geom.Size {
	min0 := dockNodeMinimumSize(d.nodes[0])
	min1 := dockNodeMinimumSize(d.nodes[1])
	var dividerSize float32
	if d.Full() {
		dividerSize = d.dock.DockDividerSize()
	}
	if d.Horizontal {
		return geom.NewSize(min0.Width+dividerSize+min1.Width, max(min0.Height, min1.Height))
	}
	return geom.NewSize(max(min0.Width, min1.Width), min0.Height+dividerSize+min1.Height)
}

// dockNodeMinimumSize returns the minimum size of a node, which is none for a node that has no say in it.
func dockNodeMinimumSize(node DockLayoutNode) geom.Size {
	if n, ok := node.(interface{ minimumSize() geom.Size }); ok {
		return n.minimumSize()
	}
	return geom.Size{}
}

// SetDividerPosition sets the new divider position. Use a value less than 0 to reset the divider to its default mode,
// which splits the available space evenly between the children, as far as their minimum sizes allow. A position that
// leaves a child less than its minimum size, or no room at all, is kept as given.
func (d *DockLayout) SetDividerPosition(pos float32) {
	d.resetDivider()
	if pos >= 0 {
		d.divider = pos
		d.dividerExtent = -1
	}
	if d.Full() {
		if d.dock.NeedsLayout {
			// This layout's frame may be stale, so bring every frame up to date.
			d.RootLayout().PerformLayout(nil)
		} else {
			d.PerformLayout(nil)
		}
	}
	d.dock.MarkForRedraw()
}

// layoutPendingAbove returns true if a panel above the Dock awaits layout, which may yet change the extents within it.
func (d *DockLayout) layoutPendingAbove() bool {
	for p := d.dock.Parent(); p != nil; p = p.Parent() {
		if p.NeedsLayout {
			return true
		}
	}
	return false
}

// PreferredSize implements DockLayoutNode.
func (d *DockLayout) PreferredSize() geom.Size {
	_, pref, _ := d.LayoutSizes(nil, geom.Size{})
	return pref
}

// FrameRect implements DockLayoutNode.
func (d *DockLayout) FrameRect() geom.Rect {
	return d.frame
}

// SetFrameRect implements DockLayoutNode.
func (d *DockLayout) SetFrameRect(r geom.Rect) {
	d.frame = r
	d.PerformLayout(nil)
}

// LayoutSizes implements Layout.
func (d *DockLayout) LayoutSizes(_ *Panel, _ geom.Size) (minSize, prefSize, maxSize geom.Size) {
	if d.nodes[0] != nil {
		prefSize = d.nodes[0].PreferredSize()
	}
	if d.nodes[1] != nil {
		prefSize = prefSize.Max(d.nodes[1].PreferredSize())
	}
	if d.Full() {
		if d.Horizontal {
			prefSize.Width *= 2
			prefSize.Width += d.dock.DockDividerSize()
		} else {
			prefSize.Height *= 2
			prefSize.Height += d.dock.DockDividerSize()
		}
	}
	return minSize, prefSize, MaxSize(prefSize)
}

// PerformLayout implements Layout.
func (d *DockLayout) PerformLayout(_ *Panel) {
	if d.parent == nil {
		d.frame = d.dock.ContentRect(false)
	}
	size := d.frame.Size
	switch {
	case d.dock.MaximizedContainer != nil:
		// The maximized container fills the Dock, so only the root layout places it.
		if d.parent == nil {
			d.ForEachDockContainer(func(dc *DockContainer) bool {
				dc.Hidden = dc != d.dock.MaximizedContainer
				return false
			})
			d.dock.MaximizedContainer.AsPanel().SetFrameRect(geom.NewRect(d.frame.X, d.frame.Y, size.Width,
				size.Height))
		}
	case d.Full():
		dividerSize := d.dock.DockDividerSize()
		available := max(d.extent(size)-dividerSize, 0)
		if d.divider >= 0 && available > 0 {
			switch {
			case d.rescaleExtent > 0:
				if available != d.rescaleExtent {
					d.divider = d.dividerPositionFor(d.rescaleExtent) * available / d.rescaleExtent
					d.dividerExtent = available
				}
				d.rescaleExtent = 0
			case d.dividerExtent < 0 && d.layoutPendingAbove():
				// A divider set by hand waits for the extent the panel above gives the Dock, so lay the Dock out again
				// after that panel, even if it leaves the Dock's size alone.
				d.dock.NeedsLayout = true
				d.dock.layoutAgain = true
			case d.dividerExtent <= 0:
				d.belowMinimum = d.belowMinimumWithin(available)
				d.dividerExtent = available
			}
		}
		primary := d.dividerPositionFor(available)
		if d.Horizontal {
			d.nodes[0].SetFrameRect(geom.NewRect(d.frame.X, d.frame.Y, primary, size.Height))
			d.nodes[1].SetFrameRect(geom.NewRect(d.frame.X+primary+dividerSize, d.frame.Y, available-primary,
				size.Height))
		} else {
			d.nodes[0].SetFrameRect(geom.NewRect(d.frame.X, d.frame.Y, size.Width, primary))
			d.nodes[1].SetFrameRect(geom.NewRect(d.frame.X, d.frame.Y+primary+dividerSize, size.Width,
				available-primary))
		}
	case d.nodes[0] != nil:
		d.nodes[0].SetFrameRect(geom.NewRect(d.frame.X, d.frame.Y, size.Width, size.Height))
	case d.nodes[1] != nil:
		d.nodes[1].SetFrameRect(geom.NewRect(d.frame.X, d.frame.Y, size.Width, size.Height))
	}
}
