// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/xos"
)

// TestFindRepoRoot verifies that the root is recognized by the module its go.mod declares, whatever the directory is
// called, and that a directory holding some other module, or none, is refused.
func TestFindRepoRoot(t *testing.T) {
	c := check.New(t)
	root := filepath.Join(t.TempDir(), "any-name")
	sub := filepath.Join(root, "cmd", "enumgen")
	c.NoError(os.MkdirAll(sub, 0o750))
	c.NoError(os.WriteFile(filepath.Join(root, "go.mod"), []byte("module "+modulePath+"\n\ngo 1.27.0\n"), 0o640))

	found, err := findRepoRoot(root)
	c.NoError(err)
	c.Equal(root, found)

	t.Chdir(sub)
	found, err = findRepoRoot("")
	c.NoError(err)
	c.Equal(root, found)

	other := t.TempDir()
	c.NoError(os.WriteFile(filepath.Join(other, "go.mod"), []byte("module example.com/other\n"), 0o640))
	_, err = findRepoRoot(other)
	c.HasError(err)
	_, err = findRepoRoot(t.TempDir())
	c.HasError(err)
}

// TestRemoveExistingGenFilesLeavesOtherCheckoutsAlone verifies that only this tree's generated files are removed, and
// not those of a worktree or a nested module kept inside it.
func TestRemoveExistingGenFilesLeavesOtherCheckoutsAlone(t *testing.T) {
	c := check.New(t)
	root := t.TempDir()
	write := func(rel string) string {
		p := filepath.Join(root, filepath.FromSlash(rel))
		c.NoError(os.MkdirAll(filepath.Dir(p), 0o750))
		c.NoError(os.WriteFile(p, nil, 0o640))
		return p
	}
	ours := write("enums/one" + genSuffix)
	worktree := write(".claude/worktrees/w/enums/one" + genSuffix)
	write("nested/go.mod")
	nested := write("nested/enums/one" + genSuffix)

	removeExistingGenFiles(root)
	c.False(xos.FileExists(ours))
	c.True(xos.FileExists(worktree))
	c.True(xos.FileExists(nested))
}
