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
	"github.com/richardwilkes/unison/enums/mod"
)

// Constants for mouse buttons.
const (
	ButtonLeft = iota
	ButtonRight
	ButtonMiddle
)

// InputCallbacks holds the callbacks that client code can hook into for user input events.
type InputCallbacks struct {
	// GainedFocusCallback is called when the keyboard focus is gained.
	GainedFocusCallback func()
	// LostFocusCallback is called when the keyboard focus is lost.
	LostFocusCallback func()
	// MouseDownCallback is called when the mouse is pressed. Return true to stop further handling or false to propagate
	// up to parents. In the active window, a right press over a panel with a contextual menu is sent to no panel unless
	// it becomes a drag that the panel asks for back through ContextMenuPressHandler, in which case the press, drags
	// and release are delivered then; a Window's own callback is still called for it, unless another button is down,
	// in which case the press is ignored entirely. In any other window, a right press is delivered as any other, since
	// no menu could open there.
	MouseDownCallback func(where geom.Point, button, clickCount int, mods mod.Modifiers) bool
	// MouseDragCallback is called when the mouse is dragged after being pressed. Return true to stop further handling
	// or false to propagate up to parents. A drag of a right press no panel was sent (see MouseDownCallback) is not
	// sent to any panel either, though a Window's own callback still is.
	MouseDragCallback func(where geom.Point, button int, mods mod.Modifiers) bool
	// MouseUpCallback is called when the mouse is released after being pressed. Return true to stop further handling or
	// false to propagate up to parents. The release of a right press no panel was sent (see MouseDownCallback) is not
	// sent to any panel either, though a Window's own callback still is when it was called for the press. A press
	// still held when a contextual menu is about to open, other than on a right-click's release, is ended with a
	// release delivered at a position within no panel, to the panel and to the Window's own callback alike: it ends
	// the gesture without counting as a click, and its position must not be applied as a final drag position.
	MouseUpCallback func(where geom.Point, button int, mods mod.Modifiers) bool
	// MouseEnterCallback is called on mouse entry. Return true to stop further handling or false to propagate up to
	// parents.
	MouseEnterCallback func(where geom.Point, mods mod.Modifiers) bool
	// MouseMoveCallback is called when the mouse moves. Return true to stop further handling or false to propagate up
	// to parents.
	MouseMoveCallback func(where geom.Point, mods mod.Modifiers) bool
	// MouseExitCallback is called on mouse exit. Return true to stop further handling or false to propagate up to
	// parents.
	MouseExitCallback func() bool
	// MouseWheelCallback is called when the mouse wheel is rotated. Return true to stop further handling or false to
	// propagate up to parents.
	MouseWheelCallback func(where, delta geom.Point, mods mod.Modifiers) bool
	// KeyDownCallback is called when a key is pressed. Return true to stop further handling or false to propagate up to
	// parents.
	KeyDownCallback func(keyCode KeyCode, mods mod.Modifiers, repeat bool) bool
	// RuneTypedCallback is called when a key is typed. Return true to stop further handling or false to propagate up to
	// parents.
	RuneTypedCallback func(ch rune) bool
	// KeyUpCallback is called when a key is released. Return true to stop further handling or false to propagate up to
	// parents.
	KeyUpCallback func(keyCode KeyCode, mods mod.Modifiers) bool
	// ContextMenuCallback is called when a contextual menu is wanted for the panel, and returns the menu to pop up, or
	// nil when there is nothing to offer just now. The position is in the panel's own coordinates and is where the menu
	// will open. The caller pops the menu up and disposes of it. It is called for a right-click on the panel while its
	// window is the active one, at the pointer; for the Menu key or shift+F10 while the panel holds the keyboard focus;
	// and for an assistive technology, since setting it makes the panel advertise accessibility.ShowContextMenu. The
	// last two pass what ContextMenuAnchorer answers, or else Panel.DefaultContextMenuAnchor. Only the panel under the
	// pointer or holding the focus is asked, never an ancestor of it: a right-click on a child without a callback is an
	// ordinary press, even within a panel that has one. A Table stands in for a widget in one of its cells, which are
	// not its children, when the widget has a callback and can take the focus, or holds something that can, for a
	// right-click within the widget's own bounds; a right-click elsewhere in the cell asks the Table. A List does not
	// stand in for the panels in its rows, nor a TableHeader for its column headers. A widget that has a callback but,
	// for a while, no menu at all implements ContextMenuWithholder. See Panel.ShowContextMenu.
	//
	// A right-click on a panel with a callback is not delivered as mouse events, whatever the click count, unless it
	// becomes a drag the panel asked to have back through ContextMenuPressHandler. The window gives the panel the
	// keyboard focus when it can hold it and opens the menu on the release.
	//
	// A Window embeds InputCallbacks but never calls this: a menu for the whole window belongs on its content panel.
	ContextMenuCallback func(where geom.Point) Menu
}
