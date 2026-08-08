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
	"os"
	"path/filepath"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/unison/enums/mod"
)

const (
	testDriveRoot = `C:\`
	testUNCRoot   = `\\server\share\`
)

func requireChain(t *testing.T, chain []*parentDirItem, want ...*parentDirItem) {
	t.Helper()
	c := check.New(t)
	c.Equal(len(want), len(chain))
	for i, w := range want {
		c.Equal(w.name, chain[i].name, "name at index", i)
		c.Equal(w.path, chain[i].path, "path at index", i)
	}
}

func TestParentDirChainUnix(t *testing.T) {
	requireChain(t, parentDirChain("/Users/rich", "", "/"),
		&parentDirItem{name: "rich", path: "/Users/rich"},
		&parentDirItem{name: "Users", path: "/Users"},
		&parentDirItem{name: "/", path: "/"},
	)
}

func TestParentDirChainUnixRoot(t *testing.T) {
	requireChain(t, parentDirChain("/", "", "/"),
		&parentDirItem{name: "/", path: "/"},
	)
}

func TestParentDirChainWindowsIncludesDriveRoot(t *testing.T) {
	// For a drive-qualified path, the chain must end with the drive root so it can be navigated to from the popup.
	requireChain(t, parentDirChain(`C:\Users\rich`, "C:", `\`),
		&parentDirItem{name: "rich", path: `C:\Users\rich`},
		&parentDirItem{name: "Users", path: `C:\Users`},
		&parentDirItem{name: testDriveRoot, path: testDriveRoot},
	)
}

func TestParentDirChainWindowsDriveRootOnly(t *testing.T) {
	// A current dir of the drive root itself must produce a single, properly-named entry, not one with an empty name.
	requireChain(t, parentDirChain(testDriveRoot, "C:", `\`),
		&parentDirItem{name: testDriveRoot, path: testDriveRoot},
	)
}

func TestParentDirChainWindowsUNC(t *testing.T) {
	requireChain(t, parentDirChain(`\\server\share\docs\misc`, `\\server\share`, `\`),
		&parentDirItem{name: "misc", path: `\\server\share\docs\misc`},
		&parentDirItem{name: "docs", path: `\\server\share\docs`},
		&parentDirItem{name: testUNCRoot, path: testUNCRoot},
	)
}

func TestParentDirChainWindowsUNCRoot(t *testing.T) {
	requireChain(t, parentDirChain(testUNCRoot, `\\server\share`, `\`),
		&parentDirItem{name: testUNCRoot, path: testUNCRoot},
	)
}

func TestParentDirChainNativeSeparatorMatchesFilepath(t *testing.T) {
	// Sanity check with the native separator and a path assembled via filepath, mirroring how rebuildParentDirs calls
	// this helper.
	dir := filepath.Join(pathSeparator, "a", "b", "c")
	chain := parentDirChain(dir, filepath.VolumeName(dir), pathSeparator)
	requireChain(t, chain,
		&parentDirItem{name: "c", path: filepath.Join(pathSeparator, "a", "b", "c")},
		&parentDirItem{name: "b", path: filepath.Join(pathSeparator, "a", "b")},
		&parentDirItem{name: "a", path: filepath.Join(pathSeparator, "a")},
		&parentDirItem{name: pathSeparator, path: pathSeparator},
	)
}

// newTestFileDialog builds a fileDialog over a throwaway directory containing two subdirectories ("adir" and "zdir")
// and two files ("b.txt" and "c.txt"). os.ReadDir sorts by name, so the rows are adir(0), b.txt(1), c.txt(2) and
// zdir(3), putting a directory both before and after the files so a selection can place its non-choosable item on
// either side of its choosable ones. The content is built for real, but the Dialog is a stand-in holding just an OK
// button and a window with no platform resources, since the handlers under test only need Button(ModalResponseOK) and
// StopModal.
func newTestFileDialog(t *testing.T, forOpen bool) *fileDialog {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"adir", "zdir"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"b.txt", "c.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	d := &fileDialog{forOpen: forOpen}
	d.initialize()
	d.initialDir = dir
	d.createContent()
	wnd := newRedrawTestWindow()
	wnd.inModal = true
	wnd.modalResultCode = ModalResponseCancel
	d.dialog = &Dialog{
		wnd: wnd,
		buttons: map[int]*buttonData{
			ModalResponseOK: {info: NewOKButtonInfo(), button: NewButton()},
		},
	}
	d.fileList.FlashAnimationTime = 0
	return d
}

// TestFileDialogSelectionRequiresEveryItemChoosable verifies that a non-choosable item anywhere in a multiple
// selection keeps the OK button disabled. The enabled flag used to be reset to true at the top of every loop
// iteration, so a later valid item silently re-enabled OK for a selection that also held a directory the dialog was
// not allowed to choose, and that directory's path came back from Paths().
func TestFileDialogSelectionRequiresEveryItemChoosable(t *testing.T) {
	c := check.New(t)
	d := newTestFileDialog(t, true)
	d.allowMultipleSelection = true
	d.canChooseDirs = false
	ok := d.dialog.Button(ModalResponseOK)

	// A directory alone is not choosable.
	d.fileList.Selection.Set(0)
	d.fileListSelectionHandler()
	c.False(ok.Enabled())

	// Adding a valid file after it must not re-enable OK, even though that file is processed last.
	d.fileList.Selection.Set(2)
	d.fileListSelectionHandler()
	c.False(ok.Enabled(), "a non-choosable directory earlier in the selection must keep OK disabled")

	// Files alone are choosable.
	d.fileList.Selection.Reset()
	d.fileList.Selection.Set(1)
	d.fileList.Selection.Set(2)
	d.fileListSelectionHandler()
	c.True(ok.Enabled())
	c.Equal([]string{filepath.Join(d.currentDir, "b.txt"), filepath.Join(d.currentDir, "c.txt")}, d.Paths())

	// An empty selection leaves nothing to accept.
	d.fileList.Selection.Reset()
	d.fileListSelectionHandler()
	c.False(ok.Enabled())

	// With directories allowed, the same mixed selection is fine.
	d.canChooseDirs = true
	d.fileList.Selection.Set(0)
	d.fileList.Selection.Set(2)
	d.fileListSelectionHandler()
	c.True(ok.Enabled())
}

// TestFileDialogSelectionOfUnchoosableFilesStaysDisabled verifies the mirror case, where files are the non-choosable
// kind and a directory follows them in the selection.
func TestFileDialogSelectionOfUnchoosableFilesStaysDisabled(t *testing.T) {
	c := check.New(t)
	d := newTestFileDialog(t, true)
	d.allowMultipleSelection = true
	d.canChooseFiles = false
	d.canChooseDirs = true
	ok := d.dialog.Button(ModalResponseOK)

	// Row 1 is a file, which may not be chosen, and row 3 is a directory, which may. The non-choosable item is
	// processed first here, so its veto has to survive the choosable item that follows it.
	d.fileList.Selection.Set(1)
	d.fileList.Selection.Set(3)
	d.fileListSelectionHandler()
	c.False(ok.Enabled(), "a non-choosable file earlier in the selection must keep OK disabled")

	// The same pair with the choosable item first must also stay disabled.
	d.fileList.Selection.Reset()
	d.fileList.Selection.Set(0)
	d.fileList.Selection.Set(1)
	d.fileListSelectionHandler()
	c.False(ok.Enabled())

	// Directories alone are choosable.
	d.fileList.Selection.Reset()
	d.fileList.Selection.Set(0)
	d.fileList.Selection.Set(3)
	d.fileListSelectionHandler()
	c.True(ok.Enabled())
}

// TestFileDialogSaveReturnKeyRecomputesPaths verifies that Return in the save dialog's name field rebuilds the paths
// from the field and the directory currently showing. It used to accept on the strength of a non-empty name field
// alone, so navigation and list selection could leave RunModal reporting success with a path in the previous
// directory, a directory itself, or nothing at all.
func TestFileDialogSaveReturnKeyRecomputesPaths(t *testing.T) {
	c := check.New(t)

	// Navigating with the parent-dir popup leaves the paths pointing into the directory that was showing before.
	d := newTestFileDialog(t, false)
	base := d.currentDir
	sub := filepath.Join(base, "adir")
	d.fileNameField.SetText("hello.txt")
	c.Equal(filepath.Join(base, "hello.txt"), d.Path())
	d.changeDirTo(sub)
	c.Equal(filepath.Join(base, "hello.txt"), d.Path(), "a stale path is the precondition for this test")
	c.True(d.fileNameFieldKeyDown(KeyReturn, mod.None, false))
	c.Equal(filepath.Join(sub, "hello.txt"), d.Path())
	c.False(d.dialog.wnd.inModal, "the modal should have been stopped")
	c.Equal(ModalResponseOK, d.dialog.wnd.modalResultCode)

	// Double-clicking into a directory clears the paths entirely.
	d = newTestFileDialog(t, false)
	base = d.currentDir
	d.fileNameField.SetText("hello.txt")
	d.fileList.Selection.Set(0)
	d.fileListDoubleClickHandler()
	c.Equal("", d.Path(), "a cleared path is the precondition for this test")
	c.True(d.fileNameFieldKeyDown(KeyReturn, mod.None, false))
	c.Equal(filepath.Join(base, "adir", "hello.txt"), d.Path())
	c.False(d.dialog.wnd.inModal, "the modal should have been stopped")

	// Selecting a directory in the list leaves the paths pointing at that directory.
	d = newTestFileDialog(t, false)
	base = d.currentDir
	d.fileNameField.SetText("hello.txt")
	d.fileList.Selection.Set(0)
	d.fileListSelectionHandler()
	c.Equal(filepath.Join(base, "adir"), d.Path(), "a directory path is the precondition for this test")
	c.False(d.dialog.Button(ModalResponseOK).Enabled())
	c.True(d.fileNameFieldKeyDown(KeyReturn, mod.None, false))
	c.Equal(filepath.Join(base, "hello.txt"), d.Path())
	c.False(d.dialog.wnd.inModal, "the modal should have been stopped")
}

// TestFileDialogSaveReturnKeyWithEmptyNameDoesNotAccept verifies that Return with an empty name field leaves the modal
// running rather than stopping it with a success the OK button itself would refuse to give.
func TestFileDialogSaveReturnKeyWithEmptyNameDoesNotAccept(t *testing.T) {
	c := check.New(t)
	d := newTestFileDialog(t, false)
	d.fileNameField.SetText("")
	c.False(d.dialog.Button(ModalResponseOK).Enabled())
	c.True(d.fileNameFieldKeyDown(KeyReturn, mod.None, false))
	c.True(d.dialog.wnd.inModal, "the modal must not have been stopped")
	c.Equal(ModalResponseCancel, d.dialog.wnd.modalResultCode)
}
