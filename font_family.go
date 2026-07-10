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
	"maps"
	"slices"
	"strings"
	"sync"

	"github.com/richardwilkes/canvas/font"
	"github.com/richardwilkes/canvas/fontmgr"
	"github.com/richardwilkes/toolbox/v2/xstrings"
	"github.com/richardwilkes/unison/enums/slant"
	"github.com/richardwilkes/unison/enums/spacing"
	"github.com/richardwilkes/unison/enums/weight"
)

var (
	slantMapping = [][]int{
		{3, 1, 2},
		{1, 3, 2},
		{1, 2, 3},
	}
	cachedFontFamiliesLock sync.RWMutex
	cachedFontFamilies     []string
)

// FontFamily holds information about one font family.
type FontFamily struct {
	set  *fontmgr.StyleSet
	name string
}

// FontFamilies retrieves the names of the installed font families, using a cached version if available.
func FontFamilies() []string {
	cachedFontFamiliesLock.RLock()
	families := cachedFontFamilies
	cachedFontFamiliesLock.RUnlock()
	if len(families) == 0 {
		return FontFamiliesNoCache()
	}
	return families
}

// FontFamiliesNoCache retrieves the names of the installed font families.
func FontFamiliesNoCache() []string {
	cachedFontFamiliesLock.Lock()
	defer cachedFontFamiliesLock.Unlock()
	fm := fontmgr.Default()
	count := fm.CountFamilies()
	names := make(map[string]struct{}, count+len(internalFonts))
	for i := range count {
		names[fm.FamilyName(i)] = struct{}{}
	}
	internalFontLock.RLock()
	for k := range internalFonts {
		names[k] = struct{}{}
	}
	internalFontLock.RUnlock()
	cachedFontFamilies = slices.SortedFunc(maps.Keys(names), func(a, b string) int {
		return xstrings.NaturalCmp(a, b, true)
	})
	return cachedFontFamilies
}

// MatchFontFamily returns a FontFamily for the specified family name. If no such family name exists, Count() will be 0.
func MatchFontFamily(family string) *FontFamily {
	internalFontLock.RLock()
	_, exists := internalFonts[family]
	internalFontLock.RUnlock()
	if exists {
		return &FontFamily{name: family}
	}
	return &FontFamily{
		name: family,
		set:  fontmgr.Default().MatchFamily(family),
	}
}

// Count returns the number of Faces within this FontFamily.
func (f *FontFamily) Count() int {
	internalFontLock.RLock()
	defer internalFontLock.RUnlock()
	if fnt, exists := internalFonts[f.name]; exists {
		return len(fnt.faces)
	}
	return f.set.Count()
}

// Style returns the style information for the given index. Must be >= 0 and < Count().
func (f *FontFamily) Style(index int) (description string, weightValue weight.Enum, spacingValue spacing.Enum, slantValue slant.Enum) {
	internalFontLock.RLock()
	defer internalFontLock.RUnlock()
	if fnt, exists := internalFonts[f.name]; exists {
		if index >= 0 && index < len(fnt.faces) {
			weightValue, spacingValue, slantValue = fnt.faces[index].Style()
			var buffer strings.Builder
			buffer.WriteString(weightValue.String())
			if spacingValue != spacing.Standard {
				buffer.WriteString(" ")
				buffer.WriteString(spacingValue.String())
			}
			if slantValue != slant.Upright {
				buffer.WriteString(" ")
				buffer.WriteString(slantValue.String())
			}
			description = buffer.String()
		}
		return description, weightValue, spacingValue, slantValue
	}
	style, name := f.set.Style(index)
	return name, weight.Enum(style.Weight()), spacing.Enum(style.Width()), slant.Enum(style.Slant())
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
	return newFace(f.set.CreateTypeface(index))
}

// MatchStyle attempts to locate the FontFace within the family with the given style. Will return nil if nothing
// suitable can be found.
func (f *FontFamily) MatchStyle(weightValue weight.Enum, spacingValue spacing.Enum, slantValue slant.Enum) *FontFace {
	spacingValue = spacingValue.EnsureValid()
	slantValue = slantValue.EnsureValid()
	internalFontLock.RLock()
	defer internalFontLock.RUnlock()
	if fnt, exists := internalFonts[f.name]; exists {
		bestScore := 0
		bestIndex := 0
		for i, face := range fnt.faces {
			w, sp, sl := face.Style()
			if weightValue == w && spacingValue == sp && slantValue == sl {
				return face
			}
			var score int
			if spacingValue <= spacing.Standard {
				if sp <= spacingValue {
					score = 10 - int(spacingValue) + int(sp)
				} else {
					score = 10 - int(sp)
				}
			} else {
				if sp > spacingValue {
					score = 10 + int(spacingValue) - int(sp)
				} else {
					score = int(sp)
				}
			}
			score <<= 8
			score += slantMapping[slantValue][sl]
			score <<= 8
			switch {
			case weightValue == w:
				score += 1000
			case weightValue < weight.Regular:
				if w <= weightValue {
					score += 1000 - int(weightValue) + int(w)
				} else {
					score += 1000 - int(w)
				}
			case weightValue <= weight.Medium:
				switch {
				case w >= weightValue && w <= weight.Medium:
					score += 1000 + int(weightValue) - int(w)
				case w <= weightValue:
					score += 500 + int(w)
				default:
					score += 1000 - int(w)
				}
			default:
				if w > weightValue {
					score += 1000 + int(weightValue) - int(w)
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
	style := font.NewStyle(int(weightValue), int(spacingValue), font.Slant(slantValue))
	return newFace(f.set.MatchStyle(style))
}

func (f *FontFamily) String() string {
	return f.name
}
