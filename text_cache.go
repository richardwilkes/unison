// Copyright ©2021-2024 by Richard A. Wilkes. All rights reserved.
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
	str        string
	decoration TextDecoration
	text       *Text
}

// Text composes the Text object, if needed, and returns it.
func (c *TextCache) Text(str string, decoration *TextDecoration) *Text {
	if c.text == nil || str != c.str || c.decoration != *decoration {
		c.str = str
		c.decoration = *decoration
		c.text = NewText(str, decoration)
	}
	return c.text
}
