// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package x11

type SetupInfo struct {
	ReleaseNumber            uint32
	ResourceIdBase           uint32
	ResourceIdMask           uint32
	MotionBufferSize         uint32
	VendorLen                uint16
	MaximumRequestLength     uint16
	RootsLen                 byte
	PixmapFormatsLen         byte
	ImageByteOrder           byte
	BitmapFormatBitOrder     byte
	BitmapFormatScanlineUnit byte
	BitmapFormatScanlinePad  byte
	MinKeycode               byte
	MaxKeycode               byte
	Vendor                   string
	PixmapFormats            []*Format
	Roots                    []*ScreenInfo
}

func (s *SetupInfo) Read(r *XReader) {
	s.ReleaseNumber = r.Uint32()
	s.ResourceIdBase = r.Uint32()
	s.ResourceIdMask = r.Uint32()
	s.MotionBufferSize = r.Uint32()
	vendorLen := r.Uint16()
	s.MaximumRequestLength = r.Uint16()
	s.RootsLen = r.Byte()
	s.PixmapFormatsLen = r.Byte()
	s.ImageByteOrder = r.Byte()
	s.BitmapFormatBitOrder = r.Byte()
	s.BitmapFormatScanlineUnit = r.Byte()
	s.BitmapFormatScanlinePad = r.Byte()
	s.MinKeycode = r.Byte()
	s.MaxKeycode = r.Byte()
	r.Skip(4)
	s.Vendor = r.String(int(vendorLen))
	r.SkipTo4ByteAlignment()
	s.PixmapFormats = ReadList[*Format](int(s.PixmapFormatsLen), r)
	s.Roots = ReadList[*ScreenInfo](int(s.RootsLen), r)
}
