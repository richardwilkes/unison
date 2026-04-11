// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package x11

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

// NewScreen reads a Screen from the specified Reader and returns it.
func NewScreen(r *Reader) *Screen {
	var s Screen
	s.Root = WindowID(r.Uint32())
	s.DefaultColorMap = ColorMapID(r.Uint32())
	s.WhitePixel = r.Uint32()
	s.BlackPixel = r.Uint32()
	s.CurrentInputMasks = r.Uint32()
	s.WidthInPixels = r.Uint16()
	s.HeightInPixels = r.Uint16()
	s.WidthInMillimeters = r.Uint16()
	s.HeightInMillimeters = r.Uint16()
	if s.WidthInMillimeters == 0 || s.HeightInMillimeters == 0 {
		// Assume 96 DPI if we don't receive useful info
		s.WidthInMillimeters = uint16(float64(s.WidthInPixels) * 25.4 / 96.0)
		s.HeightInMillimeters = uint16(float64(s.HeightInPixels) * 25.4 / 96.0)
	}
	s.MinInstalledMaps = r.Uint16()
	s.MaxInstalledMaps = r.Uint16()
	s.RootVisual = VisualID(r.Uint32())
	s.BackingStores = r.Byte()
	s.SaveUnders = r.Bool()
	s.RootDepth = r.Byte()
	s.AllowedDepths = ReadList(int(r.Byte()), r, NewDepth)
	return &s
}
