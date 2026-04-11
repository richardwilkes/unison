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
	"log/slog"
	"sync"

	"github.com/richardwilkes/toolbox/v2/errs"
)

// Opcodes for RANDR requests.
const (
	RRQueryVersionOpCode = iota
	RROldGetScreenInfoOpCode
	RRSetScreenConfigOpCode
	RROldScreenChangeSelectInputOpCode
	// v1.1
	RRSelectInputOpCode
	RRGetScreenInfoOpCode
	// v1.2
	RRGetScreenSizeRangeOpCode
	RRSetScreenSizeOpCode
	RRGetScreenResourcesOpCode
	RRGetOutputInfoOpCode
	RRListOutputPropertiesOpCode
	RRQueryOutputPropertyOpCode
	RRConfigureOutputPropertyOpCode
	RRChangeOutputPropertyOpCode
	RRDeleteOutputPropertyOpCode
	RRGetOutputPropertyOpCode
	RRCreateModeOpCode
	RRDestroyModeOpCode
	RRAddOutputModeOpCode
	RRDeleteOutputModeOpCode
	RRGetCrtcInfoOpCode
	RRSetCrtcConfigOpCode
	RRGetCrtcGammaSizeOpCode
	RRGetCrtcGammaOpCode
	RRSetCrtcGammaOpCode
	// v1.3
	RRGetScreenResourcesCurrentOpCode
	RRSetCrtcTransformOpCode
	RRGetCrtcTransformOpCode
	RRGetPanningOpCode
	RRSetPanningOpCode
	RRSetOutputPrimaryOpCode
	RRGetOutputPrimaryOpCode
	// v1.4
	RRGetProvidersOpCode
	RRGetProviderInfoOpCode
	RRSetProviderOffloadSinkOpCode
	RRSetProviderOutputSourceOpCode
	RRListProviderPropertiesOpCode
	RRQueryProviderPropertyOpCode
	RRConfigureProviderPropertyOpCode
	RRChangeProviderPropertyOpCode
	RRDeleteProviderPropertyOpCode
	RRGetProviderPropertyOpCode
	// v1.5
	RRGetMonitorsOpCode
	RRSetMonitorOpCode
	RRDeleteMonitorOpCode
	// v1.6
	RRCreateLeaseOpCode
	RRFreeLeaseOpCode
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
	lock sync.RWMutex
	extensionInfo
}

// Available determines if the extension is available on the server. No other methods on this object may be called if
// false is returned for available.
func (e *ExtRandr) Available() (available bool, majorVersion, minorVersion uint32) {
	e.lock.RLock()
	info := e.extensionInfo
	e.lock.RUnlock()
	if !info.checked {
		info = e.conn.hasExtension("RANDR")
		w := NewWriter(8)
		w.Byte(info.majorOpcode)
		w.Byte(RRQueryVersionOpCode)
		w.Uint16(3)
		w.Uint32(1) // Major version max
		w.Uint32(6) // Minor version max
		if err := e.conn.sendNewRequest(newReplyRequest("RRQueryVersion", w, func(r *Reader) {
			r.Skip(8)
			info.majorVersion = r.Uint32()
			info.minorVersion = r.Uint32()
		})); err != nil {
			slog.Error("failed to query RANDR version", "error", err)
		}
		e.lock.Lock()
		e.extensionInfo = info
		e.lock.Unlock()
	}
	return info.present, info.majorVersion, info.minorVersion
}

// GetMonitors returns information about the monitors for the specified root window. If active is true, only active
// monitors are returned.
func (e *ExtRandr) GetMonitors(root WindowID, active bool) ([]Monitor, error) {
	w := NewWriter(12)
	w.Byte(e.majorOpcode)
	w.Byte(RRGetMonitorsOpCode)
	w.Uint16(3)
	w.WindowID(root)
	w.Bool(active)
	w.Zero(3)
	var monitors []Monitor
	if err := e.conn.sendNewRequest(newReplyRequest("RRGetMonitors", w, func(r *Reader) {
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
