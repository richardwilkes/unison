// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package x11

var _ protoReader = &Visual{}

// VisualID holds an ID that refers to a Visual.
type VisualID uint32

// Visual holds the configuration of a screen's pixel composition for a specific bit depth.
type Visual struct {
	VisualID        VisualID
	Class           byte
	BitsPerRgbValue byte
	ColormapEntries uint16
	RedMask         uint32
	GreenMask       uint32
	BlueMask        uint32
}

func (v *Visual) protoRead(r *protoBufferReader) {
	v.VisualID = VisualID(r.uint32())
	v.Class = r.byte()
	v.BitsPerRgbValue = r.byte()
	v.ColormapEntries = r.uint16()
	v.RedMask = r.uint32()
	v.GreenMask = r.uint32()
	v.BlueMask = r.uint32()
	r.skip(4)
}
