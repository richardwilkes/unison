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
		focus := wnd.CurrentFocus()
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
		focus := wnd.CurrentFocus()
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
// to be added at the end. A Dockable that is already in a Dock is moved, and when it held the keyboard focus and stays
// in the same window, the panel within it that held the focus keeps it.
func (d *DockContainer) Stack(dockable Dockable, index int) {
	dockable = resolveDockable(dockable)
	if existing := d.content.IndexOfChild(dockable); existing != -1 && existing < index {
		index--
	}
	if dc := Ancestor[*DockContainer](dockable); dc != nil {
		if dc == d && len(d.content.Children()) == 1 {
			d.AcquireFocus()
			return
		}
		dc.remove(dockable, movingWithinWindow(dc, d))
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
// also removed from the Dock. When the Dockable held the keyboard focus, the focus goes to the tab that becomes current
// in its place, or is released when there is none, so that the window never goes on naming a panel that is no longer
// in it.
func (d *DockContainer) Close(dockable Dockable) {
	d.remove(dockable, false)
}

// remove takes the Dockable out of this DockContainer, as Close describes. It is also how Stack and Dock.DockTo take a
// Dockable out of one place in a window to put it back at another, which is what moving reports. The focus is then
// left alone: the window's focus names a panel that is briefly outside the window, and is within it again once the
// caller has re-added the Dockable, which happens before anything can observe the gap. The person keeps their place in
// the tab that way, and nothing is told it lost or gained the focus over a move that took it from no one. Only a move
// that stays within one window may claim this; see movingWithinWindow.
func (d *DockContainer) remove(dockable Dockable, moving bool) {
	dockable = resolveDockable(dockable)
	children := d.Dockables()
	for i, c := range children {
		if c != dockable {
			continue
		}
		hadFocus := !moving && DockableHasFocus(dockable)
		wnd := d.Window() // Looked up now, since the removal below may take this container out of the window.
		next := d.CurrentDockable()
		if next == dockable {
			// The tab being closed is the current one, so a neighbor must become current instead.
			switch {
			case i+1 < len(children):
				next = children[i+1]
			case i > 0:
				next = children[i-1]
			default:
				next = d.Dock.NextDockableFor(dockable)
			}
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
					// Reestablish the current index now that the removal has shifted the remaining children.
					for j, c2 := range d.content.Children() {
						if c2.Self == next {
							d.content.SetCurrentIndex(j)
							break
						}
					}
				}
				if hadFocus {
					dc.AcquireFocus()
				}
			}
		}
		if hadFocus && wnd.CurrentFocus() == nil {
			// No other tab took the focus the closed one held -- it was the last tab in its Dock, say -- so the
			// window's focus still names a panel that is no longer in it. Let it go, so that the panel is told it lost
			// the focus, the window knows nothing holds it and says so to an assistive technology, and the next Tab, or
			// the application, can place it afresh.
			wnd.SetFocus(nil)
		}
		return
	}
}

// movingWithinWindow reports whether a Dockable taken from one place and put back at another stays in the same window
// throughout, which is when remove may leave the focus where it is; see DockContainer.remove. A destination that is in
// no window yet cannot be that, since the focus is a window's, and the window the Dockable leaves must then let it go
// as it would for a close.
func movingWithinWindow(from, to Paneler) bool {
	wnd := from.AsPanel().Window()
	return wnd != nil && wnd == to.AsPanel().Window()
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

// ProvideAccessibility describes the container to assistive technologies. It is the group that holds a set of tabs and
// the content of whichever one is current, named after that dockable so that an assistive technology moving through the
// window can say which part of it this is.
func (d *DockContainer) ProvideAccessibility(b *AccessibilityBuilder) {
	node := b.Node()
	if node.Role == role.Auto {
		node.Role = role.Group
	}
	if node.Name == "" {
		if current := d.content.axCurrent(); current != nil {
			node.Name = current.Title()
		}
	}
}
