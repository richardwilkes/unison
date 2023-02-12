// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import "github.com/go-gl/glfw/v3.3/glfw"

func (w *Window) frameRect() Rect {
	left, top, right, bottom := w.wnd.GetFrameSize()
	r := NewRect(float32(left), float32(top), float32(right-left), float32(bottom-top))
	sx, sy := w.wnd.GetContentScale()
	r.X /= sx
	r.Y /= sy
	r.Width /= sx
	r.Height /= sy
	return r
}

// ContentRect returns the boundaries in display coordinates of the window's content area.
func (w *Window) ContentRect() Rect {
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

// SetContentRect sets the boundaries of the frame of this window by converting the content rect into a suitable frame
// rect and then applying it to the window.
func (w *Window) SetContentRect(rect Rect) {
	rect = w.adjustContentRectForMinMax(rect)
	w.lastContentRect = rect
	sx, sy := w.wnd.GetContentScale()
	rect.X *= sx
	rect.Y *= sy
	rect.Width *= sx
	rect.Height *= sy
	w.wnd.SetPos(int(rect.X), int(rect.Y))
	tx := int(rect.Width)
	ty := int(rect.Height)
	w.wnd.SetSize(tx, ty)
}

// Show makes the window visible, if it was previously hidden. If the window is already visible or is in full screen
// mode, this function does nothing.
func (w *Window) Show() {
	w.wnd.Show()
}

func (w *Window) convertMouseLocation(x, y float64) Point {
	pt := Point{X: float32(x), Y: float32(y)}
	sx, sy := w.wnd.GetContentScale()
	pt.X /= sx
	pt.Y /= sy
	return pt
}

func (w *Window) keyCallbackForGLFW(_ *glfw.Window, key glfw.Key, code int, action glfw.Action, mods glfw.ModifierKey) {
	if w.okToProcess() {
		w.commonKeyCallbackForGLFW(key, action, mods)
	}
}

// CurrentKeyModifiers returns the current key modifiers, which is usually the same as calling .LastKeyModifiers(),
// however, on platforms that are using native menus, this will also capture modifier changes that occurred while the
// menu is being displayed.
func (w *Window) CurrentKeyModifiers() Modifiers {
	return w.LastKeyModifiers()
}
