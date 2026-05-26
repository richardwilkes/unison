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
	"github.com/richardwilkes/unison/drag"
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
	// up to parents.
	MouseDownCallback func(where geom.Point, button, clickCount int, mods Modifiers) bool
	// MouseDragCallback is called when the mouse is dragged after being pressed. Return true to stop further handling
	// or false to propagate up to parents.
	MouseDragCallback func(where geom.Point, button int, mods Modifiers) bool
	// MouseUpCallback is called when the mouse is released after being pressed. Return true to stop further handling or
	// false to propagate up to parents.
	MouseUpCallback func(where geom.Point, button int, mods Modifiers) bool
	// MouseEnterCallback is called on mouse entry. Return true to stop further handling or false to propagate up to
	// parents.
	MouseEnterCallback func(where geom.Point, mods Modifiers) bool
	// MouseMoveCallback is called when the mouse moves. Return true to stop further handling or false to propagate up
	// to parents.
	MouseMoveCallback func(where geom.Point, mods Modifiers) bool
	// MouseExitCallback is called on mouse exit. Return true to stop further handling or false to propagate up to
	// parents.
	MouseExitCallback func() bool
	// MouseWheelCallback is called when the mouse wheel is rotated. Return true to stop further handling or false to
	// propagate up to parents.
	MouseWheelCallback func(where, delta geom.Point, mods Modifiers) bool
	// KeyDownCallback is called when a key is pressed. Return true to stop further handling or false to propagate up to
	// parents.
	KeyDownCallback func(keyCode KeyCode, mods Modifiers, repeat bool) bool
	// RuneTypedCallback is called when a key is typed. Return true to stop further handling or false to propagate up to
	// parents.
	RuneTypedCallback func(ch rune) bool
	// KeyUpCallback is called when a key is released. Return true to stop further handling or false to propagate up to
	// parents.
	KeyUpCallback func(keyCode KeyCode, mods Modifiers) bool
}

// DragCallbacks holds the callbacks that client code can hook into for drag and drop events.
type DragCallbacks struct {
	// DragEnteredCallback is called when a drag operation enters the window or panel. The returned drag.Op should be
	// just one of the permitted drag.Op constants, as determined by dragInfo.SourceDragOpMask().
	DragEnteredCallback func(di drag.Info, where geom.Point, mods Modifiers) drag.Op
	// DragUpdatedCallback is called when a drag operation is adjusted while within the window or panel. The returned
	// drag.Op should be just one of the permitted drag.Op constants, as determined by dragInfo.SourceDragOpMask(). For
	// performance reasons, examination of data types and/or the data should be done when DragEnteredCallback() is
	// called and not here, if at all possible. If nil, the result from the DragEnteredCallback will be returned.
	DragUpdatedCallback func(di drag.Info, where geom.Point, mods Modifiers) drag.Op
	// DragExitedCallback is called when a drag operation leaves the window or panel.
	DragExitedCallback func()
	// DropCallback is called when a drag operation is released over the window or panel. Return true if the drop is
	// accepted and false if it is not.
	DropCallback func(di drag.Info, where geom.Point, mods Modifiers) bool
}
