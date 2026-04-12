// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package x11

//nolint:unused // All available opcodes are defined here, even if not all are used by my code.
const (
	xrOpQueryVersion = iota
	xrOpQueryPictFormats
	xrOpQueryPictIndexValues // v0.7
	xrOpQueryDithers
	xrOpCreatePicture
	xrOpChangePicture
	xrOpSetPictureClipRectangles
	xrOpFreePicture
	xrOpComposite
	xrOpScale
	xrOpTrapezoids
	xrOpTriangles
	xrOpTriStrip
	xrOpTriFan
	xrOpColorTrapezoids
	xrOpColorTriangles
	xrOpTransform // Removed
	xrOpCreateGlyphSet
	xrOpReferenceGlyphSet
	xrOpFreeGlyphSet
	xrOpAddGlyphs
	xrOpAddGlyphsFromPicture
	xrOpFreeGlyphs
	xrOpCompositeGlyphs8
	xrOpCompositeGlyphs16
	xrOpCompositeGlyphs32
	xrOpFillRectangles
	xrOpCreateCursor          // v0.5
	xrOpSetPictureTransform   // v0.6
	xrOpQueryFilters          // v0.6
	xrOpSetPictureFilter      // v0.6
	xrOpCreateAnimCursor      // v0.8
	xrOpAddTraps              // v0.9
	xrOpCreateSolidFill       // v0.10
	xrOpCreateLinearGradient  // v0.10
	xrOpCreateRadialGradient  // v0.10
	xrOpCreateConicalGradient // v0.10
)
