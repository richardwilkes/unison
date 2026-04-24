// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package x11

import "github.com/richardwilkes/toolbox/v2/errs"

const (
	xfOpQueryVersion = iota
	xfOpChangeSaveSet
	xfOpSelectSelectionInput
	xfOpSelectCursorInput
	xfOpGetCursorImage
	xfOpCreateRegion
	xfOpCreateRegionFromBitmap
	xfOpCreateRegionFromWindow
	xfOpCreateRegionFromGC
	xfOpCreateRegionFromPicture
	xfOpDestroyRegion
	xfOpSetRegion
	xfOpCopyRegion
	xfOpUnionRegion
	xfOpIntersectRegion
	xfOpSubtractRegion
	xfOpInvertRegion
	xfOpTranslateRegion
	xfOpRegionExtents
	xfOpFetchRegion
	xfOpSetGCClipRegion
	xfOpSetWindowShapeRegion
	xfOpSetPictureClipRegion
	xfOpSetCursorName
	xfOpGetCursorName
	xfOpGetCursorImageAndName
	xfOpChangeCursor
	xfOpChangeCursorByName
	xfOpExpandRegion
	xfOpHideCursor
	xfOpShowCursor
	xfOpCreatePointerBarrier
	xfOpDeletePointerBarrier
	xfOpSetClientDisconnectMode
	xfOpGetClientDisconnectMode
)

// ExtXFixes provides access to the XFIXES extension. Note that only those calls that I need have been implemented.
type ExtXFixes struct {
	conn *Conn
	extensionInfo
}

func newExtXFixes(conn *Conn) *ExtXFixes {
	info := conn.hasExtension("XFIXES", xfOpQueryVersion, false, 5, 0)
	return &ExtXFixes{
		conn:          conn,
		extensionInfo: info,
	}
}

// HideCursor hides the cursor on the specified window.
func (e *ExtXFixes) HideCursor(window WindowID) {
	w := NewWriter(8)
	w.Byte(e.majorOpcode)
	w.Byte(xfOpHideCursor)
	w.Uint16(2)
	w.WindowID(window)
	if err := e.conn.sendNewRequest(newCheckedRequest(w)); err != nil {
		errs.Log(err)
	}
}

// ShowCursor shows the cursor on the specified window.
func (e *ExtXFixes) ShowCursor(window WindowID) {
	w := NewWriter(8)
	w.Byte(e.majorOpcode)
	w.Byte(xfOpShowCursor)
	w.Uint16(2)
	w.WindowID(window)
	if err := e.conn.sendNewRequest(newCheckedRequest(w)); err != nil {
		errs.Log(err)
	}
}
