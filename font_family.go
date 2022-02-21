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
	"sort"
	"strings"

	"github.com/richardwilkes/toolbox"
	"github.com/richardwilkes/toolbox/txt"
	"github.com/richardwilkes/unison/internal/skia"
)

var slantMapping = [][]int{
	{3, 1, 2},
	{1, 3, 2},
	{1, 2, 3},
}

// FontFamily holds information about one font family.
type FontFamily struct {
	name string
	set  skia.FontStyleSet
}

// FontFamilies retrieves the names of the installed font families.
func FontFamilies() []string {
	fm := skia.FontMgrRefDefault()
	count := skia.FontMgrCountFamilies(fm)
	names := make(map[string]bool)
	if runtime.GOOS == toolbox.MacOS {
		// This is a special font on macOS. Ideally, I'd find a source for an equivalent font and embed it so that the
		// same font could be used on all platforms.
		names[".Keyboard"] = true
	}
	ss := skia.StringNewEmpty()
	for i := 0; i < count; i++ {
		skia.FontMgrGetFamilyName(fm, i, ss)
		names[skia.StringGetString(ss)] = true
	}
	skia.StringDelete(ss)
	internalFontLock.RLock()
	for k := range internalFonts {
		names[k] = true
	}
	internalFontLock.RUnlock()
	families := make([]string, 0, len(names))
	for k := range names {
		families = append(families, k)
	}
	sort.Slice(families, func(i, j int) bool { return txt.NaturalLess(families[i], families[j], true) })
	return families
}

// MatchFontFamily returns a FontFamily for the specified family name. If no such family name exists, Count() will be 0.
func MatchFontFamily(family string) *FontFamily {
	internalFontLock.RLock()
	_, exists := internalFonts[family]
	internalFontLock.RUnlock()
	if exists {
		return &FontFamily{name: family}
	}
	f := &FontFamily{
		name: family,
		set:  skia.FontMgrMatchFamily(skia.FontMgrRefDefault(), family),
	}
	runtime.SetFinalizer(f, func(obj *FontFamily) {
		ReleaseOnUIThread(func() {
			skia.FontStyleSetUnref(obj.set)
		})
	})
	return f
}

// Count returns the number of Faces within this FontFamily.
func (f *FontFamily) Count() int {
	internalFontLock.RLock()
	defer internalFontLock.RUnlock()
	if fnt, exists := internalFonts[f.name]; exists {
		return len(fnt.faces)
	}
	return skia.FontStyleSetGetCount(f.set)
}

// Style returns the style information for the given index. Must be >= 0 and < Count().
func (f *FontFamily) Style(index int) (description string, weight FontWeight, spacing FontSpacing, slant FontSlant) {
	internalFontLock.RLock()
	defer internalFontLock.RUnlock()
	if fnt, exists := internalFonts[f.name]; exists {
		if index >= 0 && index < len(fnt.faces) {
			weight, spacing, slant = fnt.faces[index].Style()
			var buffer strings.Builder
			buffer.WriteString(weight.String())
			if spacing != StandardSpacing {
				buffer.WriteString(" ")
				buffer.WriteString(spacing.String())
			}
			if slant != NoSlant {
				buffer.WriteString(" ")
				buffer.WriteString(slant.String())
			}
			description = buffer.String()
		}
		return
	}
	ss := skia.StringNewEmpty()
	defer skia.StringDelete(ss)
	style := skia.FontStyleNew(0, 0, 0)
	defer skia.FontStyleDelete(style)
	skia.FontStyleSetGetStyle(f.set, index, style, ss)
	return skia.StringGetString(ss), FontWeight(skia.FontStyleGetWeight(style)),
		FontSpacing(skia.FontStyleGetWidth(style)), FontSlant(skia.FontStyleGetSlant(style))
}

// Face returns the FontFace for the given index. Must be >= 0 and < Count().
func (f *FontFamily) Face(index int) *FontFace {
	internalFontLock.RLock()
	defer internalFontLock.RUnlock()
	if fnt, exists := internalFonts[f.name]; exists {
		if index >= 0 && index < len(fnt.faces) {
			return fnt.faces[index]
		}
		return nil
	}
	return newFace(skia.FontStyleSetCreateTypeFace(f.set, index))
}

// MatchStyle attempts to locate the FontFace within the family with the given style. Will return nil if nothing
// suitable can be found.
func (f *FontFamily) MatchStyle(weight FontWeight, spacing FontSpacing, slant FontSlant) *FontFace {
	internalFontLock.RLock()
	defer internalFontLock.RUnlock()
	if fnt, exists := internalFonts[f.name]; exists {
		bestScore := 0
		bestIndex := 0
		for i, face := range fnt.faces {
			w, sp, sl := face.Style()
			if weight == w && spacing == sp && slant == sl {
				return face
			}
			var score int
			if spacing <= StandardSpacing {
				if sp <= spacing {
					score = 10 - int(spacing) + int(sp)
				} else {
					score = 10 - int(sp)
				}
			} else {
				if sp > spacing {
					score = 10 + int(spacing) - int(sp)
				} else {
					score = int(sp)
				}
			}
			score <<= 8
			score += slantMapping[slant][sl]
			score <<= 8
			switch {
			case weight == w:
				score += 1000
			case weight < NormalFontWeight:
				if w <= weight {
					score += 1000 - int(weight) + int(w)
				} else {
					score += 1000 - int(w)
				}
			case weight <= MediumFontWeight:
				switch {
				case w >= weight && w <= MediumFontWeight:
					score += 1000 + int(weight) - int(w)
				case w <= weight:
					score += 500 + int(w)
				default:
					score += 1000 - int(w)
				}
			default:
				if w > weight {
					score += 1000 + int(weight) - int(w)
				} else {
					score += int(w)
				}
			}
			if bestScore < score {
				bestScore = score
				bestIndex = i
			}
		}
		return fnt.faces[bestIndex]
	}
	style := skia.FontStyleNew(skia.FontWeight(weight), skia.FontSpacing(spacing), skia.FontSlant(slant))
	defer skia.FontStyleDelete(style)
	return newFace(skia.FontStyleSetMatchStyle(f.set, style))
}

func (f *FontFamily) String() string {
	return f.name
}
