// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package x11

var _ Event = &KeymapNotifyEvent{}

// KeymapNotifyEvent represents an X11 KeymapNotify event.
type KeymapNotifyEvent struct {
	Code byte
	Keys [31]byte
}

func newKeymapNotifyEvent(r *Reader) Event {
	var e KeymapNotifyEvent
	e.Code = r.Byte()
	r.IntoBytes(e.Keys[:])
	return &e
}

// Process the event.
func (e *KeymapNotifyEvent) Process(_conn *Conn) {
	// TODO: Implement
}
