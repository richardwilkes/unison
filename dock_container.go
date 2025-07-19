// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import "github.com/richardwilkes/toolbox/v2/geom"

var (
	_ Layout         = &DockContainer{}
	_ DockLayoutNode = &DockContainer{}
)

// Dockable represents a dockable Panel.
type Dockable interface {
	Paneler
	// TitleIcon returns an Drawable representing this Dockable.
	TitleIcon(suggestedSize geom.Size) Drawable
	// Title returns the title of this Dockable.
	Title() string
	// Tooltip returns the tooltip of this Dockable.
	Tooltip() string
	// Modified returns true if the dockable has been modified.
	Modified() bool
}

// DockContainer holds one or more Dockable panels.
type DockContainer struct {
	Dock    *Dock
	header  *dockHeader
	content *dockContainerContent
	Panel
}

// NewDockContainer creates a new DockContainer.
func NewDockContainer(dock *Dock, dockable Dockable) *DockContainer {
	d := &DockContainer{
		Dock:    dock,
		content: newDockContainerContent(),
	}
	d.Self = d
	d.SetLayout(d)
	d.content.AddChild(resolveDockable(dockable))
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
	return resolveDockable(d.content.Current())
}

// SetCurrentDockable makes the provided dockable the current one.
func (d *DockContainer) SetCurrentDockable(dockable Dockable) {
	dockable = resolveDockable(dockable)
	if d.CurrentDockable() != dockable {
		for i, c := range d.content.Children() {
			if c.Self == dockable {
				d.content.SetCurrentIndex(i)
				break
			}
		}
	}
	d.AcquireFocus()
}

// resolveDockable makes sure we're pointing to the Self version of the Dockable and not some intermediate layer.
func resolveDockable(dockable Dockable) Dockable {
	if dockable == nil {
		return nil
	}
	if resolved, ok := dockable.AsPanel().Self.(Dockable); ok {
		return resolved
	}
	return nil
}

// AcquireFocus will set the focus within the current Dockable of this DockContainer. If the focus is already within it,
// nothing is changed.
func (d *DockContainer) AcquireFocus() {
	if wnd := d.Window(); wnd != nil {
		current := d.CurrentDockable()
		focus := wnd.Focus()
		for focus != nil && focus.Self != current {
			focus = focus.Parent()
		}
		if focus == nil {
			wnd.SetFocus(current)
		}
	}
}

// UpdateTitle will cause the dock tab for the given Dockable to update itself.
func (d *DockContainer) UpdateTitle(dockable Dockable) {
	dockable = resolveDockable(dockable)
	for i, c := range d.content.Children() {
		if c.Self == dockable {
			d.header.updateTitle(i)
			break
		}
	}
}

// DockableHasFocus returns true if the given Dockable has the current focus inside it.
func DockableHasFocus(dockable Dockable) bool {
	if wnd := dockable.AsPanel().Window(); wnd != nil {
		dockable = resolveDockable(dockable)
		focus := wnd.Focus()
		for focus != nil {
			if d, ok := focus.Self.(Dockable); ok && d == dockable {
				return true
			}
			focus = focus.Parent()
		}
	}
	return false
}

// Stack adds the Dockable to this DockContainer at the specified index. An out-of-bounds index will cause the Dockable
// to be added at the end.
func (d *DockContainer) Stack(dockable Dockable, index int) {
	dockable = resolveDockable(dockable)
	if dc := Ancestor[*DockContainer](dockable); dc != nil {
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

// AttemptCloseAll attempts to close all Dockables within this DockContainer. Returns true if all Dockables are closed.
func (d *DockContainer) AttemptCloseAll() bool {
	return d.AttemptCloseAllExcept(nil)
}

// AttemptCloseAllExcept attempts to close all Dockables within this DockContainer except for the specified Dockable.
// Returns true if all Dockables except for the specified Dockable are closed.
func (d *DockContainer) AttemptCloseAllExcept(dockable Dockable) bool {
	for _, one := range d.Dockables() {
		if one != dockable && !d.AttemptClose(one) {
			return false
		}
	}
	return true
}

// AttemptClose attempts to close a Dockable within this DockContainer. This only has an affect if the Dockable is
// contained by this DockContainer and implements the TabCloser interface. Note that the TabCloser must call this
// DockContainer's Close(Dockable) method to actually close the tab. Returns true if dockable is closed.
func (d *DockContainer) AttemptClose(dockable Dockable) bool {
	if closer, ok := dockable.(TabCloser); ok {
		dockable = resolveDockable(dockable)
		for _, c := range d.content.Children() {
			if c.Self == dockable {
				if closer.MayAttemptClose() {
					return closer.AttemptClose()
				}
				break
			}
		}
	}
	return false
}

// Close the specified Dockable. If the last Dockable within this DockContainer is closed, then this DockContainer is
// also removed from the Dock.
func (d *DockContainer) Close(dockable Dockable) {
	dockable = resolveDockable(dockable)
	children := d.Dockables()
	for i, c := range children {
		if c != dockable {
			continue
		}
		var next Dockable
		if DockableHasFocus(dockable) {
			switch {
			case i+1 < len(children):
				next = children[i+1]
				d.content.SetCurrentIndex(i + 1)
			case i > 0:
				next = children[i-1]
				d.content.SetCurrentIndex(i - 1)
			default:
				next = d.Dock.NextDockableFor(dockable)
			}
		} else {
			next = d.CurrentDockable()
		}
		d.content.RemoveChild(dockable)
		d.header.close(dockable)
		d.MarkForRedraw()
		if len(children) == 1 {
			d.Dock.Restore()
			if dl := d.Dock.layout.findLayout(d); dl != nil {
				dl.Remove(d)
			}
			d.Dock.RemoveChild(d)
			d.Dock.MarkForLayoutAndRedraw()
			d.Dock = nil
		}
		if next != nil {
			if dc := Ancestor[*DockContainer](next); dc != nil {
				if dc == d {
					dc.SetCurrentDockable(next)
				}
				dc.AcquireFocus()
			}
		}
		return
	}
}

// PreferredSize implements DockLayoutNode.
func (d *DockContainer) PreferredSize() geom.Size {
	_, pref, _ := d.LayoutSizes(d.AsPanel(), geom.Size{})
	return pref
}

// LayoutSizes implements Layout.
func (d *DockContainer) LayoutSizes(target *Panel, hint geom.Size) (minSize, prefSize, maxSize geom.Size) {
	minSize, prefSize, maxSize = d.header.Sizes(geom.NewSize(hint.Width, 0))
	minSize.Height = prefSize.Height
	maxSize.Height = prefSize.Height
	min2, pref2, max2 := d.content.Sizes(geom.NewSize(hint.Width, max(hint.Height-prefSize.Height, 0)))
	minSize.Width = min2.Width
	prefSize.Width = pref2.Width
	maxSize.Width = max2.Width
	minSize.Height += min2.Height
	prefSize.Height += pref2.Height
	maxSize.Height += max2.Height
	if b := target.Border(); b != nil {
		prefSize = prefSize.Add(b.Insets().Size())
	}
	return minSize, prefSize, maxSize
}

// PerformLayout implements Layout.
func (d *DockContainer) PerformLayout(_ *Panel) {
	r := d.ContentRect(false)
	_, pref, _ := d.header.Sizes(geom.NewSize(r.Width, 0))
	hr := r
	hr.Height = pref.Height
	d.header.SetFrameRect(hr)
	fr := r
	fr.Y += pref.Height
	fr.Height = max(r.Height-pref.Height, 0)
	d.content.SetFrameRect(fr)
}
