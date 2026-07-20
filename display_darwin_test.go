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
	"github.com/richardwilkes/toolbox/v2/geom"
)

// TestUsableInWindowUnits verifies that on this platform the usable rect is handed to cross-platform window math
// unchanged, since macOS display rects are already in the logical point space window rects use.
func TestUsableInWindowUnits(t *testing.T) {
	c := check.New(t)
	d := &Display{
		Usable: geom.NewRect(0, 25, 1728, 1054),
		Scale:  geom.NewPoint(2, 2),
	}
	c.Equal(d.Usable, d.usableInWindowUnits())
}
