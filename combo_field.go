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
	titles := make([]string, 0, len(options))
	values := make(map[string]*string, len(options))
	for _, one := range options {
		title := displayFor(one)
		key := strings.ToLower(title)
		if seen[key] {
			continue
		}
		seen[key] = true
		titles = append(titles, title)
		values[title] = one
		if one == nil {
			notSetAllowed = true
		} else if *one == "" {
			emptyAllowed = true
		}
	}

	field := NewField()
	field.SetMinimumTextWidthUsing(titles...)

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

	// A choice goes through setDisplay rather than SetText, since «not set» and «empty» both show an empty field, which
	// SetText could not tell apart. A field with no options does not describe itself as expandable, since it has
	// nothing to show.
	field.installDropdown(func() []string { return titles }, func() string { return displayFor(currentValue) },
		func(title string) {
			value := values[title]
			before := currentValue
			setDisplay(value)
			if changedCallback != nil && !matchesCurrent(before) {
				changedCallback(value)
			}
		}, len(titles) != 0)

	setDisplay(initial)

	return field
}

// InstallDropdown turns the field into a combo box by installing, through InstallAccessoryPanel, a dropdown button at
// its right end. Clicking the button, or pressing the down arrow while the field has the focus, pops up a menu of the
// strings options returns, in the order given, with the one that matches the field's text exactly checked. options is
// called each time the menu is about to open, whether by a click, the down arrow or an assistive technology's request
// to expand, and at no other time, and no menu opens when it returns nothing. Choosing an item replaces the field's
// text through SetText, as an undoable edit of its own, so ModifiedCallback and ValidateCallback see it as they see any
// other edit, and the field goes on accepting free-form typing.
//
// The field is described to assistive technologies as a combo box that expands to show the choices and collapses to
// hide them, with the button not exposed at all, through Accessibility.Callback and Accessibility.ActionCallback. Any
// the field already has are chained onto rather than replaced, and one assigned afterward has to call on to the one it
// replaces. Describing a field must not run application code, so options is not consulted to describe it, and the
// field says it is expandable even while options would return nothing; a request to expand it is refused then.
//
// The borders InstallAccessoryPanel installs are the default field borders with room on the right for the button.
// Borders installed afterward replace them and so have to add AccessoryInsets to their own. Call this at most once, on
// a single-line field that has no accessory panel.
func (f *Field) InstallDropdown(options func() []string) {
	f.installDropdown(options, f.Text, f.SetText, true)
}

