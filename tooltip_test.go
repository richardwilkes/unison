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

// TestTooltipSecondaryTextFont verifies that NewTooltipWithSecondaryText honors TooltipTheme.SecondaryTextFont when it
// is set and falls back to a font one size smaller than the tooltip label font when it is not.
func TestTooltipSecondaryTextFont(t *testing.T) {
	c := check.New(t)
	secondaryLabel := func(tip *unison.Panel) *unison.Label {
		children := tip.Children()
		c.Equal(2, len(children))
		label, ok := children[1].Self.(*unison.Label)
		c.True(ok)
		return label
	}

	// Default: one size smaller than the tooltip label font.
	tip := unison.NewTooltipWithSecondaryText("primary", "secondary")
	desc := secondaryLabel(tip).Font.Descriptor()
	expected := unison.DefaultTooltipTheme.Label.Font.Descriptor()
	expected.Size--
	c.Equal(expected, desc)

	// With SecondaryTextFont set, that font must be used.
	saved := unison.DefaultTooltipTheme.SecondaryTextFont
	defer func() { unison.DefaultTooltipTheme.SecondaryTextFont = saved }()
	custom := unison.DefaultTooltipTheme.Label.Font.Descriptor()
	custom.Size += 3
	unison.DefaultTooltipTheme.SecondaryTextFont = custom.Font()
	tip = unison.NewTooltipWithSecondaryText("primary", "secondary")
	c.Equal(custom, secondaryLabel(tip).Font.Descriptor())
}
