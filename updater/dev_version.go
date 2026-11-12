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
	"regexp"
	"strings"
)

// pseudoVersionRE matches a Go VCS pseudo-version in all three forms the toolchain generates: no prior tag, a prior
// release tag, and a prior pre-release tag. Adapted from the one in golang.org/x/mod, which isn't a dependency here,
// with the leading "v" made optional because callers may have already trimmed it.
var pseudoVersionRE = regexp.MustCompile(`^v?[0-9]+\.(0\.0-|\d+\.\d+-([^+]*\.)?0\.)\d{14}-[A-Za-z0-9]+(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$`)

// IsDevVersion reports whether a version string identifies a development build rather than a release: an empty version,
// the "0.0" an unstamped build reports, a Go VCS pseudo-version (what `go build` stamps from a checkout that isn't
// exactly on a release tag, e.g. 5.48.1-0.20260826001723-41ebfa03e3e8), or any version carrying build metadata such as
// the "+dirty" Go appends for an uncommitted tree. Releases are built from a clean, tagged checkout with the version
// passed explicitly, so none of these ever describe one.
func IsDevVersion(version string) bool {
	return version == "" || version == "0.0" || strings.Contains(version, "+") || pseudoVersionRE.MatchString(version)
}
