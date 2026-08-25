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

// TestX11CursorsWithoutConnection verifies that the built-in cursors can be requested before Start has connected to an
// X server -- the permanent state of a headless process such as a test run on a machine without a display. A client's
// tests may legitimately compare the cursor a control reports against a built-in one, so the cursors must exist, be
// cached like any other, and remain distinct from one another, rather than the request dereferencing the missing
// connection and crashing the process. Destroying such a cursor has nothing to free and must not need the connection
// either.
func TestX11CursorsWithoutConnection(t *testing.T) {
	c := check.New(t)
	savedConn := x11Conn
	savedSettings := builtCursorSettings
	savedCursors := make([]*Cursor, 0, len(builtInCursors()))
	for _, p := range builtInCursors() {
		savedCursors = append(savedCursors, *p)
		*p = nil
	}
	t.Cleanup(func() {
		x11Conn = savedConn
		builtCursorSettings = savedSettings
		for i, p := range builtInCursors() {
			*p = savedCursors[i]
		}
	})
	x11Conn = nil
	builtCursorSettings = nil

	pointing := PointingCursor()
	c.NotNil(pointing, "a built-in cursor must exist without a connection")
	c.True(pointing == PointingCursor(), "and be cached rather than rebuilt on every request")
	c.True(pointing != ArrowCursor(), "and be distinct from the other built-in cursors")
	c.Equal(apiNativeCursor(0), pointing.cursor, "there is no native cursor behind it")

	pointing.Destroy()
	c.Nil(pointingCursor, "destroying it clears the built-in so the next request rebuilds it")
	c.True(pointing != PointingCursor(), "and the next request does rebuild it")
}
