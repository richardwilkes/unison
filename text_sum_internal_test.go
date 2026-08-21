// Copyright (c) 2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"strings"
	"testing"

	"github.com/richardwilkes/canvas/raster"
	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/internal/f32"
)

// These tests cover the parts of Text that were restructured around f32.Sum() and around walking decoration runs
// instead of individual runes. Unlike the tolerance-based checks in text_layout_internal_test.go, the comparisons here
// are deliberately exact: the vertical metrics are a min/max over per-run values, so folding a run's value in once
// rather than once per rune cannot move a bit, and Width() and PositionForRuneIndex() must return precisely what
// f32.Sum() returns for the same slice, since they are now the same code path.

// textSumSink keeps benchmarked results live.
var textSumSink float32

// referenceVerticalMetrics computes the height and baseline the way cache() did before it walked decoration runs: one
// pass over the decorations, folding every rune's font metrics in individually. Walking runs must not change either
// result, since the terms a run skips are identical to the one it folds in.
func referenceVerticalMetrics(t *Text) (height, baseline float32) {
	top := t.emptyTop
	bottom := t.emptyBottom
	for _, d := range t.decorations {
		b := d.Font.Baseline()
		top = min(top, min(d.BaselineOffset, 0)-b)
		bottom = max(bottom, d.Font.LineHeight()-b+max(d.BaselineOffset, 0))
	}
	return bottom - top, -top
}

// invalidate marks the cached extents stale so that the next accessor recomputes them. Tests that reach past the
// public API to install decorations have to do this by hand, since only AddRunes() does it on their behalf.
func invalidate(t *Text) {
	t.extents.Width = -1
}

// clonePerRune replaces every decoration with a distinct clone of itself. The values are untouched, so runs that were
// equivalent stay equivalent and runs that were not stay not; only the pointer identity AddRunes() establishes is
// destroyed. This is what forces cache() to see a fresh run at every rune.
func clonePerRune(t *Text) {
	for i := range t.decorations {
		t.decorations[i] = t.decorations[i].Clone()
	}
	invalidate(t)
}

