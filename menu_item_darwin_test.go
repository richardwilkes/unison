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
	"github.com/richardwilkes/unison/internal/cocoa"
)

// TestMacMenuItemMenuNilWhenDetached verifies that Menu() on an item that has not been inserted into a menu returns a
// true nil interface, matching the in-window implementation.
func TestMacMenuItemMenuNilWhenDetached(t *testing.T) {
	c := check.New(t)
	var item MenuItem = &macMenuItem{factory: &macMenuFactory{}, item: 0}
	c.True(item.Menu() == nil)
}

// TestMacMenuIDResolvesBar verifies that macMenuID reports the factory's menu bar id when handed the bar's menu,
// rather than fabricating an id from an unrelated item.
func TestMacMenuIDResolvesBar(t *testing.T) {
	c := check.New(t)
	f := &macMenuFactory{bar: &macMenu{id: RootMenuID}}
	c.Equal(RootMenuID, macMenuID(f, cocoa.Menu(0)))
}
