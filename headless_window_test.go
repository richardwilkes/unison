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

// TestHeadlessRaiseKeepsTiers verifies that raising a window puts it at the top of its own tier of the z-order, with
// normal windows below floating ones and floating ones below menu windows, so that a newer menu window sits above an
// older one and every menu window above a floating dialog.
func TestHeadlessRaiseKeepsTiers(t *testing.T) {
	c := check.New(t)
	s := &headlessState{}
	normal1 := &Window{title: "normal1"}
	normal2 := &Window{title: "normal2"}
	floating := &Window{title: "floating", floating: true}
	menu1 := &Window{title: "menu1", kind: WindowKindMenu}
	menu2 := &Window{title: "menu2", kind: WindowKindMenu}
	c.Equal(0, windowStackLevel(normal1))
	c.Equal(1, windowStackLevel(floating))
	c.Equal(2, windowStackLevel(menu1))
	s.raise(menu1)
	s.raise(floating)
	s.raise(normal1)
	c.Equal([]*Window{normal1, floating, menu1}, s.stack)
	s.raise(menu2)
	s.raise(normal2)
	c.Equal([]*Window{normal1, normal2, floating, menu1, menu2}, s.stack)
	s.raise(normal1)
	s.raise(menu1)
	s.raise(floating)
	c.Equal([]*Window{normal2, normal1, floating, menu2, menu1}, s.stack)
}
