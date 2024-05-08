// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

const previousFocusCallbacksKey = "internal.previous.focus.callbacks"

type previousFocusCallbacks struct {
	GainedFocusCallback func()
	LostFocusCallback   func()
}

// Border defines methods required of all border providers.
type Border interface {
	// Insets returns the insets describing the space the border occupies on each side.
	Insets() Insets
	// Draw the border into rect.
	Draw(canvas *Canvas, rect Rect)
}

// NewDefaultFieldBorder creates the default border for a field.
func NewDefaultFieldBorder(focused bool) Border {
	var adj float32
	var ink Ink
	if focused {
		adj = 0
		ink = &PrimaryTheme.Primary
	} else {
		adj = 1
		ink = PrimaryTheme.Surface.DeriveLightness(-0.1, 0.15)
	}
	return NewCompoundBorder(
		NewLineBorder(ink, 0, NewUniformInsets(2-adj), false),
		NewEmptyBorder(Insets{Top: 2 + adj, Left: 2 + adj, Bottom: 1 + adj, Right: 2 + adj}),
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
		if previous.GainedFocusCallback != nil {
			previous.GainedFocusCallback()
		}
	}
	focusPanel.LostFocusCallback = func() {
		borderPanel.SetBorder(unfocusedBorder)
		if previous.LostFocusCallback != nil {
			previous.LostFocusCallback()
		}
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
// the focusTarget to adjust the border as focus changes.
func InstallDefaultFieldBorder(focusTarget, borderTarget Paneler) {
	InstallFocusBorders(focusTarget, borderTarget, NewDefaultFieldBorder(true), NewDefaultFieldBorder(false))
}
