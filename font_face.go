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
	"runtime"
	"sync"

	"github.com/richardwilkes/toolbox/txt"
	"github.com/richardwilkes/toolbox/xmath"
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

// AllFontFaces returns all known font faces as FontFaceDescriptors. This will be computed each time, so it may be
// worthwhile to cache the result if you don't expect the set of font faces to be changed between calls.
func AllFontFaces() []*FontFaceDescriptor {
	var all []*FontFaceDescriptor
	for _, family := range FontFamilies() {
		if ff := MatchFontFamily(family); ff != nil {
			count := ff.Count()
			for i := 0; i < count; i++ {
				_, weight, spacing, slant := ff.Style(i)
				all = append(all, &FontFaceDescriptor{
					Family:  family,
					Weight:  weight,
					Spacing: spacing,
					Slant:   slant,
				})
			}
		}
	}
	return all
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
func (f *FontFace) Font(capHeightSizeInLogicalPixels float32) Font {
	weight, spacing, slant := f.Style()
	fd := FontDescriptor{
		FontFaceDescriptor: FontFaceDescriptor{
			Family:  f.Family(),
			Weight:  weight,
			Spacing: spacing,
			Slant:   slant,
		},
		Size: capHeightSizeInLogicalPixels,
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
	var font *fontImpl
	font = f.createFontWithSkiaSize(skiaSize)
	if font.metrics.CapHeight > 0 { // I've seen some fonts with a negative CapHeight, which won't work
		skiaSize = xmath.Floor(capHeightSizeInLogicalPixels * skiaSize / font.metrics.CapHeight)
		for {
			font = f.createFontWithSkiaSize(skiaSize)
			if font.metrics.CapHeight >= capHeightSizeInLogicalPixels {
				break
			}
			skiaSize++
		}
		for skiaSize >= 1 && font.metrics.CapHeight > capHeightSizeInLogicalPixels {
			skiaSize -= 0.5
			font = f.createFontWithSkiaSize(skiaSize)
		}
	}
	font.size = capHeightSizeInLogicalPixels
	fontSizeCacheLock.Lock()
	fontSizeCache[fd] = skiaSize
	fontSizeCacheLock.Unlock()
	return font
}

// FallbackForCharacter attempts to locate the FontFace that best matches this FontFace and has the given character.
// Will return nil if nothing suitable can be found.
func (f *FontFace) FallbackForCharacter(ch rune) *FontFace {
	style := skia.TypeFaceGetFontStyle(f.face)
	defer skia.FontStyleDelete(style)
	return newFace(skia.FontMgrMatchFamilyStyleCharacter(skia.FontMgrRefDefault(), f.Family(), style, ch))
}

func (f *FontFace) createFontWithSkiaSize(skiaSize float32) *fontImpl {
	font := &fontImpl{
		face: f,
		font: skia.FontNewWithValues(f.face, skiaSize, 1, 0),
	}
	skia.FontSetSubPixel(font.font, true)
	skia.FontSetForceAutoHinting(font.font, true)
	// Using hinting on some platforms (Linux, for example) was resulting in bad placement of the text. Carefully test
	// any changes away from FontHintingNone on all supported platforms.
	skia.FontSetHinting(font.font, skia.FontHinting(FontHintingNone))
	skia.FontGetMetrics(font.font, &font.metrics)
	runtime.SetFinalizer(font, func(obj *fontImpl) {
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

// Less returns true if this FontFace is logically before the other FontFace.
func (f *FontFace) Less(other *FontFace) bool {
	f1 := f.Family()
	f2 := other.Family()
	if txt.NaturalLess(f1, f2, true) {
		return true
	}
	if f1 != f2 {
		return false
	}
	w1, sp1, sl1 := f.Style()
	w2, sp2, sl2 := other.Style()
	if w1 < w2 {
		return true
	}
	if w1 != w2 {
		return false
	}
	if sp1 < sp2 {
		return true
	}
	if sp1 != sp2 {
		return false
	}
	if sl1 < sl2 {
		return true
	}
	return false
}
