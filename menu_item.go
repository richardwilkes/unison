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
	"fmt"

	"github.com/richardwilkes/toolbox"
	"github.com/richardwilkes/toolbox/xmath/geom32"
)

var _ MenuItem = &menuItem{}

// MenuItem describes a choice that can be made from a Menu.
type MenuItem interface {
	// Factory returns the MenuFactory that created this MenuItem.
	Factory() MenuFactory
	// ID returns the id of this menuItem.
	ID() int
	// IsSame returns true if the two items represent the same object. Do not use == to test for equality.
	IsSame(other MenuItem) bool
	// Menu returns the owning menu.
	Menu() Menu
	// Index returns the index of the menuItem within its menu. Returns -1 if it is not yet attached to a menu.
	Index() int
	// IsSeparator returns true if this menuItem is a separator.
	IsSeparator() bool
	// Title returns the menuItem's title.
	Title() string
	// SetTitle sets the menuItem's title.
	SetTitle(title string)
	// SubMenu returns the menuItem's sub-menu, if any.
	SubMenu() Menu
	// CheckState returns the menuItem's current check state.
	CheckState() CheckState
	// SetCheckState sets the menuItem's check state.
	SetCheckState(s CheckState)
}

type menuItem struct {
	factory      *inWindowMenuFactory
	id           int
	title        string
	menu         *menu
	subMenu      *menu
	panel        *Panel
	bgInk        Ink
	fgInk        Ink
	validator    func(MenuItem) bool
	handler      func(MenuItem)
	keyCode      KeyCode
	keyModifiers Modifiers
	state        CheckState
	isSeparator  bool
	enabled      bool
}

func (mi *menuItem) Factory() MenuFactory {
	return mi.factory
}

func (mi *menuItem) ID() int {
	return mi.id
}

func (mi *menuItem) IsSame(other MenuItem) bool {
	return mi == other
}

func (mi *menuItem) Menu() Menu {
	return mi.menu
}

func (mi *menuItem) Index() int {
	if mi.menu != nil {
		count := mi.menu.Count()
		for i := 0; i < count; i++ {
			if mi.IsSame(mi.menu.ItemAtIndex(i)) {
				return i
			}
		}
	}
	return -1
}

func (mi *menuItem) IsSeparator() bool {
	return mi.isSeparator
}

func (mi *menuItem) Title() string {
	return mi.title
}

func (mi *menuItem) String() string {
	return fmt.Sprintf("[%d] %s", mi.id, mi.title)
}

func (mi *menuItem) SetTitle(title string) {
	mi.title = title
}

func (mi *menuItem) SubMenu() Menu {
	return mi.subMenu
}

func (mi *menuItem) CheckState() CheckState {
	return mi.state
}

func (mi *menuItem) SetCheckState(s CheckState) {
	mi.state = s
}

func (mi *menuItem) newPanel() *Panel {
	if mi.IsSeparator() {
		sep := NewSeparator()
		sep.SetBorder(NewEmptyBorder(geom32.NewVerticalInsets(4)))
		return sep.AsPanel()
	}

	mi.panel = NewPanel()
	mi.panel.SetBorder(NewEmptyBorder(geom32.Insets{Top: 4, Left: 8, Bottom: 4, Right: 8}))
	mi.adjustColors(false)
	mi.panel.DrawCallback = mi.paint
	mi.panel.MouseEnterCallback = mi.mouseEnter
	mi.panel.MouseExitCallback = mi.mouseExit
	mi.panel.MouseDownCallback = mi.mouseDown

	title := NewLabel()
	title.Text = mi.Title()
	title.Ink = mi.fgInk
	title.Font = SystemFont
	title.MouseEnterCallback = mi.mouseEnter
	title.MouseExitCallback = mi.mouseExit
	title.SetLayoutData(&FlexLayoutData{
		HSpan:  1,
		VSpan:  1,
		VAlign: MiddleAlignment,
		HGrab:  true,
	})
	mi.panel.AddChild(title)

	lay := &FlexLayout{
		Columns:  1,
		HSpacing: StdHSpacing,
		VSpacing: StdVSpacing,
	}
	if mi.keyCode != 0 {
		keyName := KeyCodeToName[mi.keyCode]
		if keyName != "" {
			keyStroke := NewLabel()
			keyStroke.Text = mi.keyModifiers.String() + keyName
			keyStroke.Ink = mi.fgInk
			keyStroke.Font = SmallSystemFont
			keyStroke.MouseEnterCallback = mi.mouseEnter
			keyStroke.MouseExitCallback = mi.mouseExit
			keyStroke.SetLayoutData(&FlexLayoutData{
				HSpan:  1,
				VSpan:  1,
				HAlign: EndAlignment,
				VAlign: MiddleAlignment,
			})
			mi.panel.AddChild(keyStroke)
			lay.Columns = 2
			lay.HSpacing = 10
		}
	}
	mi.panel.SetLayout(lay)
	return mi.panel
}

func (mi *menuItem) mouseDown(_ geom32.Point, _, _ int, _ Modifiers) bool {
	if mi.subMenu == nil {
		mi.execute()
		return true
	}
	mi.showSubMenu()
	return true
}

func (mi *menuItem) showSubMenu() {
	if !mi.factory.showInProgress && mi.subMenu.popup == nil {
		mi.factory.showInProgress = true
		defer func() { mi.factory.showInProgress = false }()
		mi.subMenu.createPopup()
		if mi.subMenu.popup != nil {
			pr := mi.panel.RectToRoot(mi.panel.ContentRect(true))
			pr.Point.Add(mi.panel.Window().ContentRect().Point)
			fr := mi.subMenu.popup.FrameRect()
			fr.X = pr.X
			fr.Y = pr.Bottom()
			mi.subMenu.popup.SetFrameRect(fr)
			mi.subMenu.popup.Show()
		}
	}
}

func (mi *menuItem) mouseEnter(_ geom32.Point, _ Modifiers) bool {
	mi.adjustColors(true)
	mi.panel.MarkForRedraw()
	if mi.subMenu != nil && mi.menu.isActiveWindowShowingPopupMenu() {
		mi.showSubMenu()
	}
	return false
}

func (mi *menuItem) mouseExit() bool {
	mi.adjustColors(false)
	mi.panel.MarkForRedraw()
	return false
}

func (mi *menuItem) paint(gc *Canvas, rect geom32.Rect) {
	gc.DrawRect(rect, mi.bgInk.Paint(gc, rect, Fill))
}

func (mi *menuItem) adjustColors(over bool) {
	switch {
	case !mi.enabled:
		mi.bgInk = BackgroundColor
		mi.fgInk = DividerColor
	case !over:
		mi.bgInk = BackgroundColor
		mi.fgInk = OnBackgroundColor
	default:
		mi.bgInk = SelectionColor
		mi.fgInk = OnSelectionColor
	}
}

func (mi *menuItem) validate() {
	mi.enabled = true
	if mi.validator != nil {
		mi.enabled = false
		toolbox.Call(func() { mi.enabled = mi.validator(mi) })
	}
}

func (mi *menuItem) execute() {
	mi.menu.closeMenuStack()
	if mi.enabled && mi.handler != nil {
		toolbox.Call(func() { mi.handler(mi) })
	}
}
