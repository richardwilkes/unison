// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package drag

import (
	"net/url"
)

// Info contains information about the current drag operation.
type Info interface {
	// SourceDragOpMask returns the allowed drag.Op bits that may be set for a destination.
	SourceDragOpMask() Op
	// DataTypes returns the data types present in the drag.
	DataTypes() []string
	// HasString returns true if the drag contains string data (of type uti.UTF8PlainText.UTI).
	HasString() bool
	// HasFilePaths returns true if the drag contains file paths (of type uti.FilePath.UTI).
	HasFilePaths() bool
	// HasURLs returns true if the drag contains URLs (of type uti.URL.UTI).
	HasURLs() bool
	// HasDataType returns true if the drag contains data of the specified type.
	HasDataType(dataType string) bool
	// Text returns the string data (of type uti.UTF8PlainText.UTI) contained in the drag, if any.
	Text() string
	// FilePaths returns the file paths (of type uti.FilePath.UTI) contained in the drag, if any.
	FilePaths() []string
	// URLs returns the URLs (of type uti.URL.UTI) contained in the drag, if any.
	URLs() []*url.URL
	// Data returns the data for the specified data type contained in the drag, if any.
	Data(dataType string) []byte
}
