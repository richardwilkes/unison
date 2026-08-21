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

// TestAncestorOrSelfNil makes sure that the Ancestor and AncestorOrSelf methods accept a nil panel and return the
// zero value instead of a panic. UndoManagerFor sends a possibly-nil Paneler into this path, so this must not crash.
// The deprecated function forms must also accept a nil Paneler.
func TestAncestorOrSelfNil(t *testing.T) {
	c := check.New(t)
	c.Nil((*unison.Panel)(nil).Ancestor[*unison.Panel]())
	c.Nil((*unison.Panel)(nil).AncestorOrSelf[*unison.Panel]())
	c.Nil(unison.Ancestor[*unison.Panel](nil))       //nolint:staticcheck // checks the deprecated form
	c.Nil(unison.AncestorOrSelf[*unison.Panel](nil)) //nolint:staticcheck // checks the deprecated form
	c.Nil(unison.UndoManagerFor(nil))
}

// TestAncestorOrSelfResolution makes sure that AncestorOrSelf finds both the panel itself and its ancestors.
func TestAncestorOrSelfResolution(t *testing.T) {
	c := check.New(t)
	parent := unison.NewPanel()
	child := unison.NewPanel()
	parent.AddChild(child)
	c.True(child.AncestorOrSelf[*unison.Panel]() == child)
	c.True(child.Ancestor[*unison.Panel]() == parent)
}