// installDropdown implements InstallDropdown and NewComboField. titles supplies the menu's items each time it is about
// to open, the one equal to what current returns is checked, choose carries out the choice of one, and expandable says
// whether the field describes itself as able to expand.
func (f *Field) installDropdown(titles func() []string, current func() string, choose func(string), expandable bool) {
	b := NewButton()
	b.HideBase = true
	b.Drawable = dropdownGlyph{field: f}
	b.SetFocusable(false)
	b.UpdateCursorCallback = func(_ geom.Point) *Cursor { return ArrowCursor() }
	// The menu is built afresh on every click, so the one remembered here is the only one that could still be showing,
	// which is what tells an assistive technology whether the field is expanded.
	var openMenu Menu
	// fetch asks for the items, guarding against a panic in the application code that supplies them.
	fetch := func() []string {
		var items []string
		SafeCall(func() { items = titles() })
		return items
	}
	// open builds the menu of the items, which there must be some of, and pops it up.
	open := func(items []string) {
		cur := current()
		initialIndex := -1
		fac := DefaultMenuFactory()
		// The menu takes the field's own name as its title, which is what an assistive technology announces as the
		// person moves into the choices; without it they hang off an anonymous menu, with nothing to say which control
		// they belong to. This is what PopupMenu does with the menu it opens, for the same reason.
		m := fac.NewMenu(PopupMenuTemporaryBaseID, axControlName(f.AsPanel()), nil)
		defer m.Dispose()
		openMenu = m
		for i, title := range items {
			item := fac.NewItem(PopupMenuTemporaryBaseID+i+1, title, KeyBinding{}, nil, func(_ MenuItem) {
				// Menu handlers already run from the event loop, after the menu has closed, so the focus request here
				// is not undone by the menu's own teardown. A fresh undo id keeps the choice from merging with the edit
				// before it.
				f.RequestFocus()
				f.undoID = NextUndoID()
				choose(title)
			})
			// Every item is given a check state, the current one checked and the rest explicitly not, so that all of
			// them are described as the one set of exclusive choices they are. An item that has never been given a
			// state at all is described as a plain command, so marking only the current one would have a person hear
			// "checked" for it and nothing whatsoever for its alternatives. PopupMenu marks the items of its own
			// dropdown the same way, for the same reason. Only the first of any duplicates is the current one.
			state := check.Off
			if initialIndex < 0 && title == cur {
				state = check.On
				initialIndex = i
			}
			item.SetCheckState(state)
			m.InsertItem(-1, item)
		}
		m.Popup(f.RectToRoot(f.ContentRect(true)), max(initialIndex, 0))
	}
	// show gives the field the focus and pops the menu up, reporting whether it did. Nothing to choose from opens
	// nothing, exactly as PopupMenu.Click shows no menu for a popup holding nothing: an empty menu panel has nothing to
	// offer anyone.
	show := func() bool {
		if !f.Enabled() {
			// The button is not disabled along with the field, so a disabled field refuses to open here.
			return false
		}
		f.RequestFocus()
		items := fetch()
		if len(items) == 0 {
			return false
		}
		open(items)
		return true
	}
	b.ClickCallback = func() { show() }

	prevKeyDown := f.KeyDownCallback
	f.KeyDownCallback = func(keyCode KeyCode, mods mod.Modifiers, repeat bool) bool {
		// Only the bare key opens the menu: with a modifier held, the key is one of the field's own selection and caret
		// commands, and it is one of those as well when there is nothing to show.
		if keyCode == KeyDown && mods&mod.NonSticky == 0 && show() {
			return true
		}
		return prevKeyDown != nil && prevKeyDown(keyCode, mods, repeat)
	}

	f.InstallAccessoryPanel(b)

	// The field and its dropdown button are one control as far as a person is concerned, so that is what is described:
	// the field is a combo box that can be expanded, and the button beside it is not exposed at all. The field's own
	// description of itself fills in everything else about it, including the contextual menu of cut, copy, paste and
	// select-all commands that it offers and carries out — asking the dropdown for that instead would leave the field's
	// real menu unreachable and would answer the request with something a right-click never does.
	//
	// A field that is not expandable says nothing about expanding and refuses to, since an assistive technology told
	// otherwise would be left waiting for choices that are never going to appear. A previous callback runs after this
	// one, so that it keeps the last word.
	b.Accessibility.Role = role.None
	f.Accessibility.Role = role.ComboBox
	prevDescribe := f.Accessibility.Callback
	f.Accessibility.Callback = func(node *accessibility.Node) {
		node.Expandable = expandable
		node.Expanded = axMenuIsOpen(openMenu)
		if expandable {
			node.Actions = node.Actions.With(accessibility.Expand, accessibility.Collapse)
		}
		axNoteOpenMenu(node, openMenu)
		if prevDescribe != nil {
			prevDescribe(node)
		}
	}
	prevAction := f.Accessibility.ActionCallback
	f.Accessibility.ActionCallback = func(req accessibility.ActionRequest) bool {
		if expandable {
			switch req.Action {
			case accessibility.Expand:
				// The menu is queued rather than opened here. Expand is a navigation action, carried out inline on
				// macOS from within the callback the assistive technology is waiting on, and popping a menu up — a
				// native menu runs a modal tracking session that does not return until the person has dismissed it —
				// would hold the assistive technology for as long as the choices were showing. That the choices are
				// showing is reported by the next description of the field instead. The items are asked for now,
				// though, so that a request with nothing to show is refused rather than reported as carried out.
				//
				// Asking for what is already there changes nothing, both now and when the queued task comes to run:
				// opening again would tear the open menu down and build it back up, which an assistive technology that
				// was told the field is expanded would not expect. A field that has been taken out of its window in the
				// meantime has nowhere to show a menu, so nothing is done for it either, and neither is anything done
				// for one that has been disabled since: the dispatcher checked that at the moment the request arrived,
				// but the work happens a task later, and a click could not open the menu of a disabled field. The menu
				// is built in the active window rather than in this field's, so a request aimed at a field in a
				// background window is refused outright — see axMayPopupMenu — and refused again when the task runs,
				// since the window may have lost the focus in between.
				if axMenuIsOpen(openMenu) {
					return true
				}
				if !axMayPopupMenu(f.AsPanel()) {
					return false
				}
				items := fetch()
				if len(items) == 0 {
					return false
				}
				InvokeTask(func() {
					if !f.Enabled() || !axMayPopupMenu(f.AsPanel()) || axMenuIsOpen(openMenu) {
						return
					}
					f.RequestFocus()
					open(items)
				})
				return true
			case accessibility.Collapse:
				axCollapseMenu(f.AsPanel(), openMenu)
				return true
			}
		}
		return prevAction != nil && prevAction(req)
	}
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
