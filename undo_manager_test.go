// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
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

	"github.com/richardwilkes/toolbox/check"
	"github.com/richardwilkes/unison"
)

func TestUndo(t *testing.T) {
	mgr := unison.NewUndoManager(5, func(err error) { t.Error(err) })
	check.False(t, mgr.CanRedo())
	check.False(t, mgr.CanUndo())
	t1 := newTestUndo("t1")
	mgr.Add(t1)
	check.False(t, mgr.CanRedo())
	check.True(t, mgr.CanUndo())
	check.Equal(t, 0, t1.absorbed)
	mgr.Undo()
	check.True(t, mgr.CanRedo())
	check.False(t, mgr.CanUndo())
	mgr.Redo()
	check.False(t, mgr.CanRedo())
	check.True(t, mgr.CanUndo())
	check.Equal(t, 0, t1.absorbed)
	t2 := newTestUndo("t2")
	mgr.Add(t2)
	check.False(t, mgr.CanRedo())
	check.True(t, mgr.CanUndo())
	check.Equal(t, 0, t1.absorbed)
	check.Equal(t, 0, t2.absorbed)
	t2again := newTestUndo("t2")
	mgr.Add(t2again)
	check.False(t, mgr.CanRedo())
	check.True(t, mgr.CanUndo())
	check.Equal(t, 0, t1.absorbed)
	check.Equal(t, 0, t2.absorbed)
	check.Equal(t, 1, t2again.absorbed)
	check.Equal(t, 1, t2again.released)
	check.Equal(t, 1, t1.undone)
	check.Equal(t, 1, t1.redone)
	check.Equal(t, 0, t2.undone)
	check.Equal(t, 0, t2.redone)
	mgr.Undo()
	check.Equal(t, 1, t1.undone)
	check.Equal(t, 1, t1.redone)
	check.Equal(t, 1, t2.undone)
	check.Equal(t, 0, t2.redone)
	t3 := newTestUndo("t3")
	mgr.Add(t3)
	t4 := newTestUndo("t4")
	mgr.Add(t4)
	t5 := newTestUndo("t5")
	mgr.Add(t5)
	t6 := newTestUndo("t6")
	mgr.Add(t6)
	check.Equal(t, 0, t1.released)
	check.Equal(t, 1, t2.released)
	check.Equal(t, 0, t3.released)
	check.Equal(t, 0, t4.released)
	check.Equal(t, 0, t5.released)
	check.Equal(t, 0, t6.released)
	t7 := newTestUndo("t7")
	mgr.Add(t7)
	check.Equal(t, 1, t1.released)
	check.Equal(t, 1, t2.released)
	check.Equal(t, 0, t3.released)
	check.Equal(t, 0, t4.released)
	check.Equal(t, 0, t5.released)
	check.Equal(t, 0, t6.released)
	check.Equal(t, 0, t7.released)
}

type testUndo struct {
	name     string
	absorbed int
	undone   int
	redone   int
	released int
}

func newTestUndo(name string) *testUndo {
	return &testUndo{name: name}
}

func (tu *testUndo) Name() string {
	return tu.name
}

func (tu *testUndo) Cost() int {
	return 1
}

func (tu *testUndo) Undo() {
	tu.undone++
}

func (tu *testUndo) Redo() {
	tu.redone++
}

func (tu *testUndo) Absorb(other unison.Undoable) bool {
	if otu, ok := other.(*testUndo); ok && tu.name == otu.name {
		otu.absorbed++
		return true
	}
	return false
}

func (tu *testUndo) Release() {
	tu.released++
}
