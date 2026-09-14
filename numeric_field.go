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
	"math"
	"strconv"
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
// everything the field itself has to say is said first and the numeric range it is confined to is added to it. A field
// that obscures what it shows is a password, and the field withholds its text for one; the number it was parsed from is
// withheld along with it, since handing that over would give away exactly what the bullets were hiding.
func (f *NumericField[T]) ProvideAccessibility(b *AccessibilityBuilder) {
	node := b.Node()
	// Whether the role was left for the widget to decide has to be noticed before the field decides it is a text field.
	derived := node.Role == role.Auto
	f.Field.ProvideAccessibility(b)
	if derived {
		node.Role = role.SpinButton
	}
	if node.Protected {
		return
	}
	node.HasNumber = true
	node.Number = float64(f.Value())
	node.Min = float64(f.minimum)
	node.Max = float64(f.maximum)
	node.Step = float64(f.axStep())
	node.Actions = node.Actions.With(accessibility.Increment, accessibility.Decrement)
}

// PerformAccessibilityAction carries out a request from an assistive technology. Stepping the value moves it by one
// step, within the range the field allows, and replacing the value takes a number as readily as it takes text;
// everything else is left to the field.
func (f *NumericField[T]) PerformAccessibilityAction(req accessibility.ActionRequest) bool {
	switch req.Action {
	case accessibility.Increment:
		f.axStepValue(true)
		return true
	case accessibility.Decrement:
		f.axStepValue(false)
		return true
	case accessibility.SetValue:
		return f.axSetValue(req)
	default:
		return f.Field.PerformAccessibilityAction(req)
	}
}

// axSetValue replaces the value on behalf of an assistive technology. Anything that treats the field as the spin button
// it says it is sends the new value as a number and leaves the text empty — the Windows UI Automation range value
// pattern and the AT-SPI value interface have nowhere else to put it — so handing such a request straight to the field,
// which knows only about text, would blank the field rather than set it.
//
// Text is what is used when it says something the number does not. Both macOS and Windows send the two together, with
// the text nothing more than the number written out, and taking the text there would set a field that shows its values
// as anything but bare digits — a percentage, a currency, a length — to whatever those digits happen to mean when read
// back through its Extract, and would skip the clamp to the field's range besides. So a text that parses to exactly the
// number it came with says no more than the number does, and the number is taken; anything else is the formatting the
// field presents its values in, and the text is taken.
func (f *NumericField[T]) axSetValue(req accessibility.ActionRequest) bool {
	if req.Value != "" && !axValueIsNumber(req.Value, req.Number) {
		return f.Field.PerformAccessibilityAction(req)
	}
	if math.IsNaN(req.Number) {
		return false
	}
	// Brought into range before it is converted, both because a value out of range is no more acceptable here than one
	// that was typed and because converting a float far outside an integer type's range is not defined.
	f.SetValue(T(min(max(req.Number, float64(f.minimum)), float64(f.maximum))))
	return true
}

// axValueIsNumber reports whether the text an assistive technology sent with a request says no more than the number it
// sent along with it, which is the case when the text is that number written out and nothing else.
func axValueIsNumber(value string, number float64) bool {
	if math.IsNaN(number) {
		return false
	}
	parsed, err := strconv.ParseFloat(value, 64)
	return err == nil && parsed == number
}

// axStepValue moves the value one step up or down, stopping at the end of the range rather than passing it. Value
// already brings whatever has been typed into range, so a field holding something out of range steps from the nearest
// value it is allowed to hold.
func (f *NumericField[T]) axStepValue(up bool) {
	value := f.Value()
	step := f.axStep()
	switch {
	case up && value < f.maximum:
		value = min(value+step, f.maximum)
	case !up && value > f.minimum:
		value = max(value-step, f.minimum)
	}
	f.SetValue(value)
}

// axStep returns how far one increment or decrement asked for by an assistive technology moves the value. A field of a
// whole-number type can only ever move by whole numbers, so one of them is the step whatever its range. A field of a
// fractional type spanning twenty units or more is almost always counting whole numbers too — 0 to 255 for a color
// channel, 0 to 359 for a hue — so one is the step there as well; a smaller range needs a fractional step instead, and
// a twentieth of it gives the same twenty-odd stops a slider over the same range gets. A range of nothing at all still
// reports a step, since a step of zero would be advertised as a field that cannot be stepped.
func (f *NumericField[T]) axStep() T {
	var one T = 1
	valueRange := f.maximum - f.minimum
	if one/2 == 0 || valueRange >= 20 || valueRange <= 0 {
		return one
	}
	return valueRange / 20
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
		// Neither of the calls above need have done anything: the minimum text width is only a field until something
		// lays the field out again, and a value that still formats to the text already showing leaves the field
		// untouched. The range is part of what an assistive technology is told, though, and a window is only described
		// again once it has been drawn, so the new range would otherwise not be published until something unrelated
		// happened to redraw.
		f.MarkForLayoutAndRedraw()
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
