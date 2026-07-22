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
	"net/url"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/drag"
	"github.com/richardwilkes/unison/enums/mod"
)

// dropTargetTestDragInfo is a minimal drag.Info for exercising drop-target resolution without a live drag session.
type dropTargetTestDragInfo struct{}

func (d *dropTargetTestDragInfo) SourceDragOpMask() drag.Op { return drag.Copy }
func (d *dropTargetTestDragInfo) DataTypes() []string       { return nil }
func (d *dropTargetTestDragInfo) HasString() bool           { return false }
func (d *dropTargetTestDragInfo) HasFilePaths() bool        { return false }
func (d *dropTargetTestDragInfo) HasURLs() bool             { return false }
func (d *dropTargetTestDragInfo) HasDataType(_ string) bool { return false }
func (d *dropTargetTestDragInfo) Text() string              { return "" }
func (d *dropTargetTestDragInfo) FilePaths() []string       { return nil }
func (d *dropTargetTestDragInfo) URLs() []*url.URL          { return nil }
func (d *dropTargetTestDragInfo) Data(_ string) []byte      { return nil }

// newDropTargetTestWindow returns a minimal Window suitable for exercising drop-target resolution without a live
// windowing system, along with a child panel covering the left half of the content area.
func newDropTargetTestWindow() (w *Window, child *Panel) {
	w = &Window{
		wnd:            &apiWindow{},
		glCtx:          &apiGLContext{},
		surface:        &surface{},
		pressedKeys:    make(map[KeyCode]bool),
		pressedButtons: make(map[int]bool),
	}
	w.root = newRootPanel(w)
	w.root.SetFrameRect(geom.NewRect(0, 0, 200, 200))
	w.root.contentPanel.SetFrameRect(geom.NewRect(0, 0, 200, 200))
	child = NewPanel()
	child.SetFrameRect(geom.NewRect(0, 0, 100, 200))
	w.root.contentPanel.AddChild(child)
	return w, child
}

// TestFindDropTargetWithNilCanAcceptDropCallback is the regression test for the documented contract that a panel with
// a DropCallback and no CanAcceptDropCallback is treated as a drop candidate.
func TestFindDropTargetWithNilCanAcceptDropCallback(t *testing.T) {
	c := check.New(t)
	w, child := newDropTargetTestWindow()
	child.DropCallback = func(_ drag.Info, _ geom.Point, _ mod.Modifiers) bool { return true }
	target := w.findDropTarget(&dropTargetTestDragInfo{}, geom.NewPoint(50, 100))
	c.NotNil(target)
	c.True(child.Is(target), "the panel with only a DropCallback must be selected as the drop target")
}

// TestFindDropTargetHonorsCanAcceptDropCallback verifies that an explicit CanAcceptDropCallback still governs
// candidacy: true selects the panel, false declines it so the search continues up the parent hierarchy.
func TestFindDropTargetHonorsCanAcceptDropCallback(t *testing.T) {
	c := check.New(t)
	w, child := newDropTargetTestWindow()
	child.DropCallback = func(_ drag.Info, _ geom.Point, _ mod.Modifiers) bool { return true }
	child.CanAcceptDropCallback = func(_ drag.Info) bool { return true }
	target := w.findDropTarget(&dropTargetTestDragInfo{}, geom.NewPoint(50, 100))
	c.NotNil(target)
	c.True(child.Is(target))

	// Declining must pass the drag to an enclosing candidate — here the content panel, which relies on the
	// nil-callback default.
	child.CanAcceptDropCallback = func(_ drag.Info) bool { return false }
	content := w.root.contentPanel.AsPanel()
	content.DropCallback = func(_ drag.Info, _ geom.Point, _ mod.Modifiers) bool { return true }
	target = w.findDropTarget(&dropTargetTestDragInfo{}, geom.NewPoint(50, 100))
	c.NotNil(target)
	c.True(content.Is(target), "a declined panel must yield to an enclosing panel with a DropCallback")
}

// TestFindDropTargetWithoutDropCallback verifies that panels without a DropCallback are never candidates, regardless
// of the CanAcceptDropCallback default.
func TestFindDropTargetWithoutDropCallback(t *testing.T) {
	c := check.New(t)
	w, _ := newDropTargetTestWindow()
	c.Nil(w.findDropTarget(&dropTargetTestDragInfo{}, geom.NewPoint(50, 100)))
}
