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
	"strconv"

	"github.com/richardwilkes/toolbox/i18n"
	"github.com/richardwilkes/unison/enums/paintstyle"
)

var _ Layout = &dockHeader{}

// DefaultDockHeaderTheme holds the default DockHeaderTheme values for DockHeaders. Modifying this data will not alter
// existing DockHeaders, but will alter any DockHeaders created in the future.
var DefaultDockHeaderTheme = DockHeaderTheme{
	BackgroundInk: &PrimaryTheme.Surface,
	DropAreaInk:   &PrimaryTheme.Warning,
	HeaderBorder: NewCompoundBorder(
		NewLineBorder(&PrimaryTheme.Outline, 0, Insets{Bottom: 1}, false),
		NewEmptyBorder(NewHorizontalInsets(4)),
	),
	MinimumTabWidth: 50,
	TabGap:          4,
	TabInsertSize:   3,
}

// DockHeaderTheme holds theming data for a DockHeader.
type DockHeaderTheme struct {
	BackgroundInk   Ink
	DropAreaInk     Ink
	HeaderBorder    Border
	MinimumTabWidth float32
	TabGap          float32
	TabInsertSize   float32
}

type dockHeader struct {
	Panel
	DockHeaderTheme
	owner                 *DockContainer
	overflowButton        *Button
	maximizeRestoreButton *Button
	dragInsertIndex       int
}

func newDockHeader(dc *DockContainer) *dockHeader {
	d := &dockHeader{
		DockHeaderTheme:       DefaultDockHeaderTheme,
		owner:                 dc,
		overflowButton:        createDockHeaderButton(),
		maximizeRestoreButton: createDockHeaderButton(),
		dragInsertIndex:       -1,
	}
	d.Self = d
	d.DrawCallback = d.DefaultDraw
	d.DataDragOverCallback = d.DefaultDataDragOver
	d.DataDragExitCallback = d.DefaultDataDragExit
	d.DataDragDropCallback = d.DefaultDataDrop
	d.SetBorder(d.DockHeaderTheme.HeaderBorder)
	d.SetLayout(d)
	for _, dockable := range dc.Dockables() {
		d.AddChild(newDockTab(dockable))
	}
	d.overflowButton.ClickCallback = d.handleOverflowPopup
	d.AddChild(d.overflowButton)
	d.AddChild(d.maximizeRestoreButton)
	d.adjustToRestoredState()
	return d
}

func (d *dockHeader) DefaultDraw(gc *Canvas, rect Rect) {
	gc.DrawRect(rect, d.BackgroundInk.Paint(gc, rect, paintstyle.Fill))
	if d.dragInsertIndex >= 0 {
		r := d.ContentRect(false)
		r.Width = d.TabInsertSize
		tabs, _ := d.partition()
		switch {
		case d.dragInsertIndex < len(tabs):
			r.X = tabs[d.dragInsertIndex].FrameRect().X - ((d.TabGap-d.TabInsertSize)/2 + d.TabInsertSize + 1)
		default:
			r.X = tabs[len(tabs)-1].FrameRect().Right()
		}
		gc.DrawRect(r, d.DropAreaInk.Paint(gc, rect, paintstyle.Fill))
	}
}

func (d *dockHeader) DefaultDataDragOver(where Point, data map[string]any) bool {
	return d.dragOver(where, data) != nil
}

func (d *dockHeader) dragOver(where Point, data map[string]any) Dockable {
	d.dragInsertIndex = -1
	if dockable := DockableFromDragData(d.owner.Dock.DragKey, data); dockable != nil {
		tabs, _ := d.partition()
		d.dragInsertIndex = len(tabs)
		for i, one := range tabs {
			r := one.FrameRect()
			if where.X < r.CenterX() {
				d.dragInsertIndex = i
				break
			}
			if where.X < r.Right() {
				d.dragInsertIndex = i + 1
				break
			}
		}
		return dockable
	}
	return nil
}

func (d *dockHeader) DefaultDataDragExit() {
	d.dragInsertIndex = -1
}

