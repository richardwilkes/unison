// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package cocoa

import (
	"net/url"
	"sync"

	"github.com/ebitengine/purego/objc"
	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/unison/drag"
)

// DragInfo is a handle to an object conforming to the NSDraggingInfo protocol, valid only for the duration of the
// dragging-destination callback it was passed to. It is not owned: AppKit manages its lifetime.
type DragInfo objc.ID

var _ drag.Info = DragInfo(0)

// DragOp mirrors AppKit's NSDragOperation (an NSUInteger). The constant values were verified against the macOS SDK
// by compiling and running an Objective-C program.
type DragOp uint64

// Possible DragOp values
const (
	DragOpNone DragOp = 0
	DragOpCopy DragOp = 1
	DragOpMove DragOp = 16
)

// DragOpFromUnison converts a unison drag.Op into the equivalent NSDragOperation mask.
func DragOpFromUnison(op drag.Op) DragOp {
	var nativeOp DragOp
	if op&drag.Copy != 0 {
		nativeOp |= DragOpCopy
	}
	if op&drag.Move != 0 {
		nativeOp |= DragOpMove
	}
	return nativeOp
}

// ToUnisonDragOp converts an NSDragOperation mask into the equivalent unison drag.Op.
func (d DragOp) ToUnisonDragOp() drag.Op {
	var op drag.Op
	if d&DragOpCopy != 0 {
		op |= drag.Copy
	}
	if d&DragOpMove != 0 {
		op |= drag.Move
	}
	return op
}

// fileURLsOnlyKey returns AppKit's NSPasteboardURLReadingFileURLsOnlyKey constant, resolved once.
var fileURLsOnlyKey = sync.OnceValue(func() objc.ID {
	return NSStringConstant("AppKit", "NSPasteboardURLReadingFileURLsOnlyKey")
})

// pasteboard returns the drag's pasteboard.
func (d DragInfo) pasteboard() objc.ID {
	return objc.ID(d).Send(Sel("draggingPasteboard"))
}

// readURLs returns the autoreleased NSArray of NSURLs readable from the given pasteboard, optionally restricted to
// file URLs, mirroring the readObjectsForClasses:options: calls of the old bridge. Callers must provide the
// autorelease pool.
func readURLs(pb objc.ID, fileURLsOnly bool) objc.ID {
	options := objc.ID(Cls("NSDictionary")).Send(Sel("dictionaryWithObject:forKey:"),
		objc.ID(Cls("NSNumber")).Send(Sel("numberWithBool:"), fileURLsOnly), fileURLsOnlyKey())
	return pb.Send(Sel("readObjectsForClasses:options:"), NSArrayFromIDs(objc.ID(Cls("NSURL"))), options)
}

// urlPaths converts an NSArray of NSURLs to the path components of their absolute strings, mirroring the old
// bridge's ArrayOfURLToStringSlice (including its behavior of discarding everything but the path). Callers must
// provide the autorelease pool.
func urlPaths(urls objc.ID) []string {
	ids := IDsFromNSArray(urls)
	result := make([]string, 0, len(ids))
	for _, u := range ids {
		urlStr := GoStringFromNSString(u.Send(Sel("absoluteString")))
		parsed, err := url.Parse(urlStr)
		if err != nil {
			errs.Log(errs.NewWithCause("unable to parse URL", err), "url", urlStr)
			continue
		}
		result = append(result, parsed.Path)
	}
	return result
}

// SourceDragOpMask returns the allowed drag.Op bits that may be set for a destination.
func (d DragInfo) SourceDragOpMask() drag.Op {
	return DragOp(objc.Send[uint64](objc.ID(d), Sel("draggingSourceOperationMask"))).ToUnisonDragOp()
}

// DataTypes returns the data types present in the drag.
func (d DragInfo) DataTypes() (types []string) {
	WithPool(func() {
		ids := IDsFromNSArray(d.pasteboard().Send(Sel("types")))
		types = make([]string, 0, len(ids))
		for _, s := range ids {
			types = append(types, GoStringFromNSString(s))
		}
	})
	return types
}

// HasString returns true if the drag contains string data.
func (d DragInfo) HasString() (has bool) {
	WithPool(func() {
		has = pasteboardContainsType(d.pasteboard(), pasteboardTypeString())
	})
	return has
}

// HasFilePaths returns true if the drag contains file paths.
func (d DragInfo) HasFilePaths() (has bool) {
	WithPool(func() {
		has = NSArrayCount(readURLs(d.pasteboard(), true)) != 0
	})
	return has
}

// HasURLs returns true if the drag contains URLs.
func (d DragInfo) HasURLs() (has bool) {
	WithPool(func() {
		has = NSArrayCount(readURLs(d.pasteboard(), false)) != 0
	})
	return has
}

// HasDataType returns true if the drag contains data of the specified type.
func (d DragInfo) HasDataType(dataType string) (has bool) {
	WithPool(func() {
		has = pasteboardContainsType(d.pasteboard(), NSStringFromGo(dataType))
	})
	return has
}

// Text returns the string data contained in the drag, if any.
func (d DragInfo) Text() (text string) {
	WithPool(func() {
		pb := d.pasteboard()
		if pasteboardContainsType(pb, pasteboardTypeString()) {
			text = GoStringFromNSString(pb.Send(Sel("stringForType:"), pasteboardTypeString()))
		}
	})
	return text
}

// FilePaths returns the file paths contained in the drag, if any. Like the old bridge, the paths come from each
// URL's fileSystemRepresentation.
func (d DragInfo) FilePaths() (paths []string) {
	WithPool(func() {
		ids := IDsFromNSArray(readURLs(d.pasteboard(), true))
		paths = make([]string, 0, len(ids))
		for _, u := range ids {
			paths = append(paths, GoStringFromCString(objc.Send[*byte](u, Sel("fileSystemRepresentation"))))
		}
	})
	return paths
}

// URLs returns the URLs contained in the drag, if any. Matching the old bridge exactly, each URL's absolute string
// is reduced to its path component before being parsed, so the returned URLs carry only paths (no scheme or host).
func (d DragInfo) URLs() []*url.URL {
	var urlStrs []string
	WithPool(func() {
		urlStrs = urlPaths(readURLs(d.pasteboard(), false))
	})
	result := make([]*url.URL, 0, len(urlStrs))
	for _, urlStr := range urlStrs {
		u, err := url.Parse(urlStr)
		if err != nil {
			errs.Log(errs.NewWithCause("unable to parse URL", err), "url", urlStr)
			continue
		}
		result = append(result, u)
	}
	return result
}

// Data returns the data for the specified data type contained in the drag, if any.
func (d DragInfo) Data(dataType string) (data []byte) {
	WithPool(func() {
		pb := d.pasteboard()
		dt := NSStringFromGo(dataType)
		if pasteboardContainsType(pb, dt) {
			data = GoBytesFromNSData(pb.Send(Sel("dataForType:"), dt))
		}
	})
	return data
}
