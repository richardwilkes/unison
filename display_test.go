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

// TestBestDisplayAmong verifies that a rect belongs to the display whose usable area holds all of it, or else to the
// one it overlaps most, rather than to the one holding its top-left corner.
func TestBestDisplayAmong(t *testing.T) {
	c := check.New(t)
	left := &Display{Frame: geom.NewRect(0, 0, 1920, 1080), Usable: geom.NewRect(0, 24, 1920, 1056), Primary: true}
	right := &Display{Frame: geom.NewRect(1920, 0, 1920, 1080), Usable: geom.NewRect(1920, 0, 1920, 1080)}
	displays := []*Display{left, right}

	// A rect within one display's usable area belongs to it.
	c.Equal(left, bestDisplayAmong(displays, geom.NewRect(100, 100, 800, 600)))
	c.Equal(right, bestDisplayAmong(displays, geom.NewRect(2000, 100, 800, 600)))

	// A rect whose top-left corner lies on the left display while most of it lies on the right belongs to the right
	// one. The corner alone used to decide for the left.
	c.Equal(right, bestDisplayAmong(displays, geom.NewRect(1800, 100, 800, 600)))

	// A rect overlapping no display belongs to none.
	c.Nil(bestDisplayAmong(displays, geom.NewRect(-1000, -1000, 800, 600)))
	c.Nil(bestDisplayAmong(nil, geom.NewRect(100, 100, 800, 600)))
}

// TestBestDisplayForRectFallsBackToPrimary verifies that a rect overlapping no display is placed on the primary one.
func TestBestDisplayForRectFallsBackToPrimary(t *testing.T) {
	c := check.New(t)
	screen := startHeadlessTest(t, HeadlessConfig{Width: 400, Height: 300})
	var primary, best *Display
	c.True(screen.Do(func() {
		primary = PrimaryDisplay()
		best = BestDisplayForRect(geom.NewRect(-1000, -1000, 100, 100))
	}))
	c.NotNil(primary)
	c.Equal(primary, best)
}
