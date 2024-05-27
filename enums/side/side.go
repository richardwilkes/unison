// Copyright (c) 2021-2024 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package side

// Horizontal returns true if the side is to the left or right.
func (e Enum) Horizontal() bool {
	return e == Left || e == Right
}

// Vertical returns true if the side is to the top or bottom.
func (e Enum) Vertical() bool {
	return e == Top || e == Bottom
}
