// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package x11

import (
	"encoding/binary"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
)

// TestNewColormapNotifyEvent verifies that a ColormapNotify event is decoded at the offsets the protocol specifies.
// The event has only a single unused byte between the event code and the sequence number, as every other fixed-layout
// event does; skipping three instead shifted Sequence, Window, Colormap, New and State two bytes each, so every
// delivered ColormapNotify silently carried garbage.
func TestNewColormapNotifyEvent(t *testing.T) {
	c := check.New(t)

	// A complete 32-byte ColormapNotify event as the server sends it.
	data := make([]byte, 32)
	data[0] = eventCodeColormapNotify
	data[1] = 0xAA // unused; must not be mistaken for part of the sequence number
	binary.LittleEndian.PutUint16(data[2:4], 0x1234)
	binary.LittleEndian.PutUint32(data[4:8], 0x00445566)  // window
	binary.LittleEndian.PutUint32(data[8:12], 0x00778899) // colormap
	data[12] = 1                                          // new
	data[13] = 1                                          // state (ColormapInstalled)

	f, ok := newEventMap()[eventCodeColormapNotify]
	c.True(ok, "ColormapNotify must have a registered decoder")
	e, ok := f(NewReader(data)).(*ColormapNotifyEvent)
	c.True(ok)
	c.Equal(byte(eventCodeColormapNotify), e.Code)
	c.Equal(uint16(0x1234), e.Sequence)
	c.Equal(WindowID(0x00445566), e.Window)
	c.Equal(ColorMapID(0x00778899), e.Colormap)
	c.True(e.New)
	c.Equal(byte(1), e.State)

	// A colormap being uninstalled reports New as false, which the old offsets could never produce correctly.
	data[12] = 0
	data[13] = 0 // ColormapUninstalled
	e, ok = f(NewReader(data)).(*ColormapNotifyEvent)
	c.True(ok)
	c.False(e.New)
	c.Equal(byte(0), e.State)
}

// TestSynthetic verifies that an event another client sent with SendEvent is told apart from one the X server generated
// itself, and that the bit which says so does not disturb the decoding. The two differ in what they mean: a window
// manager sends a synthetic ConfigureNotify when it has moved a window without resizing it, and that one carries the
// window's position relative to the root rather than relative to its parent.
func TestSynthetic(t *testing.T) {
	c := check.New(t)
	c.False(Synthetic(nil), "there is nothing synthetic about no event at all")

	// A complete 32-byte ConfigureNotify event, first as the server sends one and then as a window manager sends one.
	data := make([]byte, 32)
	data[0] = eventCodeConfigureNotify
	binary.LittleEndian.PutUint16(data[2:4], 0x1234)
	binary.LittleEndian.PutUint32(data[4:8], 0x00445566)  // event window
	binary.LittleEndian.PutUint32(data[8:12], 0x00445566) // window
	binary.LittleEndian.PutUint16(data[16:18], uint16(0xFFC0))
	binary.LittleEndian.PutUint16(data[18:20], 100)
	binary.LittleEndian.PutUint16(data[20:22], 800)
	binary.LittleEndian.PutUint16(data[22:24], 600)

	f, ok := newEventMap()[eventCodeConfigureNotify]
	c.True(ok, "ConfigureNotify must have a registered decoder")
	e, ok := f(NewReader(data)).(*ConfigureNotifyEvent)
	c.True(ok)
	c.False(Synthetic(e))
	c.Equal(int16(-64), e.X)
	c.Equal(int16(100), e.Y)

	data[0] |= eventSyntheticFlag
	e, ok = f(NewReader(data)).(*ConfigureNotifyEvent)
	c.True(ok, "the event must still be decoded as the ConfigureNotify it is")
	c.True(Synthetic(e))
	c.Equal(int16(-64), e.X, "and must be decoded from the same offsets")
	c.Equal(int16(100), e.Y)
	c.Equal(uint16(800), e.Width)
	c.Equal(uint16(600), e.Height)
	c.Equal(WindowID(0x00445566), e.Window)
}
