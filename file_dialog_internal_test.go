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
	"path/filepath"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
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
