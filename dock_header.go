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
	"strconv"

	"github.com/richardwilkes/toolbox/i18n"
	"github.com/richardwilkes/toolbox/xmath/geom32"
	"github.com/richardwilkes/toolbox/xmath/mathf32"
)

var _ Layout = &dockHeader{}

const (
	MinimumTabWidth = 30
	TabGap          = 4
)

type dockHeader struct {
	Panel
	owner                 *DockContainer
	showTabsButton        *Button
	maximizeRestoreButton *Button
	hidden                map[*dockTab]bool
}

func newDockHeader(dc *DockContainer) *dockHeader {
	d := &dockHeader{
		owner:                 dc,
		showTabsButton:        NewButton(),
		maximizeRestoreButton: NewButton(),
		hidden:                make(map[*dockTab]bool),
	}
	d.Self = d
	d.DrawCallback = d.DefaultDraw
	d.SetBorder(NewCompoundBorder(NewLineBorder(DividerColor, 0, geom32.Insets{Bottom: 1}, false),
		NewEmptyBorder(geom32.NewHorizontalInsets(TabGap))))
	d.SetLayout(d)
	for _, dockable := range dc.Dockables() {
		d.AddChild(newDockTab(dockable))
	}
	d.showTabsButton.Text = "»"
	d.showTabsButton.HideBase = true
	d.showTabsButton.SetFocusable(false)
	d.maximizeRestoreButton.HideBase = true
	d.maximizeRestoreButton.SetFocusable(false)
	d.AddChild(d.showTabsButton)
	d.adjustToRestoredState()
	d.AddChild(d.maximizeRestoreButton)
	return d
}

func (d *dockHeader) DefaultDraw(gc *Canvas, rect geom32.Rect) {
	gc.DrawRect(rect, BackgroundColor.Paint(gc, rect, Fill))
}

func (d *dockHeader) updateTitle(index int) {
	if index >= 0 {
		if children := d.Children(); index < len(children) {
			if dt, ok := children[index].Self.(*dockTab); ok {
				dt.updateTitle()
			}
		}
	}
}

