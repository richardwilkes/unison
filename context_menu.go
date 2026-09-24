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
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xreflect"
	"github.com/richardwilkes/unison/enums/mod"
)

// A contextual menu belongs to a panel: InputCallbacks.ContextMenuCallback builds it, and every way of asking for one —
// a right-click (Window.mouseDown and Window.mouseUp), the Menu key or shift+F10 (Window.keyPressed) and an assistive
// technology's accessibility.ShowContextMenu (Panel.axDispatchAction) — ends in Panel.ShowContextMenu. Only the panel
// under the pointer, or the one holding the focus, is asked for its menu; its ancestors never are. A widget that draws
// panels it does not hold as children, as a Table does its cells, stands in for them through contextMenuOwnerResolver.
// See Window.takeContextMenuPress for how a right-click is handled.

// ContextMenuAnchorer is implemented by a widget that knows where a contextual menu opened without a pointer position,
// from the keyboard or by an assistive technology, should go: a field's caret, a table's current row. It is looked for
// on Panel.Self, so the widget type itself must implement it. A panel that does not implement it uses
// Panel.DefaultContextMenuAnchor.
type ContextMenuAnchorer interface {
	// ContextMenuAnchor returns the position, in the widget's own coordinates, to open a contextual menu at when there
	// is no pointer position. The widget may scroll to bring that position into view first.
	ContextMenuAnchor() geom.Point
}

// ContextMenuWithholder is implemented by a widget that has a ContextMenuCallback but may, for a while, offer no
// contextual menu at all. Where a callback returning nil still takes the right-click that asked for it, a withholding
// widget is treated as one with no callback: a right-click on it is an ordinary press, the Menu key opens nothing, its
// node does not advertise accessibility.ShowContextMenu and Panel.ShowContextMenu shows nothing. A dock tab alone in
// its container withholds its menu. It is looked for on Panel.Self, so the widget type itself must implement it.
type ContextMenuWithholder interface {
	// WithholdsContextMenu reports whether the widget is, for now, offering no contextual menu at all.
	WithholdsContextMenu() bool
}

// ContextMenuPressHandler is implemented by a widget that must act when a right-click that will open its contextual
// menu lands on it, since the window delivers no mouse event for such a press: a table or list selects the row under
// the pointer. Only the panel whose menu the right-click is for is asked. It is looked for on Panel.Self, so the widget
// type itself must implement it.
type ContextMenuPressHandler interface {
	// ContextMenuPressed is called on the press, with the position in the widget's own coordinates and the modifiers.
	// It returns true if the widget wants the press delivered as an ordinary mouse press after all, should the pointer
	// move beyond the drag threshold before the release; the drags and release then follow and no menu opens. False
	// leaves such a drag delivering nothing.
	//
	// It is called before the window gives the widget the keyboard focus. A widget that scrolls itself into view on
	// gaining the focus should take the focus here without scrolling, so that the view does not move under the pointer.
	// A widget in a Table cell has already been given the focus by the table by the time this is called.
	ContextMenuPressed(where geom.Point, mods mod.Modifiers) bool
}

// ShowContextMenu pops up the menu this panel's ContextMenuCallback builds, at the given position in the panel's own
// coordinates, and reports whether a menu was shown. Nothing is shown, and the callback is not called, when the panel
// has no callback, is disabled, is withholding its menu (see ContextMenuWithholder) or is not in the active window,
// where a popup menu always opens. Nothing is shown either when the callback returns nil or a menu that is empty once
// the separators at either end are removed.
//
// The window calls this for a right-click, the Menu key or shift+F10 and an assistive technology's request, so an
// application calls it only for a gesture of its own. Where menus are native, this does not return until the menu is
// dismissed, and the menu's tracking loop consumes the release of any mouse button held when it opens, so any press in
// progress is ended first, with its release delivered outside every panel; see Window.endPressesForContextMenu. It is
// nonetheless best called on a release or a key press rather than on a mouse press.
func (p *Panel) ShowContextMenu(where geom.Point) bool {
	if !p.offersContextMenu() || !axMayPopupMenu(p) {
		return false
	}
	wnd := p.Window()
	wnd.endPressesForContextMenu()
	if p.Window() != wnd || !p.offersContextMenu() || !axMayPopupMenu(p) {
		// The release ended the offer: it disabled or removed the panel, made it withhold, or activated another window.
		return false
	}
	var menu Menu
	SafeCall(func() { menu = p.ContextMenuCallback(where) })
	if xreflect.IsNil(menu) {
		return false
	}
	for menu.Count() > 0 && menu.ItemAtIndex(0).IsSeparator() {
		menu.RemoveItem(0)
	}
	for count := menu.Count(); count > 0 && menu.ItemAtIndex(count-1).IsSeparator(); count = menu.Count() {
		menu.RemoveItem(count - 1)
	}
	if menu.Count() == 0 {
		menu.Dispose()
		return false
	}
	// A tooltip showing, or about to show, would be drawn over the menu.
	wnd.ClearTooltip()
	// Worked out before anything is drawn, since drawing may detach the panel: a Table row that builds a cell afresh on
	// every call has the newest one adopted as the focused cell as it is drawn.
	at := geom.Rect{Point: p.PointToRoot(where), Size: geom.Size{Width: 1, Height: 1}}
	// A native menu blocks drawing until it is dismissed, so what the gesture changed (a right-click selecting a row)
	// must reach the screen first.
	wnd.FlushDrawing()
	menu.Popup(at, 0)
	menu.Dispose()
	return true
}

