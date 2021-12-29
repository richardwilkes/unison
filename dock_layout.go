// Copyright ©2021 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"github.com/richardwilkes/toolbox/xmath/geom32"
	"github.com/richardwilkes/toolbox/xmath/mathf32"
)

var (
	_ Layout         = &DockLayout{}
	_ Layoutable     = &DockLayout{}
	_ DockLayoutNode = &DockLayout{}
)

type DockLayoutNode interface {
	PreferredSize() geom32.Size
	FrameRect() geom32.Rect
	SetFrameRect(r geom32.Rect)
}

type DockLayout struct {
	parent     *DockLayout
	nodes      [2]DockLayoutNode
	frame      geom32.Rect
	Divider    float32
	Horizontal bool
}

func (d *DockLayout) forEachDockContainer(f func(*DockContainer)) {
	for _, node := range d.nodes {
		switch c := node.(type) {
		case *DockContainer:
			f(c)
		case *DockLayout:
			c.forEachDockContainer(f)
		}
	}
}

func (d *DockLayout) FocusedDockContainer() *DockContainer {
	return d.rootLayout().focusedDockContainer()
}

func (d *DockLayout) focusedDockContainer() *DockContainer {
	for _, node := range d.nodes {
		switch c := node.(type) {
		case *DockContainer:
			if c.Active {
				return c
			}
		case *DockLayout:
			if dc := d.focusedDockContainer(); dc != nil {
				return dc
			}
		}
	}
	return nil
}

func (d *DockLayout) rootLayout() *DockLayout {
	root := d
	for root.parent != nil {
		root = root.parent
	}
	return root
}

func (d *DockLayout) Dock() *Dock {
	return d.rootLayout().dock()
}

func (d *DockLayout) dock() *Dock {
	for _, node := range d.nodes {
		switch c := node.(type) {
		case *DockContainer:
			return c.Dock
		case *DockLayout:
			if dock := c.dock(); dock != nil {
				return dock
			}
		}
	}
	return nil
}

// FindLayout returns the DockLayout that contains the specified DockContainer, or nil if it is not present. Note that
// this method will always start at the root and work its way down, even if called on a sub-node.
func (d *DockLayout) FindLayout(dc *DockContainer) *DockLayout {
	return d.rootLayout().findLayout(dc)
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
func (d *DockLayout) DockTo(dc *DockContainer, target DockLayoutNode, side Side) {
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
			p1, p2 := dockOrder(side)
			if targetLayout.nodes[p1] != dc {
				targetLayout.nodes[p2] = targetLayout.nodes[p1]
				targetLayout.nodes[p1] = dc
			}
			targetLayout.Horizontal = side.Horizontal()
			return
		}
		// Not in the same layout. Remove the container from the hierarchy so we can re-add it.
		existingLayout.Remove(dc)
	}
	switch c := target.(type) {
	case *DockLayout:
		c.dockWithin(dc, side)
	case *DockContainer:
		d.FindLayout(c).dockWithContainer(dc, target, side)
	}
}

func (d *DockLayout) dockWithin(dc *DockContainer, side Side) {
	p1, p2 := dockOrder(side)
	if d.nodes[p1] != nil {
		if d.nodes[p2] == nil {
			d.nodes[p2] = d.nodes[p1]
		} else {
			d.nodes[p2] = d.pushDown()
			d.Divider = -1
		}
	}
	d.nodes[p1] = dc
	d.Horizontal = side.Horizontal()
}

