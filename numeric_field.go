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
	"fmt"
	"strings"
	"unicode"

	"github.com/richardwilkes/toolbox/v2/i18n"
	"github.com/richardwilkes/toolbox/v2/xmath"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/role"
)

// NumericField holds a numeric value that can be edited.
type NumericField[T xmath.Integer | xmath.Float] struct {
	*Field
	Format     func(T) string
	Extract    func(s string) (T, error)
	Prototypes func(minimum, maximum T) []T
	minimum    T
	maximum    T
}

// NewNumericField creates a new field that holds a numeric value and limits its input to a specific range of values.
// The format and extract functions allow the field to be presented as something other than numbers.
func NewNumericField[T xmath.Integer | xmath.Float](current, minimum, maximum T, format func(T) string, extract func(s string) (T, error), prototypes func(minimum, maximum T) []T) *NumericField[T] {
	f := &NumericField[T]{
		Field:      NewField(),
		Prototypes: prototypes,
		Format:     format,
		Extract:    extract,
		minimum:    minimum,
		maximum:    maximum,
	}
	f.Self = f
	UninstallFocusBorders(f, f)
	f.LostFocusCallback = f.DefaultFocusLost
	InstallDefaultFieldBorder(f, f)
	f.RuneTypedCallback = f.DefaultRuneTyped
	f.ValidateCallback = f.DefaultValidate
	f.SetText(f.Format(current))
	f.adjustMinimumTextWidth()
	return f
}

// Value returns the current value of the field.
func (f *NumericField[T]) Value() T {
	v, _ := f.Extract(strings.TrimSpace(f.Text())) //nolint:errcheck // Default value in case of error is acceptable
	return min(max(v, f.minimum), f.maximum)
}

// SetValue sets the current value of the field.
func (f *NumericField[T]) SetValue(value T) {
	text := f.Format(value)
	if text != f.Text() {
		f.SetText(text)
	}
}

// Min returns the minimum value allowed.
func (f *NumericField[T]) Min() T {
	return f.minimum
}

// Max returns the maximum value allowed.
func (f *NumericField[T]) Max() T {
	return f.maximum
}

// DefaultFocusLost is the default implementation for the LostFocusCallback.
func (f *NumericField[T]) DefaultFocusLost() {
	f.SetText(f.Format(f.Value()))
	f.Field.DefaultFocusLost()
}

// DefaultRuneTyped is the default implementation for the RuneTypedCallback.
func (f *NumericField[T]) DefaultRuneTyped(ch rune) bool {
	if !unicode.IsControl(ch) {
		if _, err := f.Extract(strings.TrimSpace(string(f.RunesIfPasted([]rune{ch})))); err != nil {
			Beep()
			return false
		}
	}
	return f.Field.DefaultRuneTyped(ch)
}

// DefaultValidate is the default implementation for the ValidateCallback.
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
	if minimum := f.minimum; v < minimum {
		return fmt.Sprintf(i18n.Text("Value must be at least %s"), f.Format(minimum))
	}
	if maximum := f.maximum; v > maximum {
		return fmt.Sprintf(i18n.Text("Value must be no more than %s"), f.Format(maximum))
	}
	return ""
}

// ProvideAccessibility describes the field to assistive technologies. It is a text field that holds a number, so
// everything the field itself has to say is said first and the numeric range it is confined to is added to it.
func (f *NumericField[T]) ProvideAccessibility(b *AccessibilityBuilder) {
	node := b.Node()
	// Whether the role was left for the widget to decide has to be noticed before the field decides it is a text field.
	derived := node.Role == role.Auto
	f.Field.ProvideAccessibility(b)
	if derived {
		node.Role = role.SpinButton
	}
	node.HasNumber = true
	node.Number = float64(f.Value())
	node.Min = float64(f.minimum)
	node.Max = float64(f.maximum)
	node.Step = 1
	node.Actions = node.Actions.With(accessibility.Increment, accessibility.Decrement)
}

// PerformAccessibilityAction carries out a request from an assistive technology. Stepping the value moves it by one,
// within the range the field allows; everything else is left to the field.
func (f *NumericField[T]) PerformAccessibilityAction(req accessibility.ActionRequest) bool {
	switch req.Action {
	case accessibility.Increment:
		f.axStepValue(true)
		return true
	case accessibility.Decrement:
		f.axStepValue(false)
		return true
	default:
		return f.Field.PerformAccessibilityAction(req)
	}
}

// axStepValue moves the value one step up or down, stopping at the end of the range rather than passing it. Value
// already brings whatever has been typed into range, so a field holding something out of range steps from the nearest
// value it is allowed to hold.
func (f *NumericField[T]) axStepValue(up bool) {
	value := f.Value()
	switch {
	case up && value < f.maximum:
		value = min(value+1, f.maximum)
	case !up && value > f.minimum:
		value = max(value-1, f.minimum)
	}
	f.SetValue(value)
}

// SetMinMax sets the minimum and maximum values and then adjusts the minimum text width, if a prototype function has
// been set.
func (f *NumericField[T]) SetMinMax(minimum, maximum T) {
	if f.minimum != minimum || f.maximum != maximum {
		f.minimum = minimum
		f.maximum = maximum
		f.adjustMinimumTextWidth()
		v, _ := f.Extract(strings.TrimSpace(f.Text())) //nolint:errcheck // Default value in case of error is acceptable
		f.SetValue(min(max(v, f.minimum), f.maximum))
	}
}

func (f *NumericField[T]) adjustMinimumTextWidth() {
	if f.Prototypes != nil {
		prototypes := f.Prototypes(f.minimum, f.maximum)
		candidates := make([]string, 0, len(prototypes))
		for _, v := range prototypes {
			candidates = append(candidates, f.Format(v))
		}
		f.SetMinimumTextWidthUsing(candidates...)
	}
}
