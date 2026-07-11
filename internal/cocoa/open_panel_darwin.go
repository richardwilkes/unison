// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package cocoa

import "github.com/ebitengine/purego/objc"

// OpenPanel is a handle to an NSOpenPanel.
type OpenPanel objc.ID

// NewOpenPanel returns an owned (+1) open panel.
func NewOpenPanel() OpenPanel {
	var p objc.ID
	WithPool(func() {
		p = Retain(objc.ID(Cls("NSOpenPanel")).Send(Sel("openPanel")))
	})
	return OpenPanel(p)
}

// DirectoryURL returns the panel's current directory as a borrowed reference.
func (p OpenPanel) DirectoryURL() URL {
	return URL(objc.ID(p).Send(Sel("directoryURL")))
}

// SetDirectoryURL sets the panel's current directory.
func (p OpenPanel) SetDirectoryURL(theURL URL) {
	objc.ID(p).Send(Sel("setDirectoryURL:"), objc.ID(theURL))
}

// AllowedFileTypes returns the panel's allowed file types as an owned (+1) reference, or 0 if none have been set.
// See SavePanel.AllowedFileTypes for why this retains where the old bridge returned a borrowed reference.
func (p OpenPanel) AllowedFileTypes() Array {
	return Array(Retain(objc.ID(p).Send(Sel("allowedFileTypes"))))
}

// SetAllowedFileTypes sets the panel's allowed file types. The property copies the array, so callers retain
// ownership of what they pass in; a nil (0) array clears the restriction.
func (p OpenPanel) SetAllowedFileTypes(types Array) {
	objc.ID(p).Send(Sel("setAllowedFileTypes:"), objc.ID(types))
}

// CanChooseFiles returns true if the panel allows choosing files.
func (p OpenPanel) CanChooseFiles() bool {
	return objc.Send[bool](objc.ID(p), Sel("canChooseFiles"))
}

// SetCanChooseFiles sets whether the panel allows choosing files.
func (p OpenPanel) SetCanChooseFiles(set bool) {
	objc.ID(p).Send(Sel("setCanChooseFiles:"), set)
}

// CanChooseDirectories returns true if the panel allows choosing directories.
func (p OpenPanel) CanChooseDirectories() bool {
	return objc.Send[bool](objc.ID(p), Sel("canChooseDirectories"))
}

// SetCanChooseDirectories sets whether the panel allows choosing directories.
func (p OpenPanel) SetCanChooseDirectories(set bool) {
	objc.ID(p).Send(Sel("setCanChooseDirectories:"), set)
}

// ResolvesAliases returns true if the panel resolves aliases to their targets.
func (p OpenPanel) ResolvesAliases() bool {
	return objc.Send[bool](objc.ID(p), Sel("resolvesAliases"))
}

// SetResolvesAliases sets whether the panel resolves aliases to their targets.
func (p OpenPanel) SetResolvesAliases(set bool) {
	objc.ID(p).Send(Sel("setResolvesAliases:"), set)
}

// AllowsMultipleSelection returns true if the panel allows selecting more than one entry.
func (p OpenPanel) AllowsMultipleSelection() bool {
	return objc.Send[bool](objc.ID(p), Sel("allowsMultipleSelection"))
}

// SetAllowsMultipleSelection sets whether the panel allows selecting more than one entry.
func (p OpenPanel) SetAllowsMultipleSelection(set bool) {
	objc.ID(p).Send(Sel("setAllowsMultipleSelection:"), set)
}

// URLs returns the panel's selected URLs as a borrowed reference, matching the old bridge (the root dialogs do not
// release the result; in production it is called from the main event loop, whose autorelease pool keeps it alive).
func (p OpenPanel) URLs() Array {
	return Array(objc.ID(p).Send(Sel("URLs")))
}

// RunModal presents the panel in an application-modal session and returns true if the user clicked OK.
func (p OpenPanel) RunModal() bool {
	return objc.Send[int64](objc.ID(p), Sel("runModal")) == NSModalResponseOK
}
