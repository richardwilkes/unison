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
	"github.com/richardwilkes/unison/enums/mod"
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

// TestMacCovered verifies which mouse-moved events a key window drops: those made while the window server finds
// another window, of this application or any other, or none at all, on top at the pointer, which AppKit sends the key
// window along with every other move, and none made over the window itself.
func TestMacCovered(t *testing.T) {
	c := check.New(t)
	prior := macWindowNumberAt
	t.Cleanup(func() { macWindowNumberAt = prior })
	var under int
	var asked []geom.Point
	macWindowNumberAt = func(pt geom.Point) int {
		asked = append(asked, pt)
		return under
	}
	w := &Window{wnd: &apiWindow{number: 7}}
	under = 7
	c.False(w.macCovered(geom.NewPoint(150, 150)), "the window itself is on top")
	c.Equal([]geom.Point{geom.NewPoint(150, 150)}, asked)
	under = 8
	c.True(w.macCovered(geom.NewPoint(150, 150)), "another window is on top")
	under = 0
	c.True(w.macCovered(geom.NewPoint(150, 150)), "no window is under the pointer")
}

// TestMacMenuStackMonitorClosesOutsidePresses verifies that an open menu stack installs the monitor that closes its
// menus on a press the owner is never told about: one in another of the application's windows, in none of its windows
// or in another application's. A press in the owner or in one of its popups is left to them, a stack opened for
// another window takes the monitor over, and only the window the monitor serves takes it down.
func TestMacMenuStackMonitorClosesOutsidePresses(t *testing.T) {
	c := check.New(t)
	prevList, prevInstall, prevRemove := windowList, macInstallMenuStackMonitor, macRemoveMenuStackMonitor
	defer func() {
		windowList, macInstallMenuStackMonitor, macRemoveMenuStackMonitor = prevList, prevInstall, prevRemove
		macMenuStackWindow = nil
	}()
	var monitor func(cocoa.Window)
	var installs, removals int
	macInstallMenuStackMonitor = func(f func(cocoa.Window)) {
		installs++
		monitor = f
	}
	macRemoveMenuStackMonitor = func() {
		removals++
		monitor = nil
	}
	owner := &Window{valid: true, wnd: &apiWindow{wnd: cocoa.Window(7)}}
	owner.root = newRootPanel(owner)
	other := &Window{valid: true, wnd: &apiWindow{wnd: cocoa.Window(9)}}
	other.root = newRootPanel(other)
	mp := newTestMenuPanel(0, 0, 100, 100)
	popup := newTestPopupWindow(owner, mp)
	popup.wnd = &apiWindow{wnd: cocoa.Window(8)}
	windowList = []*Window{owner, popup, other}

	owner.root.insertMenu(mp)
	c.Equal(1, installs, "the first menu installs the monitor")
	c.True(macMenuStackWindow == owner)
	c.NotNil(monitor)
	if monitor == nil {
		return
	}
	for _, macWnd := range []cocoa.Window{7, 8} {
		monitor(macWnd)
		c.Equal(1, len(owner.root.openMenuPanels), "a press in window %d is left to its root panel", macWnd)
	}
	monitor(9)
	c.Equal(0, len(owner.root.openMenuPanels), "a press in another window of the application's closes the menus")
	c.Equal(1, removals, "and the empty stack takes the monitor down")
	c.Nil(macMenuStackWindow)

	owner.root.insertMenu(mp)
	monitor(0)
	c.Equal(0, len(owner.root.openMenuPanels), "a press in no window of the application's closes them too")

	owner.root.insertMenu(mp)
	otherMenu := newTestMenuPanel(0, 0, 100, 100)
	newTestPopupWindow(other, otherMenu)
	other.root.insertMenu(otherMenu)
	c.True(macMenuStackWindow == other, "a stack opened for another window takes the monitor over")
	monitor(7)
	c.Equal(0, len(other.root.openMenuPanels), "whose monitor treats the first window as outside")
	removals = 0
	owner.root.closeMenuStackStoppingAt(nil)
	c.Equal(0, removals, "the window that lost the monitor does not take it down")
	other.root.insertMenu(otherMenu)
	other.root.closeMenuStackStoppingAt(nil)
	c.Equal(1, removals)
}

