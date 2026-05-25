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

	"github.com/richardwilkes/unison/internal/mac"
)

var _ DragInfo = &macDragInfo{}

type macDragInfo struct {
	native mac.DragInfo
}

func (di *macDragInfo) SourceDragOpMask() DragOp {
	var op DragOp
	mask := di.native.SourceDragOpMask()
	if mask&mac.DragOpCopy != 0 {
		op |= DragOpCopy
	}
	if mask&mac.DragOpMove != 0 {
		op |= DragOpMove
	}
	return op
}

func (di *macDragInfo) toNativeDragOp(op DragOp) mac.DragOp {
	var nativeOp mac.DragOp
	if op&DragOpCopy != 0 {
		nativeOp |= mac.DragOpCopy
	}
	if op&DragOpMove != 0 {
		nativeOp |= mac.DragOpMove
	}
	return nativeOp & di.native.SourceDragOpMask()
}

func (di *macDragInfo) DataTypes() []string {
	return di.native.DataTypes()
}

func (di *macDragInfo) HasString() bool {
	return di.native.HasString()
}

func (di *macDragInfo) HasFilePaths() bool {
	return di.native.HasFilePaths()
}

func (di *macDragInfo) HasURLs() bool {
	return di.native.HasURLs()
}

func (di *macDragInfo) HasDataType(dataType string) bool {
	return di.native.HasDataType(dataType)
}

func (di *macDragInfo) Text() string {
	return di.native.Text()
}

func (di *macDragInfo) FilePaths() []string {
	return di.native.FilePaths()
}

func (di *macDragInfo) URLs() []*url.URL {
	return di.native.URLs()
}

func (di *macDragInfo) Data(dataType string) []byte {
	return di.native.Data(dataType)
}
