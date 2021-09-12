// Copyright ©2021 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

// ResolveFileURL converts bytes into a URL. On most platforms, this is just a simple string() cast. However, macOS has
// a file reference URL type that needs special handling to resolve properly.
func ResolveFileURL(in []byte) string {
	return platformResolveFileURL(string(in))
}
