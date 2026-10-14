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
	"math"
	"slices"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/behavior"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/enums/role"
)

var _ Menu = &menu{}

// Menu holds a set of menu items.
type Menu interface {
	// Factory returns the MenuFactory that created this Menu.
	Factory() MenuFactory
	// ID returns the id of this Menu.
	ID() int
	// IsSame returns true if the two menus represent the same object. Do not use == to test for equality.
	IsSame(other Menu) bool
	// ItemAtIndex returns the menu item at the specified index within the menu.
	ItemAtIndex(index int) MenuItem
	// Item returns the menu item with the specified id anywhere in the menu and and its sub-menus.
	Item(id int) MenuItem
	// Menu returns the menu with the specified id anywhere in the menu and and its sub-menus.
	Menu(id int) Menu
	// InsertSeparator inserts a menu separator at the specified menuItem index within this menu. Pass in a negative
	// index to append to the end. If onlyIfNeeded is true, then a separator is only inserted if the menuItem that would
	// precede it at the insertion location is not a separator.
	InsertSeparator(atIndex int, onlyIfNeeded bool)
	// InsertItem inserts a menu item at the specified menuItem index within this menu. Pass in a negative index to
	// append to the end. If the menu item is nil, then nothing happens.
	InsertItem(atIndex int, mi MenuItem)
	// InsertMenu inserts a new sub-menu at the specified menuItem index within this menu. Pass in a negative index to
	// append to the end.
	InsertMenu(atIndex int, subMenu Menu)
	// RemoveItem removes the menu item at the specified index from this menu.
	RemoveItem(index int)
	// RemoveAll removes all menu items from the menu.
	RemoveAll()
	// Title returns the title of this menu.
	Title() string
	// Count of menu items in this menu.
	Count() int
	// Popup the menu at the specified position, given in the coordinates of the active window's root panel. The menu
	// is shown in a window of its own, so it may extend beyond the active window.
	Popup(where geom.Rect, itemIndex int)
	// Dispose releases any OS resources associated with this menu.
	Dispose()
}

// DefaultMenuTheme holds the default MenuTheme values for Menus. Modifying this data will not alter existing Menus,
// but will alter any Menus created in the future.
var DefaultMenuTheme = MenuTheme{
	BarBorder:  NewLineBorder(ThemeSurfaceEdge, geom.Size{}, geom.Insets{Bottom: 1}, false),
	MenuBorder: NewLineBorder(ThemeSurfaceEdge, geom.Size{}, geom.NewUniformInsets(1), false),
}

// MenuTheme holds theming data for a Menu.
type MenuTheme struct {
	BarBorder  Border
	MenuBorder Border
}

// noTopLimit is the top passed to menu.ensureOnDisplay for a menu that may cover anything above it. Zero cannot serve,
// since a display above the primary one has negative coordinates.
var noTopLimit = float32(math.Inf(-1))

type menuPanel struct {
	menu *menu
	Panel
	itemIndex int
}

type menu struct {
	factory    *inWindowMenuFactory
	titleItem  *menuItem
	popupPanel *menuPanel
	updater    func(Menu)
	items      []*menuItem
}

func (m *menu) Factory() MenuFactory {
	return m.factory
}

func (m *menu) IsSame(other Menu) bool {
	return m == other
}

func (m *menu) ItemAtIndex(index int) MenuItem {
	if index < 0 || index >= len(m.items) {
		return nil
	}
	return m.items[index]
}

func (m *menu) Item(id int) MenuItem {
	for _, mi := range m.items {
		if mi.IsSeparator() {
			continue
		}
		if id == mi.ID() {
			return mi
		}
		if mi.subMenu != nil {
			if subItem := mi.subMenu.Item(id); subItem != nil {
				return subItem
			}
		}
	}
	return nil
}

func (m *menu) Menu(id int) Menu {
	for _, mi := range m.items {
		if mi.IsSeparator() {
			continue
		}
		if mi.subMenu != nil {
			if id == mi.ID() {
				return mi.subMenu
			}
			if sub := mi.subMenu.Menu(id); sub != nil {
				return sub
			}
		}
	}
	return nil
}

