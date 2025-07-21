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
	"strings"

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/enums/paintstyle"
)

// DrawRectBase fills and strokes a rectangle.
func DrawRectBase(canvas *Canvas, rect geom.Rect, fillInk, strokeInk Ink) {
	canvas.DrawRect(rect, fillInk.Paint(canvas, rect, paintstyle.Fill))
	rect = rect.Inset(geom.NewUniformInsets(0.5))
	canvas.DrawRect(rect, strokeInk.Paint(canvas, rect, paintstyle.Stroke))
}

// DrawRoundedRectBase fills and strokes a rounded rectangle.
func DrawRoundedRectBase(canvas *Canvas, rect geom.Rect, cornerRadius geom.Size, thickness float32, fillInk, strokeInk Ink) {
	canvas.DrawRoundedRect(rect, cornerRadius, fillInk.Paint(canvas, rect, paintstyle.Fill))
	rect = rect.Inset(geom.NewUniformInsets(thickness / 2))
	p := strokeInk.Paint(canvas, rect, paintstyle.Stroke)
	p.SetStrokeWidth(thickness)
	cornerRadius.Width = max(cornerRadius.Width-thickness/2, 0)
	cornerRadius.Height = max(cornerRadius.Height-thickness/2, 0)
	canvas.DrawRoundedRect(rect, cornerRadius, p)
}

// DrawEllipseBase fills and strokes an ellipse.
func DrawEllipseBase(canvas *Canvas, rect geom.Rect, thickness float32, fillInk, strokeInk Ink) {
	canvas.DrawOval(rect, fillInk.Paint(canvas, rect, paintstyle.Fill))
	rect = rect.Inset(geom.NewUniformInsets(thickness / 2))
	p := strokeInk.Paint(canvas, rect, paintstyle.Stroke)
	p.SetStrokeWidth(thickness)
	canvas.DrawOval(rect, p)
}

// SanitizeExtensionList ensures the extension list is consistent:
//
//   - removal of leading and trailing white space
//   - removal of leading "*." or "."
//   - lower-cased
//   - removal of duplicates
//   - removal of empty extensions
func SanitizeExtensionList(in []string) []string {
	var actual []string
	existence := make(map[string]bool)
	for _, ext := range in {
		ext = strings.TrimSpace(ext)
		if strings.HasPrefix(ext, "*.") {
			ext = strings.TrimSpace(ext[2:])
		} else {
			ext = strings.TrimSpace(strings.TrimPrefix(ext, "."))
		}
		if ext != "" {
			ext = strings.ToLower(ext)
			if !existence[ext] {
				existence[ext] = true
				actual = append(actual, ext)
			}
		}
	}
	return actual
}