func (d *dockHeader) DefaultDataDrop(where Point, data map[string]any) {
	if dockable := d.dragOver(where, data); dockable != nil {
		d.owner.Stack(dockable, d.dragInsertIndex)
	}
	d.dragInsertIndex = -1
}

func (d *dockHeader) updateTitle(index int) {
	if index >= 0 {
		if tabs, _ := d.partition(); index < len(tabs) {
			tabs[index].updateTitle()
		}
	}
}

func (d *dockHeader) addTab(dockable Dockable, index int) {
	tabs, _ := d.partition()
	if index < 0 || index >= len(tabs) {
		index = len(tabs)
	}
	d.AddChildAtIndex(newDockTab(dockable), index)
	d.MarkForLayoutAndRedraw()
}

func (d *dockHeader) partition() (tabs []*dockTab, buttons []*Panel) {
	children := d.Children()
	tabs = make([]*dockTab, 0, len(children))
	buttons = make([]*Panel, 0, len(children))
	for _, c := range children {
		if dt, ok := c.Self.(*dockTab); ok {
			tabs = append(tabs, dt)
		} else {
			buttons = append(buttons, c)
		}
	}
	return tabs, buttons
}

func (d *dockHeader) LayoutSizes(target *Panel, _ Size) (minSize, prefSize, maxSize Size) {
	tabs, buttons := d.partition()
	for i, dt := range tabs {
		_, size, _ := dt.Sizes(Size{})
		prefSize.Width += max(size.Width, d.MinimumTabWidth)
		prefSize.Height = max(prefSize.Height, size.Height)
		if i == 0 {
			minSize.Width += size.Width
		}
	}
	for _, b := range buttons {
		if b.Self != d.overflowButton {
			_, size, _ := b.Sizes(Size{})
			prefSize.Width += size.Width
			prefSize.Height = max(prefSize.Height, size.Height)
			minSize.Width += size.Width
		}
	}
	gaps := float32(len(tabs)+len(buttons)-2) * d.TabGap
	minSize.Width += gaps
	prefSize.Width += gaps
	minSize.Height = prefSize.Height
	if b := target.Border(); b != nil {
		insets := b.Insets().Size()
		minSize = minSize.Add(insets)
		prefSize = prefSize.Add(insets)
	}
	return minSize, prefSize, MaxSize(prefSize)
}

