// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package skia

import (
	"time"
	"unsafe"

	"github.com/richardwilkes/toolbox/xmath/geom"
)

type (
	ArcSize            byte
	BlendMode          byte
	Blur               byte
	ClipOp             byte
	Color              uint32
	ColorChannel       byte
	Direction          byte
	EncodedImageFormat byte
	FillType           byte
	FilterMode         int32
	FontHinting        byte
	FontSlant          int32
	FontSpacing        int32
	FontWeight         int32
	InvertStyle        int32
	PaintStyle         byte
	PathEffect1DStyle  byte
	PathOp             byte
	PointMode          byte
	StrokeCap          byte
	StrokeJoin         byte
	TileMode           byte
	TrimMode           byte
)

type PathAddMode byte

const (
	PathAddModeAppend PathAddMode = iota
	PathAddModeExtend
)

type ImageCachingHint byte

const (
	ImageCachingHintAllow ImageCachingHint = iota
	ImageCachingHintDisallow
)

type TextEncoding int32

const (
	TextEncodingUTF8 TextEncoding = iota
	TextEncodingUTF16
	TextEncodingUTF32
	TextEncodingGlyphID
)

type PixelGeometry int32

const (
	PixelGeometryUnknown PixelGeometry = iota
	PixelGeometryRGBH
	PixelGeometryBGRH
	PixelGeometryRGBV
	PixelGeometryBGRV
)

type SurfaceOrigin int32

const (
	SurfaceOriginTopLeft SurfaceOrigin = iota
	SurfaceOriginBottomLeft
)

type ColorType int32

const (
	ColorTypeUnknown ColorType = iota
	ColorTypeAlpha8
	ColorTypeRGB565
	ColorTypeARGB4444
	ColorTypeRGBA8888
	ColorTypeRGB888X
	ColorTypeBGRA8888
	ColorTypeRGBA1010102
	ColorTypeBGRA1010102
	ColorTypeRGB101010X
	ColorTypeBGR101010X
	ColorTypeGray8
	ColorTypeRGBAF16Norm
	ColorTypeRGBAF16
	ColorTypeRGBAF32
	ColorTypeR8G8UNorm
	ColorTypeA16Float
	ColorTypeR16G16Float
	ColorTypeA16UNorm
	ColorTypeR16G16UNorm
	ColorTypeR16G16B16A16UNorm
	ColorTypeSRGBA8888
	ColorTypeR8UNorm
)

type AlphaType int32

const (
	AlphaTypeUnknown AlphaType = iota
	AlphaTypeOpaque
	AlphaTypePreMul
	AlphaTypeUnPreMul
)

type HighContrastConfig struct {
	Grayscale   bool
	_           bool
	_           bool
	_           bool
	InvertStyle InvertStyle
	Contrast    float32
}

type Point3 struct {
	X float32
	Y float32
	Z float32
}

type IPoint struct {
	X int32
	Y int32
}

type ISize struct {
	Width  int32
	Height int32
}

type Rect struct {
	Left   float32
	Top    float32
	Right  float32
	Bottom float32
}

func toGeomRect(r Rect) geom.Rect[float32] {
	return geom.Rect[float32]{Point: geom.NewPoint(r.Left, r.Top), Size: geom.NewSize(r.Right-r.Left, r.Bottom-r.Top)}
}

type IRect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

type Matrix struct {
	geom.Matrix[float32]
	Persp0 float32
	Persp1 float32
	Persp2 float32
}

type GLFrameBufferInfo struct {
	Fboid  uint32
	Format uint32
}

type ImageInfo struct {
	Colorspace ColorSpace
	ColorType  ColorType
	AlphaType  AlphaType
	Width      int32
	Height     int32
}

