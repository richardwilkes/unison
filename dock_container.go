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

// Dockable represents a dockable Panel.
type Dockable interface {
	Paneler
	// TitleIcon returns an Drawable representing this Dockable.
	TitleIcon() Drawable
	// Title returns the title of this Dockable.
	Title() string
	// Tooltip returns the tooltip of this Dockable.
	Tooltip() string
	// Modified returns true if the dockable has been modified.
	Modified() bool
}

// DockContainer holds one or more Dockable panels.
type DockContainer struct {
	Panel
	Dock       *Dock
	header     *dockHeader
	content    *dockContainerContent
	Background Ink
}

// NewDockContainer creates a new DockContainer.
func NewDockContainer(dock *Dock, dockable Dockable) *DockContainer {
	d := &DockContainer{
		Dock:    dock,
		content: newDockContainerContent(),
	}
	d.Self = d
	d.SetLayout(d)
	d.content.AddChild(dockable)
	d.content.SetCurrentIndex(0)
	d.header = newDockHeader(d)
	d.AddChild(d.header)
	d.AddChild(d.content)
	return d
}

// Dockables returns the list of Dockables within this DockContainer, in tab order.
func (d *DockContainer) Dockables() []Dockable {
	children := d.content.Children()
	dockables := make([]Dockable, 0, len(children))
	for _, c := range children {
		if dockable, ok := c.Self.(Dockable); ok {
			dockables = append(dockables, dockable)
		}
	}
	return dockables
}

// CurrentDockableIndex returns the index of the frontmost Dockable within this DockContainer, or -1 if there are no
// Dockables.
func (d *DockContainer) CurrentDockableIndex() int {
	return d.content.CurrentIndex()
}

// CurrentDockable returns the frontmost Dockable within this DockContainer. May return nil.
func (d *DockContainer) CurrentDockable() Dockable {
	return d.content.Current()
}

// SetCurrentDockable makes the provided dockable the current one.
func (d *DockContainer) SetCurrentDockable(dockable Dockable) {
	if d.CurrentDockable() != dockable {
		for i, c := range d.content.Children() {
			if c.Self == dockable {
				d.content.SetCurrentIndex(i)
				d.AcquireFocus()
				break
			}
		}
	}
}

// AcquireFocus will set the focus within the current Dockable of this DockContainer. If the focus is already within it,
// nothing is changed.
func (d *DockContainer) AcquireFocus() {
	if wnd := d.Window(); wnd != nil {
		current := d.CurrentDockable()
		focus := wnd.Focus()
		if focus != nil {
			for focus != nil && focus.Self != current {
				focus = focus.Parent()
			}
		}
		if focus == nil {
			wnd.SetFocus(current)
		}
	}
}

// UpdateTitle will cause the dock tab for the given Dockable to update itself.
func (d *DockContainer) UpdateTitle(dockable Dockable) {
	for i, c := range d.content.Children() {
		if c.Self == dockable {
			d.header.updateTitle(i)
			break
		}
	}
}

// FocusedDockContainerFor returns the DockContainer that holds the panel with the current focus in the given Window.
// May return nil.
func FocusedDockContainerFor(wnd *Window) *DockContainer {
	if wnd != nil {
		return DockContainerFor(wnd.Focus())
	}
	return nil
}

// DockContainerFor returns the DockContainer that holds the given Paneler in its hierarchy. May return nil.
func DockContainerFor(paneler Paneler) *DockContainer {
	if paneler != nil {
		p := paneler.AsPanel().Parent()
		for p != nil {
			if dc, ok := p.Self.(*DockContainer); ok {
				return dc
			}
			p = p.Parent()
		}
	}
	return nil
}

// Stack adds the Dockable to this DockContainer at the specified index. An out-of-bounds index will cause the Dockable
// to be added at the end.
func (d *DockContainer) Stack(dockable Dockable, index int) {
	if dc := DockContainerFor(dockable); dc != nil {
		if dc == d && len(d.content.Children()) == 1 {
			d.AcquireFocus()
			return
		}
		dc.Close(dockable)
	}
	d.content.AddChildAtIndex(dockable, index)
	d.header.addTab(dockable, index)
	d.SetCurrentDockable(dockable)
	d.AcquireFocus()
}

// AttemptClose attempts to close a Dockable within this DockContainer. This only has an affect if the Dockable is
// contained by this DockContainer and implements the TabCloser interface. Note that the TabCloser must call this
// DockContainer's close(Dockable) method to actually close the tab.
func (d *DockContainer) AttemptClose(dockable Dockable) {
	if closer, ok := dockable.(TabCloser); ok {
		for _, c := range d.content.Children() {
			if c.Self == dockable {
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
	for i, c := range d.content.Children() {
		if c.Self != dockable {
			continue
		}
		d.content.RemoveChild(dockable)
		d.header.close(dockable)
		children := d.content.Children()
		if len(children) == 0 {
			d.Dock.Restore()
			if dl := d.Dock.layout.findLayout(d); dl != nil {
				dl.Remove(d)
			}
			d.Dock.RemoveChild(d)
			d.Dock.MarkForLayoutAndRedraw()
			d.Dock = nil
		} else {
			if i > 0 {
				i--
			}
			d.SetCurrentDockable(children[i].Self.(Dockable))
		}
		return
	}
}

// PreferredSize implements DockLayoutNode.
func (d *DockContainer) PreferredSize() geom32.Size {
	_, pref, _ := d.LayoutSizes(d, geom32.Size{})
	return pref
}

// LayoutSizes implements Layout.
func (d *DockContainer) LayoutSizes(target Layoutable, hint geom32.Size) (min, pref, max geom32.Size) {
	min, pref, max = d.header.Sizes(geom32.Size{Width: hint.Width})
	min.Height = pref.Height
	max.Height = pref.Height
	min2, pref2, max2 := d.content.Sizes(geom32.Size{
		Width:  hint.Width,
		Height: mathf32.Max(hint.Height-pref.Height, 0),
	})
	min.Width = min2.Width
	pref.Width = pref2.Width
	max.Width = max2.Width
	min.Height += min2.Height
	pref.Height += pref2.Height
	max.Height += max2.Height
	if b := target.Border(); b != nil {
		pref.AddInsets(b.Insets())
	}
	return min, pref, max
}

// PerformLayout implements Layout.
func (d *DockContainer) PerformLayout(target Layoutable) {
	r := d.ContentRect(false)
	_, pref, _ := d.header.Sizes(geom32.Size{Width: r.Width})
	d.header.SetFrameRect(geom32.NewRect(r.X, r.Y, r.Width, pref.Height))
	d.content.SetFrameRect(geom32.NewRect(r.X, r.Y+pref.Height, r.Width, mathf32.Max(r.Height-pref.Height, 0)))
}
