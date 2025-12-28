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
	"github.com/richardwilkes/unison/internal/mac"
)

func (w *Window) frameRect() geom.Rect {
	if w.IsValid() {
		// TODO: This really shouldn't be necessary... but for some reason the old framework wanted a frame boundary
		contentRect := w.wnd.NativeView().Frame()
		frameRect := w.wnd.NativeWindow().FrameRectForContentRect(contentRect)
		left := contentRect.X - frameRect.X
		top := frameRect.Y + frameRect.Height - contentRect.Y - contentRect.Height
		right := frameRect.X + frameRect.Width - contentRect.X - contentRect.Width
		bottom := contentRect.Y - frameRect.Y
		return geom.NewRect(left, top, right-left, bottom-top)
	}
	return geom.NewRect(1, 1, 2, 2)
}

// ContentRect returns the boundaries in display coordinates of the window's content area.
func (w *Window) ContentRect() geom.Rect {
	if w.IsValid() {
		return w.wnd.ContentRect()
	}
	return geom.NewRect(0, 0, 1, 1)
}

// SetContentRect sets the boundaries of the frame of this window by converting the content rect into a suitable frame
// rect and then applying it to the window.
func (w *Window) SetContentRect(rect geom.Rect) {
	if w.IsValid() {
		rect = w.adjustContentRectForMinMax(rect)
		w.wnd.SetContentRect(rect)
	}
}

func (w *Window) convertRawMouseLocationForPlatform(where geom.Point) geom.Point {
	return where
}

// CurrentKeyModifiers returns the current key modifiers, which is usually the same as calling .LastKeyModifiers(),
// however, on platforms that are using native menus, this will also capture modifier changes that occurred while the
// menu is being displayed.
func (w *Window) CurrentKeyModifiers() Modifiers {
	return modifiersFromEventModifierFlags(mac.CurrentModifierFlags())
}
