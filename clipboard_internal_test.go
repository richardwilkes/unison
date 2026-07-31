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
	"github.com/richardwilkes/toolbox/v2/uti"
)

// TestSelectDataType verifies that selectDataType matches conformance in both directions. Prior to the fix, only
// desired.ConformsTo(available) was checked, which matches available types that are ancestors of the desired type, so
// asking for the generic public.plain-text when the clipboard held the concrete public.utf8-plain-text (the type all
// of the platform backends write) found no match.
func TestSelectDataType(t *testing.T) {
	c := check.New(t)

	// An exact match returns the desired type itself.
	c.Equal(uti.UTF8PlainText, selectDataType(uti.UTF8PlainText, []string{uti.UTF8PlainText.UTI}), "exact match")

	// A more specific available type satisfies a request for one of its ancestors.
	c.Equal(uti.UTF8PlainText, selectDataType(uti.PlainText, []string{uti.UTF8PlainText.UTI}),
		"descendant satisfies a request for its ancestor")

	// A more generic available type still satisfies a request for one of its descendants.
	c.Equal(uti.PlainText, selectDataType(uti.UTF8PlainText, []string{uti.PlainText.UTI}),
		"ancestor satisfies a request for its descendant")

	// The first conforming type wins when several are available.
	c.Equal(uti.UTF8PlainText, selectDataType(uti.PlainText, []string{uti.PNG.UTI, uti.UTF8PlainText.UTI}),
		"non-conforming types are skipped")

	// Unrelated and unregistered types never match, and the desired type comes back when nothing does.
	c.Equal(uti.PlainText, selectDataType(uti.PlainText, []string{uti.PNG.UTI, "com.example.unknown"}),
		"no conforming type available")
	c.Equal(uti.PlainText, selectDataType(uti.PlainText, nil), "nothing available")
}
