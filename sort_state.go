// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

// SortState holds data regarding a sort state.
type SortState struct {
	Order     int // A negative value indicates it isn't participating at the moment.
	Ascending bool
	Sortable  bool // A false value indicates it is not sortable at all
}
