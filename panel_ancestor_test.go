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

// TestAncestorOrSelfNil verifies that AncestorOrSelf tolerates a nil Paneler the same way Ancestor does, returning the
// zero value rather than panicking. UndoManagerFor forwards a possibly-nil Paneler straight into AncestorOrSelf, so
// this must not crash.
func TestAncestorOrSelfNil(t *testing.T) {
	c := check.New(t)
	c.Nil(unison.Ancestor[*unison.Panel](nil))
	c.Nil(unison.AncestorOrSelf[*unison.Panel](nil))
	c.Nil(unison.UndoManagerFor(nil))
}

// TestAncestorOrSelfResolution verifies that AncestorOrSelf still finds both the panel itself and its ancestors.
func TestAncestorOrSelfResolution(t *testing.T) {
	c := check.New(t)
	parent := unison.NewPanel()
	child := unison.NewPanel()
	parent.AddChild(child)
	c.True(unison.AncestorOrSelf[*unison.Panel](child) == child)
	c.True(unison.Ancestor[*unison.Panel](child) == parent)
}
