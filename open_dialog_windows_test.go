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
)

// TestFinalizeOpenPathsMultiSelect verifies that multi-select results are passed through unchanged. IFileOpenDialog
// returns one full filesystem path per selected item, so no reassembly is needed. Prior to the fix, the legacy
// GetOpenFileName post-processing ("element 0 is the directory, the rest are bare names") was applied instead, which
// dropped the first selected file and joined the remaining full paths onto the first one, producing garbage like
// `C:\dir\a.txt\C:\dir\b.txt` and recording a file path as the last working dir.
func TestFinalizeOpenPathsMultiSelect(t *testing.T) {
	c := check.New(t)
	in := []string{`C:\dir\a.txt`, `C:\dir\b.txt`, `C:\other\c.txt`}
	paths, dir := w32FinalizeOpenPaths(in, true)
	c.Equal(in, paths)
	c.Equal(`C:\dir`, dir)
}

func TestFinalizeOpenPathsSingleSelection(t *testing.T) {
	c := check.New(t)
	paths, dir := w32FinalizeOpenPaths([]string{`C:\dir\a.txt`}, false)
	c.Equal([]string{`C:\dir\a.txt`}, paths)
	c.Equal(`C:\dir`, dir)

	paths, dir = w32FinalizeOpenPaths([]string{`C:\dir\a.txt`}, true)
	c.Equal([]string{`C:\dir\a.txt`}, paths)
	c.Equal(`C:\dir`, dir)
}

// TestFinalizeOpenPathsExtraResultsWithoutMultiSelect verifies that only the first path is kept when multiple results
// arrive even though multiple selection was not enabled.
func TestFinalizeOpenPathsExtraResultsWithoutMultiSelect(t *testing.T) {
	c := check.New(t)
	paths, dir := w32FinalizeOpenPaths([]string{`C:\dir\a.txt`, `C:\dir\b.txt`}, false)
	c.Equal([]string{`C:\dir\a.txt`}, paths)
	c.Equal(`C:\dir`, dir)
}

func TestFinalizeOpenPathsEmpty(t *testing.T) {
	c := check.New(t)
	paths, dir := w32FinalizeOpenPaths(nil, true)
	c.Equal(0, len(paths))
	c.Equal("", dir)
}
