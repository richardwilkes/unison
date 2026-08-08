// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package w32

import (
	"net/url"
	"strings"
)

// FileURL converts a Windows file path into a file:// URL. The path is placed into the URL's Path field rather than
// being concatenated into a string and parsed, so characters that are legal in a filename but significant in a URL are
// carried through intact: '#' and '?' would otherwise cut the path short and reappear as a fragment or query, a literal
// '%' followed by two hex digits would be decoded as an escape, and a '%' followed by anything else would fail to parse
// at all, dropping the URL. The Path field holds the unescaped path; String() escapes it.
func FileURL(path string) *url.URL {
	return &url.URL{Scheme: "file", Path: "/" + strings.ReplaceAll(path, `\`, "/")}
}
