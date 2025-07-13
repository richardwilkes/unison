// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
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

func TestUndo(t *testing.T) {
	c := check.New(t)
	mgr := unison.NewUndoManager(5, func(err error) { t.Error(err) })
	c.False(mgr.CanRedo())
	c.False(mgr.CanUndo())
	t1 := newTestUndo("t1")
	mgr.Add(t1)
	c.False(mgr.CanRedo())
	c.True(mgr.CanUndo())
	c.Equal(0, t1.absorbed)
	mgr.Undo()
	c.True(mgr.CanRedo())
	c.False(mgr.CanUndo())
	mgr.Redo()
	c.False(mgr.CanRedo())
	c.True(mgr.CanUndo())
	c.Equal(0, t1.absorbed)
	t2 := newTestUndo("t2")
	mgr.Add(t2)
	c.False(mgr.CanRedo())
	c.True(mgr.CanUndo())
	c.Equal(0, t1.absorbed)
	c.Equal(0, t2.absorbed)
	t2again := newTestUndo("t2")
	mgr.Add(t2again)
	c.False(mgr.CanRedo())
	c.True(mgr.CanUndo())
	c.Equal(0, t1.absorbed)
	c.Equal(0, t2.absorbed)
	c.Equal(1, t2again.absorbed)
	c.Equal(1, t2again.released)
	c.Equal(1, t1.undone)
	c.Equal(1, t1.redone)
	c.Equal(0, t2.undone)
	c.Equal(0, t2.redone)
	mgr.Undo()
	c.Equal(1, t1.undone)
	c.Equal(1, t1.redone)
	c.Equal(1, t2.undone)
	c.Equal(0, t2.redone)
	t3 := newTestUndo("t3")
	mgr.Add(t3)
	t4 := newTestUndo("t4")
	mgr.Add(t4)
	t5 := newTestUndo("t5")
	mgr.Add(t5)
	t6 := newTestUndo("t6")
	mgr.Add(t6)
	c.Equal(0, t1.released)
	c.Equal(1, t2.released)
	c.Equal(0, t3.released)
	c.Equal(0, t4.released)
	c.Equal(0, t5.released)
	c.Equal(0, t6.released)
	t7 := newTestUndo("t7")
	mgr.Add(t7)
	c.Equal(1, t1.released)
	c.Equal(1, t2.released)
	c.Equal(0, t3.released)
	c.Equal(0, t4.released)
	c.Equal(0, t5.released)
	c.Equal(0, t6.released)
	c.Equal(0, t7.released)
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
