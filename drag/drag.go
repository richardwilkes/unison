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

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/uti"
	"github.com/richardwilkes/unison/enums/mod"
)

// Op represents the kind of drag operation being performed.
type Op byte

// Possible values for Op.
const (
	Copy Op = 1 << iota
	Move
	None Op = 0
)

// Data stores a data type and its data.
type Data struct {
	Type *uti.DataType
	Data []byte
}

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

// Callbacks holds the callbacks that client code can hook into for drag and drop events.
type Callbacks struct {
	// DragEnteredCallback is called when a drag operation enters the window or panel. The returned drag.Op should be
	// just one of the permitted drag.Op constants, as determined by dragInfo.SourceDragOpMask().
	DragEnteredCallback func(di Info, where geom.Point, mods mod.Modifiers) Op
	// DragUpdatedCallback is called when a drag operation is adjusted while within the window or panel. The returned
	// drag.Op should be just one of the permitted drag.Op constants, as determined by dragInfo.SourceDragOpMask(). For
	// performance reasons, examination of data types and/or the data should be done when DragEnteredCallback() is
	// called and not here, if at all possible. If nil, the result from the DragEnteredCallback will be returned.
	DragUpdatedCallback func(di Info, where geom.Point, mods mod.Modifiers) Op
	// DragExitedCallback is called when a drag operation leaves the window or panel.
	DragExitedCallback func()
	// DropCallback is called when a drag operation is released over the window or panel. Return true if the drop is
	// accepted and false if it is not.
	DropCallback func(di Info, where geom.Point, mods mod.Modifiers) bool
}
