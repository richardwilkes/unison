// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"slices"
	"sync"

	"github.com/richardwilkes/canvas/font"
	"github.com/richardwilkes/canvas/fontmgr"
	"github.com/richardwilkes/toolbox/v2/xmath"
	"github.com/richardwilkes/toolbox/v2/xstrings"
	"github.com/richardwilkes/unison/enums/slant"
	"github.com/richardwilkes/unison/enums/spacing"
	"github.com/richardwilkes/unison/enums/weight"
)

var (
	faceCacheLock         sync.RWMutex
	faceCache             = make(map[*font.Typeface]*FontFace)
	faceFallbackCacheLock sync.RWMutex
	faceFallbackCache     = make(map[faceFallbackCacheKey]*FontFace)
	fontSizeCacheLock     sync.RWMutex
	fontSizeCache         = make(map[FontDescriptor]float32)
)

type faceFallbackCacheKey struct {
	face *font.Typeface
	r    rune
}

// FontFace holds the immutable portions of a font description.
type FontFace struct {
	face *font.Typeface
}

func newFace(face *font.Typeface) *FontFace {
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
			for i := range count {
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
	sorter := func(a, b FontFaceDescriptor) int { return xstrings.NaturalCmp(a.String(), b.String(), true) }
	slices.SortFunc(all, sorter)
	slices.SortFunc(monospaced, sorter)
	return all, monospaced
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
	return newFace(fontmgr.Default().MatchFamilyStyle(family,
		font.NewStyle(int(weightValue), int(spacingValue), font.Slant(slantValue))))
}

// CreateFontFace creates a new FontFace from font data.
func CreateFontFace(data []byte) *FontFace {
	localData := make([]byte, len(data))
	copy(localData, data)
	return newFace(fontmgr.Default().MakeFromData(localData, 0))
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
	size, exists := fontSizeCache[fd]
	fontSizeCacheLock.RUnlock()
	if exists {
		fi := f.createFontWithSize(size)
		fi.size = capHeightSizeInLogicalPixels
		return fi
	}
	size = capHeightSizeInLogicalPixels
	var fi *fontImpl
	fi = f.createFontWithSize(size)
	if fi.metrics.CapHeight > 0 { // I've seen some fonts with a negative CapHeight, which won't work
		size = xmath.Floor(capHeightSizeInLogicalPixels * size / fi.metrics.CapHeight)
		for {
			fi = f.createFontWithSize(size)
			if fi.metrics.CapHeight >= capHeightSizeInLogicalPixels {
				break
			}
			size++
		}
		for size >= 1 && fi.metrics.CapHeight > capHeightSizeInLogicalPixels {
			size -= 0.5
			fi = f.createFontWithSize(size)
		}
	}
	fi.size = capHeightSizeInLogicalPixels
	fontSizeCacheLock.Lock()
	fontSizeCache[fd] = size
	fontSizeCacheLock.Unlock()
	return fi
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
	ff = newFace(fontmgr.Default().MatchFamilyStyleCharacter(f.Family(), f.face.Style(), nil, ch))
	faceFallbackCache[key] = ff
	return ff
}

func (f *FontFace) createFontWithSize(size float32) *fontImpl {
	fi := &fontImpl{
		face: f,
		font: font.NewFont(f.face, size, 1, 0),
	}
	fi.font.SetSubpixel(true)
	fi.font.SetForceAutoHinting(true)
	// Using hinting on some platforms (Linux, for example) was resulting in bad placement of the text. Carefully test
	// any changes away from no font hinting on all supported platforms.
	fi.font.SetHinting(font.HintingNone)
	fi.font.Metrics(&fi.metrics)
	return fi
}

// Style returns the style information for this FontFace.
func (f *FontFace) Style() (weightValue weight.Enum, spacingValue spacing.Enum, slantValue slant.Enum) {
	style := f.face.Style()
	return weight.Enum(style.Weight()), spacing.Enum(style.Width()), slant.Enum(style.Slant())
}

// Monospaced returns true if this FontFace has been marked as having a fixed width for every character.
func (f *FontFace) Monospaced() bool {
	return f.face.IsFixedPitch()
}

// Family returns the name of the FontFamily this FontFace belongs to.
func (f *FontFace) Family() string {
	return f.face.FamilyName()
}

// UnitsPerEm returns the number of coordinate units on the "em square", an abstract square whose height is the intended
// distance between lines of type in the same type size. This is the size of the design grid on which glyphs are laid
// out.
func (f *FontFace) UnitsPerEm() int {
	return f.face.UnitsPerEm()
}

// Less returns true if this FontFace is logically before the other FontFace.
func (f *FontFace) Less(other *FontFace) bool {
	f1 := f.Family()
	f2 := other.Family()
	if xstrings.NaturalLess(f1, f2, true) {
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
