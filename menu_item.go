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
	"fmt"

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xmath"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/check"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/enums/paintstyle"
	"github.com/richardwilkes/unison/enums/role"
	"github.com/richardwilkes/unison/enums/side"
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
	CheckState() check.Enum
	// SetCheckState sets the menu item's check state.
	SetCheckState(s check.Enum)
}

// DefaultMenuItemTheme holds the default MenuItemTheme values for menu items. Modifying this data will not alter
// existing menu items, but will alter any menu items created in the future.
var DefaultMenuItemTheme = MenuItemTheme{
	TitleFont:         SystemFont,
	KeyFont:           KeyboardFont,
	BackgroundColor:   ThemeSurface,
	OnBackgroundColor: ThemeOnSurface,
	SelectionColor:    ThemeFocus,
	OnSelectionColor:  ThemeOnFocus,
	ItemBorder:        NewEmptyBorder(StdInsets()),
	SeparatorBorder:   NewEmptyBorder(geom.NewVerticalInsets(4)),
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
	menu        *menu
	subMenu     *menu
	panel       *Panel
	validator   func(MenuItem) bool
	handler     func(MenuItem)
	title       string
	id          int
	keyBinding  KeyBinding
	state       check.Enum
	isSeparator bool
	enabled     bool
	over        bool
	checkable   bool
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
	if mi.menu == nil {
		// Return a true nil rather than a typed-nil *menu wrapped in the interface, which would pass != nil checks
		// and then crash on the first method call. This also matches the behavior of the macOS implementation.
		return nil
	}
	return mi.menu
}

