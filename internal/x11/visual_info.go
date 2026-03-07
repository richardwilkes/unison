// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package x11

type VisualID uint32

type VisualInfo struct {
	VisualID        VisualID
	Class           byte
	BitsPerRgbValue byte
	ColormapEntries uint16
	RedMask         uint32
	GreenMask       uint32
	BlueMask        uint32
}

func (v *VisualInfo) Read(r *XReader) {
	v.VisualID = VisualID(r.Uint32())
	v.Class = r.Byte()
	v.BitsPerRgbValue = r.Byte()
	v.ColormapEntries = r.Uint16()
	v.RedMask = r.Uint32()
	v.GreenMask = r.Uint32()
	v.BlueMask = r.Uint32()
	r.Skip(4)
}
