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
	"strconv"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/tid"
	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/unison"
)

var _ unison.TableRowData[*demoRow] = &demoRow{}

type demoRow struct {
	table        *unison.Table[*demoRow]
	parent       *demoRow
	checkbox     *unison.CheckBox
	field        *unison.Field
	text         string
	text2        string
	text3        string
	id           tid.TID
	children     []*demoRow
	container    bool
	open         bool
	doubleHeight bool
}

func (d *demoRow) CloneForTarget(target unison.Paneler, newParent *demoRow) *demoRow {
	table, ok := target.(*unison.Table[*demoRow])
	if !ok {
		xos.ExitIfErr(errs.New("invalid target"))
	}
	clone := *d
	clone.table = table
	clone.parent = newParent
	clone.id = tid.MustNewTID('a')
	// A panel may have only one parent, so the clone must not share the original's checkbox and field widgets; leaving
	// them nil causes fresh ones to be created on demand for the clone's own cells. The field's text isn't lost by
	// this, since it is kept in text3 (which the struct copy above carried over) and the clone's field will be seeded
	// from it.
	clone.checkbox = nil
	clone.field = nil
	// Deep-clone the children so the clone doesn't share its subtree with the original row.
	clone.children = nil
	for _, child := range d.children {
		clone.children = append(clone.children, child.CloneForTarget(target, &clone))
	}
	return &clone
}

func (d *demoRow) ID() tid.TID {
	return d.id
}

func (d *demoRow) Parent() *demoRow {
	return d.parent
}

func (d *demoRow) SetParent(parent *demoRow) {
	d.parent = parent
}

func (d *demoRow) CanHaveChildren() bool {
	return d.container
}

func (d *demoRow) Children() []*demoRow {
	return d.children
}

func (d *demoRow) SetChildren(children []*demoRow) {
	d.children = children
}

func (d *demoRow) CellDataForSort(index int) string {
	switch index {
	case 0:
		if d.checkbox == nil {
			d.checkbox = unison.NewCheckBox()
		}
		return strconv.Itoa(int(d.checkbox.State))
	case 1:
		return d.text
	case 2:
		return d.text2
	case 3:
		return d.text3
	default:
		return ""
	}
}

func (d *demoRow) ColumnCell(row, col int, foreground, background unison.Ink, _, _, _ bool) unison.Paneler {
	switch col {
	case 0:
		if d.checkbox == nil {
			d.checkbox = unison.NewCheckBox()
		}
		return d.checkbox
	case 1:
		wrapper := unison.NewPanel()
		wrapper.SetLayout(&unison.FlexLayout{Columns: 1})
		width := d.table.CellWidth(row, col)
		addWrappedText(wrapper, d.text, foreground, unison.LabelFont, width)
		if d.doubleHeight {
			addWrappedText(wrapper, "A little note…", foreground,
				unison.LabelFont.Face().Font(unison.LabelFont.Size()-1), width)
		}
		wrapper.UpdateTooltipCallback = func(_ geom.Point, _ geom.Rect) geom.Rect {
			wrapper.Tooltip = unison.NewTooltipWithText("A tooltip for the cell")
			return wrapper.RectToRoot(wrapper.ContentRect(true))
		}
		return wrapper
	case 2:
		wrapper := unison.NewPanel()
		wrapper.SetLayout(&unison.FlexLayout{Columns: 1})
		width := d.table.CellWidth(row, col)
		addWrappedText(wrapper, d.text2, foreground, unison.LabelFont, width)
		return wrapper
	case 3:
		// An editable cell. The same Field instance must be returned on every call, since it holds the keyboard focus
		// while it is being edited. The text itself lives in text3, which the field is seeded from and writes back to
		// on every edit, so the row's data doesn't depend on the widget existing: sorting can read it before the cell
		// has ever been shown, and a clone can carry it to another table without sharing the widget.
		if d.field == nil {
			d.field = unison.NewField()
			// Remove the border (and the focus handling that would reinstall it) that NewField() installs, so the
			// field has no insets and is the same size as the plain text cells in the other columns.
			unison.UninstallFocusBorders(d.field, d.field)
			d.field.SetText(d.text3)
			d.field.ModifiedCallback = func(_, after *unison.FieldState) { d.text3 = after.Text }
		}
		// A Field fills its bounds and draws its text using its own inks rather than the ones passed in, so copy the
		// row's inks onto it on every call to keep it blending in with the rest of the row as the selection and banding
		// change. An enabled Field draws with its editable inks, so those are the ones that need to be set.
		d.field.EditableInk = background
		d.field.OnEditableInk = foreground
		return d.field
	default:
		errs.Log(errs.New("column index out of range (0-3)"), "column", col)
		return unison.NewLabel()
	}
}

func addWrappedText(parent *unison.Panel, text string, ink unison.Ink, font unison.Font, width float32) {
	decoration := &unison.TextDecoration{Font: font}
	var lines []*unison.Text
	if width > 0 {
		lines = unison.NewTextWrappedLines(text, decoration, width)
	} else {
		lines = unison.NewTextLines(text, decoration)
	}
	for _, line := range lines {
		label := unison.NewLabel()
		label.Font = font
		label.OnBackgroundInk = ink
		label.SetTitle(line.String())
		parent.AddChild(label)
	}
}

func (d *demoRow) IsOpen() bool {
	return d.open
}

func (d *demoRow) SetOpen(open bool) {
	d.open = open
}
