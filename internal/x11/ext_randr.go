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
	"github.com/richardwilkes/toolbox/v2/errs"
)

const (
	rrOpQueryVersion = iota
	rrOpOldGetScreenInfo
	rrOpSetScreenConfig
	rrOpOldScreenChangeSelectInput
	rrOpSelectInput // v1.1 starts here
	rrOpGetScreenInfo
	rrOpGetScreenSizeRange // v1.2 starts here
	rrOpSetScreenSize
	rrOpGetScreenResources
	rrOpGetOutputInfo
	rrOpListOutputProperties
	rrOpQueryOutputProperty
	rrOpConfigureOutputProperty
	rrOpChangeOutputProperty
	rrOpDeleteOutputProperty
	rrOpGetOutputProperty
	rrOpCreateMode
	rrOpDestroyMode
	rrOpAddOutputMode
	rrOpDeleteOutputMode
	rrOpGetCrtcInfo
	rrOpSetCrtcConfig
	rrOpGetCrtcGammaSize
	rrOpGetCrtcGamma
	rrOpSetCrtcGamma
	rrOpGetScreenResourcesCurrent // v1.3 starts here
	rrOpSetCrtcTransform
	rrOpGetCrtcTransform
	rrOpGetPanning
	rrOpSetPanning
	rrOpSetOutputPrimary
	rrOpGetOutputPrimary
	rrOpGetProviders // v1.4 starts here
	rrOpGetProviderInfo
	rrOpSetProviderOffloadSink
	rrOpSetProviderOutputSource
	rrOpListProviderProperties
	rrOpQueryProviderProperty
	rrOpConfigureProviderProperty
	rrOpChangeProviderProperty
	rrOpDeleteProviderProperty
	rrOpGetProviderProperty
	rrOpGetMonitors // v1.5 starts here
	rrOpSetMonitor
	rrOpDeleteMonitor
	rrOpCreateLease // v1.6 starts here
	rrOpFreeLease
)

// Monitor holds information about a monitor.
type Monitor struct {
	Name      Atom
	Primary   bool
	Automatic bool
	X         int16
	Y         int16
	Width     uint16
	Height    uint16
	WidthMM   uint32
	HeightMM  uint32
}

// ExtRandr provides access to the XC-RANDR extension. Note that only those calls that I need have been implemented.
type ExtRandr struct {
	conn *Conn
	extensionInfo
}

func newExtRandr(conn *Conn) *ExtRandr {
	info := conn.hasExtension("RANDR", rrOpQueryVersion, false, 1, 6)
	return &ExtRandr{
		conn:          conn,
		extensionInfo: info,
	}
}

// GetMonitors returns information about the monitors for the specified root window. If active is true, only active
// monitors are returned.
func (e *ExtRandr) GetMonitors(root WindowID, active bool) ([]Monitor, error) {
	w := NewWriter(12)
	w.Byte(e.majorOpcode)
	w.Byte(rrOpGetMonitors)
	w.Uint16(3)
	w.WindowID(root)
	w.Bool(active)
	w.Zero(3)
	var monitors []Monitor
	if err := e.conn.sendNewRequest(newReplyRequest(w, func(r *Reader) {
		monitors = readGetMonitorsReply(r)
	})); err != nil {
		return nil, errs.NewWithCause("failed to get monitors", err)
	}
	return monitors, nil
}

func readGetMonitorsReply(r *Reader) []Monitor {
	r.Skip(12)
	numMonitors := int(r.Uint32())
	r.Skip(16)
	return ReadList(numMonitors, r, func(rr *Reader) Monitor {
		var m Monitor
		m.Name = rr.Atom()
		m.Primary = rr.Bool()
		m.Automatic = rr.Bool()
		numOutputs := int(rr.Uint16())
		m.X = rr.Int16()
		m.Y = rr.Int16()
		m.Width = rr.Uint16()
		m.Height = rr.Uint16()
		if m.WidthMM = rr.Uint32(); m.WidthMM == 0 {
			// Assume 96 DPI if we don't receive useful info
			m.WidthMM = uint32(float64(m.Width) * 25.4 / 96.0)
		}
		if m.HeightMM = rr.Uint32(); m.HeightMM == 0 {
			// Assume 96 DPI if we don't receive useful info
			m.HeightMM = uint32(float64(m.Height) * 25.4 / 96.0)
		}
		// Each MONITORINFO is followed by its own list of nOutput OUTPUT ids, which we don't use.
		rr.Skip(numOutputs * 4)
		return m
	})
}
