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
	"github.com/richardwilkes/unison/enums/mod"
)

func TestFieldSetTextMovesSelectionToEnd(t *testing.T) {
	c := check.New(t)
	f := unison.NewField()
	f.SetText("hello")
	c.Equal("hello", f.Text())
	start, end := f.Selection()
	c.Equal(5, start)
	c.Equal(5, end)
}

func TestFieldSetSelectionAndSelectedText(t *testing.T) {
	c := check.New(t)
	f := unison.NewField()
	f.SetText("hello")
	f.SetSelection(1, 3)
	start, end := f.Selection()
	c.Equal(1, start)
	c.Equal(3, end)
	c.Equal("el", f.SelectedText())
	c.Equal(2, f.SelectionCount())
	c.True(f.HasSelectionRange())
}

func TestFieldSetSelectionClamps(t *testing.T) {
	c := check.New(t)
	f := unison.NewField()
	f.SetText("hello")

	// Values beyond either end are constrained to the text bounds.
	f.SetSelection(-5, 100)
	start, end := f.Selection()
	c.Equal(0, start)
	c.Equal(5, end)

	// An end less than the start collapses to an empty selection at the start.
	f.SetSelection(3, 1)
	start, end = f.Selection()
	c.Equal(3, start)
	c.Equal(3, end)
	c.False(f.HasSelectionRange())
}

func TestFieldSelectAll(t *testing.T) {
	c := check.New(t)
	f := unison.NewField()
	f.SetText("hello")
	f.SetSelectionToStart()
	c.True(f.CanSelectAll())
	f.SelectAll()
	start, end := f.Selection()
	c.Equal(0, start)
	c.Equal(5, end)
	c.False(f.CanSelectAll())
}

func TestFieldDeleteSelectedRange(t *testing.T) {
	c := check.New(t)
	f := unison.NewField()
	f.SetText("hello")
	f.SetSelection(1, 3)
	f.Delete()
	c.Equal("hlo", f.Text())
	start, end := f.Selection()
	c.Equal(1, start)
	c.Equal(1, end)
}

func TestFieldDeleteActsAsBackspaceWithoutRange(t *testing.T) {
	c := check.New(t)
	f := unison.NewField()
	f.SetText("hello")
	f.SetSelectionTo(3)
	c.True(f.CanDelete())
	f.Delete()
	c.Equal("helo", f.Text())
	start, end := f.Selection()
	c.Equal(2, start)
	c.Equal(2, end)
}

func TestFieldDeleteAtStartWithoutRangeIsNoOp(t *testing.T) {
	c := check.New(t)
	f := unison.NewField()
	f.SetText("hello")
	f.SetSelectionToStart()
	c.False(f.CanDelete())
	f.Delete()
	c.Equal("hello", f.Text())
}

func TestFieldSanitizeStripsControlChars(t *testing.T) {
	c := check.New(t)
	f := unison.NewField()
	// A single-line field drops newlines but keeps tabs and printable characters.
	f.SetText("a\nb\tc")
	c.Equal("ab\tc", f.Text())
}

func TestMultiLineFieldKeepsNewlines(t *testing.T) {
	c := check.New(t)
	f := unison.NewMultiLineField()
	f.SetText("a\nb")
	c.Equal("a\nb", f.Text())
}

func TestMultiLineFieldEndKeyOnEmptyField(t *testing.T) {
	c := check.New(t)
	f := unison.NewMultiLineField()

	// End on an empty multi-line field previously panicked with an index out of range in findNextLineBreak.
	c.True(f.DefaultKeyDown(unison.KeyEnd, 0, false))
	start, end := f.Selection()
	c.Equal(0, start)
	c.Equal(0, end)

	// Cmd+Right (macOS) follows the same code path.
	f.DefaultKeyDown(unison.KeyRight, mod.OSMenuCommand(), false)
	start, end = f.Selection()
	c.Equal(0, start)
	c.Equal(0, end)

	// Shift+End extends the selection and previously hit the same panic.
	c.True(f.DefaultKeyDown(unison.KeyEnd, mod.Shift, false))
	start, end = f.Selection()
	c.Equal(0, start)
	c.Equal(0, end)
}

