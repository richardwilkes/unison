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
)

// TestDockStateApplyRejectsExcessLayoutChildren verifies that applying a malformed (e.g. hand-edited) saved state
// whose layout node claims more than two children ignores the extras instead of panicking on the fixed-size nodes
// array.
func TestDockStateApplyRejectsExcessLayoutChildren(t *testing.T) {
	c := check.New(t)
	_, layout := newTestDock()
	state := &DockState{
		Type: LayoutType,
		Children: []*DockState{
			{Type: LayoutType},
			{Type: LayoutType},
			{Type: LayoutType},
		},
	}
	state.apply(layout, func(string) Dockable { return nil })
	c.NotNil(layout.nodes[0])
	c.NotNil(layout.nodes[1])
}
