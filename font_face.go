// Copyright ©2021 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"runtime"
	"strings"
	"sync"

	"github.com/richardwilkes/toolbox/xmath/mathf32"
	"github.com/richardwilkes/unison/internal/skia"
)

var (
	fontSizeCacheLock sync.RWMutex
	fontSizeCache     = make(map[FontDescriptor]float32)
)

// FontFace holds the immutable portions of a font description.
type FontFace struct {
	face skia.TypeFace
}

func newFace(face skia.TypeFace) *FontFace {
	if face == nil {
		return nil
	}
	f := &FontFace{face: face}
	runtime.SetFinalizer(f, func(obj *FontFace) {
		ReleaseOnUIThread(func() {
			skia.TypeFaceUnref(obj.face)
		})
	})
	return f
}

// MatchFontFace attempts to locate the FontFace with the given family and style. Will return nil if nothing suitable
// can be found.
func MatchFontFace(family string, weight FontWeight, spacing FontSpacing, slant FontSlant) *FontFace {
	internalFontLock.Lock()
	_, exists := internalFonts[family]
	internalFontLock.Unlock()
	if exists {
		fam := MatchFontFamily(family)
		return fam.MatchStyle(weight, spacing, slant)
	}
	style := skia.FontStyleNew(skia.FontWeight(weight), skia.FontSpacing(spacing), skia.FontSlant(slant))
	defer skia.FontStyleDelete(style)
	return newFace(skia.FontMgrMatchFamilyStyle(skia.FontMgrRefDefault(), family, style))
}

// CreateFontFace creates a new FontFace from font data.
func CreateFontFace(data []byte) *FontFace {
	cData := skia.DataNewWithCopy(data)
	defer skia.DataUnref(cData)
	return newFace(skia.FontMgrCreateFromData(skia.FontMgrRefDefault(), cData))
}

// Font returns a Font of the given size for this FontFace.
func (f *FontFace) Font(capHeightSizeInLogicalPixels float32) *Font {
	weight, spacing, slant := f.Style()
	fd := FontDescriptor{
		Family:  f.Family(),
		Size:    capHeightSizeInLogicalPixels,
		Weight:  weight,
		Spacing: spacing,
		Slant:   slant,
	}
	fontSizeCacheLock.RLock()
	skiaSize, exists := fontSizeCache[fd]
	fontSizeCacheLock.RUnlock()
	if exists {
		font := f.createFontWithSkiaSize(skiaSize)
		font.size = capHeightSizeInLogicalPixels
		return font
	}
	skiaSize = capHeightSizeInLogicalPixels
	var font *Font
	font = f.createFontWithSkiaSize(skiaSize)
	skiaSize = mathf32.Floor(capHeightSizeInLogicalPixels * skiaSize / font.metrics.CapHeight)
	for {
		font = f.createFontWithSkiaSize(skiaSize)
		if font.metrics.CapHeight >= capHeightSizeInLogicalPixels {
			break
		}
		skiaSize++
	}
	for font.metrics.CapHeight > capHeightSizeInLogicalPixels {
		skiaSize -= 0.5
		font = f.createFontWithSkiaSize(skiaSize)
	}
	font.size = capHeightSizeInLogicalPixels
	fontSizeCacheLock.Lock()
	fontSizeCache[fd] = skiaSize
	fontSizeCacheLock.Unlock()
	return font
}

func (f *FontFace) createFontWithSkiaSize(skiaSize float32) *Font {
	font := &Font{
		face: f,
		font: skia.FontNewWithValues(f.face, skiaSize, 1, 0),
	}
	skia.FontSetSubPixel(font.font, true)
	skia.FontSetForceAutoHinting(font.font, true)
	skia.FontSetHinting(font.font, skia.FontHinting(FontHintingFull))
	skia.FontGetMetrics(font.font, (*skia.FontMetrics)(&font.metrics))
	runtime.SetFinalizer(font, func(obj *Font) {
		ReleaseOnUIThread(func() {
			skia.FontDelete(obj.font)
		})
	})
	return font
}

// Style returns the style information for this FontFace.
func (f *FontFace) Style() (weight FontWeight, spacing FontSpacing, slant FontSlant) {
	style := skia.TypeFaceGetFontStyle(f.face)
	defer skia.FontStyleDelete(style)
	return FontWeight(skia.FontStyleGetWeight(style)), FontSpacing(skia.FontStyleGetWidth(style)),
		FontSlant(skia.FontStyleGetSlant(style))
}

// Monospaced returns true if this FontFace has been marked as having a fixed width for every character.
func (f *FontFace) Monospaced() bool {
	return skia.TypeFaceIsFixedPitch(f.face)
}

// Family returns the name of the FontFamily this FontFace belongs to.
func (f *FontFace) Family() string {
	ss := skia.TypeFaceGetFamilyName(f.face)
	defer skia.StringDelete(ss)
	return skia.StringGetString(ss)
}

// UnitsPerEm returns the number of coordinate units on the "em square", an abstract square whose height is the intended
// distance between lines of type in the same type size. This is the size of the design grid on which glyphs are laid
// out.
func (f *FontFace) UnitsPerEm() int {
	return skia.TypeFaceGetUnitsPerEm(f.face)
}

func (f *FontFace) String() string {
	var buffer strings.Builder
	buffer.WriteString(f.Family())
	weight, spacing, slant := f.Style()
	buffer.WriteByte(' ')
	buffer.WriteString(weight.String())
	if spacing != StandardSpacing {
		buffer.WriteByte(' ')
		buffer.WriteString(spacing.String())
	}
	if slant != NoSlant {
		buffer.WriteByte(' ')
		buffer.WriteString(slant.String())
	}
	return buffer.String()
}
