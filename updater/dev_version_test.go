// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package updater

import (
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
)

// A plain `go build` from a git checkout stamps a VCS pseudo-version, which sorts above the latest release, so failing
// to recognize one would send such a build off to check for updates and then report that there were none. The versions
// the release workflow stamps must not be mistaken for development builds, including a pre-release tag, which looks
// like the start of a pseudo-version but isn't one.
func TestIsDevVersionAcceptsDevelopmentBuilds(t *testing.T) {
	c := check.New(t)
	for _, one := range []struct {
		version string
		want    bool
	}{
		{"", true},    // no version at all
		{"0.0", true}, // what a build with no stamped version reports
		{"5.48.1-0.20260826001723-41ebfa03e3e8", true},        // pseudo-version following a release tag
		{"5.48.1-0.20260826001723-41ebfa03e3e8+dirty", true},  // ...with uncommitted changes
		{"v5.48.1-0.20260826001723-41ebfa03e3e8+dirty", true}, // ...with the leading "v" left on
		{"0.0.0-20260826001723-41ebfa03e3e8", true},           // pseudo-version with no prior tag
		{"5.49.0-beta.0.20260826001723-41ebfa03e3e8", true},   // pseudo-version following a pre-release tag
		{"5.48.0+dirty", true},                                // a release tag, but built from a dirty tree
		{"5.48.0", false},
		{"v5.48.0", false},
		{"5.48", false},
		{"5.48.0-beta.1", false},
		{"5.0.0", false},
	} {
		c.Equal(one.want, IsDevVersion(one.version), one.version)
	}
}
