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
	"runtime"
	"slices"
	"sync"

	"github.com/richardwilkes/toolbox/txt"
	"github.com/richardwilkes/toolbox/xmath"
	"github.com/richardwilkes/unison/enums/slant"
	"github.com/richardwilkes/unison/enums/spacing"
	"github.com/richardwilkes/unison/enums/weight"
	"github.com/richardwilkes/unison/internal/skia"
)

var (
	faceCacheLock         sync.RWMutex
	faceCache             = make(map[skia.TypeFace]*FontFace)
	faceFallbackCacheLock sync.RWMutex
	faceFallbackCache     = make(map[faceFallbackCacheKey]*FontFace)
	fontSizeCacheLock     sync.RWMutex
	fontSizeCache         = make(map[FontDescriptor]float32)
)

type faceFallbackCacheKey struct {
	face skia.TypeFace
	r    rune
}

// FontFace holds the immutable portions of a font description.
type FontFace struct {
	face skia.TypeFace
}

func newFace(face skia.TypeFace) *FontFace {
	if face == nil {
		return nil
	}
	faceCacheLock.RLock()
	f, exists := faceCache[face]
	faceCacheLock.RUnlock()
	if exists {
		return f
	}
	faceCacheLock.Lock()
	defer faceCacheLock.Unlock()
	if f, exists = faceCache[face]; !exists {
		f = &FontFace{face: face}
		faceCache[face] = f
	}
	return f
}

// AllFontFaces returns all known font faces as FontFaceDescriptors. This will be computed each time, so it may be
// worthwhile to cache the result if you don't expect the set of font faces to be changed between calls.
func AllFontFaces() (all, monospaced []FontFaceDescriptor) {
	ma := make(map[FontFaceDescriptor]struct{})
	mm := make(map[FontFaceDescriptor]struct{})
	for _, family := range FontFamilies() {
		if ff := MatchFontFamily(family); ff != nil {
			count := ff.Count()
			for i := 0; i < count; i++ {
				face := ff.Face(i)
				w, sp, sl := face.Style()
				ffd := FontFaceDescriptor{
					Family:  family,
					Weight:  w,
					Spacing: sp,
					Slant:   sl,
				}
				if _, exists := ma[ffd]; !exists {
					all = append(all, ffd)
					ma[ffd] = struct{}{}
				}
				if face.Monospaced() {
					if _, exists := mm[ffd]; !exists {
						monospaced = append(monospaced, ffd)
						mm[ffd] = struct{}{}
					}
				}
			}
		}
	}
	sorter := func(a, b FontFaceDescriptor) int { return txt.NaturalCmp(a.String(), b.String(), true) }
	slices.SortFunc(all, sorter)
	slices.SortFunc(monospaced, sorter)
	return
}

// MatchFontFace attempts to locate the FontFace with the given family and style. Will return nil if nothing suitable
// can be found.
func MatchFontFace(family string, weightValue weight.Enum, spacingValue spacing.Enum, slantValue slant.Enum) *FontFace {
	internalFontLock.Lock()
	_, exists := internalFonts[family]
	internalFontLock.Unlock()
	if exists {
		fam := MatchFontFamily(family)
		return fam.MatchStyle(weightValue, spacingValue, slantValue)
	}
	style := skia.FontStyleNew(skia.FontWeight(weightValue), skia.FontSpacing(spacingValue), skia.FontSlant(slantValue))
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
	w, sp, sl := f.Style()
	fd := FontDescriptor{
		FontFaceDescriptor: FontFaceDescriptor{
			Family:  f.Family(),
			Weight:  w,
			Spacing: sp,
			Slant:   sl,
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
	key := faceFallbackCacheKey{
		face: f.face,
		r:    ch,
	}
	faceFallbackCacheLock.RLock()
	ff, exists := faceFallbackCache[key]
	faceFallbackCacheLock.RUnlock()
	if exists {
		return ff
	}
	faceFallbackCacheLock.Lock()
	defer faceFallbackCacheLock.Unlock()
	if ff, exists = faceFallbackCache[key]; exists {
		return ff
	}
	style := skia.TypeFaceGetFontStyle(f.face)
	defer skia.FontStyleDelete(style)
	ff = newFace(skia.FontMgrMatchFamilyStyleCharacter(skia.FontMgrRefDefault(), f.Family(), style, ch))
	faceFallbackCache[key] = ff
	return ff
}

func (f *FontFace) createFontWithSkiaSize(skiaSize float32) *fontImpl {
	font := &fontImpl{
		face: f,
		font: skia.FontNewWithValues(f.face, skiaSize, 1, 0),
	}
	skia.FontSetSubPixel(font.font, true)
	skia.FontSetForceAutoHinting(font.font, true)
	// Using hinting on some platforms (Linux, for example) was resulting in bad placement of the text. Carefully test
	// any changes away from no font hinting (0) on all supported platforms.
	skia.FontSetHinting(font.font, 0)
	skia.FontGetMetrics(font.font, &font.metrics)
	runtime.AddCleanup(font, func(sf skia.Font) {
		ReleaseOnUIThread(func() {
			skia.FontDelete(sf)
		})
	}, font.font)
	return font
}

// Style returns the style information for this FontFace.
func (f *FontFace) Style() (weightValue weight.Enum, spacingValue spacing.Enum, slantValue slant.Enum) {
	style := skia.TypeFaceGetFontStyle(f.face)
	defer skia.FontStyleDelete(style)
	return weight.Enum(skia.FontStyleGetWeight(style)), spacing.Enum(skia.FontStyleGetWidth(style)),
		slant.Enum(skia.FontStyleGetSlant(style))
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
