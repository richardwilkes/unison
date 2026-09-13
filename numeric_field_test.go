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
	"math"
	"strconv"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/role"
)

func TestNumericFieldFocusBorderRestoredOnFocusLoss(t *testing.T) {
	c := check.New(t)
	f := unison.NewNumericField(5, 0, 100, strconv.Itoa, strconv.Atoi, nil)
	unfocused := f.Border()
	c.NotNil(unfocused)

	// Gaining focus swaps in the focused border.
	f.GainedFocusCallback()
	c.NotEqual(unfocused, f.Border())

	// Losing focus must restore the unfocused border. This used to fail because NewNumericField assigned
	// LostFocusCallback directly, clobbering the wrapper installed by InstallDefaultFieldBorder, so the field stayed
	// stuck with the focused border forever.
	f.LostFocusCallback()
	c.Equal(unfocused, f.Border())
}

func TestNumericFieldFocusLossStillNormalizesText(t *testing.T) {
	c := check.New(t)
	f := unison.NewNumericField(5, 0, 100, strconv.Itoa, strconv.Atoi, nil)

	// The numeric field's own focus-loss behavior must remain chained in: losing focus reformats the text.
	f.SetText(" 7 ")
	f.LostFocusCallback()
	c.Equal("7", f.Text())

	// Out-of-range input is clamped on focus loss.
	f.SetText("400")
	f.LostFocusCallback()
	c.Equal("100", f.Text())
}

// TestNumericFieldAccessibilitySetValueTakesANumber verifies that a request to replace the value is honored whether the
// new value arrives as text or as a number. Anything that treats the field as the spin button it says it is — the
// Windows UI Automation range value pattern, the AT-SPI value interface — sends only a number, and handing such a
// request to the embedded field, which knows only about text, used to blank the field rather than set it.
func TestNumericFieldAccessibilitySetValueTakesANumber(t *testing.T) {
	c := check.New(t)
	f := unison.NewNumericField(5, 0, 100, strconv.Itoa, strconv.Atoi, nil)

	c.True(f.PerformAccessibilityAction(accessibility.ActionRequest{Action: accessibility.SetValue, Number: 42}))
	c.Equal("42", f.Text(), "a request carrying only a number must set the field, not blank it")
	c.Equal(42, f.Value())

	// Text is still what is used whenever it was supplied, since it carries whatever formatting the field presents its
	// values in. macOS sends both, with the text simply the number written out.
	c.True(f.PerformAccessibilityAction(accessibility.ActionRequest{
		Action: accessibility.SetValue,
		Value:  "17",
		Number: 17,
	}))
	c.Equal("17", f.Text())

	// A number outside the range is brought into it, exactly as typing one out of range would be.
	c.True(f.PerformAccessibilityAction(accessibility.ActionRequest{Action: accessibility.SetValue, Number: 1000}))
	c.Equal("100", f.Text())
	c.True(f.PerformAccessibilityAction(accessibility.ActionRequest{Action: accessibility.SetValue, Number: -1000}))
	c.Equal("0", f.Text())

	// Nothing usable at all leaves the field as it was rather than throwing it to some arbitrary value.
	f.SetValue(60)
	c.False(f.PerformAccessibilityAction(accessibility.ActionRequest{
		Action: accessibility.SetValue,
		Number: math.NaN(),
	}))
	c.Equal("60", f.Text())
}

// TestNumericFieldAccessibilityObscuredWithholdsTheNumber verifies that a numeric field that obscures what it shows
// gives up neither its text nor the number that text was parsed from. The embedded field withholds the text; the
// numeric field used to hand the same value straight back through Number.
func TestNumericFieldAccessibilityObscuredWithholdsTheNumber(t *testing.T) {
	c := check.New(t)
	var plain, secret *unison.NumericField[int]
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			plain = unison.NewNumericField(5, 0, 100, strconv.Itoa, strconv.Atoi, nil)

			secret = unison.NewNumericField(1234, 0, 9999, strconv.Itoa, strconv.Atoi, nil)
			secret.ObscurementRune = '•'

			wnd = newHeadlessWindow(t, "numeric fields", geom.NewRect(10, 10, 300, 200), axColumn(plain, secret))
		}))
	c.NotNil(wnd)

	screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(plain)
	c.True(node != nil)
	c.Equal(role.SpinButton, node.Role, "a field holding a number is a spin button")
	c.True(node.HasNumber)
	c.Equal(float64(5), node.Number)
	c.Equal(float64(0), node.Min)
	c.Equal(float64(100), node.Max)
	c.True(node.Actions.Has(accessibility.Increment))
	c.True(node.Actions.Has(accessibility.Decrement))

	secretNode := screen.AccessibilityNodeFor(secret)
	c.True(secretNode != nil)
	c.True(secretNode.Protected)
	c.Equal("", secretNode.Value, "an obscured field's text is never handed out")
	c.False(secretNode.HasNumber, "nor is the number that text was parsed from")
	c.Equal(float64(0), secretNode.Number)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
