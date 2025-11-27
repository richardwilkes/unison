// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
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

	"github.com/richardwilkes/unison/internal/mac"
)

func platformLateInit() {
	mac.InstallSystemThemeChangedCallback(ThemeChanged)
	mac.SetActivationPolicy(mac.ActivationPolicyRegular)
}

func platformBeep() {
	mac.Beep()
}

func platformIsDarkModeTrackingPossible() bool {
	return true
}

func platformIsDarkModeEnabled() bool {
	return mac.IsDarkModeEnabled()
}

func platformDoubleClickInterval() time.Duration {
	return mac.DoubleClickInterval()
}