// TestMacCoveredMoveExitsOnce verifies that a key window, which AppKit sends the mouse-moved events made over the
// windows above it, exits what the pointer left once per crossing, as the crossing events of the other platforms do,
// rather than on every move: a window's MouseExitCallback that keeps the panel it is told about would otherwise be told
// of the same crossing on every move over the other window.
func TestMacCoveredMoveExitsOnce(t *testing.T) {
	c := check.New(t)
	w := &Window{wnd: &apiWindow{}}
	w.root = newRootPanel(w)
	exits := 0
	w.MouseExitCallback = func() bool {
		exits++
		return true
	}
	c.True(w.macCoveredMove(true))
	c.True(w.macCoveredMove(true))
	c.True(w.macCoveredMove(true))
	c.Equal(1, exits, "a run of moves over another window exits once")
	c.False(w.macCoveredMove(false))
	c.Equal(1, exits, "a move back over the window exits nothing")
	c.True(w.macCoveredMove(true))
	c.Equal(2, exits, "the next crossing exits again")
}

// TestMacMouseExitedExitsOnce verifies that a crossing out of the key window exits once, though AppKit reports it both
// through the window's tracking area and through the mouse-moved events it goes on sending the key window over
// whatever the pointer is now over, and that the tracking area's report of the pointer coming back starts a new
// crossing.
func TestMacMouseExitedExitsOnce(t *testing.T) {
	c := check.New(t)
	w := &Window{wnd: &apiWindow{}, cursor: &Cursor{}}
	w.root = newRootPanel(w)
	exits := 0
	w.MouseExitCallback = func() bool {
		exits++
		return true
	}
	w.MouseEnterCallback = func(_ geom.Point, _ mod.Modifiers) bool { return true }
	w.macMouseExited()
	c.True(w.macCoveredMove(true))
	c.True(w.macCoveredMove(true))
	c.Equal(1, exits, "the tracking area's exit and the moves after it exit once")

	w.macMouseEntered(geom.NewPoint(10, 10), 0)
	exits = 0
	c.True(w.macCoveredMove(true))
	w.macMouseExited()
	c.Equal(1, exits, "as do the moves after the pointer came back and the tracking area's exit after them")

	c.False(w.macCoveredMove(false))
	w.macMouseExited()
	c.Equal(2, exits, "a move back over the window starts a new crossing")
}

// TestMacPopupShowsCursorOwnerHid verifies that the cursor typing in a window hid comes back over one of the window's
// popup menu windows, such as a list opened from the keyboard under the resting pointer. AppKit hides the cursor for
// the whole application, and the owner, the pointer not being over its content, leaves it alone when the move over the
// popup restores it, so the popup has to undo the hide as it applies its own cursor. Each hide is undone once, and an
// owner hiding the cursor on purpose hides it again once the pointer is back over it.
func TestMacPopupShowsCursorOwnerHid(t *testing.T) {
	c := check.New(t)
	prevHide, prevShow, prevHidden := macHideCursor, macShowCursor, macCursorHidden
	defer func() { macHideCursor, macShowCursor, macCursorHidden = prevHide, prevShow, prevHidden }()
	macCursorHidden = false
	var hides, shows int
	macHideCursor = func() { hides++ }
	macShowCursor = func() { shows++ }
	// Not focused, so that HideCursor and ShowCursor change only the state, as with the pointer away from the owner;
	// calling nativeUpdateCursorImage stands for the owner applying its cursor with the pointer over its content.
	owner := &Window{valid: true, wnd: &apiWindow{}, cursor: &Cursor{}}
	owner.root = newRootPanel(owner)
	popup := newTestPopupWindow(owner, newTestMenuPanel(0, 0, 100, 100))
	popup.valid = true
	popup.wnd = &apiWindow{}
	popup.cursor = &Cursor{}

	owner.HideCursorUntilMouseMoves()
	owner.nativeUpdateCursorImage()
	c.Equal(1, hides)

	popup.restoreHiddenCursor()
	c.False(owner.cursorHidden, "a move over the popup restores what its owner hid")
	popup.nativeUpdateCursorImage()
	c.Equal(1, shows, "the popup the pointer is over shows the cursor its owner hid")
	popup.nativeUpdateCursorImage()
	owner.nativeUpdateCursorImage()
	c.Equal(1, hides)
	c.Equal(1, shows, "a hide is undone once")

	owner.HideCursor()
	owner.nativeUpdateCursorImage()
	c.Equal(2, hides)
	popup.nativeUpdateCursorImage()
	c.Equal(2, shows, "the cursor shows over a popup of an owner that hides it")
	owner.nativeUpdateCursorImage()
	c.Equal(3, hides, "and is hidden again back over the owner")
}
