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
	"strings"

	"github.com/richardwilkes/toolbox/v2/tid"
	"github.com/richardwilkes/toolbox/v2/uti"
)

// CreatePrivateDataType registers and returns a new data type whose UTI is composed of the "private." prefix, the
// supplied key, and a random, unique suffix. This is intended for data types that should be unique to this instance of
// this application, such as those used for drag & drop operations that only make sense within the application. The key
// should be a dot-separated, reverse-DNS-style string (e.g. "unison.dockable"). The random suffix uses a TID with its
// underscores replaced by hyphens, since underscores are not valid within a UTI (macOS rejects them) while hyphens are.
func CreatePrivateDataType(key string) *uti.DataType {
	suffix := strings.ReplaceAll(string(tid.MustNewTID('z')), "_", "-")
	return uti.Register(&uti.DataType{UTI: "private." + key + "." + suffix})
}
