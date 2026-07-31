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
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/unison/enums/slant"
	"github.com/richardwilkes/unison/enums/spacing"
	"github.com/richardwilkes/unison/enums/weight"
)

// TestMatchStyleScoreSlantOutranksWeight verifies the tier ordering of the style matching score: slant is a more
// significant criteria than weight. The weight component can reach 2000, which previously overflowed its 8-bit tier
// into the slant tier, letting a large weight distance override the slant preference (a Black Italic request against
// {Thin Italic, Black Upright} picked the Upright face).
func TestMatchStyleScoreSlantOutranksWeight(t *testing.T) {
	c := check.New(t)
	thinItalic := matchStyleScore(weight.Black, spacing.Standard, slant.Italic, weight.Thin, spacing.Standard, slant.Italic)
	blackUpright := matchStyleScore(weight.Black, spacing.Standard, slant.Italic, weight.Black, spacing.Standard, slant.Upright)
	c.True(thinItalic > blackUpright, "an italic face must outrank an upright one regardless of weight distance (%d vs %d)",
		thinItalic, blackUpright)

	// The mirrored request must also hold: Thin Upright against {Black Upright, Thin Italic} picks the Upright face.
	blackUpright = matchStyleScore(weight.Thin, spacing.Standard, slant.Upright, weight.Black, spacing.Standard, slant.Upright)
	thinItalic = matchStyleScore(weight.Thin, spacing.Standard, slant.Upright, weight.Thin, spacing.Standard, slant.Italic)
	c.True(blackUpright > thinItalic, "an upright face must outrank an italic one regardless of weight distance (%d vs %d)",
		blackUpright, thinItalic)

	// Spacing remains the most significant criteria of all.
	matchedSpacing := matchStyleScore(weight.Black, spacing.Standard, slant.Italic, weight.Thin, spacing.Standard, slant.Upright)
	otherSpacing := matchStyleScore(weight.Black, spacing.Standard, slant.Italic, weight.Black, spacing.UltraExpanded, slant.Italic)
	c.True(matchedSpacing > otherSpacing, "a spacing match must outrank slant and weight matches (%d vs %d)",
		matchedSpacing, otherSpacing)
}

// TestRegisterFontInvalidatesFontFamiliesCache verifies that registering a font drops the cached family list, so fonts
// registered after the first FontFamilies() call show up in later calls.
func TestRegisterFontInvalidatesFontFamiliesCache(t *testing.T) {
	c := check.New(t)
	data, err := fontFS.ReadFile("resources/fonts/Roboto - Regular.ttf")
	c.NoError(err)

	families := FontFamilies()
	c.True(len(families) > 0, "expected at least one font family")
	cachedFontFamiliesLock.RLock()
	populated := len(cachedFontFamilies) > 0
	cachedFontFamiliesLock.RUnlock()
	c.True(populated, "FontFamilies should populate the cache")

	ffd, err := RegisterFont(data)
	c.NoError(err)
	cachedFontFamiliesLock.RLock()
	invalidated := cachedFontFamilies == nil
	cachedFontFamiliesLock.RUnlock()
	c.True(invalidated, "RegisterFont should invalidate the cached family list")
	c.True(slices.Contains(FontFamilies(), ffd.Family),
		"the registered font's family (%s) should appear in FontFamilies()", ffd.Family)
}
