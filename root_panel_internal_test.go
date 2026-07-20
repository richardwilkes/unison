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
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
)

// newContentTestWindow returns a minimal Window suitable for exercising content handling without a live windowing
// system. The window deliberately reports itself as invalid so layout and redraw requests skip the platform calls.
func newContentTestWindow() *Window {
	w := &Window{}
	w.root = newRootPanel(w)
	return w
}

// TestSetContentNilSupported verifies that clearing the window content with SetContent(nil) neither panics in
// setContent nor in the subsequent layout of a window with no content, and that content can be installed again
// afterwards.
func TestSetContentNilSupported(t *testing.T) {
	c := check.New(t)
	w := newContentTestWindow()
	c.NotNil(w.Content())
	w.SetContent(nil)
	c.Nil(w.Content())
	// Sizing and layout must tolerate the missing content panel.
	minSize, prefSize, maxSize := w.root.Sizes(geom.Size{})
	c.Equal(geom.Size{}, minSize)
	c.Equal(geom.Size{}, prefSize)
	c.Equal(geom.Size{}, maxSize)
	w.root.SetFrameRect(geom.NewRect(0, 0, 100, 100))
	w.root.ValidateLayout()
	panel := NewPanel()
	w.SetContent(panel)
	c.Equal(panel, w.Content())
}
