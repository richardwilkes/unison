// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package demo

import (
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/tid"
	"github.com/richardwilkes/unison"
)

// newCloneTestTable builds a table with no columns, so no cell (and thus no font/graphics work) is ever created,
// keeping these tests runnable headless.
func newCloneTestTable() *unison.Table[*demoRow] {
	return unison.NewTable[*demoRow](&unison.SimpleTableModel[*demoRow]{})
}

// TestCloneForTargetDeepClonesRow is the regression test for CloneForTarget performing a shallow copy: the clone shared
// the original's children slice (whose rows still pointed at the old parent and table) and its checkbox widget — and
// since a panel may have only one parent, a cross-table copy-drag stole the checkbox out of the source table's cell.
func TestCloneForTargetDeepClonesRow(t *testing.T) {
	c := check.New(t)
	src := newCloneTestTable()
	dst := newCloneTestTable()

	grandchild := &demoRow{table: src, id: tid.MustNewTID('a'), text: "grandchild"}
	child := &demoRow{table: src, id: tid.MustNewTID('a'), text: "child", container: true, open: true}
	child.SetChildren([]*demoRow{grandchild})
	grandchild.SetParent(child)
	original := &demoRow{
		table:        src,
		checkbox:     &unison.CheckBox{},
		text:         "original",
		text2:        "second",
		id:           tid.MustNewTID('a'),
		container:    true,
		open:         true,
		doubleHeight: true,
	}
	original.SetChildren([]*demoRow{child})
	child.SetParent(original)

	newParent := &demoRow{table: dst, id: tid.MustNewTID('a'), container: true}
	clone := original.CloneForTarget(dst, newParent)

	// The clone must be a distinct row bound to the target table and parent, with a fresh ID and the payload copied.
	c.True(clone != original)
	c.True(clone.table == dst)
	c.True(clone.parent == newParent)
	c.NotEqual(original.id, clone.id)
	c.Equal(original.text, clone.text)
	c.Equal(original.text2, clone.text2)
	c.Equal(original.container, clone.container)
	c.Equal(original.open, clone.open)
	c.Equal(original.doubleHeight, clone.doubleHeight)

	// The checkbox widget must not be shared, since a panel may have only one parent; the clone lazily creates its own.
	c.NotNil(original.checkbox, "the original must keep its checkbox")
	c.Nil(clone.checkbox, "the clone must not take the original's checkbox")

	// The children must be cloned recursively, not shared, and re-parented onto the clones.
	c.Equal(1, len(clone.children))
	clonedChild := clone.children[0]
	c.True(clonedChild != child, "children must be cloned, not shared")
	c.True(clonedChild.parent == clone)
	c.True(clonedChild.table == dst)
	c.NotEqual(child.id, clonedChild.id)
	c.Equal(child.text, clonedChild.text)
	c.Equal(1, len(clonedChild.children))
	clonedGrandchild := clonedChild.children[0]
	c.True(clonedGrandchild != grandchild, "grandchildren must be cloned, not shared")
	c.True(clonedGrandchild.parent == clonedChild)
	c.True(clonedGrandchild.table == dst)
	c.Equal(grandchild.text, clonedGrandchild.text)

	// The original's subtree must be untouched by the clone.
	c.True(original.table == src)
	c.True(original.children[0] == child)
	c.True(child.parent == original)
	c.True(child.table == src)
	c.True(grandchild.parent == child)

	// Growing the clone's child list must not disturb the original's, proving the slices are independent.
	clone.SetChildren(append(clone.Children(), &demoRow{table: dst, id: tid.MustNewTID('a')}))
	c.Equal(2, len(clone.children))
	c.Equal(1, len(original.children))
}

// TestCloneForTargetLeafRow verifies that cloning a childless row yields no children and copies the scalar fields.
func TestCloneForTargetLeafRow(t *testing.T) {
	c := check.New(t)
	src := newCloneTestTable()
	dst := newCloneTestTable()
	original := &demoRow{table: src, id: tid.MustNewTID('a'), text: "leaf"}
	clone := original.CloneForTarget(dst, nil)
	c.True(clone != original)
	c.True(clone.table == dst)
	c.Nil(clone.parent)
	c.Equal(0, len(clone.children))
	c.Equal(original.text, clone.text)
	c.NotEqual(original.id, clone.id)
}
