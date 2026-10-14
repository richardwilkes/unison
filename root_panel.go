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
	if p.menuBarPanel == nil {
		return nil
	}
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

// insertMenu records panel, the content of a popup menu window this root's window owns, as the newest open menu.
func (p *rootPanel) insertMenu(panel *menuPanel) {
	p.openMenuPanels = append(p.openMenuPanels, panel)
	if len(p.openMenuPanels) == 1 && p.window.IsValid() {
		p.window.apiMenuStackChanged(true)
	}
}

// removeMenu takes panel off the open menu stack and disposes of the popup window showing it. A panel that is not the
// content of a popup window is merely detached from its parent.
func (p *rootPanel) removeMenu(panel *menuPanel) {
	i := slices.Index(p.openMenuPanels, panel)
	if i < 0 {
		return
	}
	p.openMenuPanels = slices.Delete(p.openMenuPanels, i, i+1)
	panel.menu.popupPanel = nil
	if wnd := panel.Window(); wnd.isPopup() && wnd.IsValid() {
		wnd.Dispose()
	} else {
		panel.RemoveFromParent()
	}
	if len(p.openMenuPanels) == 0 && p.window.IsValid() {
		p.window.apiMenuStackChanged(false)
	}
	p.MarkForRedraw()
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
		index := 0
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
		index := 0
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

// preMouseDown closes the menus a press makes obsolete. A press in a popup menu window closes the menus opened after
// the one it shows, except that one on an item whose sub-menu is open leaves that sub-menu up, since the item would
// only open it again. A press that reached any other window landed outside every menu, so it closes them all, except
// that one on a title of the menu bar is left to the title, which closes its own menu or opens another; closing here
// would have the menu torn down and built again. The press is never consumed, so it still reaches the panel under it.
func (p *rootPanel) preMouseDown(wnd *Window, where geom.Point) bool {
	if wnd.isPopup() {
		if p.contentPanel == nil || wnd.owner.root == nil {
			return false
		}
		if mp, ok := p.contentPanel.Self.(*menuPanel); ok {
			stopAt := mp.menu
			if mi := p.menuItemAt(where); mi != nil && mi.subMenu != nil && mi.subMenu.popupPanel != nil {
				stopAt = mi.subMenu
			}
			SafeCall(func() { wnd.owner.root.closeMenuStackStoppingAt(stopAt) })
		}
		return false
	}
	if mi := p.menuItemAt(where); mi != nil && mi.subMenu != nil && mi.menu == p.menuBar {
		return false
	}
	SafeCall(func() { p.closeMenuStackStoppingAt(nil) })
	return false
}

// menuItemAt returns the menu item drawn at where, in root coordinates, or nil if there is none there.
func (p *rootPanel) menuItemAt(where geom.Point) *menuItem {
	if mip, ok := p.PanelAt(where).Self.(*menuItemPanel); ok {
		return mip.item
	}
	return nil
}

// preMoved closes the open menus when their owner moves. A popup menu window moves only when it is placed, which must
// leave its owner's menus alone.
func (p *rootPanel) preMoved(wnd *Window) {
	if wnd.isPopup() {
		return
	}
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

// closeMenuStackStoppingAt closes the newest open menus until it reaches stopAt, which stays open, or closes them all
// when stopAt is nil or not open. The top is read again on every pass, since disposing of a popup window synthesizes
// pointer crossings that can open or close menus in turn.
func (p *rootPanel) closeMenuStackStoppingAt(stopAt *menu) {
	for n := len(p.openMenuPanels); n != 0; n = len(p.openMenuPanels) {
		top := p.openMenuPanels[n-1]
		if top.menu == stopAt {
			return
		}
		p.removeMenu(top)
	}
}
