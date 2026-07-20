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
	"slices"

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/enums/mod"
)

var _ Layout = &rootPanel{}

type rootPanel struct {
	window         *Window
	openMenuPanels []*menuPanel
	menuBarPanel   *menuPanel
	tooltipPanel   *Panel
	contentPanel   *Panel
	menuBar        *menu
	Panel
}

func newRootPanel(wnd *Window) *rootPanel {
	p := &rootPanel{}
	p.Self = p
	p.SetLayout(p)
	p.window = wnd
	content := NewPanel()
	content.SetLayout(&FlowLayout{
		HSpacing: StdHSpacing,
		VSpacing: StdVSpacing,
	})
	p.setContent(content)
	return p
}

func (p *rootPanel) MenuBar() *Panel {
	return p.menuBarPanel.AsPanel()
}

func (p *rootPanel) setMenuBar(menuBar *menu) {
	if p.menuBarPanel != nil {
		p.closeMenuStackStoppingAt(nil)
		p.RemoveChild(p.menuBarPanel)
	}
	p.menuBar = menuBar
	if menuBar != nil {
		p.menuBarPanel = menuBar.newPanel(true)
		p.AddChildAtIndex(p.menuBarPanel, 0)
	} else {
		p.menuBarPanel = nil
	}
	p.MarkForLayoutAndRedraw()
}

func (p *rootPanel) insertMenu(panel *menuPanel) {
	p.openMenuPanels = append(p.openMenuPanels, panel)
	p.AddChildAtIndex(panel, 0)
}

func (p *rootPanel) removeMenu(panel *menuPanel) {
	for i, one := range p.openMenuPanels {
		if one != panel {
			continue
		}
		p.openMenuPanels = slices.Delete(p.openMenuPanels, i, i+1)
		panel.RemoveFromParent()
		panel.menu.popupPanel = nil
		p.MarkForRedraw()
		break
	}
}

func (p *rootPanel) setContent(content Paneler) {
	if p.contentPanel != nil {
		p.RemoveChild(p.contentPanel)
	}
	var panel *Panel
	if content != nil {
		panel = content.AsPanel()
	}
	p.contentPanel = panel
	if panel != nil {
		index := len(p.openMenuPanels)
		if p.menuBarPanel != nil {
			index++
		}
		if p.tooltipPanel != nil {
			index++
		}
		p.AddChildAtIndex(panel, index)
	}
	p.NeedsLayout = true
	p.MarkForRedraw()
}

func (p *rootPanel) setTooltip(tip *Panel) {
	if p.tooltipPanel != nil {
		p.tooltipPanel.MarkForRedraw()
		p.RemoveChild(p.tooltipPanel)
	}
	p.tooltipPanel = tip
	if tip != nil {
		index := len(p.openMenuPanels)
		if p.menuBarPanel != nil {
			index++
		}
		p.AddChildAtIndex(tip, index)
		tip.MarkForRedraw()
	}
}

func (p *rootPanel) LayoutSizes(_ *Panel, hint geom.Size) (minSize, prefSize, maxSize geom.Size) {
	if p.contentPanel != nil {
		minSize, prefSize, maxSize = p.contentPanel.Sizes(hint)
	}
	if p.menuBarPanel != nil {
		_, barSize, _ := p.menuBarPanel.Sizes(geom.Size{})
		for _, size := range []*geom.Size{&minSize, &prefSize, &maxSize} {
			size.Height += barSize.Height
			if size.Width < barSize.Width {
				size.Width = barSize.Width
			}
		}
	}
	return minSize, prefSize, maxSize
}

func (p *rootPanel) PerformLayout(_ *Panel) {
	rect := p.FrameRect()
	rect.X = 0
	rect.Y = 0
	if p.menuBarPanel != nil {
		_, size, _ := p.menuBarPanel.Sizes(geom.Size{})
		p.menuBarPanel.SetFrameRect(geom.NewRect(0, 0, rect.Width, size.Height))
		rect.Y += size.Height
		rect.Height -= size.Height
	}
	if p.contentPanel != nil {
		p.contentPanel.SetFrameRect(rect)
	}
}

func (p *rootPanel) preKeyDown(wnd *Window, keyCode KeyCode, mods mod.Modifiers, repeat bool) bool {
	if len(p.openMenuPanels) != 0 {
		var handled bool
		SafeCall(func() { handled = p.openMenuPanels[len(p.openMenuPanels)-1].KeyDownCallback(keyCode, mods, repeat) })
		if handled {
			return true
		}
	}
	if p.menuBar != nil {
		var stop bool
		SafeCall(func() { stop = p.menuBar.preKeyDown(wnd, keyCode, mods, repeat) })
		return stop
	}
	return len(p.openMenuPanels) != 0
}

func (p *rootPanel) preRuneTyped(_ *Window, _ rune) bool {
	return len(p.openMenuPanels) != 0
}

func (p *rootPanel) preKeyUp(_ *Window, _ KeyCode, _ mod.Modifiers) bool {
	return len(p.openMenuPanels) != 0
}

func (p *rootPanel) preMouseDown(_ *Window, where geom.Point) bool {
	// Iterate from the most recently opened popup to the oldest, matching the draw/hit-test z-order (the newest popup
	// is topmost). This ensures that when a child menu overlaps its parent, a click in the overlap is attributed to the
	// child rather than the parent, which would otherwise tear the child back down.
	for i := len(p.openMenuPanels) - 1; i >= 0; i-- {
		one := p.openMenuPanels[i]
		if where.In(one.FrameRect()) {
			SafeCall(func() { p.closeMenuStackStoppingAt(one.menu) })
			return false
		}
	}
	SafeCall(func() { p.closeMenuStackStoppingAt(nil) })
	return false
}

func (p *rootPanel) preMoved(_ *Window) {
	SafeCall(func() { p.closeMenuStackStoppingAt(nil) })
}

func (p *rootPanel) postLostFocus(w *Window) {
	// Need to give the event loop a chance to potentially refocus it before we decide to tear it down.
	InvokeTask(func() {
		if ActiveWindow() != w {
			p.closeMenuStackStoppingAt(nil)
		}
	})
}

func (p *rootPanel) closeMenuStackStoppingAt(stopAt *menu) {
	for i := len(p.openMenuPanels) - 1; i >= 0; i-- {
		if p.openMenuPanels[i].menu == stopAt {
			return
		}
		p.removeMenu(p.openMenuPanels[i])
	}
}