func (m *menu) InsertSeparator(atIndex int, onlyIfNeeded bool) {
	if onlyIfNeeded && atIndex != 0 && len(m.items) != 0 {
		if atIndex < 0 || atIndex > len(m.items) {
			atIndex = len(m.items)
		}
		if m.items[atIndex-1].IsSeparator() {
			return
		}
	}
	m.insertItem(atIndex, &menuItem{
		factory:     m.factory,
		menu:        m,
		isSeparator: true,
	})
}

func (m *menu) InsertItem(atIndex int, mi MenuItem) {
	if mi != nil {
		if item, ok := mi.(*menuItem); ok {
			m.insertItem(atIndex, item)
		}
	}
}

func (m *menu) InsertMenu(atIndex int, subMenu Menu) {
	if sub, ok := subMenu.(*menu); ok {
		m.insertItem(atIndex, sub.titleItem)
	}
}

func (m *menu) insertItem(atIndex int, mi *menuItem) {
	mi.menu = m
	if atIndex < 0 || atIndex >= len(m.items) {
		m.items = append(m.items, mi)
	} else {
		m.items = append(m.items, nil)
		copy(m.items[atIndex+1:], m.items[atIndex:])
		m.items[atIndex] = mi
	}
}

// RemoveItem takes the item at index out of the menu. The item keeps its reference to the menu, since its panel may
// still be showing in the menu's open popup, which draws and chooses it through that reference; see menuItem.Menu.
func (m *menu) RemoveItem(index int) {
	if index >= 0 && index < len(m.items) {
		m.items = slices.Delete(m.items, index, index+1)
	}
}

// RemoveAll takes every item out of the menu. See RemoveItem for what the items keep.
func (m *menu) RemoveAll() {
	m.items = nil
}

func (m *menu) ID() int {
	return m.titleItem.id
}

func (m *menu) Title() string {
	return m.titleItem.title
}

func (m *menu) String() string {
	return m.titleItem.String()
}

func (m *menu) Count() int {
	return len(m.items)
}

func (m *menu) Popup(where geom.Rect, itemIndex int) {
	if m.popupPanel != nil {
		return
	}
	owner := ActiveWindow()
	if !m.createPopup(owner) {
		// No window is active (a transient state; see FrontmostWindow's comment), so there is nothing to show the
		// menu for.
		return
	}
	if itemIndex >= 0 && itemIndex < len(m.items) {
		if sp, ok := m.popupPanel.Children()[0].Self.(*ScrollPanel); ok {
			where.Y -= sp.Content().AsPanel().Children()[itemIndex].FrameRect().Y
		}
	}
	fr := m.popupPanel.FrameRect()
	where.Height = fr.Height
	where.Width = max(fr.Width, where.Width)
	where.Point = owner.apiPointToScreen(where.Point)
	m.ensureOnDisplay(where, menuFit{}, owner, itemIndex)
	m.setKeyIndex(itemIndex)
}

// menuFit says how fitMenuRect places a menu that does not fit where it was asked to go, beyond shifting it onto the
// display and shrinking it: a menu opened from the menu bar goes above the bar when the space below the bar would show
// too little of it, and a sub-menu goes to the left of its item when it does not fit to the right of it. The edges are
// in the space window rects use, whose origins are the display's own on every platform (see
// Display.apiRectToWindowUnits), and minHeight is in window units until menuWindowRect converts it. The zero value asks
// for neither.
type menuFit struct {
	// barTop and barBottom are the top and bottom of the title a menu bar menu opens from, and minHeight is the height
	// the menu must be able to show below the bar to be placed there: that of its first menuMinVisibleItems items.
	barTop, barBottom, minHeight float32
	// leftOf is the x a sub-menu's right edge goes at when it does not fit to the right of where it was asked to go:
	// the left edge of the item it opens from.
	leftOf float32
	bar    bool
	sub    bool
}

// menuMinVisibleItems is how many of its items a menu opened from the menu bar must be able to show below the bar to be
// placed there rather than above it.
const menuMinVisibleItems = 3

