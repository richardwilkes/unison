// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package x11

var _ protoReader = &Screen{}

// Screen holds the configuration of a monitor.
type Screen struct {
	AllowedDepths       []*Depth
	Root                WindowID
	DefaultColorMap     ColorMapID
	WhitePixel          uint32
	BlackPixel          uint32
	CurrentInputMasks   uint32
	WidthInPixels       uint16
	HeightInPixels      uint16
	WidthInMillimeters  uint16
	HeightInMillimeters uint16
	MinInstalledMaps    uint16
	MaxInstalledMaps    uint16
	RootVisual          VisualID
	BackingStores       byte
	SaveUnders          bool
	RootDepth           byte
}

func (s *Screen) protoRead(r *protoBufferReader) {
	s.Root = WindowID(r.uint32())
	s.DefaultColorMap = ColorMapID(r.uint32())
	s.WhitePixel = r.uint32()
	s.BlackPixel = r.uint32()
	s.CurrentInputMasks = r.uint32()
	s.WidthInPixels = r.uint16()
	s.HeightInPixels = r.uint16()
	s.WidthInMillimeters = r.uint16()
	s.HeightInMillimeters = r.uint16()
	s.MinInstalledMaps = r.uint16()
	s.MaxInstalledMaps = r.uint16()
	s.RootVisual = VisualID(r.uint32())
	s.BackingStores = r.byte()
	s.SaveUnders = r.bool()
	s.RootDepth = r.byte()
	s.AllowedDepths = readProtoList[*Depth](int(r.byte()), r)
}