// TestTextSumCacheMetricsMatchPerRuneWalk locks the vertical metrics produced by cache() to the per-rune reference
// computation, bit for bit, whether the decorations arrive as long runs of a single pointer (what AddRunes() builds) or
// as a distinct pointer per rune (which no public path produces, but which cache() must still handle).
func TestTextSumCacheMetricsMatchPerRuneWalk(t *testing.T) {
	plain := &TextDecoration{Font: SystemFont, OnBackgroundInk: Black}
	bold := &TextDecoration{Font: EmphasizedSystemFont, OnBackgroundInk: Black, Underline: true}
	mono := &TextDecoration{Font: MonospacedFont, OnBackgroundInk: Black, StrikeThrough: true}
	bigFont := MonospacedFont.Face().Font(20)
	big := &TextDecoration{Font: bigFont, OnBackgroundInk: Black}
	sup := &TextDecoration{Font: SystemFont, OnBackgroundInk: Black, BaselineOffset: -6}
	sub := &TextDecoration{Font: bigFont, OnBackgroundInk: Black, BaselineOffset: 7}

	for _, tc := range []struct {
		build func() *Text
		name  string
	}{
		{
			name:  "empty",
			build: func() *Text { return NewText("", plain) },
		},
		{
			name:  "empty with an offset creation decoration",
			build: func() *Text { return NewText("", sup) },
		},
		{
			name:  "single rune",
			build: func() *Text { return NewText("H", plain) },
		},
		{
			name:  "one long shared pointer run",
			build: func() *Text { return NewText(strings.Repeat("Hello, World! ", 16), plain) },
		},
		{
			name: "several shared pointer runs",
			build: func() *Text {
				txt := NewText("plain ", plain)
				txt.AddString("bold ", bold)
				txt.AddString("mono ", mono)
				txt.AddString("big ", big)
				txt.AddString("raised ", sup)
				txt.AddString("lowered", sub)
				return txt
			},
		},
		{
			name: "a distinct pointer per rune, all equivalent",
			build: func() *Text {
				txt := NewText("Hello, World", plain)
				clonePerRune(txt)
				return txt
			},
		},
		{
			name: "a distinct pointer per rune across several runs",
			build: func() *Text {
				txt := NewText("plain ", plain)
				txt.AddString("big ", big)
				txt.AddString("lowered", sub)
				clonePerRune(txt)
				return txt
			},
		},
		{
			name: "a different decoration at every rune",
			build: func() *Text {
				decorations := []*TextDecoration{plain, bold, mono, big, sup, sub}
				txt := NewText(strings.Repeat("x", 4*len(decorations)), plain)
				for i := range txt.decorations {
					txt.decorations[i] = decorations[i%len(decorations)].Clone()
				}
				invalidate(txt)
				return txt
			},
		},
		{
			name: "the tallest decoration appears only once, mid run",
			build: func() *Text {
				txt := NewText(strings.Repeat("x", 64), plain)
				txt.decorations[37] = sub.Clone()
				invalidate(txt)
				return txt
			},
		},
		{
			name: "the tallest decoration appears only once, as the final rune",
			build: func() *Text {
				txt := NewText(strings.Repeat("x", 64), plain)
				txt.decorations[len(txt.decorations)-1] = sub.Clone()
				invalidate(txt)
				return txt
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			txt := tc.build()
			expectedHeight, expectedBaseline := referenceVerticalMetrics(txt)
			c.Equal(expectedHeight, txt.Height(), "height")
			c.Equal(expectedBaseline, txt.Baseline(), "baseline")
			c.Equal(expectedHeight, txt.Extents().Height, "extents height")

			// A second, freshly computed pass must land on the same values, and the cached values must survive being
			// read again without recomputation.
			cachedHeight := txt.Height()
			cachedBaseline := txt.Baseline()
			invalidate(txt)
			c.Equal(cachedHeight, txt.Height(), "height after recomputation")
			c.Equal(cachedBaseline, txt.Baseline(), "baseline after recomputation")

			// Metrics are also unaffected by whether the decorations share pointers, so a clone-per-rune copy of the
			// same Text must report exactly the same numbers.
			clonePerRune(txt)
			c.Equal(cachedHeight, txt.Height(), "height with a distinct pointer per rune")
			c.Equal(cachedBaseline, txt.Baseline(), "baseline with a distinct pointer per rune")
		})
	}
}

