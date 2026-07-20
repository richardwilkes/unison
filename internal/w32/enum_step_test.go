// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package w32

import (
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
)

// TestEnumFetchCount exercises the IEnumFORMATETC::Next position arithmetic, including the partial-fetch case that
// must map to S_FALSE, requests past the end, and hostile inputs (negative counts, positions beyond the end).
func TestEnumFetchCount(t *testing.T) {
	c := check.New(t)
	check2 := func(wantPos, wantFetched, pos, total, requested int) {
		t.Helper()
		newPos, fetched := enumFetchCount(pos, total, requested)
		c.Equal(wantPos, newPos)
		c.Equal(wantFetched, fetched)
	}
	check2(2, 2, 0, 3, 2) // full fetch
	check2(3, 1, 2, 3, 2) // partial fetch at the end
	check2(3, 0, 3, 3, 2) // already exhausted
	check2(0, 0, 0, 0, 5) // empty sequence
	check2(5, 0, 5, 3, 1) // position past the end must not fetch or rewind
	check2(0, 0, 0, 3, -1)
}

// TestEnumSkipAdvance exercises the IEnumFORMATETC::Skip position arithmetic: a skip within bounds succeeds, one past
// the end clamps to the end and reports S_FALSE.
func TestEnumSkipAdvance(t *testing.T) {
	c := check.New(t)
	pos, all := enumSkipAdvance(0, 3, 2)
	c.Equal(2, pos)
	c.True(all)
	pos, all = enumSkipAdvance(2, 3, 2)
	c.Equal(3, pos)
	c.False(all)
	pos, all = enumSkipAdvance(3, 3, 0)
	c.Equal(3, pos)
	c.True(all)
	pos, all = enumSkipAdvance(5, 3, 1)
	c.Equal(3, pos)
	c.False(all)
}
