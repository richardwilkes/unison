// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package cocoa

// This file provides the exported CoreFoundation-flavored wrapper types (Array, String, URL) that the cgo bridge
// exposed to the root package's open/save dialogs. They are now backed by the toll-free-bridged Foundation classes
// (NSArray, NSString, NSURL) through objc_msgSend, with the ownership discipline of the old bridge preserved:
// constructors return owned (+1) references, index accessors return borrowed references, and Release balances one
// owned reference. One deliberate difference: operating on a nil handle is now a harmless no-op yielding zero values
// (Objective-C nil-messaging semantics), where the CF functions the old bridge used (CFRelease, CFArrayGetCount, ...)
// crashed on NULL.

import (
	"net/url"
	"strings"

	"github.com/ebitengine/purego/objc"
	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/xos"
)

// Array is a handle to an NSArray (toll-free bridged with the CFArray the cgo bridge used).
type Array objc.ID

// NewArrayFromStringSlice returns an owned (+1) array containing the contents of slice as NSStrings.
func NewArrayFromStringSlice(slice []string) Array {
	a := objc.ID(Cls("NSMutableArray")).Send(Sel("alloc")).Send(Sel("initWithCapacity:"), uint64(len(slice)))
	for _, s := range slice {
		str := NewNSString(s)
		a.Send(Sel("addObject:"), str)
		Release(str)
	}
	return Array(a)
}

// Count returns the number of elements in the array. A nil array yields 0.
func (a Array) Count() int {
	return int(NSArrayCount(objc.ID(a)))
}

// URLAtIndex returns the URL at the given index as a borrowed reference.
func (a Array) URLAtIndex(index int) URL {
	return URL(NSArrayObjectAt(objc.ID(a), uint64(index)))
}

// StringAtIndex returns the string at the given index as a borrowed reference.
func (a Array) StringAtIndex(index int) String {
	return String(NSArrayObjectAt(objc.ID(a), uint64(index)))
}

// Release releases one owned reference to the array.
func (a Array) Release() {
	Release(objc.ID(a))
}

// ArrayOfURLToStringSlice returns the paths of an array of URLs. Matching the cgo bridge, each URL's absolute string
// is parsed as a URL and reduced to its path component, so any scheme/host information a non-file URL carries is
// discarded.
func (a Array) ArrayOfURLToStringSlice() []string {
	count := a.Count()
	result := make([]string, 0, count)
	for i := range count {
		urlStr := a.URLAtIndex(i).AbsoluteString()
		u, err := url.Parse(urlStr)
		if err != nil {
			errs.Log(errs.NewWithCause("unable to parse URL", err), "url", urlStr)
			continue
		}
		result = append(result, u.Path)
	}
	return result
}

// ArrayOfStringToStringSlice returns the contents of an array of NSStrings as a Go string slice.
func (a Array) ArrayOfStringToStringSlice() []string {
	count := a.Count()
	result := make([]string, 0, count)
	for i := range count {
		result = append(result, a.StringAtIndex(i).String())
	}
	return result
}

// String is a handle to an NSString (toll-free bridged with the CFString the cgo bridge used).
type String objc.ID

// NewString returns an owned (+1) string with the contents of str.
func NewString(str string) String {
	return String(NewNSString(str))
}

// String returns the contents as a Go string. A nil handle yields "".
func (s String) String() (str string) {
	WithPool(func() {
		str = GoStringFromNSString(objc.ID(s))
	})
	return str
}

// Release releases one owned reference to the string.
func (s String) Release() {
	Release(objc.ID(s))
}

// URL is a handle to an NSURL (toll-free bridged with the CFURL the cgo bridge used).
type URL objc.ID

// NewFileURL returns an owned (+1) file URL for the given file system path. The path is treated as a directory when
// it ends in a path separator or names an existing directory, mirroring the cgo bridge's
// CFURLCreateFromFileSystemRepresentation call. fileURLWithFileSystemRepresentation:isDirectory:relativeToURL: is
// that function's documented NSURL counterpart, and was verified (by compiling and running an Objective-C program
// against the SDK) to produce byte-identical absolute URL strings for plain, space-containing, CJK, trailing-slash,
// and directory paths.
func NewFileURL(str string) URL {
	isDir := strings.HasSuffix(str, "/") || xos.IsDir(str)
	var u objc.ID
	WithPool(func() {
		u = Retain(objc.ID(Cls("NSURL")).Send(
			Sel("fileURLWithFileSystemRepresentation:isDirectory:relativeToURL:"), str, isDir, objc.ID(0)))
	})
	return URL(u)
}

// AbsoluteString returns the URL in absolute form as a string. A nil handle yields "".
func (u URL) AbsoluteString() (s string) {
	WithPool(func() {
		s = GoStringFromNSString(objc.ID(u).Send(Sel("absoluteString")))
	})
	return s
}

// Release releases one owned reference to the URL.
func (u URL) Release() {
	Release(objc.ID(u))
}
