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
	"fmt"

	"github.com/richardwilkes/toolbox"
	"github.com/richardwilkes/toolbox/xmath"
)

var _ MenuItem = &menuItem{}

// MenuItem describes a choice that can be made from a Menu.
type MenuItem interface {
	// Factory returns the MenuFactory that created this MenuItem.
	Factory() MenuFactory
	// ID returns the id of this menu item.
	ID() int
	// IsSame returns true if the two items represent the same object. Do not use == to test for equality.
	IsSame(other MenuItem) bool
	// Menu returns the owning menu.
	Menu() Menu
	// Index returns the index of the menu item within its menu. Returns -1 if it is not yet attached to a menu.
	Index() int
	// IsSeparator returns true if this menu item is a separator.
	IsSeparator() bool
	// Title returns the menu item's title.
	Title() string
	// SetTitle sets the menu item's title.
	SetTitle(title string)
	// KeyBinding returns the key binding for the menu item.
	KeyBinding() KeyBinding
	// SetKeyBinding sets the key binding for the menu item.
	SetKeyBinding(keyBinding KeyBinding)
	// SubMenu returns the menu item's sub-menu, if any.
	SubMenu() Menu
	// CheckState returns the menu item's current check state.
	CheckState() CheckState
	// SetCheckState sets the menu item's check state.
	SetCheckState(s CheckState)
}

// DefaultMenuItemTheme holds the default MenuItemTheme values for menu items. Modifying this data will not alter
// existing menu items, but will alter any menu items created in the future.
var DefaultMenuItemTheme = MenuItemTheme{
	TitleFont:         SystemFont,
	KeyFont:           KeyboardFont,
	BackgroundColor:   BackgroundColor,
	OnBackgroundColor: OnBackgroundColor,
	SelectionColor:    SelectionColor,
	OnSelectionColor:  OnSelectionColor,
	ItemBorder:        NewEmptyBorder(StdInsets()),
	SeparatorBorder:   NewEmptyBorder(NewVerticalInsets(4)),
	KeyGap:            16,
}

// MenuItemTheme holds theming data for a menu item.
type MenuItemTheme struct {
	TitleFont         Font
	KeyFont           Font
	BackgroundColor   Ink
	OnBackgroundColor Ink
	SelectionColor    Ink
	OnSelectionColor  Ink
	ItemBorder        Border
	SeparatorBorder   Border
	KeyGap            float32
}

