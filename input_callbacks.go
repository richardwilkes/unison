// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
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
)

// Constants for mouse buttons.
const (
	ButtonLeft   = int(glfw.MouseButtonLeft)
	ButtonRight  = int(glfw.MouseButtonRight)
	ButtonMiddle = int(glfw.MouseButtonMiddle)
)

// InputCallbacks holds the callbacks that client code can hook into for user input events.
type InputCallbacks struct {
	// GainedFocusCallback is called when the keyboard focus is gained.
	GainedFocusCallback func()
	// LostFocusCallback is called when the keyboard focus is lost.
	LostFocusCallback func()
	// MouseDownCallback is called when the mouse is pressed. Return true to stop further handling or false to propagate
	// up to parents.
	MouseDownCallback func(where Point, button, clickCount int, mod Modifiers) bool
	// MouseDragCallback is called when the mouse is dragged after being pressed. Return true to stop further handling
	// or false to propagate up to parents.
	MouseDragCallback func(where Point, button int, mod Modifiers) bool
	// MouseUpCallback is called when the mouse is released after being pressed. Return true to stop further handling or
	// false to propagate up to parents.
	MouseUpCallback func(where Point, button int, mod Modifiers) bool
	// MouseEnterCallback is called on mouse entry. Return true to stop further handling or false to propagate up to
	// parents.
	MouseEnterCallback func(where Point, mod Modifiers) bool
	// MouseMoveCallback is called when the mouse moves. Return true to stop further handling or false to propagate up
	// to parents.
	MouseMoveCallback func(where Point, mod Modifiers) bool
	// MouseExitCallback is called on mouse exit. Return true to stop further handling or false to propagate up to
	// parents.
	MouseExitCallback func() bool
	// MouseWheelCallback is called when the mouse wheel is rotated. Return true to stop further handling or false to
	// propagate up to parents.
	MouseWheelCallback func(where, delta Point, mod Modifiers) bool
	// KeyDownCallback is called when a key is pressed. Return true to stop further handling or false to propagate up to
	// parents.
	KeyDownCallback func(keyCode KeyCode, mod Modifiers, repeat bool) bool
	// KeyUpCallback is called when a key is released. Return true to stop further handling or false to propagate up to
	// parents.
	KeyUpCallback func(keyCode KeyCode, mod Modifiers) bool
	// RuneTypedCallback is called when a key is typed. Return true to stop further handling or false to propagate up to
	// parents.
	RuneTypedCallback func(ch rune) bool
	// FileDropCallback is called when files are drag & dropped from the OS.
	FileDropCallback func(files []string)
}
