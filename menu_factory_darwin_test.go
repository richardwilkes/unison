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

// TestMacMenuUpdaterReceivesID verifies that the Menu handed to an updater callback carries the id the menu was
// created with, so updaters that dispatch on Menu.ID() work on macOS just as they do with the in-window menus.
func TestMacMenuUpdaterReceivesID(t *testing.T) {
	c := check.New(t)
	f := &macMenuFactory{}
	var got Menu
	u := f.wrapUpdater(42, func(m Menu) { got = m })
	c.NotNil(u)
	u(cocoa.Menu(0))
	c.NotNil(got)
	c.Equal(42, got.ID())
	c.True(got.Factory() == MenuFactory(f))
	c.True(f.wrapUpdater(42, nil) == nil)
	// Wrappers handed to updaters merely navigate a menu the tree owns, so they must never own the reference —
	// otherwise an updater calling Dispose would release a menu out from under its parent item.
	if mm, ok := got.(*macMenu); ok {
		c.False(mm.owned)
	}
}

// TestMacMenuDisposeOwnership verifies the Dispose ownership contract: only a wrapper that owns its menu's reference
// releases it, exactly once, while Dispose of a borrowed wrapper (e.g. one obtained via SubMenu) is a no-op left to
// the owning tree's root.
func TestMacMenuDisposeOwnership(t *testing.T) {
	c := check.New(t)
	m := &macMenu{owned: true}
	m.Dispose()
	c.False(m.owned)
	m.Dispose() // a second Dispose must not release again
	borrowed := &macMenu{}
	c.NotPanics(func() { borrowed.Dispose() })
	c.False(borrowed.owned)
}
