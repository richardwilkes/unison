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
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/richardwilkes/toolbox/xmath/geom32"
)

// Constants for mouse buttons.
const (
	ButtonLeft   = int(glfw.MouseButtonLeft)
	ButtonRight  = int(glfw.MouseButtonRight)
	ButtonMiddle = int(glfw.MouseButtonMiddle)
)

// InputCallbacks holds the callbacks that client code can hook into for user input events.
type InputCallbacks struct {
	// GainedFocusCallback is called when the keyboard focus is gained on this window.
	GainedFocusCallback func()
	// LostFocusCallback is called when the keyboard focus is lost from this window.
	LostFocusCallback func()
	// MouseDownCallback is called when the mouse is pressed within this window. Return true to stop further handling.
	MouseDownCallback func(where geom32.Point, button, clickCount int, mod Modifiers) bool
	// MouseDragCallback is called when the mouse is dragged after being pressed within this window. Return true to stop
	// further handling.
	MouseDragCallback func(where geom32.Point, button int, mod Modifiers) bool
	// MouseUpCallback is called when the mouse is released after being pressed within this window. Return true to stop
	// further handling.
	MouseUpCallback func(where geom32.Point, button int, mod Modifiers) bool
	// MouseEnterCallback is called when the mouse enters this window. Return true to stop further handling.
	MouseEnterCallback func(where geom32.Point, mod Modifiers) bool
	// MouseMoveCallback is called when the mouse moves within this window. Return true to stop further handling.
	MouseMoveCallback func(where geom32.Point, mod Modifiers) bool
	// MouseExitCallback is called when the mouse exits this window. Return true to stop further handling.
	MouseExitCallback func() bool
	// MouseWheelCallback is called when the mouse wheel is rotated over this window. Return true to stop further
	// handling.
	MouseWheelCallback func(where, delta geom32.Point, mod Modifiers) bool
	// KeyDownCallback is called when a key is pressed in this window. Return true to stop further handling.
	KeyDownCallback func(keyCode KeyCode, mod Modifiers, repeat bool) bool
	// KeyUpCallback is called when a key is released in this window. Return true to stop further handling.
	KeyUpCallback func(keyCode KeyCode, mod Modifiers) bool
	// RuneTypedCallback is called when a key is typed in this window. Return true to stop further handling.
	RuneTypedCallback func(ch rune) bool
	// FileDropCallback is called when files are drag & dropped into the window.
	FileDropCallback func(files []string)
}
