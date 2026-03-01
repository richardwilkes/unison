// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
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

type apiNativeCursor struct {
	cursor int // TODO: Need actual type
	system bool
}

func apiNewCursor(img *image.NRGBA, xhot, yhot int) *Cursor {
	// TODO: Need implementation
	return nil
}

func (c *Cursor) apiDestroy() {
	// TODO: Need implementation
}

func apiArrowCursor() *Cursor {
	// TODO: Need implementation
	return nil
}

func apiPointingCursor() *Cursor {
	// TODO: Need implementation
	return nil
}

func apiTextCursor() *Cursor {
	// TODO: Need implementation
	return nil
}
