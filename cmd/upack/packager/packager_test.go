// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package packager

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	content := []byte("payload")
	if err := os.WriteFile(src, content, 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "sub", "dir", "dst")
	if err := copyFile(src, dst, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, content) {
		t.Errorf("content = %q, want %q", data, content)
	}
	if runtime.GOOS != "windows" {
		fi, statErr := os.Stat(dst)
		if statErr != nil {
			t.Fatal(statErr)
		}
		if fi.Mode().Perm() != 0o755 {
			t.Errorf("mode = %o, want 755", fi.Mode().Perm())
		}
	}
	if err = copyFile(filepath.Join(dir, "missing"), filepath.Join(dir, "dst2"), 0o644); err == nil {
		t.Error("expected an error for a missing source file")
	}

	// A source that cannot be opened must leave the destination untouched. Creating the destination first truncated an
	// existing file to zero even though nothing was ever copied.
	existing := filepath.Join(dir, "existing")
	if err = os.WriteFile(existing, content, 0o644); err != nil {
		t.Fatal(err)
	}
	if err = copyFile(filepath.Join(dir, "missing"), existing, 0o644); err == nil {
		t.Error("expected an error for a missing source file")
	}
	if data, err = os.ReadFile(existing); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, content) {
		t.Errorf("destination content after a failed copy = %q, want %q", data, content)
	}

	// Nor may it leave a newly created empty file and the directories made for it behind.
	fresh := filepath.Join(dir, "fresh", "dst")
	if err = copyFile(filepath.Join(dir, "missing"), fresh, 0o644); err == nil {
		t.Error("expected an error for a missing source file")
	}
	if _, err = os.Stat(filepath.Dir(fresh)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a failed copy created %s", filepath.Dir(fresh))
	}
}
