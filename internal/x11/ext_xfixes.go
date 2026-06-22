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

// Shape kinds for SetWindowShapeRegion, as defined by the SHAPE extension.
const (
	ShapeKindBounding = iota
	ShapeKindClip
	ShapeKindInput
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

// CreateRegion creates a new, empty region and returns its ID.
func (e *ExtXFixes) CreateRegion() RegionID {
	id := nextXID[RegionID](e.conn)
	if id != 0 {
		w := NewWriter(8)
		w.Byte(e.majorOpcode)
		w.Byte(xfOpCreateRegion)
		w.Uint16(2)
		w.Uint32(uint32(id))
		if err := e.conn.sendNewRequest(newCheckedRequest(w)); err != nil {
			errs.Log(err)
			return 0
		}
	}
	return id
}

// DestroyRegion destroys the specified region.
func (e *ExtXFixes) DestroyRegion(region RegionID) {
	w := NewWriter(8)
	w.Byte(e.majorOpcode)
	w.Byte(xfOpDestroyRegion)
	w.Uint16(2)
	w.Uint32(uint32(region))
	if err := e.conn.sendNewRequest(newUncheckedRequest(w)); err != nil {
		errs.Log(err)
	}
}

// SetWindowShapeRegion sets the shape of the specified kind (one of the ShapeKind constants) on the given window to
// the specified region.
func (e *ExtXFixes) SetWindowShapeRegion(window WindowID, shapeKind byte, region RegionID) {
	w := NewWriter(20)
	w.Byte(e.majorOpcode)
	w.Byte(xfOpSetWindowShapeRegion)
	w.Uint16(5)
	w.WindowID(window)
	w.Byte(shapeKind)
	w.Zero(3)
	w.Int16(0) // x offset
	w.Int16(0) // y offset
	w.Uint32(uint32(region))
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
