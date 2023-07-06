// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"time"
)

func platformEarlyInit() {
}

func platformLateInit() {
}

func platformFinishedStartup() {
}

func platformBeep() {
	// TODO: Need implementation
}

func platformIsDarkModeTrackingPossible() bool {
	return false
}

func platformIsDarkModeEnabled() bool {
	return false
}

func platformDoubleClickInterval() time.Duration {
	return 500 * time.Millisecond
}
