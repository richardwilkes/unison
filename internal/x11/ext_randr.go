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

//nolint:unused // All available opcodes are defined here, even if not all are used by my code.
const (
	rrOpQueryVersion = iota
	rrOpOldGetScreenInfo
	rrOpSetScreenConfig
	rrOpOldScreenChangeSelectInput
	// v1.1
	rrOpSelectInput
	rrOpGetScreenInfo
	// v1.2
	rrOpGetScreenSizeRange
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
	// v1.3
	rrOpGetScreenResourcesCurrent
	rrOpSetCrtcTransform
	rrOpGetCrtcTransform
	rrOpGetPanning
	rrOpSetPanning
	rrOpSetOutputPrimary
	rrOpGetOutputPrimary
	// v1.4
	rrOpGetProviders
	rrOpGetProviderInfo
	rrOpSetProviderOffloadSink
	rrOpSetProviderOutputSource
	rrOpListProviderProperties
	rrOpQueryProviderProperty
	rrOpConfigureProviderProperty
	rrOpChangeProviderProperty
	rrOpDeleteProviderProperty
	rrOpGetProviderProperty
	// v1.5
	rrOpGetMonitors
	rrOpSetMonitor
	rrOpDeleteMonitor
	// v1.6
	rrOpCreateLease
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
	info := conn.hasExtension32("RANDR", 1, 6)
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
		r.Skip(12)
		numMonitors := int(r.Uint32())
		numOutputs := int(r.Uint32())
		r.Skip(12)
		monitors = ReadList(numMonitors, r, func(rr *Reader) Monitor {
			var m Monitor
			m.Name = rr.Atom()
			m.Primary = rr.Bool()
			m.Automatic = rr.Bool()
			rr.Skip(2)
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
			return m
		})
		r.Skip(numOutputs * 4)
	})); err != nil {
		return nil, errs.NewWithCause("failed to get monitors", err)
	}
	return monitors, nil
}
