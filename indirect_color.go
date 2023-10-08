// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import "github.com/richardwilkes/unison/enums/paintstyle"

var _ ColorProvider = &IndirectColor{}

// IndirectColor holds a color that references another color.
type IndirectColor struct {
	Target ColorProvider
}

// Paint returns a Paint for this IndirectColor. Here to satisfy the Ink interface.
func (c *IndirectColor) Paint(canvas *Canvas, rect Rect, style paintstyle.Enum) *Paint {
	return c.Target.Paint(canvas, rect, style)
}

// GetColor returns the current color. Here to satisfy the ColorProvider interface.
func (c *IndirectColor) GetColor() Color {
	return c.Target.GetColor()
}
