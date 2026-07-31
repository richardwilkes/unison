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
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
)

// writeMonitorInfo appends a RANDR 1.5 MONITORINFO to w: 24 fixed bytes followed by the monitor's inline list of
// OUTPUT ids.
func writeMonitorInfo(w *Writer, m Monitor, outputs []uint32) {
	w.Atom(m.Name)
	w.Bool(m.Primary)
	w.Bool(m.Automatic)
	w.Uint16(uint16(len(outputs)))
	w.Int16(m.X)
	w.Int16(m.Y)
	w.Uint16(m.Width)
	w.Uint16(m.Height)
	w.Uint32(m.WidthMM)
	w.Uint32(m.HeightMM)
	w.Uint32Slice(outputs)
}

func TestReadGetMonitorsReply(t *testing.T) {
	c := check.New(t)
	expected := []Monitor{
		{
			Name:      Atom(100),
			Primary:   true,
			Automatic: true,
			X:         0,
			Y:         0,
			Width:     1920,
			Height:    1080,
			WidthMM:   509,
			HeightMM:  286,
		},
		{
			Name:      Atom(101),
			Automatic: true,
			X:         1920,
			Y:         -120,
			Width:     2560,
			Height:    1440,
			WidthMM:   597,
			HeightMM:  336,
		},
		{
			Name:     Atom(102),
			X:        4480,
			Y:        0,
			Width:    1024,
			Height:   768,
			WidthMM:  270,
			HeightMM: 203,
		},
	}
	outputs := [][]uint32{{201}, {202, 203}, {}}
	w := NewWriter(32)
	w.Byte(1) // reply
	w.Zero(1)
	w.Uint16(1)                     // sequence
	w.Uint32(0)                     // length (unused by the parser)
	w.Uint32(12345678)              // timestamp
	w.Uint32(uint32(len(expected))) // nMonitors
	var numOutputs uint32
	for _, ids := range outputs {
		numOutputs += uint32(len(ids))
	}
	w.Uint32(numOutputs)
	w.Zero(12)
	for i, m := range expected {
		writeMonitorInfo(w, m, outputs[i])
	}
	monitors := readGetMonitorsReply(NewReader(w.Retrieve()))
	c.Equal(expected, monitors)
}

func TestReadGetMonitorsReplyMissingMMFallsBackTo96DPI(t *testing.T) {
	c := check.New(t)
	w := NewWriter(32)
	w.Byte(1)
	w.Zero(1)
	w.Uint16(1)
	w.Uint32(0)
	w.Uint32(0)
	w.Uint32(1) // nMonitors
	w.Uint32(1) // nOutputs
	w.Zero(12)
	writeMonitorInfo(w, Monitor{Name: Atom(7), Width: 960, Height: 480}, []uint32{300})
	monitors := readGetMonitorsReply(NewReader(w.Retrieve()))
	c.Equal(1, len(monitors))
	c.Equal(uint32(float64(960)*25.4/96.0), monitors[0].WidthMM)
	c.Equal(uint32(float64(480)*25.4/96.0), monitors[0].HeightMM)
}
