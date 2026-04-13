// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package x11

import "github.com/richardwilkes/toolbox/v2/errs"

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

// ExtRender provides access to the XRender extension. Note that only those calls that I need have been implemented.
type ExtRender struct {
	conn *Conn
	extensionInfo
}

func newExtRender(conn *Conn) *ExtRender {
	info := conn.hasExtension("RENDER", 0, 11)
	return &ExtRender{
		conn:          conn,
		extensionInfo: info,
	}
}

// Repeat specifies the repeat mode for a Picture.
type Repeat byte

// Possible Repeat mode values.
const (
	RepeatNone Repeat = iota
	RepeatRegular
	RepeatPad
	RepeatReflect
)

// PolyEdge specifies the edge mode for a Picture.
type PolyEdge byte

// Possible PolyEdge mode values.
const (
	PolyEdgeSharp PolyEdge = iota
	PolyEdgeSmooth
)

// PolyMode specifies the mode for rendering polygons in a Picture.
type PolyMode byte

// Possible PolyMode values.
const (
	PolyModePrecise PolyMode = iota
	PolyModeImprecise
)

// PictValueMask specifies which attributes of a Picture are being set or queried.
type PictValueMask uint32

// Possible PictValueMask values.
const (
	PictureValueMaskRepeat PictValueMask = 1 << iota
	PictureValueMaskAlphaMap
	PictureValueMaskAlphaXOrigin
	PictureValueMaskAlphaYOrigin
	PictureValueMaskClipXOrigin
	PictureValueMaskClipYOrigin
	PictureValueMaskClipMask
	PictureValueMaskGraphicsExposures
	PictureValueMaskSubwindowMode
	PictureValueMaskPolyEdge
	PictureValueMaskPolyMode
	PictureValueMaskDither
	PictureValueMaskComponentAlpha
)

// PictAttributes specifies the attributes of a Picture resource.
type PictAttributes struct {
	AlphaMap          PictureID
	AlphaXOrigin      int16
	AlphaYOrigin      int16
	ClipXOrigin       int16
	ClipYOrigin       int16
	ClipMask          PixMapID
	SubwindowMode     SubwindowMode
	PolyEdge          PolyEdge
	PolyMode          PolyMode
	Repeat            Repeat
	Dither            Atom
	GraphicsExposures bool
	ComponentAlpha    bool
}

func (a *PictAttributes) values(mask PictValueMask) []uint32 {
	if a == nil {
		return nil
	}
	v := make([]uint32, 0, 23)
	if mask&PictureValueMaskRepeat != 0 {
		v = append(v, uint32(a.Repeat))
	}
	if mask&PictureValueMaskAlphaMap != 0 {
		v = append(v, uint32(a.AlphaMap))
	}
	if mask&PictureValueMaskAlphaXOrigin != 0 {
		v = append(v, uint32(a.AlphaXOrigin))
	}
	if mask&PictureValueMaskAlphaYOrigin != 0 {
		v = append(v, uint32(a.AlphaYOrigin))
	}
	if mask&PictureValueMaskClipXOrigin != 0 {
		v = append(v, uint32(a.ClipXOrigin))
	}
	if mask&PictureValueMaskClipYOrigin != 0 {
		v = append(v, uint32(a.ClipYOrigin))
	}
	if mask&PictureValueMaskClipMask != 0 {
		v = append(v, uint32(a.ClipMask))
	}
	if mask&PictureValueMaskGraphicsExposures != 0 {
		var ge uint32
		if a.GraphicsExposures {
			ge = 1
		}
		v = append(v, ge)
	}
	if mask&PictureValueMaskSubwindowMode != 0 {
		v = append(v, uint32(a.SubwindowMode))
	}
	if mask&PictureValueMaskPolyEdge != 0 {
		v = append(v, uint32(a.PolyEdge))
	}
	if mask&PictureValueMaskPolyMode != 0 {
		v = append(v, uint32(a.PolyMode))
	}
	if mask&PictureValueMaskDither != 0 {
		v = append(v, uint32(a.Dither))
	}
	if mask&PictureValueMaskComponentAlpha != 0 {
		var ca uint32
		if a.ComponentAlpha {
			ca = 1
		}
		v = append(v, ca)
	}
	return v
}

// DirectFormat specifies the bit shifts and masks for the color components in a direct color format.
type DirectFormat struct {
	RedShift   uint16
	RedMask    uint16
	GreenShift uint16
	GreenMask  uint16
	BlueShift  uint16
	BlueMask   uint16
	AlphaShift uint16
	AlphaMask  uint16
}

// PictType specifies the type of a Picture.
type PictType byte

// Possible PictType values.
const (
	PictTypeIndexed PictType = iota
	PictTypeDirect
)

// PictFormat is an identifier for a Picture format.
type PictFormat uint32

// PictFormatInfo specifies the information about a Picture format.
type PictFormatInfo struct {
	ID       PictFormat
	Type     PictType
	Depth    byte
	Direct   DirectFormat
	ColorMap ColorMapID
}

