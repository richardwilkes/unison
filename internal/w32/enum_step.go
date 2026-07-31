// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package w32

// enumFetchCount reports how many elements an IEnumXXXX::Next call starting at pos in a sequence of total elements
// can actually return when requested elements were asked for, along with the position after the fetch. Per COM
// enumerator semantics, the caller returns S_OK only when the full requested count was delivered, S_FALSE otherwise.
func enumFetchCount(pos, total, requested int) (newPos, fetched int) {
	remaining := total - pos
	if remaining < 0 {
		remaining = 0
	}
	if requested < 0 {
		requested = 0
	}
	if requested > remaining {
		requested = remaining
	}
	return pos + requested, requested
}

// enumSkipAdvance computes the position after an IEnumXXXX::Skip of count elements starting at pos in a sequence of
// total elements, and whether the full count could be skipped (S_OK) or the end was hit first (S_FALSE, position
// clamped to the end).
func enumSkipAdvance(pos, total, count int) (newPos int, all bool) {
	remaining := total - pos
	if remaining < 0 {
		remaining = 0
	}
	if count > remaining {
		return total, false
	}
	return pos + count, true
}