func (d *dockHeader) addTab(dockable Dockable, index int) {
	d.AddChildAtIndex(newDockTab(dockable), index)
	d.MarkForRedraw()
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

func (d *dockHeader) LayoutSizes(target Layoutable, hint geom32.Size) (min, pref, max geom32.Size) {
	tabs, buttons := d.partition()
	for i, dt := range tabs {
		_, size, _ := dt.Sizes(geom32.Size{})
		pref.Width += mathf32.Max(size.Width, MinimumTabWidth)
		pref.Height = mathf32.Max(pref.Height, size.Height)
		if i == 0 {
			min.Width += size.Width
		}
	}
	for _, b := range buttons {
		if b.Self != d.showTabsButton {
			_, size, _ := b.Sizes(geom32.Size{})
			pref.Width += size.Width
			pref.Height = mathf32.Max(pref.Height, size.Height)
			min.Width += size.Width
		}
	}
	gaps := float32((len(tabs) + len(buttons) - 2) * TabGap)
	min.Width += gaps
	pref.Width += gaps
	min.Height = pref.Height
	if b := target.Border(); b != nil {
		insets := b.Insets()
		min.AddInsets(insets)
		pref.AddInsets(insets)
	}
	return min, pref, MaxSize(pref)
}

func (d *dockHeader) PerformLayout(target Layoutable) {
	d.hidden = make(map[*dockTab]bool)
	contentRect := d.ContentRect(false)
	tabs, buttons := d.partition()
	tabSizes := make([]geom32.Size, len(tabs))
	extra := contentRect.Width
	for i, dt := range tabs {
		_, tabSizes[i], _ = dt.Sizes(geom32.Size{})
		tabSizes[i].Width = mathf32.Max(tabSizes[i].Width, MinimumTabWidth)
		extra -= tabSizes[i].Width
	}
	buttonSizes := make([]geom32.Size, len(buttons))
	showTabsIndex := -1
	for i, b := range buttons {
		_, buttonSizes[i], _ = b.Sizes(geom32.Size{})
		if b.Self == d.showTabsButton {
			showTabsIndex = i
		} else {
			extra -= buttonSizes[i].Width
		}
	}
	extra -= float32((len(tabs) + len(buttons) - 2) * TabGap)
	if extra < 0 {
		// Shrink the non-current tabs down
		current := d.owner.CurrentDockableIndex()
		remaining := -extra
		found := true
		for found && remaining > 0 {
			fatTabs := 0
			found = false
			for i := range tabs {
				if i != current && tabSizes[i].Width > MinimumTabWidth {
					fatTabs++
				}
			}
			if fatTabs > 0 {
				perTab := mathf32.Max(remaining/float32(fatTabs), 1)
				for i := range tabs {
					if i != current && tabSizes[i].Width > MinimumTabWidth {
						found = true
						remaining -= perTab
						tabSizes[i].Width -= perTab
						if tabSizes[i].Width < MinimumTabWidth {
							remaining += MinimumTabWidth - tabSizes[i].Width
							tabSizes[i].Width = MinimumTabWidth
						}
					}
					if remaining <= 0 {
						break
					}
				}
			}
		}
		if remaining > 0 {
			// Still not small enough... add the show button and start trimming out tabs
			if len(tabs) > 1 {
				remaining += buttonSizes[showTabsIndex].Width + TabGap
				for i := len(tabs) - 1; i >= 0 && remaining > 0; i-- {
					if i != current {
						remaining -= buttonSizes[showTabsIndex].Width
						d.hidden[tabs[i]] = true
						d.showTabsButton.Text = "»" + strconv.Itoa(len(d.hidden))
						d.MarkForRedraw()
						_, buttonSizes[showTabsIndex], _ = d.showTabsButton.Sizes(geom32.Size{})
						remaining += buttonSizes[showTabsIndex].Width
						remaining -= tabSizes[i].Width + TabGap
					}
				}
			}
			if remaining > 0 {
				// STILL not small enough... reduce the size of the current tab, too
				tabSizes[current].Width = mathf32.Max(tabSizes[current].Width-remaining, MinimumTabWidth)
				remaining = 0
			}
			extra = -remaining
		} else {
			extra = 0
		}
	}
	x := contentRect.X
	for i, dt := range tabs {
		if d.hidden[dt] {
			dt.SetEnabled(false)
			dt.frame.X = -32000
			dt.frame.Y = -32000
		} else {
			dt.SetEnabled(true)
			r := geom32.NewRect(x, contentRect.Y+(contentRect.Height-tabSizes[i].Height)/2, tabSizes[i].Width, tabSizes[i].Height)
			r.Align()
			dt.SetFrameRect(r)
			x += tabSizes[i].Width + TabGap
		}
	}
	x += extra
	for i, b := range buttons {
		if b.Self == d.showTabsButton && len(d.hidden) == 0 {
			b.SetEnabled(false)
			b.frame.X = -32000
			b.frame.Y = -32000
		} else {
			b.SetEnabled(true)
			r := geom32.NewRect(x, contentRect.Y+(contentRect.Height-buttonSizes[i].Height)/2, buttonSizes[i].Width, buttonSizes[i].Height)
			r.Align()
			b.SetFrameRect(r)
			x += buttonSizes[i].Width + TabGap
		}
	}
}

func (d *dockHeader) adjustToMaximizedState() {
	d.maximizeRestoreButton.ClickCallback = d.owner.Restore
	fSize := ChooseFont(d.showTabsButton.Font, LabelFont).Baseline()
	d.maximizeRestoreButton.Drawable = &DrawableSVG{
		SVG:  WindowRestoreSVG(),
		Size: geom32.Size{Width: fSize, Height: fSize},
	}
	d.maximizeRestoreButton.Tooltip = NewTooltipWithText(i18n.Text("Restore"))
}

func (d *dockHeader) adjustToRestoredState() {
	d.maximizeRestoreButton.ClickCallback = d.owner.Maximize
	fSize := ChooseFont(d.showTabsButton.Font, LabelFont).Baseline()
	d.maximizeRestoreButton.Drawable = &DrawableSVG{
		SVG:  WindowMaximizeSVG(),
		Size: geom32.Size{Width: fSize, Height: fSize},
	}
	d.maximizeRestoreButton.Tooltip = NewTooltipWithText(i18n.Text("Maximize"))
}

func (d *dockHeader) close(dockable Dockable) {
	for i, c := range d.Children() {
		if dt, ok := c.Self.(*dockTab); ok && dockable == dt.dockable {
			d.RemoveChildAtIndex(i)
			break
		}
	}
}
