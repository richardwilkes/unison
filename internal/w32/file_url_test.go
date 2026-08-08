// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package w32_test

import (
	"net/url"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/unison/internal/w32"
)

// TestFileURLPreservesURLSignificantCharacters verifies that characters which are legal in a Windows filename but
// significant in a URL survive the conversion. Building the URL by concatenating the raw path into url.Parse() used to
// truncate the path at '#' or '?' (moving the remainder into the fragment or query), decode '%xx' sequences, and fail
// to parse at all for a '%' not followed by two hex digits, which silently dropped the file from the result.
func TestFileURLPreservesURLSignificantCharacters(t *testing.T) {
	c := check.New(t)
	for _, tc := range []struct {
		path     string
		wantPath string
		wantStr  string
	}{
		{path: `C:\dir\file.txt`, wantPath: "/C:/dir/file.txt", wantStr: "file:///C:/dir/file.txt"},
		{path: `C:\dir\file#1.txt`, wantPath: "/C:/dir/file#1.txt", wantStr: "file:///C:/dir/file%231.txt"},
		{path: `C:\dir\what?.txt`, wantPath: "/C:/dir/what?.txt", wantStr: "file:///C:/dir/what%3F.txt"},
		{path: `C:\dir\a%20b.txt`, wantPath: "/C:/dir/a%20b.txt", wantStr: "file:///C:/dir/a%2520b.txt"},
		{path: `C:\dir\100%.txt`, wantPath: "/C:/dir/100%.txt", wantStr: "file:///C:/dir/100%25.txt"},
		{path: `C:\dir\sp ace.txt`, wantPath: "/C:/dir/sp ace.txt", wantStr: "file:///C:/dir/sp%20ace.txt"},
		{path: `C:\dir\café.txt`, wantPath: "/C:/dir/café.txt", wantStr: "file:///C:/dir/caf%C3%A9.txt"},
	} {
		u := w32.FileURL(tc.path)
		c.Equal("file", u.Scheme, tc.path)
		c.Equal(tc.wantPath, u.Path, tc.path)
		c.Equal("", u.Fragment, "no part of the path may become a fragment:", tc.path)
		c.Equal("", u.RawQuery, "no part of the path may become a query:", tc.path)
		c.Equal(tc.wantStr, u.String(), tc.path)

		// The rendered URL must parse back to the same path, which is what a consumer of the URL actually reads.
		parsed, err := url.Parse(u.String())
		c.NoError(err, tc.path)
		c.Equal(tc.wantPath, parsed.Path, "round trip:", tc.path)
	}
}

// TestFileURLUNCPath verifies that a UNC path keeps its leading separators, matching the form produced before the
// escaping fix.
func TestFileURLUNCPath(t *testing.T) {
	c := check.New(t)
	u := w32.FileURL(`\\server\share\file.txt`)
	c.Equal("///server/share/file.txt", u.Path)
	c.Equal("file://///server/share/file.txt", u.String())
}
