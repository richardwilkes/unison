// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package x11

var _ Event = &errorEvent{}

// Event represents an X11 event.
type Event interface {
	protoReader
	ID() byte
}

type errorEvent struct {
	err error
}

func (e *errorEvent) protoRead(r *Reader) {
}

func (e *errorEvent) ID() byte {
	return 0
}
