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
	"time"

	"github.com/go-gl/glfw/v3.3/glfw"
)

func (w *Window) frameRect() Rect {
	if w.IsValid() {
		left, top, right, bottom := w.wnd.GetFrameSize()
		r := NewRect(float32(left), float32(top), float32(right-left), float32(bottom-top))
		sx, sy := w.wnd.GetContentScale()
		r.X /= sx
		r.Y /= sy
		r.Width /= sx
		r.Height /= sy
		return r
	}
	return NewRect(0, 0, 1, 1)
}

// ContentRect returns the boundaries in display coordinates of the window's content area.
func (w *Window) ContentRect() Rect {
	if w.IsValid() {
		x, y := w.wnd.GetPos()
		width, height := w.wnd.GetSize()
		r := NewRect(float32(x), float32(y), float32(width), float32(height))
		sx, sy := w.wnd.GetContentScale()
		r.X /= sx
		r.Y /= sy
		r.Width /= sx
		r.Height /= sy
		return r
	}
	return NewRect(0, 0, 1, 1)
}

// SetContentRect sets the boundaries of the frame of this window by converting the content rect into a suitable frame
// rect and then applying it to the window.
func (w *Window) SetContentRect(rect Rect) {
	if w.IsValid() {
		rect = w.adjustContentRectForMinMax(rect)
		sx, sy := w.wnd.GetContentScale()
		rect.X *= sx
		rect.Y *= sy
		rect.Width *= sx
		rect.Height *= sy
		w.wnd.SetPos(int(rect.X), int(rect.Y))
		tx := int(rect.Width)
		ty := int(rect.Height)
		w.wnd.SetSize(tx, ty)

		// X11 responds asynchronously to window positioning and sizing requests. Due to this, we need to wait for it to
		// catch up, or subsequent code that is relying on the coordinates being updated will get the wrong information.
		// We do put a cap on the amount of time we are willing to wait, however, to ensure we don't hang should
		// something go wrong.
		for i := 0; i < 50; i++ {
			time.Sleep(time.Millisecond)
			if !w.IsValid() {
				return
			}
			nx, ny := w.wnd.GetSize()
			if nx == tx && ny == ty {
				return
			}
		}
	}
}

func (w *Window) convertMouseLocation(x, y float64) Point {
	if w.IsValid() {
		pt := Point{X: float32(x), Y: float32(y)}
		sx, sy := w.wnd.GetContentScale()
		pt.X /= sx
		pt.Y /= sy
		return pt
	}
	return Point{}
}

func (w *Window) keyCallbackForGLFW(_ *glfw.Window, key glfw.Key, _ int, action glfw.Action, mods glfw.ModifierKey) {
	if w.okToProcess() {
		if action == glfw.Release {
			mods &= ^keyToModifierForGLFW(key)
		} else {
			mods |= keyToModifierForGLFW(key)
		}
		w.commonKeyCallbackForGLFW(key, action, mods)
	}
}

// CurrentKeyModifiers returns the current key modifiers, which is usually the same as calling .LastKeyModifiers(),
// however, on platforms that are using native menus, this will also capture modifier changes that occurred while the
// menu is being displayed.
func (w *Window) CurrentKeyModifiers() Modifiers {
	return w.LastKeyModifiers()
}

func keyToModifierForGLFW(key glfw.Key) glfw.ModifierKey {
	switch key {
	case glfw.KeyLeftControl, glfw.KeyRightControl:
		return glfw.ModControl
	case glfw.KeyLeftShift, glfw.KeyRightShift:
		return glfw.ModShift
	case glfw.KeyLeftAlt, glfw.KeyRightAlt:
		return glfw.ModAlt
	case glfw.KeyLeftSuper, glfw.KeyRightSuper:
		return glfw.ModSuper
	default:
		return 0
	}
}