func (mi *menuItem) Index() int {
	if mi.menu != nil {
		count := mi.menu.Count()
		for i := range count {
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

func (mi *menuItem) CheckState() check.Enum {
	return mi.state
}

func (mi *menuItem) SetCheckState(s check.Enum) {
	mi.state = s
	// Remembered so that an item that has been unchecked is still described as something with a check state. Nothing
	// about the item itself says it is checkable, since an unchecked item draws exactly as a plain command does.
	mi.checkable = true
}

func (mi *menuItem) newPanel() *Panel {
	mi.panel = NewPanel()
	if mi.isSeparator {
		mi.panel.SetBorder(DefaultMenuItemTheme.SeparatorBorder)
		mi.panel.Accessibility.Role = role.Separator
	} else {
		mi.panel.SetBorder(DefaultMenuItemTheme.ItemBorder)
		mi.panel.Accessibility.Role = role.MenuItem
		// Everything else about the item — its title, its key binding, whether it is enabled, checked, or the item the
		// menu is pointing at — is read when a description is actually being built, since all of it can change while
		// the menu is open. Both of these only ever run then.
		mi.panel.Accessibility.Callback = mi.describeForAccessibility
		mi.panel.Accessibility.ActionCallback = mi.performAccessibilityAction
	}
	mi.over = false
	mi.panel.DrawCallback = mi.paint
	mi.panel.MouseEnterCallback = mi.mouseEnter
	mi.panel.MouseMoveCallback = mi.mouseMove
	mi.panel.MouseExitCallback = mi.mouseExit
	if !mi.isSeparator {
		// A separator is a line drawn between the things that can be chosen rather than one of them: execute does
		// nothing for one, so a click over it does nothing either. It goes without the two halves of a click, since a
		// panel that handles both is described as something that can be pressed, and such a press would be reported as
		// carried out while nothing had happened. The remaining mouse callbacks stay, so that the pointer passing over
		// a separator behaves as it does everywhere else in the menu.
		mi.panel.MouseDownCallback = mi.mouseDown
		mi.panel.MouseUpCallback = mi.mouseUp
	}
	mi.panel.SetSizer(mi.sizer)
	return mi.panel
}

// describeForAccessibility fills in what an assistive technology is told about the item. Whether this is the item a
// person is choosing from is not settled here: an item is highlighted by the pointer merely passing over it, which on
// the menu bar happens with no menu open at all, so the snapshot picks the one item that stands for the focus and marks
// it, along with the focusable that has to accompany it. See axSnapshot.openMenuFocus.
//
// A check state is reported for an item that has ever been given one, whether or not it is showing a mark now. An item
// that has been unchecked draws nothing to tell it from an ordinary command, but it is still a thing with two states,
// and an assistive technology that stopped saying so the moment it was turned off would leave the person who had just
// turned it off with nothing to hear and no way back.
func (mi *menuItem) describeForAccessibility(node *accessibility.Node) {
	node.Name = mi.title
	node.Disabled = !mi.enabled
	node.Actions = node.Actions.With(accessibility.Press)
	if mi.keyBinding.KeyCode != 0 {
		node.Shortcut = mi.keyBinding.String()
	}
	if mi.subMenu != nil {
		node.Expandable = true
		node.Expanded = mi.subMenu.popupPanel != nil
		node.Actions = node.Actions.With(accessibility.Expand, accessibility.Collapse)
	} else if mi.checkable {
		node.HasCheck = true
		node.Checked = mi.state
	}
}

// performAccessibilityAction carries out a request from an assistive technology. Pressing an item is choosing it, which
// runs its handler or opens its sub-menu exactly as clicking it or pressing Return on it would. Collapsing an item
// takes its sub-menu away again, along with anything opened from within it, which is what moving the pointer back onto
// the item or pressing Escape in the sub-menu does.
func (mi *menuItem) performAccessibilityAction(req accessibility.ActionRequest) bool {
	switch req.Action {
	case accessibility.Press:
		mi.click()
		return true
	case accessibility.Expand:
		if mi.subMenu == nil {
			return false
		}
		mi.showSubMenu()
		return true
	case accessibility.Collapse:
		if mi.subMenu == nil {
			return false
		}
		if wnd := mi.panel.Window(); wnd != nil {
			// Everything above the menu this item sits in goes, which is the item's own sub-menu and whatever was
			// opened from that. The menu the item belongs to stays, since the item is still there to be chosen from.
			wnd.root.closeMenuStackStoppingAt(mi.menu)
		}
		return true
	default:
		return false
	}
}

func (mi *menuItem) mouseDown(_ geom.Point, _, _ int, _ mod.Modifiers) bool {
	if mi.subMenu != nil {
		mi.showSubMenu()
	}
	return true
}

func (mi *menuItem) mouseUp(where geom.Point, _ int, _ mod.Modifiers) bool {
	if mi.subMenu == nil && where.In(mi.panel.ContentRect(true)) {
		mi.execute()
	}
	return true
}

func (mi *menuItem) click() {
	if mi.subMenu != nil {
		mi.showSubMenu()
	} else {
		mi.execute()
	}
}

func (mi *menuItem) showSubMenu() {
	if !mi.factory.showInProgress && mi.subMenu != nil && mi.subMenu.popupPanel == nil {
		mi.factory.showInProgress = true
		defer func() { mi.factory.showInProgress = false }()
		mi.subMenu.createPopup()
		if mi.subMenu.popupPanel == nil {
			// createPopup is a no-op when no window is active, so there is no panel to position.
			return
		}
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

func (mi *menuItem) mouseEnter(_ geom.Point, _ mod.Modifiers) bool {
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

func (mi *menuItem) mouseMove(_ geom.Point, _ mod.Modifiers) bool {
	stopAt := mi.menu
	if mi.subMenu != nil && mi.subMenu.popupPanel != nil {
		stopAt = mi.subMenu
	}
	if w := ActiveWindow(); w != nil {
		w.root.closeMenuStackStoppingAt(stopAt)
	}
	return false
}

func (mi *menuItem) mouseExit() bool {
	mi.over = false
	mi.panel.MarkForRedraw()
	return false
}

func (mi *menuItem) sizer(hint geom.Size) (minSize, prefSize, maxSize geom.Size) {
	if mi.isSeparator {
		prefSize.Height = 1
	} else {
		prefSize, _ = LabelContentSizes(NewText(mi.Title(), &TextDecoration{
			Font:            DefaultMenuItemTheme.TitleFont,
			OnBackgroundInk: DefaultMenuItemTheme.OnBackgroundColor,
		}), nil, DefaultMenuItemTheme.TitleFont, side.Left, 0)
		if !mi.isRoot() {
			prefSize.Width += (DefaultMenuItemTheme.KeyFont.Baseline() + 2) * 2
		}
		if mi.keyBinding.KeyCode != 0 {
			keys := mi.keyBinding.String()
			if keys != "" {
				size := NewText(keys, &TextDecoration{
					Font:            DefaultMenuItemTheme.KeyFont,
					OnBackgroundInk: DefaultMenuItemTheme.OnBackgroundColor,
				}).Extents()
				prefSize.Width += DefaultMenuItemTheme.KeyGap + size.Width
				prefSize.Height = max(prefSize.Height, size.Height)
			}
		}
	}
	prefSize = prefSize.Add(DefaultMenuItemTheme.ItemBorder.Insets().Size()).Ceil().ConstrainForHint(hint)
	return prefSize, prefSize, prefSize
}

func (mi *menuItem) paint(gc *Canvas, rect geom.Rect) {
	var fg, bg Ink
	if !mi.over || !mi.enabled {
		fg = DefaultMenuItemTheme.OnBackgroundColor
		bg = DefaultMenuItemTheme.BackgroundColor
	} else {
		fg = DefaultMenuItemTheme.OnSelectionColor
		bg = DefaultMenuItemTheme.SelectionColor
	}
	bgPaint := bg.Paint(gc, rect, paintstyle.Fill)
	gc.DrawRect(rect, bgPaint)

	if !mi.enabled {
		fg = &ColorFilteredInk{
			OriginalInk: fg,
			ColorFilter: Grayscale30Filter(),
		}
	}
	rect = mi.panel.ContentRect(false)
	if mi.isSeparator {
		separatorPaint := fg.Paint(gc, rect, paintstyle.Fill)
		gc.DrawLine(rect.Point, geom.NewPoint(rect.Right(), rect.Y), separatorPaint)
	} else {
		t := NewText(mi.Title(), &TextDecoration{
			Font:            DefaultMenuItemTheme.TitleFont,
			OnBackgroundInk: fg,
		})
		size := t.Extents()
		baseline := DefaultMenuItemTheme.KeyFont.Baseline()
		var shifted float32
		if !mi.isRoot() {
			shifted = baseline + 2
		}
		t.Draw(gc, geom.NewPoint(rect.X+shifted, xmath.Floor(rect.Y+(rect.Height-size.Height)/2)+t.Baseline()))
		if mi.subMenu == nil {
			if !mi.isRoot() && mi.state != check.Off {
				r := rect
				r.Width = baseline
				r.Height = baseline
				r.Y += (rect.Height - baseline) / 2
				drawable := &DrawableSVG{Size: geom.NewSize(baseline, baseline)}
				if mi.state == check.On {
					drawable.SVG = CheckmarkSVG
				} else {
					drawable.SVG = DashSVG
				}
				statePaint := fg.Paint(gc, r, paintstyle.Fill)
				drawable.DrawInRect(gc, r, nil, statePaint)
			}
			if mi.keyBinding.KeyCode != 0 {
				keys := mi.keyBinding.String()
				if keys != "" {
					t = NewText(keys, &TextDecoration{
						Font:            DefaultMenuItemTheme.KeyFont,
						OnBackgroundInk: fg,
					})
					size = t.Extents()
					t.Draw(gc, geom.NewPoint(xmath.Floor(rect.Right()-size.Width),
						xmath.Floor(rect.Y+(rect.Height-size.Height)/2)+t.Baseline()))
				}
			}
		} else if !mi.isRoot() {
			rect.X = rect.Right() - baseline
			rect.Width = baseline
			drawable := &DrawableSVG{
				SVG:  ChevronRightSVG,
				Size: geom.NewSize(baseline, baseline),
			}
			chevronPaint := fg.Paint(gc, rect, paintstyle.Fill)
			drawable.DrawInRect(gc, rect, nil, chevronPaint)
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
			SafeCall(func() { mi.enabled = mi.validator(mi) })
		}
	}
}

func (mi *menuItem) execute() {
	if mi.isSeparator {
		return
	}
	mi.menu.closeMenuStack()
	if mi.enabled && mi.handler != nil {
		// Enablement is sampled now, while the menu is still the state the user chose from, but the handler itself
		// runs from the event loop rather than from inside the mouse or key event that chose the item. A handler that
		// opens a window or runs a modal dialog is then not doing so from the middle of an event dispatch, which is
		// where the platforms behave least predictably about which window ends up with the focus. processNextTask
		// wraps the call in SafeCall.
		InvokeTask(func() { mi.handler(mi) })
	}
}
