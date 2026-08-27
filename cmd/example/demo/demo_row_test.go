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

// TestCloneForTargetCarriesEditedFieldText is the regression test for a cross-table copy-drag reverting the editable
// cell: the clone dropped the original's Field (correctly, since a panel may have only one parent) but the text lived
// only in that Field, so the clone's fresh Field was seeded with the placeholder text rather than what the user typed.
// The text is now kept in the row itself, where the Field writes it back on every edit, so it survives the clone, is
// what the clone's own Field is seeded from, and is what sorting sees whether or not a Field has been created yet.
func TestCloneForTargetCarriesEditedFieldText(t *testing.T) {
	c := check.New(t)
	src := newCloneTestTable()
	dst := newCloneTestTable()
	original := &demoRow{table: src, id: tid.MustNewTID('a'), text3: "initial"}

	// Sorting must see the text before the cell has ever been materialized.
	c.Equal("initial", original.CellDataForSort(3))

	// Materializing the cell seeds the field from the row's text, and the same field is returned on every call.
	field, ok := original.ColumnCell(0, 3, nil, nil, false, false, false).(*unison.Field)
	c.True(ok, "column 3 must be a Field")
	c.Equal("initial", field.Text())
	c.True(original.ColumnCell(0, 3, nil, nil, false, false, false) == field, "the same field must be returned")

	// An edit made through the field must be reflected in the row's data.
	field.SetText("edited")
	c.Equal("edited", original.text3)
	c.Equal("edited", original.CellDataForSort(3))

	// The clone must carry the edited text without sharing the widget that holds it.
	clone := original.CloneForTarget(dst, nil)
	c.Nil(clone.field, "the clone must not take the original's field")
	c.Equal("edited", clone.text3)
	c.Equal("edited", clone.CellDataForSort(3))
	cloneField, ok := clone.ColumnCell(0, 3, nil, nil, false, false, false).(*unison.Field)
	c.True(ok, "the clone's column 3 must be a Field")
	c.True(cloneField != field, "the clone must create its own field")
	c.Equal("edited", cloneField.Text())

	// Once cloned, the two rows must be independent of one another.
	cloneField.SetText("changed in clone")
	c.Equal("changed in clone", clone.text3)
	c.Equal("edited", original.text3)
	c.Equal("edited", field.Text())
}

// TestFieldCellHasNoBorder verifies that the editable cell's Field has had its default border removed, so that it has
// no insets and is the same size as the plain text cells, and that the border is not reinstalled when the focus moves
// in and out of the field.
func TestFieldCellHasNoBorder(t *testing.T) {
	c := check.New(t)
	row := &demoRow{table: newCloneTestTable(), id: tid.MustNewTID('a'), text3: "xyz"}
	field, ok := row.ColumnCell(0, 3, nil, nil, false, false, false).(*unison.Field)
	c.True(ok, "column 3 must be a Field")
	c.Nil(field.Border(), "the field must have no border")
	field.GainedFocusCallback()
	c.Nil(field.Border(), "gaining the focus must not install a border")
	field.LostFocusCallback()
	c.Nil(field.Border(), "losing the focus must not install a border")
}

// TestFieldCellUsesRowInks verifies that the editable cell's Field takes on the inks the table passes in for the row,
// so that it blends in with the rest of the row, and that they are refreshed on every call, since the table hands in
// different inks as the row's selection and banding state changes.
func TestFieldCellUsesRowInks(t *testing.T) {
	c := check.New(t)
	row := &demoRow{table: newCloneTestTable(), id: tid.MustNewTID('a'), text3: "xyz"}
	field, ok := row.ColumnCell(0, 3, unison.Black, unison.White, false, false, false).(*unison.Field)
	c.True(ok, "column 3 must be a Field")
	c.Equal(unison.Ink(unison.White), field.EditableInk)
	c.Equal(unison.Ink(unison.Black), field.OnEditableInk)
	c.True(row.ColumnCell(0, 3, unison.Red, unison.Blue, true, false, true) == field,
		"the same field must be returned")
	c.Equal(unison.Ink(unison.Blue), field.EditableInk)
	c.Equal(unison.Ink(unison.Red), field.OnEditableInk)
}
