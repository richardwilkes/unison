// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package mac

import "github.com/ebitengine/purego/objc"

// NSModalResponseOK is AppKit's NSModalResponseOK. Value verified by compiling and running an Objective-C program
// against the SDK (OK=1, Cancel=0; NSModalResponse is 8 bytes).
const NSModalResponseOK = 1

// SavePanel is a handle to an NSSavePanel.
type SavePanel objc.ID

// NewSavePanel returns an owned (+1) save panel.
func NewSavePanel() SavePanel {
	var p objc.ID
	WithPool(func() {
		p = Retain(objc.ID(Cls("NSSavePanel")).Send(Sel("savePanel")))
	})
	return SavePanel(p)
}

// DirectoryURL returns the panel's current directory as a borrowed reference.
func (p SavePanel) DirectoryURL() URL {
	return URL(objc.ID(p).Send(Sel("directoryURL")))
}

// SetDirectoryURL sets the panel's current directory.
func (p SavePanel) SetDirectoryURL(theURL URL) {
	objc.ID(p).Send(Sel("setDirectoryURL:"), objc.ID(theURL))
}

// InitialFileName returns the initial value shown in the panel's file name field.
func (p SavePanel) InitialFileName() (name string) {
	WithPool(func() {
		name = GoStringFromNSString(objc.ID(p).Send(Sel("nameFieldStringValue")))
	})
	return name
}

// SetInitialFileName sets the initial value shown in the panel's file name field.
func (p SavePanel) SetInitialFileName(name string) {
	WithPool(func() {
		objc.ID(p).Send(Sel("setNameFieldStringValue:"), NSStringFromGo(name))
	})
}

// AllowedFileTypes returns the panel's allowed file types as an owned (+1) reference, or 0 if none have been set.
// The old bridge returned a borrowed reference here even though the root dialogs release the result — an
// over-release (and a crash when no types were set) if AllowedExtensions was ever used; retaining before returning
// makes the existing caller contract balanced. This keeps using the API the old bridge used (allowedFileTypes,
// deprecated in favor of the UTType-based allowedContentTypes) so behavior is unchanged.
func (p SavePanel) AllowedFileTypes() Array {
	return Array(Retain(objc.ID(p).Send(Sel("allowedFileTypes"))))
}

// SetAllowedFileTypes sets the panel's allowed file types. The property copies the array, so callers retain
// ownership of what they pass in; a nil (0) array clears the restriction.
func (p SavePanel) SetAllowedFileTypes(types Array) {
	objc.ID(p).Send(Sel("setAllowedFileTypes:"), objc.ID(types))
}

// URL returns the panel's selected URL as a borrowed reference.
func (p SavePanel) URL() URL {
	return URL(objc.ID(p).Send(Sel("URL")))
}

// RunModal presents the panel in an application-modal session and returns true if the user clicked OK.
func (p SavePanel) RunModal() bool {
	return objc.Send[int64](objc.ID(p), Sel("runModal")) == NSModalResponseOK
}