// TestTextSumWidthMatchesSum locks Width() and PositionForRuneIndex() to f32.Sum() of the widths they cover. They now
// share that code path, so the equality is exact and must hold through every mutation the public API allows.
func TestTextSumWidthMatchesSum(t *testing.T) {
	plain := &TextDecoration{Font: SystemFont, OnBackgroundInk: Black}
	bold := &TextDecoration{Font: EmphasizedSystemFont, OnBackgroundInk: Black, Underline: true}
	mono := &TextDecoration{Font: MonospacedFont, OnBackgroundInk: Black}

	checkWidth := func(c check.Checker, txt *Text, msg string, args ...any) {
		c.Helper()
		all := make([]any, 0, len(args)+1)
		all = append(all, msg)
		all = append(all, args...)
		c.Equal(f32.Sum(txt.widths), txt.Width(), all...)
		c.Equal(txt.Width(), txt.Extents().Width, all...)
		c.Equal(txt.Width(), txt.PositionForRuneIndex(len(txt.widths)), all...)
	}

	t.Run("through mutations", func(t *testing.T) {
		c := check.New(t)
		txt := NewText("", plain)
		checkWidth(c, txt, "empty")
		txt.AddString("Hello, ", plain)
		checkWidth(c, txt, "after the first AddString")
		txt.AddRunes([]rune("World"), bold)
		checkWidth(c, txt, "after AddRunes")
		txt.AddString("", mono)
		checkWidth(c, txt, "after adding an empty string")
		txt.AddString(strings.Repeat(" and more text", 8), mono)
		checkWidth(c, txt, "after a long AddString")
		txt.AddString("tail", plain)
		checkWidth(c, txt, "after the final AddString")
	})

	t.Run("through slices", func(t *testing.T) {
		c := check.New(t)
		txt := NewText("Hello ", plain)
		txt.AddString("World", bold)
		txt.AddString(" again", mono)
		for i := range len(txt.runes) + 1 {
			for j := i; j <= len(txt.runes); j++ {
				slice := txt.Slice(i, j)
				checkWidth(c, slice, "Slice(%d, %d)", i, j)
				c.Equal(f32.Sum(txt.widths[i:j]), slice.Width(), "Slice(%d, %d) vs the widths it covers", i, j)
			}
		}
		// Out of range indexes clamp rather than panicking, and the clamped slice is still the sum of its widths.
		checkWidth(c, txt.Slice(-10, 10000), "Slice(-10, 10000)")
		c.Equal(txt.Width(), txt.Slice(-10, 10000).Width(), "a fully clamped slice covers the whole width")
	})

	t.Run("positions are prefix sums", func(t *testing.T) {
		c := check.New(t)
		txt := NewText("Hello ", plain)
		txt.AddString("World", bold)
		for i := range len(txt.widths) + 1 {
			c.Equal(f32.Sum(txt.widths[:i]), txt.PositionForRuneIndex(i), "PositionForRuneIndex(%d)", i)
		}
		// Indexes at or past the end all clamp to the full width, and non-positive indexes are exactly zero.
		for _, index := range []int{len(txt.widths), len(txt.widths) + 1, len(txt.widths) + 9999} {
			c.Equal(txt.Width(), txt.PositionForRuneIndex(index), "PositionForRuneIndex(%d) must clamp", index)
		}
		for _, index := range []int{0, -1, -9999} {
			c.Equal(float32(0), txt.PositionForRuneIndex(index), "PositionForRuneIndex(%d)", index)
		}
		empty := NewText("", plain)
		c.Equal(float32(0), empty.PositionForRuneIndex(5), "an empty Text has no positions")
	})

	t.Run("exact widths sum exactly", func(t *testing.T) {
		c := check.New(t)
		// Powers of two sum exactly no matter how the additions associate, so these are ground truth rather than a
		// restatement of what f32.Sum() happens to return.
		txt := exactWidthText(plain, 4, 8, 16, 32, 64)
		c.Equal(float32(124), txt.Width())
		for i, expected := range []float32{0, 4, 12, 28, 60, 124, 124} {
			c.Equal(expected, txt.PositionForRuneIndex(i), "PositionForRuneIndex(%d)", i)
		}
	})
}

