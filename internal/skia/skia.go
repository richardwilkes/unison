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

func (r *Rect) ToRect() geom.Rect[float32] {
	return geom.Rect[float32]{
		Point: geom.Point[float32]{
			X: r.Left,
			Y: r.Top,
		},
		Size: geom.Size[float32]{
			Width:  r.Right - r.Left,
			Height: r.Bottom - r.Top,
		},
	}
}

func RectToSkRect(r *geom.Rect[float32]) *Rect {
	return &Rect{
		Left:   r.X,
		Top:    r.Y,
		Right:  r.Right(),
		Bottom: r.Bottom(),
	}
}

type IRect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

func RectToSkIRect(r *geom.Rect[float32]) *IRect {
	return &IRect{
		Left:   int32(r.X),
		Top:    int32(r.Y),
		Right:  int32(r.Right()),
		Bottom: int32(r.Bottom()),
	}
}

type Matrix struct {
	ScaleX float32
	SkewX  float32
	TransX float32
	SkewY  float32
	ScaleY float32
	TransY float32
	Persp0 float32
	Persp1 float32
	Persp2 float32
}

func (m *Matrix) ToMatrix2D() *geom.Matrix2D[float32] {
	return &geom.Matrix2D[float32]{
		ScaleX: m.ScaleX,
		SkewX:  m.SkewX,
		TransX: m.TransX,
		SkewY:  m.SkewY,
		ScaleY: m.ScaleY,
		TransY: m.TransY,
	}
}

func Matrix2DtoMatrix(m *geom.Matrix2D[float32]) *Matrix {
	if m == nil {
		return nil
	}
	return &Matrix{
		ScaleX: m.ScaleX,
		SkewX:  m.SkewX,
		TransX: m.TransX,
		SkewY:  m.SkewY,
		ScaleY: m.ScaleY,
		TransY: m.TransY,
		Persp2: 1,
	}
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
