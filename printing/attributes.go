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

// Copy creates a new copy of these attributes.
func (a Attributes) Copy() Attributes {
	other := make(Attributes, len(a))
	for k, v := range a {
		values := make(goipp.Values, 0, len(v))
		for _, one := range v {
			values.Add(one.T, one.V)
		}
		other[k] = values
	}
	return other
}

// ForPrinter returns an Attributes that has extra methods for easily accessing the printer-specific attributes.
func (a Attributes) ForPrinter() *PrinterAttributes {
	return &PrinterAttributes{Attributes: a}
}

// ForJob returns an Attributes that has extra methods for easily accessing the job-specific attributes.
func (a Attributes) ForJob() *JobAttributes {
	return &JobAttributes{Attributes: a}
}

func (a Attributes) toIPP() goipp.Attributes {
	var other goipp.Attributes
	for k, v := range a {
		other = append(other, goipp.Attribute{
			Name:   k,
			Values: v,
		})
	}
	return other
}

// Boolean returns the first boolean value for the given key.
func (a Attributes) Boolean(key string, def bool) bool {
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

// SetBoolean sets the boolean value for the given key.
func (a Attributes) SetBoolean(key string, value, replaceExisting bool) {
	existing, ok := a[key]
	if replaceExisting || !ok {
		a[key] = goipp.Values{
			{
				T: goipp.TagBoolean,
				V: goipp.Boolean(value),
			},
		}
	} else {
		existing.Add(goipp.TagBoolean, goipp.Boolean(value))
	}
}

// Integer returns the first integer value for the given key.
func (a Attributes) Integer(key string, def int) int {
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

// SetInteger sets the integer value for the given key.
func (a Attributes) SetInteger(key string, value int, replaceExisting bool) {
	a.setInteger(key, value, goipp.TagInteger, replaceExisting)
}

// SetEnum sets the enum (integer) value for the given key.
func (a Attributes) SetEnum(key string, value int, replaceExisting bool) {
	a.setInteger(key, value, goipp.TagEnum, replaceExisting)
}

func (a Attributes) setInteger(key string, value int, tag goipp.Tag, replaceExisting bool) {
	existing, ok := a[key]
	if replaceExisting || !ok {
		a[key] = goipp.Values{
			{
				T: tag,
				V: goipp.Integer(value),
			},
		}
	} else {
		existing.Add(tag, goipp.Integer(value))
	}
}

// String returns the first string value for the given key.
func (a Attributes) String(key, def string) string {
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

// SetString sets the string value for the given key. If replaceExisting is true and the new value is empty, the key
// will be removed instead.
func (a Attributes) SetString(key, value string, replaceExisting bool) {
	a.setString(key, value, goipp.TagString, replaceExisting)
}

// SetText sets the text (string) value for the given key. If replaceExisting is true and the new value is empty, the
// key will be removed instead.
func (a Attributes) SetText(key, value string, replaceExisting bool) {
	a.setString(key, value, goipp.TagText, replaceExisting)
}

// SetReservedString sets the reserved string (string) value for the given key. If replaceExisting is true and the new
// value is empty, the key will be removed instead.
func (a Attributes) SetReservedString(key, value string, replaceExisting bool) {
	a.setString(key, value, goipp.TagReservedString, replaceExisting)
}

// SetKeyword sets the keyword (string) value for the given key. If replaceExisting is true and the new value is empty,
// the key will be removed instead.
func (a Attributes) SetKeyword(key, value string, replaceExisting bool) {
	a.setString(key, value, goipp.TagKeyword, replaceExisting)
}

// SetURI sets the URI (string) value for the given key. If replaceExisting is true and the new value is empty, the key
// will be removed instead.
func (a Attributes) SetURI(key, value string, replaceExisting bool) {
	a.setString(key, value, goipp.TagURI, replaceExisting)
}

// SetURIScheme sets the URI scheme (string) value for the given key. If replaceExisting is true and the new value is
// empty, the key will be removed instead.
func (a Attributes) SetURIScheme(key, value string, replaceExisting bool) {
	a.setString(key, value, goipp.TagURIScheme, replaceExisting)
}

// SetCharset sets the character set (string) value for the given key. If replaceExisting is true and the new value is
// empty, the key will be removed instead.
func (a Attributes) SetCharset(key, value string, replaceExisting bool) {
	a.setString(key, value, goipp.TagCharset, replaceExisting)
}

// SetLanguage sets the language (string) value for the given key. If replaceExisting is true and the new value is
// empty, the key will be removed instead.
func (a Attributes) SetLanguage(key, value string, replaceExisting bool) {
	a.setString(key, value, goipp.TagLanguage, replaceExisting)
}

// SetMimeType sets the MIME type (string) value for the given key. If replaceExisting is true and the new value is
// empty, the key will be removed instead.
func (a Attributes) SetMimeType(key, value string, replaceExisting bool) {
	a.setString(key, value, goipp.TagMimeType, replaceExisting)
}

// SetMemberName sets the member name (string) value for the given key. If replaceExisting is true and the new value is
// empty, the key will be removed instead.
func (a Attributes) SetMemberName(key, value string, replaceExisting bool) {
	a.setString(key, value, goipp.TagMemberName, replaceExisting)
}

func (a Attributes) setString(key, value string, tag goipp.Tag, replaceExisting bool) {
	if value == "" {
		if !replaceExisting {
			return
		}
		delete(a, key)
		return
	}
	existing, ok := a[key]
	if replaceExisting || !ok {
		a[key] = goipp.Values{
			{
				T: tag,
				V: goipp.String(value),
			},
		}
	} else {
		existing.Add(tag, goipp.String(value))
	}
}

// Time returns the first time value for the given key.
func (a Attributes) Time(key string, def time.Time) time.Time {
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

// SetTime sets the time/date value for the given key.
func (a Attributes) SetTime(key string, value time.Time, replaceExisting bool) {
	existing, ok := a[key]
	if replaceExisting || !ok {
		a[key] = goipp.Values{
			{
				T: goipp.TagDateTime,
				V: goipp.Time{Time: value},
			},
		}
	} else {
		existing.Add(goipp.TagDateTime, goipp.Time{Time: value})
	}
}

// Resolution returns the first resolution value for the given key.
func (a Attributes) Resolution(key string, def goipp.Resolution) goipp.Resolution {
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

// SetResolution sets the resolution value for the given key.
func (a Attributes) SetResolution(key string, value goipp.Resolution, replaceExisting bool) {
	existing, ok := a[key]
	if replaceExisting || !ok {
		a[key] = goipp.Values{
			{
				T: goipp.TagResolution,
				V: value,
			},
		}
	} else {
		existing.Add(goipp.TagResolution, value)
	}
}

// Range returns the first Range value for the given key.
func (a Attributes) Range(key string, def goipp.Range) goipp.Range {
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

// SetRange sets the range value for the given key.
func (a Attributes) SetRange(key string, value goipp.Range, replaceExisting bool) {
	existing, ok := a[key]
	if replaceExisting || !ok {
		a[key] = goipp.Values{
			{
				T: goipp.TagRange,
				V: value,
			},
		}
	} else {
		existing.Add(goipp.TagRange, value)
	}
}

// TextWithLang returns the first TextWithLang value for the given key.
func (a Attributes) TextWithLang(key string, def goipp.TextWithLang) goipp.TextWithLang {
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

// SetTextWithLang sets the text with language value for the given key.
func (a Attributes) SetTextWithLang(key string, value goipp.TextWithLang, replaceExisting bool) {
	a.setTextWithLang(key, value, goipp.TagTextLang, replaceExisting)
}

// SetNameWithLang sets the name with language (TextWithLang) value for the given key.
func (a Attributes) SetNameWithLang(key string, value goipp.TextWithLang, replaceExisting bool) {
	a.setTextWithLang(key, value, goipp.TagNameLang, replaceExisting)
}

func (a Attributes) setTextWithLang(key string, value goipp.TextWithLang, tag goipp.Tag, replaceExisting bool) {
	existing, ok := a[key]
	if replaceExisting || !ok {
		a[key] = goipp.Values{
			{
				T: tag,
				V: value,
			},
		}
	} else {
		existing.Add(tag, value)
	}
}

// Collection returns the first collection value for the given key.
func (a Attributes) Collection(key string) Attributes {
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