type menuItem struct {
	factory     *inWindowMenuFactory
	id          int
	title       string
	titleCache  TextCache
	keyCache    TextCache
	menu        *menu
	subMenu     *menu
	panel       *Panel
	validator   func(MenuItem) bool
	handler     func(MenuItem)
	keyBinding  KeyBinding
	state       CheckState
	isSeparator bool
	enabled     bool
	over        bool
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

func (mi *menuItem) KeyBinding() KeyBinding {
	return mi.keyBinding
}

func (mi *menuItem) SetKeyBinding(keyBinding KeyBinding) {
	mi.keyBinding = keyBinding
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
	mi.panel = NewPanel()
	if mi.isSeparator {
		mi.panel.SetBorder(DefaultMenuItemTheme.SeparatorBorder)
	} else {
		mi.panel.SetBorder(DefaultMenuItemTheme.ItemBorder)
	}
	mi.over = false
	mi.panel.DrawCallback = mi.paint
	mi.panel.MouseEnterCallback = mi.mouseEnter
	mi.panel.MouseMoveCallback = mi.mouseMove
	mi.panel.MouseExitCallback = mi.mouseExit
	mi.panel.MouseDownCallback = mi.mouseDown
	mi.panel.SetSizer(mi.sizer)
	return mi.panel
}

func (mi *menuItem) mouseDown(_ Point, _, _ int, _ Modifiers) bool {
	if mi.subMenu == nil {
		mi.execute()
		return true
	}
	mi.showSubMenu()
	return true
}

func (mi *menuItem) showSubMenu() {
	if !mi.factory.showInProgress && mi.subMenu.popupPanel == nil {
		mi.factory.showInProgress = true
		defer func() { mi.factory.showInProgress = false }()
		mi.subMenu.createPopup()
		pr := mi.panel.RectToRoot(mi.panel.ContentRect(true))
		fr := mi.subMenu.popupPanel.FrameRect()
		if mi.isRoot() {
			fr.X = pr.X
			fr.Y = pr.Bottom()
		} else {
			fr.X = pr.Right()
			fr.Y = pr.Y
		}
		mi.subMenu.ensureInWindow(fr)
	}
}

func (mi *menuItem) scrollIntoView() {
	mi.panel.ScrollIntoView()
}

func (mi *menuItem) mouseEnter(_ Point, _ Modifiers) bool {
	if mi.menu != nil {
		for _, item := range mi.menu.items {
			if item.over {
				item.over = false
				item.panel.MarkForRedraw()
			}
		}
	}
	mi.over = true
	mi.panel.MarkForRedraw()
	if mi.subMenu != nil && len(mi.panel.Window().root.openMenuPanels) != 0 {
		mi.showSubMenu()
	}
	return false
}

func (mi *menuItem) mouseMove(_ Point, _ Modifiers) bool {
	stopAt := mi.menu
	if mi.subMenu != nil && mi.subMenu.popupPanel != nil {
		stopAt = mi.subMenu
	}
	if w := ActiveWindow(); w != nil {
		mi.menu.closeMenuStackStoppingAt(w, stopAt)
	}
	return false
}

func (mi *menuItem) mouseExit() bool {
	mi.over = false
	mi.panel.MarkForRedraw()
	return false
}

func (mi *menuItem) sizer(hint Size) (min, pref, max Size) {
	if mi.isSeparator {
		pref.Height = 1
	} else {
		pref = LabelSize(mi.titleCache.Text(mi.Title(), DefaultMenuItemTheme.TitleFont), nil, LeftSide, 0)
		if !mi.keyBinding.KeyCode.ShouldOmit() {
			keys := mi.keyBinding.String()
			if keys != "" {
				size := mi.keyCache.Text(keys, DefaultMenuItemTheme.KeyFont).Extents()
				pref.Width += DefaultMenuItemTheme.KeyGap + size.Width
				pref.Height = xmath.Max(pref.Height, size.Height)
			}
		}
	}
	pref.AddInsets(DefaultMenuItemTheme.ItemBorder.Insets())
	pref.GrowToInteger()
	pref.ConstrainForHint(hint)
	return pref, pref, pref
}

func (mi *menuItem) paint(gc *Canvas, rect Rect) {
	var fg, bg Ink
	if !mi.over || !mi.enabled {
		fg = DefaultMenuItemTheme.OnBackgroundColor
		bg = DefaultMenuItemTheme.BackgroundColor
	} else {
		fg = DefaultMenuItemTheme.OnSelectionColor
		bg = DefaultMenuItemTheme.SelectionColor
	}
	gc.DrawRect(rect, bg.Paint(gc, rect, Fill))

	if !mi.enabled {
		fg = &ColorFilteredInk{
			OriginalInk: fg,
			ColorFilter: Grayscale30Filter(),
		}
	}
	rect = mi.panel.ContentRect(false)
	if mi.isSeparator {
		gc.DrawLine(rect.X, rect.Y, rect.Right(), rect.Y, fg.Paint(gc, rect, Fill))
	} else {
		t := mi.titleCache.Text(mi.Title(), DefaultMenuItemTheme.TitleFont)
		t.AdjustDecorations(func(decoration *TextDecoration) { decoration.Foreground = fg })
		size := t.Extents()
		t.Draw(gc, rect.X, xmath.Floor(rect.Y+(rect.Height-size.Height)/2)+t.Baseline())
		if mi.subMenu == nil {
			if !mi.keyBinding.KeyCode.ShouldOmit() {
				keys := mi.keyBinding.String()
				if keys != "" {
					t = mi.keyCache.Text(keys, DefaultMenuItemTheme.KeyFont)
					t.AdjustDecorations(func(decoration *TextDecoration) { decoration.Foreground = fg })
					size = t.Extents()
					t.Draw(gc, xmath.Floor(rect.Right()-size.Width),
						xmath.Floor(rect.Y+(rect.Height-size.Height)/2)+t.Baseline())
				}
			}
		} else if !mi.isRoot() {
			baseline := DefaultMenuItemTheme.KeyFont.Baseline()
			rect.X = rect.Right() - baseline
			rect.Width = baseline
			drawable := &DrawableSVG{
				SVG:  ChevronRightSVG(),
				Size: NewSize(baseline, baseline),
			}
			drawable.DrawInRect(gc, rect, nil, fg.Paint(gc, rect, Fill))
		}
	}
}

func (mi *menuItem) isRoot() bool {
	return mi.menu.popupPanel == nil
}

func (mi *menuItem) validate() {
	if mi.isSeparator {
		return
	}
	if DisableMenus {
		mi.enabled = false
	} else {
		mi.enabled = true
		if mi.validator != nil {
			mi.enabled = false
			toolbox.Call(func() { mi.enabled = mi.validator(mi) })
		}
	}
}

func (mi *menuItem) execute() {
	if mi.isSeparator {
		return
	}
	mi.menu.closeMenuStack()
	if mi.enabled && mi.handler != nil {
		toolbox.Call(func() { mi.handler(mi) })
	}
}
