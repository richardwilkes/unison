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
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/internal/plaf"
)

func (w *Window) frameRect() geom.Rect {
	if w.IsValid() {
		left, top, right, bottom := w.wnd.GetFrameSize()
		r := geom.NewRect(float32(left), float32(top), float32(right-left), float32(bottom-top))
		sx, sy := w.wnd.GetContentScale()
		r.X /= sx
		r.Y /= sy
		r.Width /= sx
		r.Height /= sy
		return r
	}
	return geom.NewRect(0, 0, 1, 1)
}

// ContentRect returns the boundaries in display coordinates of the window's content area.
func (w *Window) ContentRect() geom.Rect {
	if w.IsValid() {
		x, y := w.wnd.GetPos()
		width, height := w.wnd.GetSize()
		r := geom.NewRect(float32(x), float32(y), float32(width), float32(height))
		sx, sy := w.wnd.GetContentScale()
		r.X /= sx
		r.Y /= sy
		r.Width /= sx
		r.Height /= sy
		return r
	}
	return geom.NewRect(0, 0, 1, 1)
}

// SetContentRect sets the boundaries of the frame of this window by converting the content rect into a suitable frame
// rect and then applying it to the window.
func (w *Window) SetContentRect(rect geom.Rect) {
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
	}
}

func (w *Window) convertRawMouseLocationForPlatform(where geom.Point) geom.Point {
	if w.IsValid() {
		sx, sy := w.wnd.GetContentScale()
		where.X /= sx
		where.Y /= sy
	}
	return where
}

func (w *Window) keyCallbackForPlatform(_ *plaf.Window, key plaf.Key, _ int, action plaf.Action, mods plaf.ModifierKey) {
	if w.okToProcess() {
		w.commonKeyCallbackForPlatform(key, action, mods)
	}
}

// CurrentKeyModifiers returns the current key modifiers, which is usually the same as calling .LastKeyModifiers(),
// however, on platforms that are using native menus, this will also capture modifier changes that occurred while the
// menu is being displayed.
func (w *Window) CurrentKeyModifiers() Modifiers {
	return w.LastKeyModifiers()
}
