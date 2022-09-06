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
	"fmt"
	"strings"
	"unicode"

	"github.com/richardwilkes/toolbox/i18n"
	"github.com/richardwilkes/toolbox/xmath"
)

// NumericField holds a numeric value that can be edited.
type NumericField[T xmath.Numeric] struct {
	*Field
	Format     func(T) string
	Extract    func(s string) (T, error)
	Prototypes func(min, max T) []T
	min        T
	max        T
}

// NewNumericField creates a new field that holds a numeric value and limits its input to a specific range of values.
// The format and extract functions allow the field to be presented as something other than numbers.
func NewNumericField[T xmath.Numeric](current, min, max T, format func(T) string, extract func(s string) (T, error), prototypes func(min, max T) []T) *NumericField[T] {
	f := &NumericField[T]{
		Field:      NewField(),
		Prototypes: prototypes,
		Format:     format,
		Extract:    extract,
		min:        min,
		max:        max,
	}
	f.Self = f
	f.LostFocusCallback = f.DefaultFocusLost
	f.RuneTypedCallback = f.DefaultRuneTyped
	f.ValidateCallback = f.DefaultValidate
	f.SetText(f.Format(current))
	f.adjustMinimumTextWidth()
	return f
}

func (f *NumericField[T]) Value() T {
	v, _ := f.Extract(strings.TrimSpace(f.Text())) //nolint:errcheck // Default value in case of error is acceptable
	return xmath.Min(xmath.Max(v, f.min), f.max)
}

func (f *NumericField[T]) SetValue(value T) {
	text := f.Format(value)
	if text != f.Text() {
		f.SetText(text)
	}
}

// Min returns the minimum value allowed.
func (f *NumericField[T]) Min() T {
	return f.min
}

// Max returns the maximum value allowed.
func (f *NumericField[T]) Max() T {
	return f.max
}

func (f *NumericField[T]) DefaultFocusLost() {
	f.SetText(f.Format(f.Value()))
	f.Field.DefaultFocusLost()
}

func (f *NumericField[T]) DefaultRuneTyped(ch rune) bool {
	if !unicode.IsControl(ch) {
		if _, err := f.Extract(strings.TrimSpace(string(f.RunesIfPasted([]rune{ch})))); err != nil {
			Beep()
			return false
		}
	}
	return f.Field.DefaultRuneTyped(ch)
}

func (f *NumericField[T]) DefaultValidate() bool {
	if text := f.tooltipTextForValidation(); text != "" {
		f.Tooltip = NewTooltipWithText(text)
		return false
	}
	f.Tooltip = nil
	return true
}

func (f *NumericField[T]) tooltipTextForValidation() string {
	s := strings.TrimSpace(f.Text())
	v, err := f.Extract(s)
	if err != nil || s == "-" || s == "+" {
		return i18n.Text("Invalid value")
	}
	if minimum := f.min; v < minimum {
		return fmt.Sprintf(i18n.Text("Value must be at least %s"), f.Format(minimum))
	}
	if maximum := f.max; v > maximum {
		return fmt.Sprintf(i18n.Text("Value must be no more than %s"), f.Format(maximum))
	}
	return ""
}

// SetMinMax sets the minimum and maximum values and then adjusts the minimum text width, if a prototype function has
// been set.
func (f *NumericField[T]) SetMinMax(min, max T) {
	if f.min != min || f.max != max {
		f.min = min
		f.max = max
		f.adjustMinimumTextWidth()
		v, _ := f.Extract(strings.TrimSpace(f.Text())) //nolint:errcheck // Default value in case of error is acceptable
		f.SetValue(xmath.Min(xmath.Max(v, f.min), f.max))
	}
}

func (f *NumericField[T]) adjustMinimumTextWidth() {
	if f.Prototypes != nil {
		prototypes := f.Prototypes(f.min, f.max)
		candidates := make([]string, 0, len(prototypes))
		for _, v := range prototypes {
			candidates = append(candidates, f.Format(v))
		}
		f.SetMinimumTextWidthUsing(candidates...)
	}
}
