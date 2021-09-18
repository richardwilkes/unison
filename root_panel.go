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
)

var _ Layout = &rootPanel{}

type rootPanel struct {
	Panel
	window                *Window
	menubar               *Panel
	preMovedCallback      func(*Window)
	postLostFocusCallback func(*Window)
	preMouseDownCallback  func(*Window, geom32.Point) bool
	preKeyDownCallback    func(*Window, KeyCode, Modifiers) bool
	preKeyUpCallback      func(*Window, KeyCode, Modifiers) bool
	preRuneTypedCallback  func(*Window, rune) bool
	content               *Panel
	tooltip               *Panel
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

// MenuBar fulfills the menu.barHolder interface.
func (p *rootPanel) MenuBar() *Panel {
	return p.menubar
}

// SetMenuBar fulfills the menu.barHolder interface.
func (p *rootPanel) SetMenuBar(bar *Panel, preMovedCallback, postLostFocusCallback func(*Window),
	preMouseDownCallback func(*Window, geom32.Point) bool,
	preKeyDownCallback, preKeyUpCallback func(*Window, KeyCode, Modifiers) bool,
	preRuneTypedCallback func(*Window, rune) bool) {
	if p.menubar != nil {
		p.RemoveChild(p.menubar)
	}
	if bar != nil {
		p.menubar = bar.AsPanel()
	} else {
		p.menubar = nil
	}
	p.preMovedCallback = preMovedCallback
	p.postLostFocusCallback = postLostFocusCallback
	p.preMouseDownCallback = preMouseDownCallback
	p.preKeyDownCallback = preKeyDownCallback
	p.preKeyUpCallback = preKeyUpCallback
	p.preRuneTypedCallback = preRuneTypedCallback
	if bar != nil {
		index := 0
		if p.tooltip != nil {
			index++
		}
		p.AddChildAtIndex(bar, index)
	}
	p.NeedsLayout = true
	p.MarkForRedraw()
}

func (p *rootPanel) setContent(content Paneler) {
	if p.content != nil {
		p.RemoveChild(p.content)
	}
	p.content = content.AsPanel()
	if content != nil {
		index := 0
		if p.tooltip != nil {
			index++
		}
		if p.menubar != nil {
			index++
		}
		p.AddChildAtIndex(content, index)
	}
	p.NeedsLayout = true
	p.MarkForRedraw()
}

func (p *rootPanel) setTooltip(tip *Panel) {
	if p.tooltip != nil {
		p.tooltip.MarkForRedraw()
		p.RemoveChild(p.tooltip)
	}
	p.tooltip = tip
	if tip != nil {
		p.AddChildAtIndex(tip, 0)
		tip.MarkForRedraw()
	}
}

func (p *rootPanel) LayoutSizes(_ Layoutable, hint geom32.Size) (min, pref, max geom32.Size) {
	min, pref, max = p.content.Sizes(hint)
	if p.menubar != nil {
		_, barSize, _ := p.menubar.Sizes(geom32.Size{})
		for _, size := range []*geom32.Size{&min, &pref, &max} {
			size.Height += barSize.Height
			if size.Width < barSize.Width {
				size.Width = barSize.Width
			}
		}
	}
	return
}

func (p *rootPanel) PerformLayout(_ Layoutable) {
	rect := p.FrameRect()
	rect.X = 0
	rect.Y = 0
	if p.menubar != nil {
		_, size, _ := p.menubar.Sizes(geom32.Size{})
		p.menubar.SetFrameRect(geom32.Rect{Size: geom32.Size{Width: rect.Width, Height: size.Height}})
		rect.Y += size.Height
		rect.Height -= size.Height
	}
	p.content.SetFrameRect(rect)
}
