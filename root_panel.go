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
	"github.com/richardwilkes/toolbox/xmath/geom"
)

var _ Layout = &rootPanel{}

type rootPanel struct {
	Panel
	window         *Window
	openMenuPanels []*Panel
	menuBarPanel   *Panel
	tooltipPanel   *Panel
	contentPanel   *Panel
	menuBar        *menu
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
	return p.menuBarPanel
}

func (p *rootPanel) setMenuBar(menuBar *menu) {
	if p.menuBarPanel != nil {
		p.menuBar.closeMenuStackStoppingAt(p.window, nil)
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

func (p *rootPanel) setContent(content Paneler) {
	if p.contentPanel != nil {
		p.RemoveChild(p.contentPanel)
	}
	p.contentPanel = content.AsPanel()
	if content != nil {
		index := len(p.openMenuPanels)
		if p.menuBarPanel != nil {
			index++
		}
		if p.tooltipPanel != nil {
			index++
		}
		p.AddChildAtIndex(content, index)
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

func (p *rootPanel) LayoutSizes(_ *Panel, hint geom.Size[float32]) (min, pref, max geom.Size[float32]) {
	min, pref, max = p.contentPanel.Sizes(hint)
	if p.menuBarPanel != nil {
		_, barSize, _ := p.menuBarPanel.Sizes(geom.Size[float32]{})
		for _, size := range []*geom.Size[float32]{&min, &pref, &max} {
			size.Height += barSize.Height
			if size.Width < barSize.Width {
				size.Width = barSize.Width
			}
		}
	}
	return
}

func (p *rootPanel) PerformLayout(_ *Panel) {
	rect := p.FrameRect()
	rect.X = 0
	rect.Y = 0
	if p.menuBarPanel != nil {
		_, size, _ := p.menuBarPanel.Sizes(geom.Size[float32]{})
		p.menuBarPanel.SetFrameRect(geom.Rect[float32]{Size: geom.Size[float32]{Width: rect.Width, Height: size.Height}})
		rect.Y += size.Height
		rect.Height -= size.Height
	}
	p.contentPanel.SetFrameRect(rect)
}
