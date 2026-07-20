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
	"strings"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
)

// TestFieldAutoScrollBottomOverflowTracksSelectionStart verifies that when a selection is being extended backward
// (anchor at the end) and its start lies below the visible area, autoScroll brings the start of the selection into
// view. A copy-paste bug used to make this case scroll the end of the selection into view instead.
func TestFieldAutoScrollBottomOverflowTracksSelectionStart(t *testing.T) {
	c := check.New(t)
	f := NewMultiLineField()
	lines := make([]string, 10)
	for i := range lines {
		lines[i] = "line" // Every line is 5 runes long, counting its newline, so line i starts at rune i*5.
	}
	f.SetText(strings.Join(lines, "\n"))
	var insets geom.Insets
	if b := f.Border(); b != nil {
		insets = b.Insets()
	}
	lineHeight := f.Font.LineHeight()
	f.SetFrameRect(geom.NewRect(0, 0, 200+insets.Width(), 2*lineHeight+insets.Height()))

	// Scroll back to the top with the caret at the start.
	f.SetSelectionToStart()
	c.Equal(float32(0), f.ScrollOffset().Y)

	// Extend the selection backward from an anchor at the end of line 8 to the start of line 5. Both lines are below
	// the two visible lines, and since the anchor is the selection end, autoScroll must scroll the selection start
	// (line 5) into view, not the selection end (line 8).
	f.setSelection(5*5, 8*5, 8*5)
	rect := f.ContentRect(false)
	pt := f.FromSelectionIndex(f.selectionStart)
	bottomGap := rect.Bottom() - (pt.Y + f.lineHeightAt(pt.Y))
	if bottomGap < 0 {
		bottomGap = -bottomGap
	}
	c.True(bottomGap < 1, "the start of the selection should be bottom-aligned, but is off by %v", bottomGap)
	c.True(f.FromSelectionIndex(f.selectionEnd).Y > rect.Bottom(),
		"the end of the selection should remain below the visible area")
}

// TestFieldApplyFieldStateRedrawsWhenOnlyTextChanges verifies that ApplyFieldState marks the field for redraw when the
// text changes but the selection is identical, as happens when undoing a forward-delete: the deletion removes the rune
// at the caret without moving it, so undo restores different text with the same selection triple.
func TestFieldApplyFieldStateRedrawsWhenOnlyTextChanges(t *testing.T) {
	c := check.New(t)
	w := newRedrawTestWindow()
	swapRedrawSet(t)
	f := NewField()
	w.root.AddChild(f)
	f.SetText("ab") // Leaves the caret at the end: selection (2, 2), anchor 2.
	redrawSet = make(map[*Window]struct{})
	f.ApplyFieldState(&FieldState{Text: "abX", SelectionStart: 2, SelectionEnd: 2, SelectionAnchor: 2})
	c.Equal("abX", f.Text())
	_, pending := redrawSet[w]
	c.True(pending, "restoring text without changing the selection must still mark the field for redraw")
}
