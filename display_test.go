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
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
)

// TestDisplayPPI verifies the pixels-per-inch computation, in particular that the zero physical sizes commonly
// reported by virtual displays and VMs fall back to the default instead of producing an infinite result.
func TestDisplayPPI(t *testing.T) {
	c := check.New(t)

	// A 27" 4K panel: 3840 pixels across 596.7mm is ~163 PPI.
	c.Equal(163, displayPPI(3840, 596.7))

	// A 24" 1080p panel: 1920 pixels across 527mm is ~92 PPI.
	c.Equal(92, displayPPI(1920, 527))

	// Zero or negative physical sizes cannot be divided by and fall back to the default.
	c.Equal(defaultDisplayPPI, displayPPI(1920, 0))
	c.Equal(defaultDisplayPPI, displayPPI(1920, -1))

	// A zero pixel extent yields an implausible zero PPI and falls back to the default as well.
	c.Equal(defaultDisplayPPI, displayPPI(0, 527))
}
