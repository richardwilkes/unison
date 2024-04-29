// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

// Border defines methods required of all border providers.
type Border interface {
	// Insets returns the insets describing the space the border occupies on each side.
	Insets() Insets
	// Draw the border into rect.
	Draw(canvas *Canvas, rect Rect)
}

// NewDefaultFieldBorder creates the default border for a field.
func NewDefaultFieldBorder(focused bool) Border {
	color := &PrimaryTheme.Outline
	adj := float32(1)
	if focused {
		adj = 0
		color = &PrimaryTheme.Primary
	}
	return NewCompoundBorder(NewLineBorder(color, 0, NewUniformInsets(2-adj), false),
		NewEmptyBorder(Insets{Top: 2 + adj, Left: 2 + adj, Bottom: 1 + adj, Right: 2 + adj}))
}

// InstallDefaultFieldBorder installs the default field border on the borderTarget and chains into the focus handling of
// the focusTarget to adjust the border as focus changes.
func InstallDefaultFieldBorder(focusTarget, borderTarget Paneler) {
	unfocusedBorder := NewDefaultFieldBorder(false)
	focusedBorder := NewDefaultFieldBorder(true)
	focusPanel := focusTarget.AsPanel()
	savedFocusGainedCallback := focusPanel.GainedFocusCallback
	focusPanel.GainedFocusCallback = func() {
		borderTarget.AsPanel().SetBorder(focusedBorder)
		if savedFocusGainedCallback != nil {
			savedFocusGainedCallback()
		}
	}
	savedFocusLostCallback := focusPanel.LostFocusCallback
	focusPanel.LostFocusCallback = func() {
		borderTarget.AsPanel().SetBorder(unfocusedBorder)
		if savedFocusLostCallback != nil {
			savedFocusLostCallback()
		}
	}
	borderTarget.AsPanel().SetBorder(unfocusedBorder)
}
