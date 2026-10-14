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
	"github.com/richardwilkes/unison/enums/thememode"
	"github.com/richardwilkes/unison/internal/cocoa"
)

// TestMacAppearanceNameFor verifies the application appearance each theme mode asks for: an explicit choice names the
// matching appearance, and Auto names none, so that the application goes back to following the system.
func TestMacAppearanceNameFor(t *testing.T) {
	c := check.New(t)
	c.Equal(cocoa.AppearanceNameDarkAqua, macAppearanceNameFor(thememode.Dark))
	c.Equal(cocoa.AppearanceNameAqua, macAppearanceNameFor(thememode.Light))
	c.Equal("", macAppearanceNameFor(thememode.Auto))
}

// TestMacDragImageAndFrameNilImage is the regression test for nativeStartDrag panicking on a nil drag image.
// Window.StartDrag and Panel.StartDrag both document that the image may be nil, and the Linux and Windows
// implementations guard for it, but the macOS path dereferenced it unconditionally. A nil image must yield nil pixel
// data and a minimal 1x1 frame anchored at the origin so the drag proceeds without an image, matching the other
// platforms.
func TestMacDragImageAndFrameNilImage(t *testing.T) {
	c := check.New(t)
	origin := geom.NewPoint(17, 42)
	nrgba, r := macDragImageAndFrame(nil, origin)
	c.Nil(nrgba)
	c.Equal(geom.Rect{Point: origin, Size: geom.NewSize(1, 1)}, r)
}

// TestMacDragImageAndFrameWithImage verifies the normal path: a valid image produces its pixel data and a frame whose
// size is the image's logical (scaled) size, anchored at the drag origin.
func TestMacDragImageAndFrameWithImage(t *testing.T) {
	c := check.New(t)
	const w, h = 4, 2
	img, err := NewImageFromPixels(w, h, distinctPixels(w, h, 47), geom.NewPoint(2, 2))
	c.NoError(err)
	c.NotNil(img)
	origin := geom.NewPoint(5, 9)
	nrgba, r := macDragImageAndFrame(img, origin)
	c.NotNil(nrgba)
	c.Equal(w, nrgba.Rect.Dx())
	c.Equal(h, nrgba.Rect.Dy())
	c.Equal(geom.Rect{Point: origin, Size: img.LogicalSize()}, r)
}

// TestMacDragSourceFinishedForDisposedSource verifies the report AppKit makes when a drag ends after its source window
// was disposed, as happens when the drop closes the source. The window is no longer in the window list, so the lookup
// by native window fails; the drag still has to be wound up, running the cleanup the source registered and no longer
// recording a drag as in progress, and a second report of the same end must not run the cleanup again.
func TestMacDragSourceFinishedForDisposedSource(t *testing.T) {
	c := check.New(t)
	prevList, prevSource := windowList, dragSource
	windowList, dragSource = nil, nil
	defer func() { windowList, dragSource = prevList, prevSource }()

	// No drag in progress: an unknown window is only logged, and nothing else happens.
	macDragSourceFinished(cocoa.Window(1))
	c.Nil(dragSource)

	cleanups := 0
	w := &Window{dragSourceCleanup: func() { cleanups++ }}
	dragSource = w
	macDragSourceFinished(cocoa.Window(1))
	c.Equal(1, cleanups, "the disposed source's cleanup should have run when its drag ended")
	c.Nil(dragSource, "the drag should no longer be recorded as in progress")
	c.Nil(w.dragSourceCleanup, "a cleanup that has run should not be kept to run again")

	macDragSourceFinished(cocoa.Window(1))
	c.Equal(1, cleanups, "a second report of the same end should not run the cleanup again")
}

// TestMacFrameMouseDownClosesMenus verifies that a press on a window's title bar or frame closes the window's open
// menus, and that one on a window that is not in the list is ignored.
func TestMacFrameMouseDownClosesMenus(t *testing.T) {
	c := check.New(t)
	prevList := windowList
	defer func() { windowList = prevList }()
	owner := &Window{wnd: &apiWindow{wnd: cocoa.Window(7)}}
	owner.root = newRootPanel(owner)
	windowList = []*Window{owner}
	mp := newTestMenuPanel(0, 0, 100, 100)
	newTestPopupWindow(owner, mp)
	owner.root.insertMenu(mp)
	macFrameMouseDown(cocoa.Window(8))
	c.Equal(1, len(owner.root.openMenuPanels), "a press on an unknown window leaves the menus alone")
	macFrameMouseDown(cocoa.Window(7))
	c.Equal(0, len(owner.root.openMenuPanels))
	c.Nil(mp.menu.popupPanel)
}
