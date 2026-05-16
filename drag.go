// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import "github.com/richardwilkes/toolbox/v2/geom"

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
	// Source returns the source of the drag data. Will be nil if the drag originated outside of this application.
	Source() any
	// SourceDragOpMask returns the allowed DragOp bits that may be set for a destination.
	SourceDragOpMask() DragOp
	// DataTypes returns the data types present in the drag.
	DataTypes() []string
	// Text returns the string data (of type uti.UTF8PlainText.UTI) contained in the drag, if any.
	Text() string
	// Data returns the data for the specified data type contained in the drag, if any.
	Data(dataType string) []byte
}

// DragCallbacks holds the callbacks that client code can hook into for drag and drop events.
type DragCallbacks struct {
	// DragEnteredCallback is called when a drag operation enters the window or panel. The returned DragOp should be
	// just one of the permitted DragOp constants, as determined by dragInfo.SourceDragOpMask().
	DragEnteredCallback func(where geom.Point, dragInfo DragInfo) DragOp
	// DragUpdatedCallback is called when a drag operation is adjusted while within the window or panel. The returned
	// DragOp should be just one of the permitted DragOp constants, as determined by dragInfo.SourceDragOpMask(). For
	// performance reasons, examination of data types and/or the data should be done when DragEnteredCallback() is
	// called and not here, if at all possible.
	DragUpdatedCallback func(where geom.Point, dragInfo DragInfo) DragOp
	// DragExitedCallback is called when a drag operation leaves the window or panel.
	DragExitedCallback func()
	// DropCallback is called when a drag operation is released over the window or panel. Return true if the drop is
	// accepted and false if it is not.
	DropCallback func(where geom.Point, dragInfo DragInfo) bool
	// DragEndedCallback is called after a drag operation completes, whether a successful drop was made or not.
	DragEndedCallback func()

	// FileDropCallback is called when files are drag & dropped from the OS.
	FileDropCallback func(files []string)
}
