// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

// TextCache provides a simple caching mechanism for Text objects.
type TextCache struct {
	str  string
	font FontDescriptor
	text *Text
}

// Text composes the Text object, if needed, and returns it. The paint will be nil or whatever was last used, so be
// sure to call .ReplacePaint() with the paint you want to use. Likewise, underline and strike through will be false,
// so calls to .ReplaceUnderline() and/or .ReplaceStrikeThrough() should be made to enable them, if desired.
func (c *TextCache) Text(str string, font Font) *Text {
	desc := font.Descriptor()
	if c.text == nil || str != c.str || desc != c.font {
		c.str = str
		c.font = desc
		c.text = NewText(str, &TextDecoration{Font: font})
	}
	return c.text
}
