// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package printing_test

import (
	"testing"
	"time"

	"github.com/OpenPrinting/goipp"
	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/unison/printing"
)

func TestSetBooleanAppends(t *testing.T) {
	chk := check.New(t)
	a := make(printing.Attributes)
	a.SetBoolean("key", true, false)
	a.SetBoolean("key", false, false)
	chk.Equal([]bool{true, false}, a.Booleans("key", nil))
	a.SetBoolean("key", true, true)
	chk.Equal([]bool{true}, a.Booleans("key", nil))
}

func TestSetIntegerAppends(t *testing.T) {
	chk := check.New(t)
	a := make(printing.Attributes)
	a.SetInteger("key", 1, false)
	a.SetInteger("key", 2, false)
	a.SetInteger("key", 3, false)
	chk.Equal([]int{1, 2, 3}, a.Integers("key", nil))
	a.SetInteger("key", 4, true)
	chk.Equal([]int{4}, a.Integers("key", nil))
}

func TestSetEnumAppends(t *testing.T) {
	chk := check.New(t)
	a := make(printing.Attributes)
	a.SetEnum("key", 1, false)
	a.SetEnum("key", 2, false)
	chk.Equal([]int{1, 2}, a.Integers("key", nil))
}

func TestSetKeywordAppends(t *testing.T) {
	chk := check.New(t)
	a := make(printing.Attributes)
	a.SetKeyword("key", "one", false)
	a.SetKeyword("key", "two", false)
	chk.Equal([]string{"one", "two"}, a.Strings("key", nil))
	a.SetKeyword("key", "three", true)
	chk.Equal([]string{"three"}, a.Strings("key", nil))
}

func TestSetTimeAppends(t *testing.T) {
	chk := check.New(t)
	a := make(printing.Attributes)
	t1 := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	t2 := time.Date(2026, 6, 7, 8, 9, 10, 0, time.UTC)
	a.SetTime("key", t1, false)
	a.SetTime("key", t2, false)
	chk.Equal([]time.Time{t1, t2}, a.Times("key", nil))
	a.SetTime("key", t1, true)
	chk.Equal([]time.Time{t1}, a.Times("key", nil))
}

func TestSetResolutionAppends(t *testing.T) {
	chk := check.New(t)
	a := make(printing.Attributes)
	r1 := goipp.Resolution{Xres: 300, Yres: 300, Units: goipp.UnitsDpi}
	r2 := goipp.Resolution{Xres: 600, Yres: 600, Units: goipp.UnitsDpi}
	a.SetResolution("key", r1, false)
	a.SetResolution("key", r2, false)
	chk.Equal([]goipp.Resolution{r1, r2}, a.Resolutions("key", nil))
	a.SetResolution("key", r1, true)
	chk.Equal([]goipp.Resolution{r1}, a.Resolutions("key", nil))
}

func TestSetRangeAppends(t *testing.T) {
	chk := check.New(t)
	a := make(printing.Attributes)
	r1 := goipp.Range{Lower: 1, Upper: 2}
	r2 := goipp.Range{Lower: 5, Upper: 6}
	a.SetRange("key", r1, false)
	a.SetRange("key", r2, false)
	chk.Equal([]goipp.Range{r1, r2}, a.Ranges("key", nil))
	a.SetRange("key", r1, true)
	chk.Equal([]goipp.Range{r1}, a.Ranges("key", nil))
}

func TestSetTextWithLangAppends(t *testing.T) {
	chk := check.New(t)
	a := make(printing.Attributes)
	v1 := goipp.TextWithLang{Lang: "en", Text: "hello"}
	v2 := goipp.TextWithLang{Lang: "fr", Text: "bonjour"}
	a.SetTextWithLang("key", v1, false)
	a.SetTextWithLang("key", v2, false)
	chk.Equal([]goipp.TextWithLang{v1, v2}, a.TextWithLangs("key", nil))
	a.SetTextWithLang("key", v1, true)
	chk.Equal([]goipp.TextWithLang{v1}, a.TextWithLangs("key", nil))
}

func TestSetPageRangesKeepsAllRanges(t *testing.T) {
	chk := check.New(t)
	ja := make(printing.Attributes).ForJob()
	ranges := []goipp.Range{{Lower: 1, Upper: 2}, {Lower: 5, Upper: 6}}
	ja.SetPageRanges(ranges)
	chk.Equal(ranges, ja.PageRanges())
	ja.SetPageRanges([]goipp.Range{{Lower: 3, Upper: 4}})
	chk.Equal([]goipp.Range{{Lower: 3, Upper: 4}}, ja.PageRanges())
	ja.SetPageRanges(nil)
	chk.Nil(ja.PageRanges())
}