func (d *DockLayout) pushDown() *DockLayout {
	layout := &DockLayout{
		parent:     d,
		Divider:    d.Divider,
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

func (d *DockLayout) dockWithContainer(dc *DockContainer, target DockLayoutNode, side Side) {
	p1, p2 := dockOrder(side)
	if d.nodes[p1] != nil {
		if d.nodes[p2] == nil {
			d.nodes[p2] = d.nodes[p1]
			d.nodes[p1] = dc
			d.Horizontal = side.Horizontal()
		} else {
			layout := &DockLayout{
				parent:     d,
				Divider:    -1,
				Horizontal: side.Horizontal(),
			}
			layout.nodes[p1] = dc
			which := p1
			if target != d.nodes[p1] {
				which = p2
			}
			layout.nodes[p2] = d.nodes[which]
			d.nodes[which] = layout
			if which == 0 {
				layout.Divider = d.Divider
				d.Divider = -1
			}
		}
	} else {
		d.nodes[p1] = dc
		d.Horizontal = side.Horizontal()
	}
}

func dockOrder(side Side) (p1, p2 int) {
	if side == TopSide || side == LeftSide {
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
		extent -= DockDividerSize
		if extent < 0 {
			extent = 0
		}
		return extent
	}
	return 0
}

// DividerPosition returns the current divider position.
func (d *DockLayout) DividerPosition() float32 {
	if !d.Full() {
		return 0
	}
	if d.Divider < 0 {
		frame := d.nodes[0].FrameRect()
		if d.Horizontal {
			return frame.Width
		}
		return frame.Height
	}
	return mathf32.Min(d.Divider, d.DividerMaximum())
}

func (d *DockLayout) PreferredSize() geom32.Size {
	_, pref, _ := d.LayoutSizes(nil, geom32.Size{})
	return pref
}

func (d *DockLayout) FrameRect() geom32.Rect {
	return d.frame
}

func (d *DockLayout) SetFrameRect(r geom32.Rect) {
	d.frame = r
	d.PerformLayout(d)
}

// LayoutSizes implements Layout.
func (d *DockLayout) LayoutSizes(_ Layoutable, _ geom32.Size) (min, pref, max geom32.Size) {
	if d.nodes[0] != nil {
		pref = d.nodes[0].PreferredSize()
	}
	if d.nodes[1] != nil {
		pref.Max(d.nodes[1].PreferredSize())
	}
	if d.Full() {
		if d.Horizontal {
			pref.Width *= 2
			pref.Width += DockDividerSize
		} else {
			pref.Height *= 2
			pref.Height += DockDividerSize
		}
	}
	return min, pref, MaxSize(pref)
}

// PerformLayout implements Layout.
func (d *DockLayout) PerformLayout(target Layoutable) {
	var insets geom32.Insets
	if b := target.Border(); b != nil {
		insets = b.Insets()
	}
	d.frame = target.FrameRect()
	size := d.frame.Size
	size.SubtractInsets(insets)
	dock := d.Dock()
	switch {
	case dock != nil && dock.MaximizedContainer != nil:
		d.forEachDockContainer(func(dc *DockContainer) { dc.Hidden = dc != dock.MaximizedContainer })
		dock.MaximizedContainer.AsPanel().SetFrameRect(geom32.NewRect(insets.Left, insets.Top, size.Width, size.Height))
	case d.Full():
		available := size.Height
		if d.Horizontal {
			available = size.Width
		}
		available -= DockDividerSize
		if available < 0 {
			available = 0
		}
		var primary float32
		if d.Divider < 0 {
			primary = available / 2
		} else {
			if d.Divider > available {
				d.Divider = available
			}
			primary = d.Divider
		}
		if d.Horizontal {
			d.nodes[0].SetFrameRect(geom32.NewRect(insets.Left, insets.Top, primary, size.Height))
			d.nodes[1].SetFrameRect(geom32.NewRect(insets.Left+primary+DockDividerSize, insets.Top, available-primary, size.Height))
		} else {
			d.nodes[0].SetFrameRect(geom32.NewRect(insets.Left, insets.Top, size.Width, primary))
			d.nodes[1].SetFrameRect(geom32.NewRect(insets.Left, insets.Top+primary+DockDividerSize, size.Width, available-primary))
		}
	case d.nodes[0] != nil:
		d.nodes[0].SetFrameRect(geom32.NewRect(insets.Left, insets.Top, size.Width, size.Height))
	case d.nodes[1] != nil:
		d.nodes[1].SetFrameRect(geom32.NewRect(insets.Left, insets.Top, size.Width, size.Height))
	}
}

func (d *DockLayout) SetLayout(layout Layout) {
}

func (d *DockLayout) LayoutData() interface{} {
	return nil
}

func (d *DockLayout) SetLayoutData(data interface{}) {
}

func (d *DockLayout) Sizes(hint geom32.Size) (min, pref, max geom32.Size) {
	return d.LayoutSizes(d, hint)
}

func (d *DockLayout) Border() Border {
	return nil
}

func (d *DockLayout) ChildrenForLayout() []Layoutable {
	return nil
}
