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
	"image"

	"github.com/richardwilkes/toolbox/v2/geom"
)

type apiWindow struct {
	// TODO: Need implementation
}

func (w *Window) apiInit(cfg *WindowConfig) error {
	// TODO: Need implementation
	return nil
}

func (w *Window) apiSetTitle(title string) {
	// TODO: Need implementation
}

func (w *Window) apiSetTitleIcons(_images []*image.NRGBA) {
	// TODO: Need implementation
}

func (w *Window) apiDisplay() *Display {
	// TODO: Need implementation
	return nil
}

func (w *Window) apiFrameRect() geom.Rect {
	// TODO: Need to fix implementation
	// left, top, right, bottom := w.wnd.GetFrameSize()
	// r := geom.NewRect(float32(left), float32(top), float32(right-left), float32(bottom-top))
	// sx, sy := w.wnd.GetContentScale()
	// r.X /= sx
	// r.Y /= sy
	// r.Width /= sx
	// r.Height /= sy
	// return r
	return geom.Rect{}
}

func (w *Window) apiFrameRectForContentRect(contentRect geom.Rect) geom.Rect {
	// TODO: Need implementation
	return contentRect
}

func (w *Window) apiEnsureOnDisplay() {
	// TODO: Need implementation
}

func (w *Window) apiContentRect() geom.Rect {
	// TODO: Need to fix implementation
	// x, y := w.wnd.GetPos()
	// width, height := w.wnd.GetSize()
	// r := geom.NewRect(float32(x), float32(y), float32(width), float32(height))
	// sx, sy := w.wnd.GetContentScale()
	// r.X /= sx
	// r.Y /= sy
	// r.Width /= sx
	// r.Height /= sy
	// return r
	return geom.Rect{}
}

func (w *Window) apiContentRectForFrameRect(frameRect geom.Rect) geom.Rect {
	// TODO: Need implementation
	return frameRect
}

func (w *Window) apiSetContentRect(rect geom.Rect) {
	// TODO: Need to fix implementation
	// sx, sy := w.wnd.GetContentScale()
	// rect.X *= sx
	// rect.Y *= sy
	// rect.Width *= sx
	// rect.Height *= sy
	// w.wnd.SetPos(int(rect.X), int(rect.Y))
	// tx := int(rect.Width)
	// ty := int(rect.Height)
	// w.wnd.SetSize(tx, ty)

	// X11 responds asynchronously to window positioning and sizing requests. Due to this, we need to wait for it to
	// catch up, or subsequent code that is relying on the coordinates being updated will get the wrong information.
	// We do put a cap on the amount of time we are willing to wait, however, to ensure we don't hang should
	// something go wrong.
	// for i := 0; i < 50; i++ {
	// 	time.Sleep(time.Millisecond)
	// 	if !w.IsValid() {
	// 		return
	// 	}
	// 	nx, ny := w.wnd.GetSize()
	// 	if nx == tx && ny == ty {
	// 		return
	// 	}
	// }
}

func (w *Window) apiConvertRawMouse(where geom.Point) geom.Point {
	// TODO: Need to fix implementation
	// sx, sy := w.wnd.GetContentScale()
	// where.X /= sx
	// where.Y /= sy
	// return where
	return geom.Point{}
}

func (w *Window) apiCurrentKeyModifiers() Modifiers {
	// TODO: Need to fix implementation
	return w.LastKeyModifiers()
}

func (w *Window) apiUpdateCursorImage() {
	// TODO: Need implementation
}

func (w *Window) apiCursorInContentArea() bool {
	// TODO: Need implementation
	return false
}

func (w *Window) apiCursorPosition() geom.Point {
	// TODO: Need implementation
	return geom.Point{}
}

func (w *Window) apiBackingScale() geom.Point {
	// TODO: Need implementation
	return geom.NewPoint(1, 1)
}

func (w *Window) apiMinimize() {
	// TODO: Need implementation
}

func (w *Window) apiMaximize() {
	// TODO: Need implementation
}

func (w *Window) apiAcquireFocus() {
	// TODO: Need implementation
}

func (w *Window) apiVisible() bool {
	// TODO: Need implementation
	return false
}

func (w *Window) apiShow() {
	// TODO: Need implementation
}

func (w *Window) apiHide() {
	// TODO: Need implementation
}

func (w *Window) apiDestroy() {
	// TODO: Need implementation
}

func (w *Window) keyCallbackForPlatform(_ *Window, key KeyCode, _ int, action Action, mods Modifiers) {
	// TODO: Is this actually needed? If so, needs fixups to work with the new API.
	// if w.okToProcess() {
	// 	if action == Release {
	// 		mods &= ^keyToModifierForPlatform(key)
	// 	} else {
	// 		mods |= keyToModifierForPlatform(key)
	// 	}
	// 	w.commonKeyCallbackForPlatform(key, action, mods)
	// }
}

func keyToModifierForPlatform(key KeyCode) Modifiers {
	// TODO: Is this actually needed? If so, needs fixups to work with the new API.
	// switch key {
	// case KeyLeftControl, KeyRightControl:
	// 	return ModControl
	// case KeyLeftShift, KeyRightShift:
	// 	return ModShift
	// case KeyLeftAlt, KeyRightAlt:
	// 	return ModAlt
	// case KeyLeftSuper, KeyRightSuper:
	// 	return ModSuper
	// default:
	return 0
	// }
}
