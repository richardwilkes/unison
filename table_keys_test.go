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
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/enums/mod"
)

// selectedTableIndexes returns the indexes of the selected rows, in order.
func selectedTableIndexes(table *unison.Table[*tableTestRow]) []int {
	var indexes []int
	for i := range table.LastRowIndex() + 1 {
		if table.IsRowSelected(i) {
			indexes = append(indexes, i)
		}
	}
	return indexes
}

// TestTableKeyNavigationIgnoresUnrecognizedModifiers verifies that the arrow, home and end keys only navigate when
// they carry the modifiers the table gives a meaning to. A navigation key with any other modifier is typically a menu
// shortcut that the menu declined because its command was disabled, and treating it as a plain key press moved the
// selection when the user expected nothing at all to happen. Such a key is reported as unhandled, so that whatever is
// above the table gets a look at it.
func TestTableKeyNavigationIgnoresUnrecognizedModifiers(t *testing.T) {
	c := check.New(t)
	table := newTestTable(flatRows(4)...)
	table.SelectByIndex(2)

	for _, mods := range []mod.Modifiers{
		mod.OSMenuCommand(),
		mod.Shift | mod.OSMenuCommand(),
		mod.Option,
		mod.Shift | mod.Option,
	} {
		for _, key := range []unison.KeyCode{unison.KeyUp, unison.KeyDown, unison.KeyHome, unison.KeyEnd} {
			c.False(table.DefaultKeyDown(key, mods, false), "%v with %v must be reported as unhandled", key, mods)
			c.Equal([]int{2}, selectedTableIndexes(table), "%v with %v must leave the selection alone", key, mods)
		}
	}

	// The modifiers the table does recognize keep working.
	c.True(table.DefaultKeyDown(unison.KeyUp, 0, false), "an unmodified Up must be handled")
	c.Equal([]int{1}, selectedTableIndexes(table), "an unmodified Up must move the selection up")
	c.True(table.DefaultKeyDown(unison.KeyDown, mod.Shift, false), "a shifted Down must be handled")
	c.Equal([]int{1, 2}, selectedTableIndexes(table), "a shifted Down must extend the selection")
	c.True(table.DefaultKeyDown(unison.KeyEnd, 0, false), "an unmodified End must be handled")
	c.Equal([]int{3}, selectedTableIndexes(table), "an unmodified End must select the last row")
	c.True(table.DefaultKeyDown(unison.KeyHome, mod.Shift, false), "a shifted Home must be handled")
	c.Equal([]int{0, 1, 2, 3}, selectedTableIndexes(table), "a shifted Home must extend the selection to the first row")
}

// TestTableDisclosureKeysIgnoreUnrecognizedModifiers does the same for the left and right arrows, which close and open
// containers and recognize only the option modifier, for doing so recursively.
func TestTableDisclosureKeysIgnoreUnrecognizedModifiers(t *testing.T) {
	c := check.New(t)
	inner := newTableTestRow("inner")
	inner.SetChildren([]*tableTestRow{newTableTestRow("leaf")})
	outer := newTableTestRow("outer")
	outer.SetChildren([]*tableTestRow{inner})
	table := newTestTable(outer)
	table.SelectByIndex(0)
	c.False(outer.IsOpen(), "the container starts out closed")

	for _, mods := range []mod.Modifiers{
		mod.OSMenuCommand(),
		mod.Shift | mod.OSMenuCommand(),
		mod.Shift,
		mod.Option | mod.OSMenuCommand(),
	} {
		c.False(table.DefaultKeyDown(unison.KeyRight, mods, false), "Right with %v must be reported as unhandled", mods)
		c.False(outer.IsOpen(), "Right with %v must not open the container", mods)
	}

	c.True(table.DefaultKeyDown(unison.KeyRight, 0, false), "an unmodified Right must be handled")
	c.True(outer.IsOpen(), "an unmodified Right must open the container")
	c.False(inner.IsOpen(), "an unmodified Right must open only the selected container")
	c.True(table.DefaultKeyDown(unison.KeyLeft, 0, false), "an unmodified Left must be handled")
	c.False(outer.IsOpen(), "an unmodified Left must close the container")
	c.True(table.DefaultKeyDown(unison.KeyRight, mod.Option, false), "an option-modified Right must be handled")
	c.True(outer.IsOpen() && inner.IsOpen(), "an option-modified Right must open the container and its descendants")
	c.False(table.DefaultKeyDown(unison.KeyLeft, mod.OSMenuCommand(), false),
		"Left with the command modifier must be reported as unhandled")
	c.True(outer.IsOpen() && inner.IsOpen(), "Left with the command modifier must not close anything")
}
