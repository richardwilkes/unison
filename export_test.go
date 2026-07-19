// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

// SetDragTableDataForTest sets the package-level drag data used by table drag & drop, allowing external tests to
// simulate an in-progress table row drag without a live drag gesture. Tests should reset it to nil when done.
func SetDragTableDataForTest(data any) {
	dragTableData = data
}
