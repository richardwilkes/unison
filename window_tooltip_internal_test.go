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
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/enums/mod"
)

// TestUpdateTooltipPassesAlignedAvoidRect verifies that updateTooltip hands UpdateTooltipCallback — and, through it,
// the tooltip sequencer — a pixel-aligned rect to avoid. geom.Rect.Align has a value receiver, so calling it without
// keeping its result left the avoid rect at whatever fractional position the panel happened to occupy, placing
// tooltips at fractional pixel positions.
func TestUpdateTooltipPassesAlignedAvoidRect(t *testing.T) {
	c := check.New(t)
	w := newCursorTestWindow()
	content := w.root.contentPanel
	content.SetFrameRect(geom.NewRect(0.25, 20.75, 200, 200))
	child := NewPanel()
	child.SetFrameRect(geom.NewRect(30.5, 40.25, 100.5, 100.25))
	content.AddChild(child)

	var got geom.Rect
	called := 0
	child.UpdateTooltipCallback = func(_ geom.Point, avoid geom.Rect) geom.Rect {
		called++
		got = avoid
		return avoid
	}
	w.updateTooltip(child.AsPanel(), geom.NewPoint(50, 70))
	c.Equal(1, called)

	unaligned := child.RectToRoot(child.ContentRect(true))
	c.NotEqual(unaligned, unaligned.Align(), "the panel must sit at a fractional position for this test to mean anything")
	c.Equal(unaligned.Align(), got)
}

// TestModifierKeyDownKeepsTooltip verifies that pressing a modifier on its own leaves a showing tooltip in place, while
// any other key still clears it.
func TestModifierKeyDownKeepsTooltip(t *testing.T) {
	c := check.New(t)
	w := newModalInputTestWindow()
	tip := NewPanel()
	modifiers := []KeyCode{
		KeyLShift, KeyRShift, KeyLControl, KeyRControl, KeyLOption, KeyROption, KeyLCommand, KeyRCommand,
	}
	for _, key := range modifiers {
		w.root.setTooltip(tip)
		w.keyPressed(key, mod.Shift|mod.Command)
		w.keyReleased(key, 0)
		c.True(w.root.tooltipPanel == tip, key.String())
	}
	w.keyPressed(KeyS, mod.Shift|mod.Command)
	c.True(w.root.tooltipPanel == nil)
}
