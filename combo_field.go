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

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/i18n"
	"github.com/richardwilkes/toolbox/v2/xmath"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/check"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/enums/role"
)

// NewComboField creates a Field with a dropdown button embedded at its right end. Clicking the button (or pressing the
// down arrow while the field has the focus) pops up a menu of the options, and the field also accepts free-form typing,
// so the value need not be one of the options. A `nil` option means "not set" and an empty string means "empty"; these
// are shown as the «not set» and «empty» watermarks rather than as text. Duplicate options (compared
// case-insensitively) are dropped. `changedCallback` is invoked only when the value actually changes, with the new
// value (`nil` for "not set"). Clearing the text yields `nil` when a `nil` option was provided, an empty string when an
// empty option was provided, and is otherwise treated as invalid and does not invoke the callback. The field's minimum
// width is sized to fit the widest option.
//
// The returned field is described to assistive technologies as the whole combo box, which the field and its dropdown
// button amount to, and both Accessibility.Callback and Accessibility.ActionCallback of the returned field belong to
// that: the first reports whether the choices are showing and offers the Expand and Collapse that show and hide them,
// and the second carries them out. An application that assigns either of them replaces that reporting outright, leaving
// the field described as something that does not expand — or as something that does, while refusing to. There is no
// other hook, since what comes back is a bare *Field, so an application that needs one of those callbacks has to call
// on to the one it is replacing.
func NewComboField(options []*string, initial *string, changedCallback func(value *string)) *Field {
	notSetDisplay := i18n.Text("«not set»")
	emptyDisplay := i18n.Text("«empty»")
	displayFor := func(value *string) string {
		switch {
		case value == nil:
			return notSetDisplay
		case *value == "":
			return emptyDisplay
		default:
			return *value
		}
	}

	emptyAllowed := false
	notSetAllowed := false
	seen := make(map[string]bool, len(options))
	deduped := make([]*string, 0, len(options))
	for _, one := range options {
		key := strings.ToLower(displayFor(one))
		if seen[key] {
			continue
		}
		seen[key] = true
		deduped = append(deduped, one)
		if one == nil {
			notSetAllowed = true
		} else if *one == "" {
			emptyAllowed = true
		}
	}
	options = deduped

	widthCandidates := make([]string, len(options))
	for i, one := range options {
		widthCandidates[i] = displayFor(one)
	}
	field := NewField()
	field.SetMinimumTextWidthUsing(widthCandidates...)

	var currentValue *string
	matchesCurrent := func(value *string) bool {
		switch {
		case value == nil:
			return currentValue == nil
		case *value == "":
			return currentValue != nil && *currentValue == ""
		default:
			return currentValue != nil && *currentValue == *value
		}
	}
	updating := false
	fieldValueSet := false
	setDisplay := func(value *string) {
		if !fieldValueSet || !matchesCurrent(value) {
			updating = true
			fieldValueSet = true
			currentValue = value
			switch {
			case value == nil:
				field.Watermark = notSetDisplay
				field.SetText("")
			case *value == "":
				field.Watermark = emptyDisplay
				field.SetText("")
			default:
				field.Watermark = ""
				field.SetText(*value)
			}
			field.MarkForRedraw()
			updating = false
		}
	}

	b := NewButton()
	b.HideBase = true
	b.Drawable = dropdownGlyph{field: field}
	b.SetFocusable(false)
	b.UpdateCursorCallback = func(_ geom.Point) *Cursor { return ArrowCursor() }
	// The menu is built afresh on every click, so the one remembered here is the only one that could still be showing,
	// which is what tells an assistive technology whether the combo field is expanded.
	var openMenu Menu
	b.ClickCallback = func() {
		field.RequestFocus()
		if len(options) == 0 {
			// A combo field with nothing to choose from opens nothing, exactly as PopupMenu.Click shows no menu for a
			// popup holding nothing: an empty menu panel has nothing to offer anyone, and the field describes itself as
			// something that does not expand, which every way of opening it has to agree with.
			return
		}
		initialIndex := 0
		fac := DefaultMenuFactory()
		// The menu takes the field's own name as its title, which is what an assistive technology announces as the
		// person moves into the choices; without it they hang off an anonymous menu, with nothing to say which control
		// they belong to. This is what PopupMenu does with the menu it opens, for the same reason.
		m := fac.NewMenu(PopupMenuTemporaryBaseID, axControlName(field.AsPanel()), nil)
		defer m.Dispose()
		openMenu = m
		for i, c := range options {
			display := displayFor(c)
			isCurrent := displayFor(currentValue) == display
			if isCurrent {
				initialIndex = i
			}
			value := c
			item := fac.NewItem(PopupMenuTemporaryBaseID+i+1, display, KeyBinding{}, nil,
				func(_ MenuItem) {
					// Menu handlers already run from the event loop, after the menu has closed, so the focus request
					// here is not undone by the menu's own teardown.
					before := currentValue
					setDisplay(value)
					field.RequestFocus()
					if changedCallback != nil && !matchesCurrent(before) {
						changedCallback(value)
					}
				})
			// Every option is given a check state, the field's current value checked and the rest explicitly not, so
			// that all of them are described as the one set of exclusive choices they are. An item that has never been
			// given a state at all is described as a plain command, so marking only the current one would have a person
			// hear "checked" for it and nothing whatsoever for its alternatives — no "not checked", and nothing saying
			// the items belong together. PopupMenu marks the items of its own dropdown the same way, for the same
			// reason.
			state := check.Off
			if isCurrent {
				state = check.On
			}
			item.SetCheckState(state)
			m.InsertItem(-1, item)
		}
		m.Popup(field.RectToRoot(field.ContentRect(true)), initialIndex)
	}

	field.ModifiedCallback = func(_, after *FieldState) {
		if updating {
			return
		}
		if !emptyAllowed && !notSetAllowed && after.Text == "" {
			return
		}
		text := after.Text
		before := currentValue
		value := &text
		if !emptyAllowed && notSetAllowed && text == "" {
			value = nil
		}
		setDisplay(value)
		if changedCallback != nil && !matchesCurrent(before) {
			changedCallback(value)
		}
	}

	field.ValidateCallback = func() bool {
		return emptyAllowed || notSetAllowed || field.Text() != ""
	}

	defaultKeyDown := field.KeyDownCallback
	field.KeyDownCallback = func(keyCode KeyCode, mods mod.Modifiers, repeat bool) bool {
		if keyCode == KeyDown {
			b.ClickCallback()
			return true
		}
		return defaultKeyDown(keyCode, mods, repeat)
	}

	field.InstallAccessoryPanel(b)

	// The field and its dropdown button are one control as far as a person is concerned, so that is what is described:
	// the field is a combo box that can be expanded, and the button beside it is not exposed at all. The field's own
	// description of itself fills in everything else about it, including the contextual menu of cut, copy, paste and
	// select-all commands that it offers and carries out — asking the dropdown for that instead would leave the field's
	// real menu unreachable and would answer the request with something a right-click never does.
	//
	// A field built with no options has no choices to show, so it says nothing about expanding: the dropdown opens
	// nothing either, and an assistive technology told otherwise would be left waiting for choices that are never going
	// to appear.
	b.Accessibility.Role = role.None
	field.Accessibility.Role = role.ComboBox
	hasOptions := len(options) != 0
	field.Accessibility.Callback = func(node *accessibility.Node) {
		node.Expandable = hasOptions
		node.Expanded = axMenuIsOpen(openMenu)
		if hasOptions {
			node.Actions = node.Actions.With(accessibility.Expand, accessibility.Collapse)
		}
		axNoteOpenMenu(node, openMenu)
	}
	field.Accessibility.ActionCallback = func(req accessibility.ActionRequest) bool {
		if !hasOptions {
			return false
		}
		switch req.Action {
		case accessibility.Expand:
			// The click is queued rather than performed here. Expand is a navigation action, carried out inline on
			// macOS from within the callback the assistive technology is waiting on, and clicking the dropdown pops a
			// menu up — a native menu runs a modal tracking session that does not return until the person has
			// dismissed it, so answering only once the click returned would hold the assistive technology for as long
			// as the choices were showing. That the choices are showing is reported by the next description of the
			// field instead.
			//
			// Asking for what is already there changes nothing, both now and when the queued click comes to run:
			// clicking again would tear the open menu down and build it back up, which an assistive technology that
			// was told the field is expanded would not expect. A field that has been taken out of its window in the
			// meantime has nowhere to show a menu, so nothing is done for it either, and neither is anything done for
			// one that has been disabled since: the dispatcher checked that at the moment the request arrived, but the
			// work happens a task later, and a click could not open the menu of a disabled field. The menu is built in
			// the active window rather than in this field's, so a request aimed at a field in a background window is
			// refused outright — see axMayPopupMenu — and refused again when the task runs, since the window may have
			// lost the focus in between.
			if axMenuIsOpen(openMenu) {
				return true
			}
			if !axMayPopupMenu(field.AsPanel()) {
				return false
			}
			InvokeTask(func() {
				if !field.Enabled() || !axMayPopupMenu(field.AsPanel()) || axMenuIsOpen(openMenu) {
					return
				}
				b.ClickCallback()
			})
			return true
		case accessibility.Collapse:
			axCollapseMenu(field.AsPanel(), openMenu)
			return true
		default:
			return false
		}
	}

	setDisplay(initial)

	return field
}

type dropdownGlyph struct {
	field *Field
}

func (d dropdownGlyph) LogicalSize() geom.Size {
	// Match the sizing that an actual PopupMenu uses
	width := xmath.Floor(d.field.Font.LineHeight() * 0.75)
	return geom.NewSize(width, width/2)
}

func (d dropdownGlyph) DrawInRect(canvas *Canvas, rect geom.Rect, _ *SamplingOptions, paint *Paint) {
	path := NewPath()
	path.MoveTo(geom.NewPoint(rect.X, rect.Y))
	path.LineTo(geom.NewPoint(rect.Right(), rect.Y))
	path.LineTo(geom.NewPoint(rect.X+rect.Width/2, rect.Bottom()))
	path.Close()
	canvas.DrawPath(path, paint)
}
