// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package x11

var _ Event = &MappingNotifyEvent{}

// MappingNotifyEvent represents an X11 MappingNotify event.
type MappingNotifyEvent struct {
	Sequence     uint16
	Code         byte
	Request      byte
	FirstKeycode byte
	Count        byte
}

func newMappingNotifyEvent(r *Reader) Event {
	var e MappingNotifyEvent
	e.Code = r.Byte()
	r.Skip(1)
	e.Sequence = r.Uint16()
	e.Request = r.Byte()
	e.FirstKeycode = r.Byte()
	e.Count = r.Byte()
	r.Skip(1)
	return &e
}

// Process the event.
func (e *MappingNotifyEvent) Process(_conn *Conn) {
	// TODO: Implement
}