// ensureOnDisplay places the popup window at where, in the space window rects use, shifting it and, when that is not
// enough, shrinking it so that it lies within the usable area of its display, placed as fit says, highlights the item
// at keyIndex, if there is one, as the one the keys act on, and then shows the window. A menu opened from the menu bar
// passes the bar's edges in fit so that it scrolls rather than covering the bar. The display is the one where overlaps
// most, or anchor's when where overlaps none. The fitting is done in the display's own space, since the window-rect
// space is not one its edges can be computed in on every platform; see Display.apiRectFromWindowUnits.
//
// The highlight comes before the show because on X11 the window is drawn as it is shown, and every draw of a popup
// describes the owner's menus to assistive technologies (see Window.draw): a first draw with nothing highlighted would
// report the focus on the menu, and the next one, once setKeyIndex had run, on the item, so a screen reader would
// announce the menu only to be cut off by the item.
func (m *menu) ensureOnDisplay(where geom.Rect, fit menuFit, anchor *Window, keyIndex int) {
	if m.popupPanel == nil {
		return
	}
	popup := m.popupPanel.Window()
	if !popup.IsValid() {
		return
	}
	popup.SetContentRect(menuWindowRect(where, fit, menuDisplayFor(where, anchor)))
	popup.ValidateLayout()
	m.highlightKeyIndex(keyIndex)
	popup.Show()
}

// highlightKeyIndex highlights the item at index, if there is one, as the one the keys act on and scrolls it into view.
// Unlike setKeyIndex, it leaves the item's sub-menu closed, since one opened before the menu is shown would be stacked
// below it.
func (m *menu) highlightKeyIndex(index int) {
	m.popupPanel.itemIndex = index
	if index >= 0 && index < len(m.items) {
		m.items[index].highlight()
		m.items[index].scrollIntoView()
	}
}

// menuWindowRect returns where, in the space window rects use, fitted onto d as fit says (see fitMenuRect), or where as
// it is when d is nil, which it is on macOS while every display is asleep: there is nothing to fit the menu onto, and
// the popup is still shown so that the menu stack it has joined matches what is on the screen.
func menuWindowRect(where geom.Rect, fit menuFit, d *Display) geom.Rect {
	if d == nil {
		return where
	}
	fit.minHeight = d.apiRectFromWindowUnits(geom.Rect{Size: geom.NewSize(0, fit.minHeight)}).Height
	return d.apiRectToWindowUnits(fitMenuRect(d.apiRectFromWindowUnits(where), d.Usable, fit))
}

// minVisibleHeight returns the height of the popup showing the menu's first menuMinVisibleItems items that are not
// separators, or every item when there are fewer, as the popup is laid out now.
func (m *menu) minVisibleHeight() float32 {
	var content *Panel
	var bottom float32
	count := 0
	for _, mi := range m.items {
		if mi.isSeparator || mi.panel == nil {
			continue
		}
		content = mi.panel.Parent()
		bottom = mi.panel.FrameRect().Bottom()
		count++
		if count == menuMinVisibleItems {
			break
		}
	}
	if content == nil || m.popupPanel == nil {
		return 0
	}
	// What the popup adds around its content, the borders and the scroll panel, wraps the first items just the same.
	return m.popupPanel.FrameRect().Height - content.FrameRect().Height + bottom
}

// fitMenuRect returns where shifted and, when that is not enough, shrunk so that it lies within usable. A menu bar menu
// (fit.bar) is kept no higher than the bar's bottom, unless the space below the bar would show less than fit.minHeight
// of it and the space above the bar would show more, in which case it is placed above the bar, with its bottom at the
// bar's top, and no lower. A sub-menu (fit.sub) that does not fit to the right of where it was asked to go is placed
// with its right edge at fit.leftOf first. The rects and edges are all in the same space.
func fitMenuRect(where, usable geom.Rect, fit menuFit) geom.Rect {
	top := noTopLimit
	switch {
	case fit.bar:
		top = fit.barBottom
		below := usable.Bottom() - max(fit.barBottom, usable.Y)
		if above := min(fit.barTop, usable.Bottom()) - usable.Y; below < fit.minHeight && above > below {
			usable.Height = above
			where.Y = fit.barTop - where.Height
			top = noTopLimit
		}
	case fit.sub:
		if where.Right() > usable.Right() {
			where.X = fit.leftOf - where.Width
		}
	}
	top = min(max(top, usable.Y), usable.Bottom())
	if where.Width > usable.Width {
		where.Width = usable.Width
	}
	if where.Right() > usable.Right() {
		where.X = usable.Right() - where.Width
	}
	if where.X < usable.X {
		where.X = usable.X
	}
	if where.Height > usable.Bottom()-top {
		where.Height = usable.Bottom() - top
	}
	if where.Bottom() > usable.Bottom() {
		where.Y = usable.Bottom() - where.Height
	}
	if where.Y < top {
		where.Y = top
	}
	where.Height = max(where.Height, 1)
	return where
}

