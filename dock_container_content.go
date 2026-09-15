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
	"github.com/richardwilkes/unison/enums/role"
)

var _ Layout = &dockContainerContent{}

type dockContainerContent struct {
	Panel
	currentIndex int
}

func newDockContainerContent() *dockContainerContent {
	d := &dockContainerContent{
		currentIndex: -1,
	}
	d.Self = d
	d.SetLayout(d)
	return d
}

func (d *dockContainerContent) Current() Dockable {
	children := d.Children()
	if d.currentIndex >= 0 && d.currentIndex < len(children) {
		if resolved, ok := children[d.currentIndex].Self.(Dockable); ok {
			return resolved
		}
		return nil
	}
	if len(children) != 0 {
		d.SetCurrentIndex(len(children) - 1)
		return d.Current()
	}
	return nil
}

func (d *dockContainerContent) CurrentIndex() int {
	return d.currentIndex
}

func (d *dockContainerContent) SetCurrentIndex(index int) {
	children := d.Children()
	if index >= 0 && index < len(children) {
		d.currentIndex = index
		for i, c := range children {
			c.Hidden = i != index
		}
		d.MarkForRedraw()
		if p := d.Parent(); p != nil {
			if dc, ok := p.Self.(*DockContainer); ok {
				dc.header.MarkForLayoutAndRedraw()
			}
		}
	}
}

func (d *dockContainerContent) LayoutSizes(_ *Panel, hint geom.Size) (minSize, prefSize, maxSize geom.Size) {
	for _, c := range d.Children() {
		min2, pref2, max2 := c.AsPanel().Sizes(hint)
		minSize = minSize.Max(min2)
		prefSize = prefSize.Max(pref2)
		maxSize = maxSize.Max(max2)
	}
	if b := d.Border(); b != nil {
		insets := b.Insets().Size()
		minSize = minSize.Add(insets)
		prefSize = prefSize.Add(insets)
		maxSize = maxSize.Add(insets)
	}
	return minSize.Ceil(), prefSize.Ceil(), maxSize.Ceil()
}

func (d *dockContainerContent) PerformLayout(_ *Panel) {
	r := d.ContentRect(false)
	for i, c := range d.Children() {
		c.Hidden = i != d.currentIndex
		c.SetFrameRect(r)
	}
}

// axCurrent returns the dockable that is showing, or nil if there is none. Unlike Current, it never adjusts the current
// index to bring a stale one back into range: describing a window must not change what it is describing, and the next
// draw heals the index in any case.
func (d *dockContainerContent) axCurrent() Dockable {
	children := d.Children()
	if d.currentIndex < 0 || d.currentIndex >= len(children) {
		return nil
	}
	if resolved, ok := children[d.currentIndex].Self.(Dockable); ok {
		return resolved
	}
	return nil
}

// ProvideAccessibility describes the content area to assistive technologies. It holds every dockable of its container
// but shows only the current one, which is what a tab panel is, and takes its name and description from that dockable.
func (d *dockContainerContent) ProvideAccessibility(b *AccessibilityBuilder) {
	node := b.Node()
	if node.Role == role.Auto {
		node.Role = role.TabPanel
	}
	current := d.axCurrent()
	if current == nil {
		return
	}
	if node.Name == "" {
		node.Name = current.Title()
	}
	if node.Description == "" {
		node.Description = current.Tooltip()
	}
}
