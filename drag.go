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
	"net/url"

	"github.com/richardwilkes/toolbox/v2/geom"
)

// DragOp represents the kind of drag operation being performed.
type DragOp byte

// Possible values for DragOp.
const (
	DragOpCopy DragOp = 1 << iota
	DragOpMove
	DragOpNone DragOp = 0
)

// DragInfo contains information about the current drag operation.
type DragInfo interface {
	// SourceDragOpMask returns the allowed DragOp bits that may be set for a destination.
	SourceDragOpMask() DragOp
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

// DragCallbacks holds the callbacks that client code can hook into for drag and drop events.
type DragCallbacks struct {
	// DragEnteredCallback is called when a drag operation enters the window or panel. The returned DragOp should be
	// just one of the permitted DragOp constants, as determined by dragInfo.SourceDragOpMask().
	DragEnteredCallback func(dragInfo DragInfo, where geom.Point, mods Modifiers) DragOp
	// DragUpdatedCallback is called when a drag operation is adjusted while within the window or panel. The returned
	// DragOp should be just one of the permitted DragOp constants, as determined by dragInfo.SourceDragOpMask(). For
	// performance reasons, examination of data types and/or the data should be done when DragEnteredCallback() is
	// called and not here, if at all possible. If nil, the result from the DragEnteredCallback will be returned.
	DragUpdatedCallback func(dragInfo DragInfo, where geom.Point, mods Modifiers) DragOp
	// DragExitedCallback is called when a drag operation leaves the window or panel.
	DragExitedCallback func()
	// DropCallback is called when a drag operation is released over the window or panel. Return true if the drop is
	// accepted and false if it is not.
	DropCallback func(dragInfo DragInfo, where geom.Point, mods Modifiers) bool
}
