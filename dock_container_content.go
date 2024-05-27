// Copyright (c) 2021-2024 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

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
		return children[d.currentIndex].Self.(Dockable)
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

func (d *dockContainerContent) LayoutSizes(_ *Panel, hint Size) (minSize, prefSize, maxSize Size) {
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