// FontMetrics holds various metrics about a font.
type FontMetrics struct {
	Flags              uint32  // Flags indicating which metrics are valid
	Top                float32 // Greatest extent above origin of any glyph bounding box; typically negative; deprecated with variable fonts; only if Flags & BoundsInvalidFontMetricsFlag == 0
	Ascent             float32 // Distance to reserve above baseline; typically negative
	Descent            float32 // Distance to reserve below baseline; typically positive
	Bottom             float32 // Greatest extent below origin of any glyph bounding box; typically positive; deprecated with variable fonts; only if Flags & BoundsInvalidFontMetricsFlag == 0
	Leading            float32 // Distance to add between lines; typically positive or zero
	AvgCharWidth       float32 // Average character width; zero if unknown
	MaxCharWidth       float32 // Maximum character width; zero if unknown
	XMin               float32 // Greatest extent to left of origin of any glyph bounding box; typically negative; deprecated with variable fonts; only if Flags & BoundsInvalidFontMetricsFlag == 0
	XMax               float32 // Greatest extent to right of origin of any glyph bounding box; typically positive; deprecated with variable fonts; only if Flags & BoundsInvalidFontMetricsFlag == 0
	XHeight            float32 // Height of lowercase 'x'; zero if unknown; typically negative
	CapHeight          float32 // Height of uppercase letter; zero if unknown; typically negative
	UnderlineThickness float32 // Underline thickness; only if Flags & UnderlineThicknessIsValidFontMetricsFlag != 0
	UnderlinePosition  float32 // Distance from baseline to top of stroke; typically positive; only if Flags & UnderlinePositionIsValidFontMetricsFlag != 0
	StrikeoutThickness float32 // Strikeout thickness; only if Flags & StrikeoutThicknessIsValidFontMetricsFlag != 0
	StrikeoutPosition  float32 // Distance from baseline to bottom of stroke; typically negative; only if Flags & StrikeoutPositionIsValidFontMetricsFlag != 0
}

type dateTime struct {
	TimeZoneMinutes int16
	Year            uint16
	Month           uint8
	DayOfWeek       uint8
	Day             uint8
	Hour            uint8
	Minute          uint8
	Second          uint8
}

func (dt *dateTime) set(t time.Time) {
	_, offset := t.Zone()
	dt.TimeZoneMinutes = int16(offset / 60)
	dt.Year = uint16(t.Year())
	dt.Month = uint8(t.Month())
	dt.DayOfWeek = uint8(t.Weekday())
	dt.Day = uint8(t.Day())
	dt.Hour = uint8(t.Hour())
	dt.Minute = uint8(t.Minute())
	dt.Second = uint8(t.Second())
}

type metaData struct {
	Title           uintptr
	Author          uintptr
	Subject         uintptr
	Keywords        uintptr
	Creator         uintptr
	Producer        uintptr
	Creation        dateTime
	Modified        dateTime
	RasterDPI       float32
	_               float32
	EncodingQuality int32
}

func (m *metaData) set(md *MetaData) {
	producer := md.Producer
	if producer == "" {
		producer = "unison"
	}
	creation := md.Creation
	if creation.IsZero() {
		creation = time.Now()
	}
	modified := md.Modified
	if modified.IsZero() {
		modified = creation
	}
	rasterDPI := md.RasterDPI
	if rasterDPI < 36 {
		rasterDPI = 72
	}
	encodingQuality := md.EncodingQuality
	if encodingQuality < 1 {
		encodingQuality = 101
	}
	m.Title = toCStr(md.Title)
	m.Author = toCStr(md.Author)
	m.Subject = toCStr(md.Subject)
	m.Keywords = toCStr(md.Keywords)
	m.Creator = toCStr(md.Creator)
	m.Producer = toCStr(producer)
	m.Creation.set(creation)
	m.Modified.set(modified)
	m.RasterDPI = rasterDPI
	m.EncodingQuality = encodingQuality
}

type MetaData struct {
	Title           string
	Author          string
	Subject         string
	Keywords        string
	Creator         string
	Producer        string
	Creation        time.Time
	Modified        time.Time
	RasterDPI       float32
	EncodingQuality int32
}

func toCStr(s string) uintptr {
	cstr := make([]byte, len(s)+1)
	copy(cstr, s)
	return uintptr(unsafe.Pointer(&cstr[0]))
}
