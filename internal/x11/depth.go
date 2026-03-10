// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package x11

// Depth holds the Visuals for a given screen bit depth.
type Depth struct {
	Visuals []*Visual
	Depth   byte
}

// NewDepth reads a Depth from the specified Reader and returns it.
func NewDepth(r *Reader) *Depth {
	var d Depth
	d.Depth = r.Byte()
	r.Skip(1)
	count := r.Uint16()
	r.Skip(4)
	d.Visuals = ReadList(int(count), r, NewVisual)
	return &d
}
