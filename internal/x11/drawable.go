// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package x11

// DrawableID represents an X11 drawable (window or pixmap) identifier.
type DrawableID uint32

// GCID represents an X11 graphics context identifier.
type GCID uint32

// GCAttributes specifies the attributes of a graphics context resource.
type GCAttributes struct {
	PlaneMask          uint32
	Foreground         uint32
	Background         uint32
	DashOffset         uint32
	Font               FontID
	ClipMask           PixMapID
	Tile               PixMapID
	Stipple            PixMapID
	ClipOriginX        int16
	ClipOriginY        int16
	TileStippleOriginX int16
	TileStippleOriginY int16
	LineWidth          uint16
	LineStyle          LineStyle
	CapStyle           CapStyle
	JoinStyle          JoinStyle
	FillStyle          FillStyle
	FillRule           FillRule
	SubwindowMode      SubwindowMode
	Function           GCFunction
	GraphicsExposures  bool
	Dashes             byte
	ArcMode            ArcMode
}

func (a *GCAttributes) values(mask GCValueMask) []uint32 {
	if a == nil {
		return nil
	}
	v := make([]uint32, 0, 23)
	if mask&GCMaskFunction != 0 {
		v = append(v, uint32(a.Function))
	}
	if mask&GCMaskPlaneMask != 0 {
		v = append(v, a.PlaneMask)
	}
	if mask&GCMaskForeground != 0 {
		v = append(v, a.Foreground)
	}
	if mask&GCMaskBackground != 0 {
		v = append(v, a.Background)
	}
	if mask&GCMaskLineWidth != 0 {
		v = append(v, uint32(a.LineWidth))
	}
	if mask&GCMaskLineStyle != 0 {
		v = append(v, uint32(a.LineStyle))
	}
	if mask&GCMaskCapStyle != 0 {
		v = append(v, uint32(a.CapStyle))
	}
	if mask&GCMaskJoinStyle != 0 {
		v = append(v, uint32(a.JoinStyle))
	}
	if mask&GCMaskFillStyle != 0 {
		v = append(v, uint32(a.FillStyle))
	}
	if mask&GCMaskFillRule != 0 {
		v = append(v, uint32(a.FillRule))
	}
	if mask&GCMaskTile != 0 {
		v = append(v, uint32(a.Tile))
	}
	if mask&GCMaskStipple != 0 {
		v = append(v, uint32(a.Stipple))
	}
	if mask&GCMaskTileStippleOriginX != 0 {
		v = append(v, uint32(a.TileStippleOriginX))
	}
	if mask&GCMaskTileStippleOriginY != 0 {
		v = append(v, uint32(a.TileStippleOriginY))
	}
	if mask&GCMaskFont != 0 {
		v = append(v, uint32(a.Font))
	}
	if mask&GCMaskSubwindowMode != 0 {
		v = append(v, uint32(a.SubwindowMode))
	}
	if mask&GCMaskGraphicsExposures != 0 {
		var ge uint32
		if a.GraphicsExposures {
			ge = 1
		}
		v = append(v, ge)
	}
	if mask&GCMaskClipOriginX != 0 {
		v = append(v, uint32(a.ClipOriginX))
	}
	if mask&GCMaskClipOriginY != 0 {
		v = append(v, uint32(a.ClipOriginY))
	}
	if mask&GCMaskClipMask != 0 {
		v = append(v, uint32(a.ClipMask))
	}
	if mask&GCMaskDashOffset != 0 {
		v = append(v, a.DashOffset)
	}
	if mask&GCMaskDashList != 0 {
		v = append(v, uint32(a.Dashes))
	}
	if mask&GCMaskArcMode != 0 {
		v = append(v, uint32(a.ArcMode))
	}
	return v
}

// GCValueMask represents the bitmask for specifying which GC attributes to set or get.
type GCValueMask uint32

// GC value mask bits.
const (
	GCMaskFunction GCValueMask = 1 << iota
	GCMaskPlaneMask
	GCMaskForeground
	GCMaskBackground
	GCMaskLineWidth
	GCMaskLineStyle
	GCMaskCapStyle
	GCMaskJoinStyle
	GCMaskFillStyle
	GCMaskFillRule
	GCMaskTile
	GCMaskStipple
	GCMaskTileStippleOriginX
	GCMaskTileStippleOriginY
	GCMaskFont
	GCMaskSubwindowMode
	GCMaskGraphicsExposures
	GCMaskClipOriginX
	GCMaskClipOriginY
	GCMaskClipMask
	GCMaskDashOffset
	GCMaskDashList
	GCMaskArcMode
)

// GCFunction represents an X11 graphics function.
type GCFunction byte

// Graphics function constants.
const (
	GxClear GCFunction = iota
	GxAnd
	GxAndReverse
	GxCopy
	GxAndInverted
	GxNoop
	GxXor
	GxOr
	GxNor
	GxEquiv
	GxInvert
	GxOrReverse
	GxCopyInverted
	GxOrInverted
	GxNand
	GxSet
)

// LineStyle represents the line style for drawing operations.
type LineStyle byte

// Possible LineStyle values.
const (
	LineStyleSolid LineStyle = iota
	LineStyleOnOffDash
	LineStyleDoubleDash
)

// CapStyle represents the cap style for line endpoints.
type CapStyle byte

// Possible CapStyle values.
const (
	CapStyleNotLast CapStyle = iota
	CapStyleButt
	CapStyleRound
	CapStyleProjecting
)

// JoinStyle represents the join style for line segments.
type JoinStyle byte

// Possible JoinStyle values.
const (
	JoinStyleMiter JoinStyle = iota
	JoinStyleRound
	JoinStyleBevel
)

// FillStyle represents the fill style for drawing operations.
type FillStyle byte

// Possible FillStyle values.
const (
	FillStyleSolid FillStyle = iota
	FillStyleTiled
	FillStyleStippled
	FillStyleOpaqueStippled
)

// FillRule represents the fill rule for polygon filling operations.
type FillRule byte

// Possible FillRule values.
const (
	FillRuleEvenOdd FillRule = iota
	FillRuleWinding
)

// SubwindowMode represents the subwindow mode for graphics contexts and pictures.
type SubwindowMode byte

// Possible SubwindowMode values.
const (
	SubwindowModeClipByChildren SubwindowMode = iota
	SubwindowModeIncludeInferiors
)

// ArcMode represents the mode for rendering arcs in a graphics context.
type ArcMode byte

// Possible ArcMode values.
const (
	ArcModeChord ArcMode = iota
	ArcModePieSlice
)

// ImageFormat represents the format for image data in X11 operations.
type ImageFormat byte

// Possible ImageFormat values.
const (
	ImageFormatXYBitmap ImageFormat = iota
	ImageFormatXYPixmap
	ImageFormatZPixmap
)
