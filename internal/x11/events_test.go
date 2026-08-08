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
