// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"github.com/richardwilkes/toolbox"
)

var (
	_                      ColorProvider = &DynamicColor{}
	needDynamicColorUpdate               = true
	dynamicColors          []*DynamicColor
)

// DynamicColor holds a color that may be changed.
type DynamicColor struct {
	Color     Color
	Rebuilder func() Color
}

// NewDynamicColor creates a new DynamicColor and registers it for theme updates. If your color relies on another
// dynamic color to calculate its value, make sure it is created *after* the colors it relies on, since all dynamic
// colors are rebuilt in the order they were created.
func NewDynamicColor(rebuilder func() Color) *DynamicColor {
	c := &DynamicColor{Color: rebuilder(), Rebuilder: rebuilder}
	dynamicColors = append(dynamicColors, c)
	return c
}

// GetColor returns the current color. Here to satisfy the ColorProvider interface.
func (c *DynamicColor) GetColor() Color {
	return c.Color
}

// Paint returns a Paint for this DynamicColor. Here to satisfy the Ink interface.
func (c *DynamicColor) Paint(canvas *Canvas, rect Rect, style PaintStyle) *Paint {
	return c.Color.Paint(canvas, rect, style)
}

// Unregister removes this DynamicColor from participating in rebuilds via RebuildDynamicColors.
func (c *DynamicColor) Unregister() {
	for i, other := range dynamicColors {
		if c == other {
			if len(dynamicColors) == 1 {
				dynamicColors = nil
			} else {
				copy(dynamicColors[i:], dynamicColors[i+1:])
				dynamicColors[len(dynamicColors)-1] = nil
				dynamicColors = dynamicColors[:len(dynamicColors)-1]
			}
			break
		}
	}
}

// MarkDynamicColorsForRebuild marks the dynamic colors to be updated the next time RebuildDynamicColors() is called.
func MarkDynamicColorsForRebuild() {
	needPlatformDarkModeUpdate = true
	needDynamicColorUpdate = true
}

// RebuildDynamicColors rebuilds the dynamic colors, but only if a call to MarkDynamicColorsForRebuild() has been made
// since the last time this function was called.
func RebuildDynamicColors() {
	if needDynamicColorUpdate {
		needDynamicColorUpdate = false
		for _, color := range dynamicColors {
			toolbox.Call(func() { color.Color = color.Rebuilder() })
		}
	}
}
