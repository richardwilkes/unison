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

// TestMenuItemMenuReturnsTrueNil verifies that Menu() on an item that has not been inserted into a menu returns a
// true nil interface rather than a typed-nil *menu, which would pass != nil checks and then crash on the first method
// call. This matches the behavior of the macOS implementation.
func TestMenuItemMenuReturnsTrueNil(t *testing.T) {
	c := check.New(t)
	var item MenuItem = &menuItem{}
	c.True(item.Menu() == nil)
}
