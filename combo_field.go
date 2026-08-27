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
	"github.com/richardwilkes/unison/enums/mod"
)

// NewComboField creates a Field with a dropdown button embedded at its right end. Clicking the button (or pressing the
// down arrow while the field has the focus) pops up a menu of the options, and the field also accepts free-form typing,
// so the value need not be one of the options. A `nil` option means "not set" and an empty string means "empty"; these
// are shown as the «not set» and «empty» watermarks rather than as text. Duplicate options (compared
// case-insensitively) are dropped. `changedCallback` is invoked only when the value actually changes, with the new
// value (`nil` for "not set"). Clearing the text yields `nil` when a `nil` option was provided, an empty string when an
// empty option was provided, and is otherwise treated as invalid and does not invoke the callback. The field's minimum
// width is sized to fit the widest option.
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
	setDisplay := func(value *string) {
		if !matchesCurrent(value) {
			updating = true
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
	b.ClickCallback = func() {
		field.RequestFocus()
		initialIndex := 0
		fac := DefaultMenuFactory()
		m := fac.NewMenu(PopupMenuTemporaryBaseID, "", nil)
		defer m.Dispose()
		for i, c := range options {
			display := displayFor(c)
			if displayFor(currentValue) == display {
				initialIndex = i
			}
			value := c
			m.InsertItem(-1, fac.NewItem(PopupMenuTemporaryBaseID+i+1, display, KeyBinding{}, nil,
				func(_ MenuItem) {
					InvokeTask(func() {
						before := currentValue
						setDisplay(value)
						field.RequestFocus()
						if changedCallback != nil && !matchesCurrent(before) {
							changedCallback(value)
						}
					})
				}))
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
