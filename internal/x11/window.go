// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package x11

// WindowNone is a special WindowID value that represents the absence of a window.
const WindowNone = WindowID(0)

// Constants for X11 window classes.
const (
	WindowClassCopyFromParent = iota
	WindowClassInputOutput
	WindowClassInputOnly
)

// Constants for X11 window bit masks.
const (
	WindowBitMaskBackPixMap = 1 << iota
	WindowBitMaskBackPixel
	WindowBitMaskBorderPixMap
	WindowBitMaskBorderPixel
	WindowBitMaskBitGravity
	WindowBitMaskWinGravity
	WindowBitMaskBackingStore
	WindowBitMaskBackingPlanes
	WindowBitMaskBackingPixel
	WindowBitMaskOverrideRedirect
	WindowBitMaskSaveUnder
	WindowBitMaskEventMask
	WindowBitMaskDontPropagate
	WindowBitMaskColorMap
	WindowBitMaskCursor
)

// Constants for X11 event masks.
const (
	EventMaskKeyPress = 1 << iota
	EventMaskKeyRelease
	EventMaskButtonPress
	EventMaskButtonRelease
	EventMaskEnterWindow
	EventMaskLeaveWindow
	EventMaskPointerMotion
	EventMaskPointerMotionHint
	EventMaskButton1Motion
	EventMaskButton2Motion
	EventMaskButton3Motion
	EventMaskButton4Motion
	EventMaskButton5Motion
	EventMaskButtonMotion
	EventMaskKeymapState
	EventMaskExposure
	EventMaskVisibilityChange
	EventMaskStructureNotify
	EventMaskResizeRedirect
	EventMaskSubstructureNotify
	EventMaskSubstructureRedirect
	EventMaskFocusChange
	EventMaskPropertyChange
	EventMaskColormapChange
	EventMaskOwnerGrabButton
	EventMaskNone = 0
)

// WindowID holds an ID that refers to a Window.
type WindowID uint32

// Window represents an X11 window.
type Window struct {
	ID WindowID
}

// PixMapID holds an ID that refers to a PixMap.
type PixMapID uint32

// CursorID holds an ID that refers to a Cursor.
type CursorID uint32

// WindowAttributes holds the attributes that can be set on a window.
type WindowAttributes struct {
	BackgroundPixMap   PixMapID
	BackgroundPixel    uint32
	BorderPixMap       PixMapID
	BorderPixel        uint32
	BitGravity         uint32
	WinGravity         uint32
	BackingStore       uint32
	BackingPlanes      uint32
	BackingPixel       uint32
	EventMask          uint32
	DoNotPropagateMask uint32
	ColorMap           ColorMapID
	Cursor             CursorID
	OverrideRedirect   bool
	SaveUnder          bool
}

func (a *WindowAttributes) toValues(mask uint32) []uint32 {
	list := make([]uint32, 0, 15)
	if mask&WindowBitMaskBackPixMap != 0 {
		list = append(list, uint32(a.BackgroundPixMap))
	}
	if mask&WindowBitMaskBackPixel != 0 {
		list = append(list, a.BackgroundPixel)
	}
	if mask&WindowBitMaskBorderPixMap != 0 {
		list = append(list, uint32(a.BorderPixMap))
	}
	if mask&WindowBitMaskBorderPixel != 0 {
		list = append(list, a.BorderPixel)
	}
	if mask&WindowBitMaskBitGravity != 0 {
		list = append(list, a.BitGravity)
	}
	if mask&WindowBitMaskWinGravity != 0 {
		list = append(list, a.WinGravity)
	}
	if mask&WindowBitMaskBackingStore != 0 {
		list = append(list, a.BackingStore)
	}
	if mask&WindowBitMaskBackingPlanes != 0 {
		list = append(list, a.BackingPlanes)
	}
	if mask&WindowBitMaskBackingPixel != 0 {
		list = append(list, a.BackingPixel)
	}
	if mask&WindowBitMaskOverrideRedirect != 0 {
		if a.OverrideRedirect {
			list = append(list, 1)
		} else {
			list = append(list, 0)
		}
	}
	if mask&WindowBitMaskSaveUnder != 0 {
		if a.SaveUnder {
			list = append(list, 1)
		} else {
			list = append(list, 0)
		}
	}
	if mask&WindowBitMaskEventMask != 0 {
		list = append(list, a.EventMask)
	}
	if mask&WindowBitMaskDontPropagate != 0 {
		list = append(list, a.DoNotPropagateMask)
	}
	if mask&WindowBitMaskColorMap != 0 {
		list = append(list, uint32(a.ColorMap))
	}
	if mask&WindowBitMaskCursor != 0 {
		list = append(list, uint32(a.Cursor))
	}
	return list
}
