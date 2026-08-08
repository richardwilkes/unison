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
)

// TestNeedsOverwritePrompt verifies that the overwrite confirmation is skipped only for the exact path the native
// dialog itself prompted about. When the required extension revises the chosen path into a different one, the actual
// target was never presented by the dialog and must be confirmed here, even when the originally chosen path also
// exists, or that target is silently overwritten.
func TestNeedsOverwritePrompt(t *testing.T) {
	c := check.New(t)
	dir := t.TempDir()
	write := func(name string) string {
		path := filepath.Join(dir, name)
		c.NoError(os.WriteFile(path, []byte(name), 0o600))
		return path
	}
	existing := write("existing.txt")
	other := write("other.json")
	missing := filepath.Join(dir, "missing.txt")

	// A target that does not exist is never prompted for.
	c.False(needsOverwritePrompt(missing, missing, false))
	c.False(needsOverwritePrompt(missing, missing, true))
	c.False(needsOverwritePrompt(existing, missing, true), "the revised target is what matters, not the chosen path")

	// The native dialog already asked about the path it presented, unless the caller forces the question.
	c.False(needsOverwritePrompt(existing, existing, false))
	c.True(needsOverwritePrompt(existing, existing, true))

	// Applying the required extension produced a different, existing file that the dialog never asked about. This is
	// the case that used to be skipped merely because the chosen path also existed.
	c.True(needsOverwritePrompt(filepath.Join(dir, "other.txt"), other, false),
		"a revised target the dialog never presented must be confirmed")
	c.True(needsOverwritePrompt(existing, other, false),
		"a revised target must be confirmed even when the originally chosen path exists too")
}
