// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
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
		return children[d.currentIndex].Self.(Dockable)
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

func (d *dockContainerContent) LayoutSizes(target *Panel, hint geom32.Size) (min, pref, max geom32.Size) {
	for _, c := range d.Children() {
		min2, pref2, max2 := c.AsPanel().Sizes(hint)
		min.Max(min2)
		pref.Max(pref2)
		max.Max(max2)
	}
	if b := d.Border(); b != nil {
		insets := b.Insets()
		min.AddInsets(insets)
		pref.AddInsets(insets)
		max.AddInsets(insets)
	}
	min.GrowToInteger()
	pref.GrowToInteger()
	max.GrowToInteger()
	return min, pref, max
}

func (d *dockContainerContent) PerformLayout(target *Panel) {
	r := d.ContentRect(false)
	for i, c := range d.Children() {
		c.Hidden = i != d.currentIndex
		c.SetFrameRect(r)
	}
}
