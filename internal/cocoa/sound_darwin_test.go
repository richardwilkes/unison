// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package cocoa

import "testing"

// TestBeepRegistration validates that the NSBeep symbol resolves without actually playing the alert sound on every
// test run (RegisterLibFunc panics if the symbol is missing).
func TestBeepRegistration(t *testing.T) {
	ensureBeep()
	if nsBeep == nil {
		t.Error("NSBeep was not registered")
	}
}
