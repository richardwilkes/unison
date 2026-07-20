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
	"strconv"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/unison"
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
