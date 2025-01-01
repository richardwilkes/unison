// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"github.com/richardwilkes/toolbox"
	"github.com/richardwilkes/unison/enums/side"
)

// TODO: Fix scaling for docks, too

var (
	_ Layout         = &DockLayout{}
	_ DockLayoutNode = &DockLayout{}
)

// DockLayoutNode defines the methods for nodes within a DockLayout.
type DockLayoutNode interface {
	PreferredSize() Size
	FrameRect() Rect
	SetFrameRect(r Rect)
}

// DockLayout provides layout of DockContainers and other DockLayouts within a Dock.
type DockLayout struct {
	dock       *Dock
	parent     *DockLayout
	nodes      [2]DockLayoutNode
	frame      Rect
	divider    float32
	Horizontal bool
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
			toolbox.Call(func() { stop = f(c) })
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
			targetLayout.Horizontal = s.Horizontal()
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
			d.divider = -1
		}
	}
	d.nodes[p1] = dc
	d.Horizontal = s.Horizontal()
}

func (d *DockLayout) pushDown() *DockLayout {
	layout := &DockLayout{
		dock:       d.dock,
		parent:     d,
		divider:    d.divider,
		Horizontal: d.Horizontal,
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
			d.Horizontal = s.Horizontal()
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
			if which == 0 {
				layout.divider = d.divider
				d.divider = -1
			}
		}
	} else {
		d.nodes[p1] = dc
		d.Horizontal = s.Horizontal()
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
	switch {
	case node == d.nodes[0]:
		d.nodes[0] = nil
		d.pullUp(d.nodes[1])
		return true
	case node == d.nodes[1]:
		d.nodes[1] = nil
		d.pullUp(d.nodes[0])
		return true
	default:
		for _, n := range d.nodes {
			if dl, ok := n.(*DockLayout); ok {
				if dl.Remove(node) {
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
		var extent float32
		if d.Horizontal {
			extent = d.frame.Width
		} else {
			extent = d.frame.Height
		}
		extent -= d.dock.DockDividerSize()
		if extent < 0 {
			extent = 0
		}
		return extent
	}
	return 0
}

// RawDividerPosition returns the divider position, unadjusted for the current content.
func (d *DockLayout) RawDividerPosition() float32 {
	return d.divider
}

// DividerPosition returns the current divider position.
func (d *DockLayout) DividerPosition() float32 {
	if !d.Full() {
		return 0
	}
	if d.divider < 0 {
		frame := d.nodes[0].FrameRect()
		if d.Horizontal {
			return frame.Width
		}
		return frame.Height
	}
	return min(d.divider, d.DividerMaximum())
}

// SetDividerPosition sets the new divider position. Use a value less than 0 to reset the divider to its default mode,
// which splits the available space evenly between the children.
func (d *DockLayout) SetDividerPosition(pos float32) {
	old := d.divider
	if pos < 0 {
		d.divider = -1
	} else {
		d.divider = pos
	}
	if d.divider != old && d.Full() {
		d.PerformLayout(nil)
		d.dock.MarkForRedraw()
	}
}

// PreferredSize implements DockLayoutNode.
func (d *DockLayout) PreferredSize() Size {
	_, pref, _ := d.LayoutSizes(nil, Size{})
	return pref
}

// FrameRect implements DockLayoutNode.
func (d *DockLayout) FrameRect() Rect {
	return d.frame
}

// SetFrameRect implements DockLayoutNode.
func (d *DockLayout) SetFrameRect(r Rect) {
	d.frame = r
	d.PerformLayout(nil)
}

// LayoutSizes implements Layout.
func (d *DockLayout) LayoutSizes(_ *Panel, _ Size) (minSize, prefSize, maxSize Size) {
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
		d.ForEachDockContainer(func(dc *DockContainer) bool {
			dc.Hidden = dc != d.dock.MaximizedContainer
			return false
		})
		d.dock.MaximizedContainer.AsPanel().SetFrameRect(Rect{Point: d.frame.Point, Size: size})
	case d.Full():
		available := size.Height
		if d.Horizontal {
			available = size.Width
		}
		dividerSize := d.dock.DockDividerSize()
		available -= dividerSize
		if available < 0 {
			available = 0
		}
		var primary float32
		if d.divider < 0 {
			primary = available / 2
		} else {
			if d.divider > available {
				d.divider = available
			}
			primary = d.divider
		}
		if d.Horizontal {
			d.nodes[0].SetFrameRect(Rect{Point: d.frame.Point, Size: Size{Width: primary, Height: size.Height}})
			d.nodes[1].SetFrameRect(Rect{
				Point: Point{X: d.frame.X + primary + dividerSize, Y: d.frame.Y},
				Size:  Size{Width: available - primary, Height: size.Height},
			})
		} else {
			d.nodes[0].SetFrameRect(Rect{Point: d.frame.Point, Size: Size{Width: size.Width, Height: primary}})
			d.nodes[1].SetFrameRect(Rect{
				Point: Point{X: d.frame.X, Y: d.frame.Y + primary + dividerSize},
				Size:  Size{Width: size.Width, Height: available - primary},
			})
		}
	case d.nodes[0] != nil:
		d.nodes[0].SetFrameRect(Rect{Point: d.frame.Point, Size: size})
	case d.nodes[1] != nil:
		d.nodes[1].SetFrameRect(Rect{Point: d.frame.Point, Size: size})
	}
}