func (d *dockHeader) PerformLayout(_ *Panel) {
	contentRect := d.ContentRect(false)
	tabs, buttons := d.partition()
	tabSizes := make([]Size, len(tabs))
	extra := contentRect.Width
	for i, dt := range tabs {
		_, tabSizes[i], _ = dt.Sizes(Size{})
		tabSizes[i].Width = max(tabSizes[i].Width, d.MinimumTabWidth)
		extra -= tabSizes[i].Width
	}
	buttonSizes := make([]Size, len(buttons))
	overflowIndex := -1
	for i, b := range buttons {
		_, buttonSizes[i], _ = b.Sizes(Size{})
		if b.Self == d.overflowButton {
			overflowIndex = i
		} else {
			extra -= buttonSizes[i].Width
		}
	}
	hidden := make(map[*dockTab]bool)
	extra -= float32(len(tabs)+len(buttons)-2) * d.TabGap
	if extra < 0 {
		// Shrink the non-current tabs down
		current := d.owner.CurrentDockableIndex()
		remaining := -extra
		found := true
		for found && remaining > 0 {
			fatTabs := 0
			found = false
			for i := range tabs {
				if i != current && tabSizes[i].Width > d.MinimumTabWidth {
					fatTabs++
				}
			}
			if fatTabs > 0 {
				perTab := max(remaining/float32(fatTabs), 1)
				for i := range tabs {
					if i != current && tabSizes[i].Width > d.MinimumTabWidth {
						found = true
						remaining -= perTab
						tabSizes[i].Width -= perTab
						if tabSizes[i].Width < d.MinimumTabWidth {
							remaining += d.MinimumTabWidth - tabSizes[i].Width
							tabSizes[i].Width = d.MinimumTabWidth
						}
					}
					if remaining <= 0 {
						break
					}
				}
			}
		}
		if remaining > 0 {
			// Still not small enough... add the overflow button and start trimming out tabs
			if len(tabs) > 1 {
				remaining += buttonSizes[overflowIndex].Width + d.TabGap
				for i := len(tabs) - 1; i >= 0 && remaining > 0; i-- {
					if i == current {
						continue
					}
					remaining -= buttonSizes[overflowIndex].Width
					hidden[tabs[i]] = true
					d.overflowButton.Text = "»" + strconv.Itoa(len(hidden))
					_, buttonSizes[overflowIndex], _ = d.overflowButton.Sizes(Size{})
					remaining += buttonSizes[overflowIndex].Width
					remaining -= tabSizes[i].Width + d.TabGap
				}
			}
			if remaining > 0 {
				// STILL not small enough... reduce the size of the current tab, too
				tabSizes[current].Width = max(tabSizes[current].Width-remaining, d.MinimumTabWidth)
				remaining = 0
			}
			extra = -remaining
		} else {
			extra = 0
		}
	}
	x := contentRect.X
	for i, dt := range tabs {
		if hidden[dt] {
			dt.Hidden = true
		} else {
			dt.Hidden = false
			dt.SetFrameRect(Rect{
				Point: Point{X: x, Y: contentRect.Y + (contentRect.Height-tabSizes[i].Height)/2},
				Size:  tabSizes[i],
			}.Align())
			x += tabSizes[i].Width + d.TabGap
		}
	}
	x += extra
	for i, b := range buttons {
		if b.Self == d.overflowButton && len(hidden) == 0 {
			b.Hidden = true
		} else {
			b.Hidden = false
			b.SetFrameRect(Rect{
				Point: Point{X: x, Y: contentRect.Y + (contentRect.Height-buttonSizes[i].Height)/2},
				Size:  buttonSizes[i],
			}.Align())
			x += buttonSizes[i].Width + d.TabGap
		}
	}
}

func (d *dockHeader) close(dockable Dockable) {
	for i, c := range d.Children() {
		if dt, ok := c.Self.(*dockTab); ok && dockable == dt.dockable {
			d.RemoveChildAtIndex(i)
			break
		}
	}
}

func createDockHeaderButton() *Button {
	b := NewButton()
	b.HideBase = true
	b.SetFocusable(false)
	return b
}

func (d *dockHeader) adjustToMaximizedState() {
	d.maximizeRestoreButton.ClickCallback = func() { d.owner.Dock.Restore() }
	fSize := d.maximizeRestoreButton.ButtonTheme.Font.Baseline()
	d.maximizeRestoreButton.Drawable = &DrawableSVG{
		SVG:  WindowRestoreSVG,
		Size: Size{Width: fSize, Height: fSize},
	}
	d.maximizeRestoreButton.Tooltip = NewTooltipWithText(i18n.Text("Restore"))
}

func (d *dockHeader) adjustToRestoredState() {
	d.maximizeRestoreButton.ClickCallback = func() { d.owner.Dock.Maximize(d.owner) }
	fSize := d.maximizeRestoreButton.ButtonTheme.Font.Baseline()
	d.maximizeRestoreButton.Drawable = &DrawableSVG{
		SVG:  WindowMaximizeSVG,
		Size: Size{Width: fSize, Height: fSize},
	}
	d.maximizeRestoreButton.Tooltip = NewTooltipWithText(i18n.Text("Maximize"))
}

func (d *dockHeader) handleOverflowPopup() {
	tabs, _ := d.partition()
	m := DefaultMenuFactory().NewMenu(PopupMenuTemporaryBaseID, "", nil)
	defer m.Dispose()
	for i, tab := range tabs {
		if tab.Hidden {
			m.InsertItem(-1, m.Factory().NewItem(PopupMenuTemporaryBaseID+i+1, tab.dockable.Title(), KeyBinding{}, nil, func(item MenuItem) {
				d.owner.SetCurrentDockable(tabs[item.ID()-(PopupMenuTemporaryBaseID+1)].dockable)
			}))
		}
	}
	m.Popup(d.overflowButton.RectToRoot(d.overflowButton.ContentRect(true)), 0)
}
