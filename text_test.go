// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison_test

import (
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/unison"
)

// TestTextSliceDoesNotShareBackingWithParent verifies that mutating a slice of a Text cannot corrupt the Text it was
// sliced from: appending to a non-tail slice previously overwrote the parent's backing arrays.
func TestTextSliceDoesNotShareBackingWithParent(t *testing.T) {
	c := check.New(t)
	dec := &unison.TextDecoration{Font: unison.SystemFont, OnBackgroundInk: unison.Black}
	parent := unison.NewText("abcdef", dec)
	sl := parent.Slice(0, 3)
	c.Equal("abc", sl.String())

	sl.AddString("XY", dec)
	c.Equal("abcXY", sl.String())
	c.Equal("abcdef", parent.String())
	c.Equal(unison.NewText("abcdef", dec).Width(), parent.Width())
}

// TestTextEmptySliceMetricsMatchEmptyText verifies that an empty slice of a Text reserves the same vertical metrics an
// empty Text does, rather than reporting a zero height.
func TestTextEmptySliceMetricsMatchEmptyText(t *testing.T) {
	c := check.New(t)
	dec := &unison.TextDecoration{Font: unison.SystemFont, OnBackgroundInk: unison.Black}
	parent := unison.NewText("abcdef", dec)
	empty := parent.Slice(2, 2)
	ref := unison.NewText("", dec)
	c.True(ref.Height() > 0, "an empty Text should still reserve a full line")
	c.Equal(ref.Height(), empty.Height())
	c.Equal(ref.Baseline(), empty.Baseline())
	c.Equal(float32(0), empty.Width())
}

// TestSmallCapsTextKeepsItsCase verifies that small caps are a matter of drawing alone: the text keeps its letters as
// they were written, so String() -- and everything read from it, such as what an assistive technology is told a label
// says -- answers "Title" for a label drawn TITLE, while the lowercase letters are still measured as the smaller
// capitals they are drawn as.
func TestSmallCapsTextKeepsItsCase(t *testing.T) {
	c := check.New(t)
	dec := &unison.TextDecoration{Font: unison.SystemFont, OnBackgroundInk: unison.Black}
	text := unison.NewSmallCapsText("Title (ST)", dec)
	c.Equal("Title (ST)", text.String())
	c.Equal([]rune("Title (ST)"), text.Runes())

	fd := unison.SystemFont.Descriptor()
	fd.Size *= 0.75
	smaller := &unison.TextDecoration{Font: fd.Font(), OnBackgroundInk: unison.Black}
	drawn := unison.NewText("T", dec)
	drawn.AddString("ITLE", smaller)
	drawn.AddString(" (ST)", dec)
	c.Equal(drawn.Width(), text.Width(), "the lowercase letters are measured as smaller capitals")
	c.NotEqual(unison.NewText("Title (ST)", dec).Width(), text.Width(), "and not as the lowercase letters they hold")
}
