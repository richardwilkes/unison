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
	"image"
)

// Default size of a cursor should be content scale * 16

type apiNativeCursor struct {
	cursor int // TODO: Need actual type
	system bool
}

func apiNewCursor(img *image.NRGBA, xhot, yhot int) *Cursor {
	// TODO: Need implementation
	return nil
}

func (c *Cursor) apiDestroy() {
	if !c.cursor.system {
		// TODO: Need implementation
	}
}

func apiArrowCursor() *Cursor {
	return x11LoadSystemCursor("default")
}

func apiPointingCursor() *Cursor {
	return x11LoadSystemCursor("pointer")
}

func apiTextCursor() *Cursor {
	return x11LoadSystemCursor("text")
}

func x11LoadSystemCursor(name string) *Cursor {
	// TODO: Need implementation
	return &Cursor{
		cursor: apiNativeCursor{
			system: true,
		},
	}
}
