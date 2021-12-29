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
	_ Layout         = &DockContainer{}
	_ DockLayoutNode = &DockContainer{}
)

type TabCloser interface {
	MayAttemptClose() bool
	AttemptClose()
}

// Dockable represents a dockable Panel.
type Dockable interface {
	Paneler
	// TitleIcon returns an Drawable representing this Dockable.
	TitleIcon() Drawable
	// Title returns the title of this Dockable.
	Title() string
	// Tooltip returns the tooltip of this Dockable.
	Tooltip() string
	// Activated is called when this Dockable is made active within a DockContainer. This can be called many times in a
	// row without other Dockables receiving a call in between.
	Activated()
}

// DockContainer holds one or more Dockable panels.
type DockContainer struct {
	Panel
	Dock          *Dock
	header        *dockHeader
	DockablesList []Dockable
	current       Dockable
	Background    Ink
	Active        bool
}

func NewDockContainer(dock *Dock, dockable Dockable) *DockContainer {
	d := &DockContainer{
		Dock:          dock,
		DockablesList: []Dockable{dockable},
		current:       dockable,
	}
	d.Self = d
	d.SetLayout(d)
	d.AddChild(dockable)
	d.header = newDockHeader(d)
	d.AddChildAtIndex(d.header, 0)
	return d
}

func (d *DockContainer) Dockables() []Dockable {
	children := d.Children()
	dockables := make([]Dockable, 0, len(children))
	for _, c := range children {
		if dockable, ok := c.Self.(Dockable); ok {
			dockables = append(dockables, dockable)
		}
	}
	return dockables
}

func (d *DockContainer) CurrentDockableIndex() int {
	for i, c := range d.DockablesList {
		if c == d.current {
			return i
		}
	}
	return -1
}

func (d *DockContainer) CurrentDockable() Dockable {
	return d.current
}

// SetCurrentDockable makes the provided dockable the current one.
func (d *DockContainer) SetCurrentDockable(dockable Dockable) {
	if dockable != d.current {
		for _, c := range d.DockablesList {
			if c == dockable {
				if d.current != nil {
					d.RemoveChild(d.current)
				}
				d.current = dockable
				d.AddChild(d.current)
				d.MarkForLayoutAndRedraw()
				d.AcquireFocus()
				if d.Active {
					dockable.Activated()
				}
				break
			}
		}
	}
}

func (d *DockContainer) AcquireFocus() {
	if wnd := d.Window(); wnd != nil {
		if focus := wnd.Focus(); focus != nil {
			current := d.current.AsPanel()
			for focus != nil && focus != current {
				focus = focus.Parent()
			}
			if focus == nil {
				wnd.SetFocus(current)
			}
		}
	}
}

func (d *DockContainer) UpdateTitle(dockable Dockable) {
	for i, c := range d.DockablesList {
		if c == dockable {
			d.header.updateTitle(i)
			break
		}
	}
}

func DockContainerFor(dockable Dockable) *DockContainer {
	p := dockable.AsPanel().Parent()
	for p != nil {
		if dc, ok := p.Self.(*DockContainer); ok {
			return dc
		}
		p = p.Parent()
	}
	return nil
}

func (d *DockContainer) Stack(dockable Dockable, index int) {
	if dc := DockContainerFor(dockable); dc != nil {
		if dc == d && len(d.DockablesList) == 1 {
			d.AcquireFocus()
			return
		}
		dc.Close(dockable)
	}
	if index < 0 || index >= len(d.DockablesList) {
		d.DockablesList = append(d.DockablesList, dockable)
		index = len(d.DockablesList) - 1
	} else {
		d.DockablesList = append(d.DockablesList, nil)
		copy(d.DockablesList[index+1:], d.DockablesList[index:])
		d.DockablesList[index] = dockable
	}
	d.header.addTab(dockable, index)
	d.SetCurrentDockable(dockable)
	d.AcquireFocus()
}

// AttemptClose attempts to close a Dockable within this DockContainer. This only has an affect if the Dockable is
// contained by this DockContainer and implements the TabCloser interface. Note that the TabCloser must call this
// DockContainer's close(Dockable) method to actually close the tab.
func (d *DockContainer) AttemptClose(dockable Dockable) {
	if closer, ok := dockable.(TabCloser); ok {
		for _, c := range d.DockablesList {
			if c == dockable {
				if closer.MayAttemptClose() {
					closer.AttemptClose()
				}
				break
			}
		}
	}
}

// Close the specified Dockable. If the last Dockable within this DockContainer is closed, then this DockContainer is
// also removed from the Dock.
func (d *DockContainer) Close(dockable Dockable) {
	for i, c := range d.DockablesList {
		if c == dockable {
			d.RemoveChild(dockable)
			copy(d.DockablesList[i:], d.DockablesList[i+1:])
			d.DockablesList[len(d.DockablesList)-1] = nil
			d.DockablesList = d.DockablesList[:len(d.DockablesList)-1]
			d.header.close(dockable)
			if len(d.DockablesList) == 0 {
				d.Dock.Restore()
				d.Dock.RemoveChild(d)
				d.Dock.MarkForLayoutAndRedraw()
				d.Dock = nil
			} else {
				if i > 0 {
					i--
				}
				d.SetCurrentDockable(d.DockablesList[i])
			}
			break
		}
	}
}

func (d *DockContainer) PreferredSize() geom32.Size {
	_, pref, _ := d.LayoutSizes(d, geom32.Size{})
	return pref
}

func (d *DockContainer) LayoutSizes(target Layoutable, hint geom32.Size) (min, pref, max geom32.Size) {
	min, pref, max = d.header.Sizes(geom32.Size{Width: hint.Width})
	min.Height = pref.Height
	max.Height = pref.Height
	if d.current != nil {
		min2, pref2, max2 := d.current.AsPanel().Sizes(geom32.Size{
			Width:  hint.Width,
			Height: mathf32.Max(hint.Height-pref.Height, 0),
		})
		min.Width = mathf32.Max(min.Width, min2.Width)
		pref.Width = mathf32.Max(pref.Width, pref2.Width)
		max.Width = mathf32.Max(max.Width, max2.Width)
		min.Height += min2.Height
		pref.Height += pref2.Height
		max.Height += max2.Height
	}
	if b := target.Border(); b != nil {
		pref.AddInsets(b.Insets())
	}
	return min, pref, max
}

func (d *DockContainer) PerformLayout(target Layoutable) {
	var insets geom32.Insets
	if b := target.Border(); b != nil {
		insets = b.Insets()
	}
	size := target.FrameRect().Size
	size.SubtractInsets(insets)
	_, pref, _ := d.header.Sizes(geom32.Size{Width: size.Width})
	d.header.SetFrameRect(geom32.NewRect(insets.Left, insets.Top, size.Width, pref.Height))
	if d.current != nil {
		d.current.AsPanel().SetFrameRect(geom32.NewRect(insets.Left, insets.Top+pref.Height, size.Width, mathf32.Max(size.Height-pref.Height, 0)))
	}
}

func (d *DockContainer) Maximize() {
	d.Dock.Maximize(d)
}

func (d *DockContainer) Restore() {
	d.Dock.Restore()
}
