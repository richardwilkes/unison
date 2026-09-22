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
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/unison/enums/thememode"
	"github.com/richardwilkes/unison/internal/cocoa"
)

// TestMacAppearanceNameFor verifies the application appearance each theme mode asks for: an explicit choice names the
// matching appearance, and Auto names none, so that the application goes back to following the system.
func TestMacAppearanceNameFor(t *testing.T) {
	c := check.New(t)
	c.Equal(cocoa.AppearanceNameDarkAqua, macAppearanceNameFor(thememode.Dark))
	c.Equal(cocoa.AppearanceNameAqua, macAppearanceNameFor(thememode.Light))
	c.Equal("", macAppearanceNameFor(thememode.Auto))
}
