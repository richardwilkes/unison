// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package x11

// Atom represents an X11 Atom, which is a unique identifier for a string used in various parts of the X11 protocol,
// such as properties and types.
type Atom uint32

// Predefined Atom values as per the X11 protocol specification.
const (
	AtomNone Atom = iota
	AtomPrimary
	AtomSecondary
	AtomArc
	AtomAtom
	AtomBitmap
	AtomCardinal
	AtomColormap
	AtomCursor
	AtomCutBuffer0
	AtomCutBuffer1
	AtomCutBuffer2
	AtomCutBuffer3
	AtomCutBuffer4
	AtomCutBuffer5
	AtomCutBuffer6
	AtomCutBuffer7
	AtomDrawable
	AtomFont
	AtomInteger
	AtomPixmap
	AtomPoint
	AtomRectangle
	AtomResourceManager
	AtomRgbColorMap
	AtomRgbBestMap
	AtomRgbBlueMap
	AtomRgbDefaultMap
	AtomRgbGrayMap
	AtomRgbGreenMap
	AtomRgbRedMap
	AtomString
	AtomVisualid
	AtomWindow
	AtomWmCommand
	AtomWmHints
	AtomWmClientMachine
	AtomWmIconName
	AtomWmIconSize
	AtomWmName
	AtomWmNormalHints
	AtomWmSizeHints
	AtomWmZoomHints
	AtomMinSpace
	AtomNormSpace
	AtomMaxSpace
	AtomEndSpace
	AtomSuperscriptX
	AtomSuperscriptY
	AtomSubscriptX
	AtomSubscriptY
	AtomUnderlinePosition
	AtomUnderlineThickness
	AtomStrikeoutAscent
	AtomStrikeoutDescent
	AtomItalicAngle
	AtomXHeight
	AtomQuadWidth
	AtomWeight
	AtomPointSize
	AtomResolution
	AtomCopyright
	AtomNotice
	AtomFontName
	AtomFamilyName
	AtomFullName
	AtomCapHeight
	AtomWmClass
	AtomWmTransientFor
	AtomAny = AtomNone
)
