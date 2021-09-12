// Copyright ©2021 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

var _ FontProvider = &DynamicFont{}

// DynamicFont holds a FontProvider that dynamically creates a font.
type DynamicFont struct {
	Resolver func() FontDescriptor
	lastDesc FontDescriptor
	lastFont *Font
}

// ResolvedFont implements the FontProvider interface.
func (d *DynamicFont) ResolvedFont() *Font {
	fd := d.Resolver()
	if fd != d.lastDesc {
		d.lastDesc = fd
		d.lastFont = fd.Font()
	}
	return d.lastFont
}
