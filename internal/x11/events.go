// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package x11

var _ Event = &ErrorEvent{}

// Event represents an X11 event.
type Event interface {
	isEvent() // marker method to indicate that this type is an Event
	protoReader
}

// ErrorEvent represents an error that occurred while processing a request or event.
type ErrorEvent struct {
	Error error
}

func (e *ErrorEvent) isEvent() {}

func (e *ErrorEvent) protoRead(r *Reader) {
}
