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

var _ Event = &ErrorEvent{}

// ErrorEvent is an error delivered as an event.
type ErrorEvent struct {
	Error error
}

// Process the event.
func (e *ErrorEvent) Process(_conn *Conn) {
	errs.Log(e.Error)
}
