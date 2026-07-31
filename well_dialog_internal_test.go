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

// TestWellDialogSwitchEditorTracksCurrent verifies that switchEditor records the editor it switched to, so that a
// second request for the same editor is a no-op instead of installing a duplicate editor panel.
func TestWellDialogSwitchEditorTracksCurrent(t *testing.T) {
	c := check.New(t)
	w := NewWell()
	w.Mask = ColorWellMask
	d := &wellDialog{
		well:        w,
		originalInk: w.Ink(),
		ink:         w.Ink(),
		right:       NewPanel(),
		current:     255,
	}
	d.switchEditor(ColorWellMask)
	c.Equal(ColorWellMask, d.current)
	c.Equal(1, len(d.right.Children()))
	d.switchEditor(ColorWellMask)
	c.Equal(1, len(d.right.Children()), "switching to the already-current editor must not install another one")
}