// PictVisual specifies the visual information for a Picture format.
type PictVisual struct {
	Visual VisualID
	Format PictFormat
}

// PictDepth specifies the depth information for a Picture format, including the visuals that support that depth.
type PictDepth struct {
	Visuals []PictVisual
	Depth   byte
}

// PictScreen specifies the screen information for a Picture format, including the fallback format and the depths
// supported.
type PictScreen struct {
	Depths   []PictDepth
	Fallback PictFormat
}

// PictureFormats specifies the formats, screens, and subpixel orders supported by the XRender extension.
type PictureFormats struct {
	Formats   []PictFormatInfo
	Screens   []PictScreen
	Subpixels []uint32
}

// QueryPictFormats queries the X server for the supported Picture formats, screens, and subpixel orders.
func (e *ExtRender) QueryPictFormats() PictureFormats {
	w := NewWriter(4)
	w.Byte(e.majorOpcode)
	w.Byte(xrOpQueryPictFormats)
	w.Uint16(1)
	var reply PictureFormats
	if err := e.conn.sendNewRequest(newReplyRequest(w, func(r *Reader) {
		r.Skip(8)
		numFormats := r.Uint32()
		numScreens := r.Uint32()
		r.Skip(8)
		numSubPixel := r.Uint32()
		r.Skip(4)
		reply.Formats = ReadList(int(numFormats), r, func(rr *Reader) PictFormatInfo {
			var info PictFormatInfo
			info.ID = PictFormat(rr.Uint32())
			info.Type = PictType(rr.Byte())
			info.Depth = rr.Byte()
			rr.Skip(2)
			info.Direct.RedShift = rr.Uint16()
			info.Direct.RedMask = rr.Uint16()
			info.Direct.GreenShift = rr.Uint16()
			info.Direct.GreenMask = rr.Uint16()
			info.Direct.BlueShift = rr.Uint16()
			info.Direct.BlueMask = rr.Uint16()
			info.Direct.AlphaShift = rr.Uint16()
			info.Direct.AlphaMask = rr.Uint16()
			info.ColorMap = ColorMapID(rr.Uint32())
			return info
		})
		reply.Screens = ReadList(int(numScreens), r, func(rr *Reader) PictScreen {
			var screen PictScreen
			screenNumDepths := rr.Uint32()
			screen.Fallback = PictFormat(rr.Uint32())
			screen.Depths = ReadList(int(screenNumDepths), rr, func(rrr *Reader) PictDepth {
				var depth PictDepth
				depth.Depth = rrr.Byte()
				rrr.Skip(1)
				depthNumVisuals := rrr.Uint16()
				rrr.Skip(4)
				depth.Visuals = ReadList(int(depthNumVisuals), rrr, func(rrrr *Reader) PictVisual {
					var visual PictVisual
					visual.Visual = VisualID(rrrr.Uint32())
					visual.Format = PictFormat(rrrr.Uint32())
					return visual
				})
				return depth
			})
			return screen
		})
		reply.Subpixels = r.Uint32Slice(int(numSubPixel))
	})); err != nil {
		errs.Log(err)
	}
	return reply
}

// CreatePicture creates a new Picture resource with the specified drawable, format, and attributes, returning the new
// PictureID.
func (e *ExtRender) CreatePicture(drawable DrawableID, format PictFormat, valueMask PictValueMask, attrs *PictAttributes) PictureID {
	id := e.conn.nextPictureID()
	if id == 0 {
		return 0
	}
	values := attrs.values(valueMask)
	w := NewWriter(20 + 4*len(values))
	w.Byte(e.majorOpcode)
	w.Byte(xrOpCreatePicture)
	w.Uint16(5 + uint16(len(values)))
	w.PictureID(id)
	w.DrawableID(drawable)
	w.Uint32(uint32(format))
	w.Uint32Slice(values)
	if err := e.conn.sendNewRequest(newCheckedRequest(w)); err != nil {
		errs.Log(err)
		return 0
	}
	return id
}

// FreePicture frees the specified Picture resource.
func (e *ExtRender) FreePicture(picture PictureID) {
	w := NewWriter(8)
	w.Byte(e.majorOpcode)
	w.Byte(xrOpFreePicture)
	w.Uint16(2)
	w.PictureID(picture)
	if err := e.conn.sendNewRequest(newUncheckedRequest(w)); err != nil {
		errs.Log(err)
	}
}

// CreateCursor creates a new cursor using the specified source PictureID and hot spot coordinates, returning the new
// CursorID.
func (e *ExtRender) CreateCursor(src PictureID, hotX, hotY uint16) CursorID {
	id := e.conn.nextCursorID()
	if id != 0 {
		w := NewWriter(16)
		w.Byte(e.majorOpcode)
		w.Byte(xrOpCreateCursor)
		w.Uint16(4)
		w.CursorID(id)
		w.PictureID(src)
		w.Uint16(hotX)
		w.Uint16(hotY)
		if err := e.conn.sendNewRequest(newCheckedRequest(w)); err != nil {
			errs.Log(err)
			return 0
		}
	}
	return id
}