// menuDisplayFor returns the display a menu placed at where, in the space window rects use, belongs on: the one where
// overlaps most, or anchor's when where overlaps none.
func menuDisplayFor(where geom.Rect, anchor *Window) *Display {
	if best := menuDisplayAmong(AllDisplays(), where); best != nil {
		return best
	}
	return anchor.Display()
}

// menuDisplayAmong returns the one of displays that where, in the space window rects use, overlaps most, or nil when it
// overlaps none. Each display's overlap is measured in that display's own space.
func menuDisplayAmong(displays []*Display, where geom.Rect) *Display {
	var best *Display
	var bestArea float32
	for _, d := range displays {
		r := d.apiRectFromWindowUnits(where)
		if d.Usable.Contains(r) {
			return d
		}
		if ri := r.Intersect(d.Usable); !ri.Empty() {
			if area := ri.Width * ri.Height; bestArea < area {
				bestArea = area
				best = d
			}
		}
	}
	return best
}

// createPopup builds the menu's panel and the popup window showing it, as the newest menu of owner's stack, closing
// whatever the stack holds above the menu this one belongs to. The window is left hidden for the caller to place and
// show. Returns true if the popup was created.
func (m *menu) createPopup(owner *Window) bool {
	if m.popupPanel != nil || !owner.IsValid() {
		return false
	}
	owner.root.closeMenuStackStoppingAt(m.titleItem.menu)
	owner.ClearTooltip()
	// Built before the window exists, since building runs the menu's updater and its items' validators.
	panel := m.newPanel(false)
	popup, err := newPopupWindow(owner, m.Title())
	if err != nil {
		errs.Log(err)
		return false
	}
	m.popupPanel = panel
	// Not Window.SetContent, which would lay the panel out at the window's initial size.
	popup.root.setContent(panel)
	panel.Pack()
	panel.ValidateLayout()
	owner.root.insertMenu(panel)
	return true
}

func (m *menu) newPanel(forBar bool) *menuPanel {
	p := &menuPanel{
		menu:      m,
		itemIndex: -1,
	}
	p.Self = p
	if forBar {
		p.SetBorder(DefaultMenuTheme.BarBorder)
		p.Accessibility.Role = role.MenuBar
	} else {
		p.SetBorder(DefaultMenuTheme.MenuBorder)
		p.Accessibility.Role = role.Menu
		p.Accessibility.Name = m.Title()
	}
	content := NewPanel()
	if m.updater != nil {
		m.updater(m)
	}
	for _, mi := range m.items {
		mi.validate()
		child := mi.newPanel()
		content.AddChild(child)
		if !forBar {
			child.SetLayoutData(&FlexLayoutData{
				HAlign: align.Fill,
				VAlign: align.Middle,
				HGrab:  true,
			})
		}
	}
	p.KeyDownCallback = func(keyCode KeyCode, mods mod.Modifiers, _repeat bool) bool {
		if mods&mod.NonSticky != 0 {
			return false
		}
		switch keyCode {
		case KeyDown:
			old := m.popupPanel.itemIndex
			next := old
			for {
				next++
				if next < 0 {
					next = 0
				}
				if next >= len(p.menu.items) {
					break
				}
				if !p.menu.items[next].isSeparator {
					m.popupPanel.itemIndex = next
					break
				}
			}
			m.doExitEnter(old)
			return true
		case KeyUp:
			old := m.popupPanel.itemIndex
			next := old
			for {
				next--
				if next < 0 {
					break
				}
				if next >= len(p.menu.items) {
					next = len(p.menu.items) - 1
					if next < 0 {
						break
					}
				}
				if !p.menu.items[next].isSeparator {
					m.popupPanel.itemIndex = next
					break
				}
			}
			m.doExitEnter(old)
			return true
		case KeyReturn, KeyNumPadEnter, KeySpace:
			if m.popupPanel.itemIndex >= 0 && m.popupPanel.itemIndex < len(p.menu.items) {
				m.items[m.popupPanel.itemIndex].click()
				return true
			}
		case KeyEscape, KeyLeft:
			if wnd := p.Window().menuOwner(); wnd != nil {
				wnd.root.closeMenuStackStoppingAt(m.titleItem.menu)
			}
			return true
		case KeyRight:
			if m.popupPanel.itemIndex >= 0 && m.popupPanel.itemIndex < len(m.items) && m.items[m.popupPanel.itemIndex].subMenu != nil {
				m.doExitEnter(-1)
			}
		}
		return false
	}
	lay := &FlexLayout{Columns: 1}
	if forBar {
		lay.Columns = len(content.Children())
	}
	content.SetLayout(lay)
	content.Pack()
	s := NewScrollPanel()
	// The scroll panel is how a menu too tall for its display is shown, and nothing more: an assistive technology
	// walking a menu must find the items in it, not an anonymous scroll area holding them. Hiding the panel itself
	// leaves the items as the menu's own children, while the scroll bar within it is still described when there is
	// something to scroll.
	s.Accessibility.Role = role.None
	s.SetContent(content, behavior.Follow, behavior.Fill)
	s.SetLayoutData(&FlexLayoutData{
		HAlign: align.Fill,
		VAlign: align.Fill,
		HGrab:  true,
		VGrab:  true,
	})
	p.SetLayout(&FlexLayout{Columns: 1})
	p.AddChild(s)
	return p
}

