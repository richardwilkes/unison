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
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/enums/paintstyle"
)

const previousFocusCallbacksKey = "unison.internal.previous.focus.callbacks"

type previousFocusCallbacks struct {
	GainedFocusCallback func()
	LostFocusCallback   func()
}

// Border defines methods required of all border providers.
type Border interface {
	// Insets returns the insets describing the space the border occupies on each side.
	Insets() geom.Insets
	// Draw the border into rect.
	Draw(canvas *Canvas, rect geom.Rect)
}

// StdInsets returns insets preset to the standard spacing.
func StdInsets() geom.Insets {
	return geom.Insets{
		Top:    StdVSpacing,
		Left:   StdHSpacing,
		Bottom: StdVSpacing,
		Right:  StdHSpacing,
	}
}

var _ Ink = &fieldBorderInk{}

// fieldBorderInk paints with the field's ErrorInk while the field is invalid and with its normal ink otherwise.
type fieldBorderInk struct {
	normal Ink
	field  *Field
}

// Paint implements Ink.
func (i *fieldBorderInk) Paint(canvas *Canvas, rect geom.Rect, style paintstyle.Enum) *Paint {
	if i.field.invalid {
		return i.field.ErrorInk.Paint(canvas, rect, style)
	}
	return i.normal.Paint(canvas, rect, style)
}

// fieldProvider is satisfied by Field and anything that embeds it.
type fieldProvider interface {
	asField() *Field
}

// fieldOf returns the Field p is or embeds, looking at p itself and then at its Self, or nil if there is none.
func fieldOf(p Paneler) *Field {
	if fp, ok := p.(fieldProvider); ok {
		return fp.asField()
	}
	if fp, ok := p.AsPanel().Self.(fieldProvider); ok {
		return fp.asField()
	}
	return nil
}

// NewDefaultFieldBorder creates the default border for a field. A border made this way never shows a field's invalid
// state; InstallDefaultFieldBorder installs borders that do.
func NewDefaultFieldBorder(focused bool) Border {
	return newDefaultFieldBorder(focused, nil)
}

// newDefaultFieldBorder creates the default border for a field. When field is not nil, the line is drawn in the field's
// ErrorInk while it is invalid.
func newDefaultFieldBorder(focused bool, field *Field) Border {
	var adj float32
	var ink Ink
	if focused {
		adj = 0
		ink = ThemeFocus
	} else {
		adj = 1
		ink = ThemeSurfaceEdge
	}
	if field != nil {
		ink = &fieldBorderInk{normal: ink, field: field}
	}
	return NewCompoundBorder(
		NewLineBorder(ink, geom.Size{}, geom.NewUniformInsets(2-adj), false),
		NewEmptyBorder(geom.Insets{Top: 2 + adj, Left: 2 + adj, Bottom: 1 + adj, Right: 2 + adj}),
	)
}

// InstallFocusBorders installs the provided borders on the borderTarget and chains into the focus handling of the
// focusTarget to adjust the border as focus changes. To prevent the display from shifting around, the borders should
// have the same insets.
func InstallFocusBorders(focusTarget, borderTarget Paneler, focusedBorder, unfocusedBorder Border) {
	focusPanel := focusTarget.AsPanel()
	borderPanel := borderTarget.AsPanel()
	clientData := focusPanel.ClientData()
	previous, ok := clientData[previousFocusCallbacksKey].(previousFocusCallbacks)
	if !ok {
		previous = previousFocusCallbacks{
			GainedFocusCallback: focusPanel.GainedFocusCallback,
			LostFocusCallback:   focusPanel.LostFocusCallback,
		}
	}
	clientData[previousFocusCallbacksKey] = previous
	focusPanel.GainedFocusCallback = func() {
		borderPanel.SetBorder(focusedBorder)
		SafeCall(previous.GainedFocusCallback)
	}
	focusPanel.LostFocusCallback = func() {
		borderPanel.SetBorder(unfocusedBorder)
		SafeCall(previous.LostFocusCallback)
	}
	borderPanel.SetBorder(unfocusedBorder)
}

// UninstallFocusBorders removes the focus handling and border from the borderTarget that was installed by a previous
// call to InstallFocusBorders.
func UninstallFocusBorders(focusTarget, borderTarget Paneler) {
	focusPanel := focusTarget.AsPanel()
	borderPanel := borderTarget.AsPanel()
	clientData := focusPanel.ClientData()
	if previous, ok := clientData[previousFocusCallbacksKey].(previousFocusCallbacks); ok {
		focusPanel.GainedFocusCallback = previous.GainedFocusCallback
		focusPanel.LostFocusCallback = previous.LostFocusCallback
	}
	borderPanel.SetBorder(nil)
	delete(clientData, previousFocusCallbacksKey)
}

// InstallDefaultFieldBorder installs the default field border on the borderTarget and chains into the focus handling of
// the focusTarget to adjust the border as focus changes. When the focusTarget, or failing that the borderTarget, is or
// embeds a Field, the border's line is drawn in that field's ErrorInk while the field is invalid.
func InstallDefaultFieldBorder(focusTarget, borderTarget Paneler) {
	field := fieldOf(focusTarget)
	if field == nil {
		field = fieldOf(borderTarget)
	}
	InstallFocusBorders(focusTarget, borderTarget, newDefaultFieldBorder(true, field), newDefaultFieldBorder(false, field))
}