// TestTextSumDrawPointerFastPath locks the run-boundary fast path added to Draw(): comparing pointers before falling
// back to Equivalent() must not change which runs are emitted, so a Text whose decorations are equivalent but distinct
// pointers has to rasterize to exactly the same pixels as the shared-pointer Text it was cloned from.
func TestTextSumDrawPointerFastPath(t *testing.T) {
	const width = int32(200)
	const height = int32(64)
	origin := geom.NewPoint(4, 32)
	// The background ink makes run boundaries visible: each run paints one filled rectangle, so a Text that stopped
	// coalescing would paint a rectangle per rune and leave anti-aliased seams behind.
	base := &TextDecoration{Font: SystemFont, OnBackgroundInk: Black, BackgroundInk: Blue}

	drawn := func(txt *Text) *raster.Pixmap {
		canvas, pix := newPixmapCanvas(width, height)
		txt.Draw(canvas, origin)
		return pix
	}

	t.Run("one equivalent run", func(t *testing.T) {
		c := check.New(t)
		shared := NewText("Hello, World", base)
		distinct := NewText("Hello, World", base)
		clonePerRune(distinct)
		for i := 1; i < len(distinct.decorations); i++ {
			c.True(distinct.decorations[i] != distinct.decorations[i-1], "the clones must be distinct pointers")
			c.True(distinct.decorations[i].Equivalent(distinct.decorations[i-1]), "the clones must remain equivalent")
		}
		sharedPix := drawn(shared)
		c.True(hasInk(sharedPix), "nothing was drawn")
		differing, maxDelta := pixmapDelta(sharedPix, drawn(distinct))
		c.Equal(0, differing, "%d pixels differ, by up to %d levels, from the shared-pointer rendering", differing,
			maxDelta)
	})

	t.Run("distinct pointers within genuinely different runs", func(t *testing.T) {
		c := check.New(t)
		colored := base.Clone()
		colored.OnBackgroundInk = Red
		build := func() *Text {
			txt := NewText("Hel", base)
			txt.AddString("lo, ", colored)
			txt.AddString("World", base)
			return txt
		}
		shared := build()
		distinct := build()
		clonePerRune(distinct)
		// The boundaries between the three runs must survive the cloning: only the pointer identity within a run is
		// gone, the values on either side of a boundary are still not equivalent.
		c.False(distinct.decorations[3].Equivalent(distinct.decorations[2]), "the first boundary was lost")
		c.False(distinct.decorations[7].Equivalent(distinct.decorations[6]), "the second boundary was lost")
		sharedPix := drawn(shared)
		c.True(hasInk(sharedPix), "nothing was drawn")
		c.True(hasColoredPixel(sharedPix, 0, width), "the middle run's ink is not visible")
		differing, maxDelta := pixmapDelta(sharedPix, drawn(distinct))
		c.Equal(0, differing, "%d pixels differ, by up to %d levels, from the shared-pointer rendering", differing,
			maxDelta)
	})

	t.Run("a single rune and an empty text", func(t *testing.T) {
		c := check.New(t)
		single := NewText("H", base)
		singleDistinct := NewText("H", base)
		clonePerRune(singleDistinct)
		differing, maxDelta := pixmapDelta(drawn(single), drawn(singleDistinct))
		c.Equal(0, differing, "%d pixels differ, by up to %d levels, from the shared-pointer rendering", differing,
			maxDelta)
		c.False(hasInk(drawn(NewText("", base))), "an empty Text must not draw anything")
	})
}

// benchText returns a Text of roughly count runes made of a handful of decoration runs, which is the shape the hot
// loops are tuned for: long stretches of a single decoration pointer.
func benchText(count int) *Text {
	plain := &TextDecoration{Font: SystemFont, OnBackgroundInk: Black}
	bold := &TextDecoration{Font: EmphasizedSystemFont, OnBackgroundInk: Black, Underline: true}
	mono := &TextDecoration{Font: MonospacedFont, OnBackgroundInk: Black}
	decorations := []*TextDecoration{plain, bold, mono}
	txt := NewText("", plain)
	const chunk = "The quick brown fox jumps over the lazy dog. "
	for i := 0; len(txt.runes) < count; i++ {
		txt.AddString(chunk, decorations[i%len(decorations)])
	}
	return txt
}

// BenchmarkTextCacheWidths measures cache(): the width sum over every rune plus the vertical metrics over each
// decoration run. The extents are invalidated on each iteration, exactly as AddRunes() does, so that every iteration
// recomputes rather than reading the cached value.
func BenchmarkTextCacheWidths(b *testing.B) {
	txt := benchText(1000)
	b.SetBytes(int64(4 * len(txt.widths)))
	for b.Loop() {
		txt.extents.Width = -1
		textSumSink = txt.Width()
	}
}

// BenchmarkTextPositionForRuneIndex measures the prefix sum an index near the end of a long Text has to walk, which is
// the worst case for that call and the one a caret at the end of a line makes.
func BenchmarkTextPositionForRuneIndex(b *testing.B) {
	txt := benchText(1000)
	index := len(txt.widths) - 1
	b.SetBytes(int64(4 * index))
	for b.Loop() {
		textSumSink = txt.PositionForRuneIndex(index)
	}
}