func (m *menu) doExitEnter(previousIndex int) {
	if previousIndex != m.popupPanel.itemIndex && m.popupPanel.itemIndex >= 0 && m.popupPanel.itemIndex < len(m.items) {
		if previousIndex >= 0 && previousIndex < len(m.items) {
			m.items[previousIndex].mouseExit()
		}
		m.items[m.popupPanel.itemIndex].mouseEnter(geom.Point{}, 0) // params are unused
		m.items[m.popupPanel.itemIndex].scrollIntoView()
		if subMenu := m.items[m.popupPanel.itemIndex].subMenu; subMenu != nil {
			subMenu.setKeyIndex(0)
		}
	}
}

// setKeyIndex makes the item at index, if there is one, the one the keys act on: it is highlighted and scrolled into
// view, and its sub-menu, if it has one, is opened.
func (m *menu) setKeyIndex(index int) {
	if m.popupPanel == nil {
		// The popup may not exist if createPopup had no window to open it for.
		return
	}
	m.popupPanel.itemIndex = index
	if index >= 0 && index < len(m.items) {
		m.items[index].mouseEnter(geom.Point{}, 0) // params are unused
		m.items[index].scrollIntoView()
	}
}

func (m *menu) Dispose() {
}

func (m *menu) preKeyDown(wnd *Window, keyCode KeyCode, mods mod.Modifiers, repeat bool) bool {
	if m.updater != nil && m.popupPanel == nil && mods.OSMenuCommandDown() {
		// The updater runs only for key presses with the command modifier down, since it may do relatively expensive
		// work to set the menu up and must not slow down every key a user types, and not while the menu is open: its
		// popup shows the items the updater produced when it opened, and replacing them would leave the popup showing
		// panels of items that no longer belong to the menu.
		m.updater(m)
	}
	for _, mi := range m.items {
		if mi.keyBinding.KeyCode != 0 && mi.keyBinding.KeyCode == keyCode &&
			mi.keyBinding.Modifiers == mods&mod.NonSticky {
			mi.validate()
			if mi.enabled {
				mi.execute()
				return true
			}
			return len(wnd.root.openMenuPanels) != 0
		}
		if mi.subMenu != nil {
			if mi.subMenu.preKeyDown(wnd, keyCode, mods, repeat) {
				return true
			}
		}
	}
	return len(wnd.root.openMenuPanels) != 0
}

// closeMenuStack closes every open menu of the window this menu was opened for. The focus never left that window, so
// there is nothing to give back to it.
func (m *menu) closeMenuStack() {
	var wnd *Window
	if m.popupPanel != nil {
		wnd = m.popupPanel.Window().menuOwner()
	}
	if wnd == nil {
		wnd = ActiveWindow()
	}
	if wnd != nil {
		wnd.root.closeMenuStackStoppingAt(nil)
		wnd.MarkForRedraw()
		wnd.FlushDrawing()
	}
}