func TestMultiLineFieldEndKeyMovesToEndOfLine(t *testing.T) {
	c := check.New(t)
	f := unison.NewMultiLineField()
	f.SetText("first\nsecond")
	f.SetSelectionToStart()
	c.True(f.DefaultKeyDown(unison.KeyEnd, 0, false))
	start, end := f.Selection()
	c.Equal(5, start)
	c.Equal(5, end)
}

func TestMultiLineFieldHomeKeyMovesToLineStart(t *testing.T) {
	c := check.New(t)
	f := unison.NewMultiLineField()
	f.SetText("first\nsecond")
	f.SetSelectionTo(9) // Middle of "second"
	c.True(f.DefaultKeyDown(unison.KeyHome, 0, false))
	start, end := f.Selection()
	c.Equal(6, start)
	c.Equal(6, end)

	// Home when already at the start of the line stays put.
	c.True(f.DefaultKeyDown(unison.KeyHome, 0, false))
	start, end = f.Selection()
	c.Equal(6, start)
	c.Equal(6, end)

	// Shift+Home extends the selection back to the start of the line.
	f.SetSelectionTo(9)
	c.True(f.DefaultKeyDown(unison.KeyHome, mod.Shift, false))
	start, end = f.Selection()
	c.Equal(6, start)
	c.Equal(9, end)
}

func TestMultiLineFieldHomeKeyOnEmptyTrailingLine(t *testing.T) {
	c := check.New(t)
	f := unison.NewMultiLineField()
	f.SetText("ab\n")
	f.SetSelectionTo(3)

	// The caret is on the empty line following the trailing newline, so Home must keep it there rather than jumping
	// to the start of the text.
	c.True(f.DefaultKeyDown(unison.KeyHome, 0, false))
	start, end := f.Selection()
	c.Equal(3, start)
	c.Equal(3, end)
}

func TestMultiLineFieldHomeKeyWithLineBreakAtStart(t *testing.T) {
	c := check.New(t)
	f := unison.NewMultiLineField()
	f.SetText("\nabc")
	f.SetSelectionTo(2)

	// The line containing the caret starts just past the newline at index 0; Home must not treat "line break at
	// index 0" as "no line break" and move to the start of the text.
	c.True(f.DefaultKeyDown(unison.KeyHome, 0, false))
	start, end := f.Selection()
	c.Equal(1, start)
	c.Equal(1, end)

	// Shift+Home from the same spot extends back to the line start only.
	f.SetSelectionTo(2)
	c.True(f.DefaultKeyDown(unison.KeyHome, mod.Shift, false))
	start, end = f.Selection()
	c.Equal(1, start)
	c.Equal(2, end)
}

func TestMultiLineFieldHomeKeyOnEmptyField(t *testing.T) {
	c := check.New(t)
	f := unison.NewMultiLineField()
	c.True(f.DefaultKeyDown(unison.KeyHome, 0, false))
	start, end := f.Selection()
	c.Equal(0, start)
	c.Equal(0, end)
}

func TestFieldCanCutCopyTrackSelection(t *testing.T) {
	c := check.New(t)
	f := unison.NewField()
	f.SetText("hello")
	f.SetSelectionTo(2)
	c.False(f.CanCut())
	c.False(f.CanCopy())
	f.SetSelection(1, 4)
	c.True(f.CanCut())
	c.True(f.CanCopy())
}

func TestFieldModifiedCallbackFiresOnEdit(t *testing.T) {
	c := check.New(t)
	f := unison.NewField()
	calls := 0
	f.ModifiedCallback = func(_, _ *unison.FieldState) { calls++ }
	f.SetText("hi")
	c.Equal(1, calls)
	f.SetSelectionToEnd()
	f.Delete()
	c.Equal(2, calls)
}
