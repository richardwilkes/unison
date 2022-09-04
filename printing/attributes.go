// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package printing

import (
	"time"

	"github.com/OpenPrinting/goipp"
)

// Attributes provides the base support for printer attributes.
type Attributes map[string]goipp.Values

// NewAttributes creates a new set of printer attributes from a set of goipp.Attributes.
func NewAttributes(attrs goipp.Attributes) Attributes {
	a := make(Attributes, len(attrs))
	for _, one := range attrs {
		a[one.Name] = one.Values
	}
	return a
}

// ForPrinter returns an Attributes that has extra methods for easily accessing the Printer-specific attributes.
func (a Attributes) ForPrinter() *PrinterAttributes {
	return &PrinterAttributes{Attributes: a}
}

// FirstBoolean returns the first boolean value for the given key.
func (a Attributes) FirstBoolean(key string, def bool) bool {
	if v, ok := a[key]; ok && v[0].T.Type() == goipp.TypeBoolean {
		return bool(v[0].V.(goipp.Boolean))
	}
	return def
}

// Booleans returns the boolean values for the given key.
func (a Attributes) Booleans(key string, def []bool) []bool {
	if v, ok := a[key]; ok {
		all := make([]bool, 0, len(v))
		for _, one := range v {
			if one.T.Type() == goipp.TypeBoolean {
				all = append(all, bool(one.V.(goipp.Boolean)))
			}
		}
		return all
	}
	return def
}

// FirstInteger returns the first integer value for the given key.
func (a Attributes) FirstInteger(key string, def int) int {
	if v, ok := a[key]; ok && v[0].T.Type() == goipp.TypeInteger {
		return int(v[0].V.(goipp.Integer))
	}
	return def
}

// Integers returns the integer values for the given key.
func (a Attributes) Integers(key string, def []int) []int {
	if v, ok := a[key]; ok {
		all := make([]int, 0, len(v))
		for _, one := range v {
			if one.T.Type() == goipp.TypeInteger {
				all = append(all, int(one.V.(goipp.Integer)))
			}
		}
		return all
	}
	return def
}

// FirstString returns the first string value for the given key.
func (a Attributes) FirstString(key, def string) string {
	if v, ok := a[key]; ok && v[0].T.Type() == goipp.TypeString {
		return v[0].V.String()
	}
	return def
}

// Strings returns the string values for the given key.
func (a Attributes) Strings(key string, def []string) []string {
	if v, ok := a[key]; ok {
		keywords := make([]string, 0, len(v))
		for _, one := range v {
			if one.T.Type() == goipp.TypeString {
				keywords = append(keywords, one.V.String())
			}
		}
		return keywords
	}
	return def
}

// FirstTime returns the first time value for the given key.
func (a Attributes) FirstTime(key string, def time.Time) time.Time {
	if v, ok := a[key]; ok && v[0].T.Type() == goipp.TypeDateTime {
		return v[0].V.(goipp.Time).Time
	}
	return def
}

// Times returns the time values for the given key.
func (a Attributes) Times(key string, def []time.Time) []time.Time {
	if v, ok := a[key]; ok {
		all := make([]time.Time, 0, len(v))
		for _, one := range v {
			if one.T.Type() == goipp.TypeDateTime {
				all = append(all, one.V.(goipp.Time).Time)
			}
		}
		return all
	}
	return def
}

// FirstResolution returns the first resolution value for the given key.
func (a Attributes) FirstResolution(key string, def goipp.Resolution) goipp.Resolution {
	if v, ok := a[key]; ok && v[0].T.Type() == goipp.TypeResolution {
		return v[0].V.(goipp.Resolution)
	}
	return def
}

// Resolutions returns the Resolution values for the given key.
func (a Attributes) Resolutions(key string, def []goipp.Resolution) []goipp.Resolution {
	if v, ok := a[key]; ok {
		all := make([]goipp.Resolution, 0, len(v))
		for _, one := range v {
			if one.T.Type() == goipp.TypeResolution {
				all = append(all, one.V.(goipp.Resolution))
			}
		}
		return all
	}
	return def
}

// FirstRange returns the first Range value for the given key.
func (a Attributes) FirstRange(key string, def goipp.Range) goipp.Range {
	if v, ok := a[key]; ok && v[0].T.Type() == goipp.TypeRange {
		return v[0].V.(goipp.Range)
	}
	return def
}

// Ranges returns the Range values for the given key.
func (a Attributes) Ranges(key string, def []goipp.Range) []goipp.Range {
	if v, ok := a[key]; ok {
		all := make([]goipp.Range, 0, len(v))
		for _, one := range v {
			if one.T.Type() == goipp.TypeRange {
				all = append(all, one.V.(goipp.Range))
			}
		}
		return all
	}
	return def
}

// FirstTextWithLang returns the first TextWithLang value for the given key.
func (a Attributes) FirstTextWithLang(key string, def goipp.TextWithLang) goipp.TextWithLang {
	if v, ok := a[key]; ok && v[0].T.Type() == goipp.TypeTextWithLang {
		return v[0].V.(goipp.TextWithLang)
	}
	return def
}

// TextWithLangs returns the TextWithLang values for the given key.
func (a Attributes) TextWithLangs(key string, def []goipp.TextWithLang) []goipp.TextWithLang {
	if v, ok := a[key]; ok {
		all := make([]goipp.TextWithLang, 0, len(v))
		for _, one := range v {
			if one.T.Type() == goipp.TypeTextWithLang {
				all = append(all, one.V.(goipp.TextWithLang))
			}
		}
		return all
	}
	return def
}

// FirstBinary returns the first binary value for the given key.
func (a Attributes) FirstBinary(key string) []byte {
	if v, ok := a[key]; ok && v[0].T.Type() == goipp.TypeBinary {
		return v[0].V.(goipp.Binary)
	}
	return nil
}

// Binaries returns the binary values for the given key.
func (a Attributes) Binaries(key string) [][]byte {
	if v, ok := a[key]; ok {
		all := make([][]byte, 0, len(v))
		for _, one := range v {
			if one.T.Type() == goipp.TypeBinary {
				all = append(all, one.V.(goipp.Binary))
			}
		}
		return all
	}
	return nil
}

// FirstCollection returns the first collection value for the given key.
func (a Attributes) FirstCollection(key string) Attributes {
	if v, ok := a[key]; ok && v[0].T.Type() == goipp.TypeCollection {
		return NewAttributes(goipp.Attributes(v[0].V.(goipp.Collection)))
	}
	return make(Attributes)
}

// Collections returns the collection values for the given key.
func (a Attributes) Collections(key string) []Attributes {
	if v, ok := a[key]; ok {
		all := make([]Attributes, 0, len(v))
		for _, one := range v {
			if one.T.Type() == goipp.TypeCollection {
				all = append(all, NewAttributes(goipp.Attributes(one.V.(goipp.Collection))))
			}
		}
		return all
	}
	return nil
}
