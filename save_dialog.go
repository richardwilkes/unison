// Copyright ©2021 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

var lastWorkingDir = ""

// SaveDialog represents a dialog that permits a user to select where to save a file.
type SaveDialog interface {
	// InitialDirectory returns a path pointing to the directory the dialog will open up in.
	InitialDirectory() string
	// SetInitialDirectory sets the directory the dialog will open up in.
	SetInitialDirectory(dir string)
	// AllowedExtensions returns the set of permitted file extensions. nil will be returned if all files are allowed.
	AllowedExtensions() []string
	// SetAllowedExtensions sets the permitted file extensions that may be selected. Just the extension is needed, e.g.
	// "txt", not ".txt" or "*.txt", etc. Pass in nil to allow all files.
	SetAllowedExtensions(extensions ...string)
	// RunModal displays the dialog, allowing the user to make a selection. Returns true if successful or false if
	// canceled.
	RunModal() bool
	// Path returns the path that was chosen.
	Path() string
}

// NewSaveDialog creates a new save dialog using native support where possible.
func NewSaveDialog() SaveDialog {
	return platformNewSaveDialog()
}
