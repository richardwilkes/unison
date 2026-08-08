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

// fontStyleTriple is one candidate face's style, standing in for a *FontFace in the selection tests below.
type fontStyleTriple struct {
	w  weight.Enum
	sp spacing.Enum
	sl slant.Enum
}

// bestStyleFor mirrors the argmax loop MatchStyle runs over a family's faces, returning the index of the winner. Its
// exact-match early return is intentionally omitted, since it cannot fire in these cases: every candidate here differs
// from the request in weight or slant. The embedded fonts are all Standard spacing, so this stands in for a family
// carrying condensed or expanded faces.
func bestStyleFor(wantW weight.Enum, wantSp spacing.Enum, wantSl slant.Enum, faces []fontStyleTriple) int {
	bestScore := 0
	bestIndex := 0
	for i, face := range faces {
		if score := matchStyleScore(wantW, wantSp, wantSl, face.w, face.sp, face.sl); bestScore < score {
			bestScore = score
			bestIndex = i
		}
	}
	return bestIndex
}

// TestMatchStyleSelectsExactSpacingOverWiderFace reproduces the reported selection failure: asking for SemiExpanded
// picked an UltraExpanded face over the exact SemiExpanded one, because the exact match was scored as int(sp) rather
// than the full 10 in the dominant spacing tier.
func TestMatchStyleSelectsExactSpacingOverWiderFace(t *testing.T) {
	c := check.New(t)
	faces := []fontStyleTriple{
		{w: weight.Black, sp: spacing.UltraExpanded, sl: slant.Upright},
		{w: weight.Regular, sp: spacing.SemiExpanded, sl: slant.Italic},
	}
	c.Equal(1, bestStyleFor(weight.Regular, spacing.SemiExpanded, slant.Upright, faces),
		"the exact spacing match must be chosen over a wider face")

	// The same holds with the faces in the opposite order, so the win is not an artifact of iteration order.
	slices.Reverse(faces)
	c.Equal(0, bestStyleFor(weight.Regular, spacing.SemiExpanded, slant.Upright, faces),
		"the exact spacing match must be chosen over a wider face")
}

// TestMatchStyleScoreExactSpacingOutranksEveryOther verifies that, for every requested spacing, an exactly matching
// face outranks a face of any other spacing, even when the exact face's weight and slant are the worst available and
// the others' are perfect. Spacing is the dominant tier, so nothing below it may overturn the match.
func TestMatchStyleScoreExactSpacingOutranksEveryOther(t *testing.T) {
	c := check.New(t)
	for _, want := range spacing.All {
		// Handicap the exactly-matching face on both lesser tiers, and give every rival the requested weight and slant.
		exact := matchStyleScore(weight.Regular, want, slant.Upright, weight.Black, want, slant.Italic)
		for _, other := range spacing.All {
			if other == want {
				continue
			}
			score := matchStyleScore(weight.Regular, want, slant.Upright, weight.Regular, other, slant.Upright)
			c.True(exact > score,
				"requested %v: the exact match scored %d, but a %v face scored %d", want, exact, other, score)
		}
	}
}