// DefaultContextMenuAnchor returns where a contextual menu opened without a pointer position goes for a panel with
// nothing better: the center of the visible part of its content, in its own coordinates, or the center of the whole
// content when none of it is visible. A widget implementing ContextMenuAnchorer may fall back to it, as Table and List
// do when no row is selected.
func (p *Panel) DefaultContextMenuAnchor() geom.Point {
	content := p.ContentRect(false)
	visible := p.RectToRoot(content)
	for ancestor := p.parent; ancestor != nil; ancestor = ancestor.parent {
		visible = visible.Intersect(ancestor.RectToRoot(ancestor.ContentRect(true)))
	}
	if visible.Empty() {
		return content.Center()
	}
	return p.PointFromRoot(visible.Center())
}

// contextMenuAnchor returns the ContextMenuAnchorer's answer, or DefaultContextMenuAnchor when the widget is not one or
// asking it panicked.
func (p *Panel) contextMenuAnchor() geom.Point {
	if anchorer, ok := p.Self.(ContextMenuAnchorer); ok {
		var where geom.Point
		answered := false
		SafeCall(func() {
			where = anchorer.ContextMenuAnchor()
			answered = true
		})
		if answered {
			return where
		}
	}
	return p.DefaultContextMenuAnchor()
}

// contextMenuOwnerResolver is implemented by a widget that draws panels it does not hold as children, as a Table does
// its cells, so that a right press over one of them can find a contextual menu offered by it, since the window's search
// for the panel under the pointer stops at the widget. It is looked for on Panel.Self of the enabled panel under the
// pointer, before that panel's own menu is considered; see Window.contextMenuOwnerAt.
type contextMenuOwnerResolver interface {
	// contextMenuOwnerWithin returns the panel whose contextual menu a right press at the given position, in the
	// widget's own coordinates, is for, among the panels it draws there without holding them as children, or nil when
	// none offers one. The panel returned must stay attached to the window until the release that opens the menu, so a
	// widget that keeps a borrowed panel attached only while the focus is within it gives it the focus first.
	contextMenuOwnerWithin(where geom.Point) *Panel
	// offersContextMenuWithin reports whether contextMenuOwnerWithin would find an owner, without moving the focus or
	// selecting anything. The window asks this of a right press made while another button is down, which must change
	// nothing.
	offersContextMenuWithin(where geom.Point) bool
}

// offersContextMenu reports whether this panel itself offers a contextual menu just now: it is enabled, has a
// ContextMenuCallback and is not withholding its menu. A ContextMenuWithholder that panics is taken as not withholding,
// since this is reached from the snapshot walk and from event dispatch, neither of which may be unwound by application
// code.
func (p *Panel) offersContextMenu() bool {
	if p.ContextMenuCallback == nil || !p.Enabled() {
		return false
	}
	withholder, ok := p.Self.(ContextMenuWithholder)
	if !ok {
		return true
	}
	withholds := false
	SafeCall(func() { withholds = withholder.WithholdsContextMenu() })
	return !withholds
}

// isContextMenuKey reports whether a key press is the chord for the focused panel's contextual menu: the Menu
// (Applications) key alone, or shift+F10, on every platform. Caps lock and num lock are ignored.
func isContextMenuKey(key KeyCode, mods mod.Modifiers) bool {
	switch key {
	case KeyMenu:
		return mods&mod.NonSticky == 0
	case KeyF10:
		return mods&mod.NonSticky == mod.Shift
	default:
		return false
	}
}
