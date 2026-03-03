// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
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

func apiBeginStartup() error {
	// TODO: Need implementation
	return nil
}

func apiLateInit() {
	// TODO: Need implementation
}

func apiFinalFinishStartup() {
	// TODO: Need implementation
}

func apiTerminate() error {
	// TODO: Need implementation
	return nil
}

func apiBeep() {
	// TODO: Need implementation
}

func apiIsColorModeTrackingPossible() bool {
	// TODO: Need implementation
	return false
}

func apiIsDarkModeEnabled() bool {
	// TODO: Need implementation
	return false
}

func apiDoubleClickInterval() time.Duration {
	return 500 * time.Millisecond
}

func apiPollEvents() {
	// TODO: Need implementation
}

func apiWaitEvents() {
	// TODO: Need implementation
}

func apiPostEmptyEvent() {
	// TODO: Need implementation
}
