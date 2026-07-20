// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison_test

import (
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
)

// TestEmptyTableWithRowDividerSizes verifies that an empty table with ShowRowDivider enabled reports a non-negative
// preferred and minimum height, rather than the -1 that used to result from counting dividers for an empty row range.
func TestEmptyTableWithRowDividerSizes(t *testing.T) {
	c := check.New(t)
	table := newTestTable()
	table.ShowRowDivider = true
	minSize, prefSize, _ := table.DefaultSizes(geom.Size{})
	c.True(minSize.Height >= 0, "minimum height %v must not be negative", minSize.Height)
	c.True(prefSize.Height >= 0, "preferred height %v must not be negative", prefSize.Height)
}
