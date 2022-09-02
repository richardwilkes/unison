// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/richardwilkes/toolbox/i18n"
	"github.com/richardwilkes/toolbox/xio/fs"
)

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

// ValidateSaveFilePath ensures the given path is should be used to save a file. If requiredExtension isn't empty, this
// function will ensure filePath ends with that extension. If the resulting file already exists, the user will be
// prompted to verify they intend to overwrite the destination. If the approved, the file will be removed.
func ValidateSaveFilePath(filePath, requiredExtension string) (revisedPath string, ok bool) {
	if requiredExtension != "" {
		if !strings.HasPrefix(requiredExtension, ".") {
			requiredExtension = "." + requiredExtension
		}
		if filepath.Ext(filePath) != requiredExtension {
			filePath = fs.TrimExtension(filePath) + requiredExtension
		}
	}
	if fs.FileExists(filePath) {
		if result := QuestionDialog(i18n.Text("File already exists! Do you want to overwrite it?"), filePath); result != ModalResponseOK {
			return "", false
		}
		if err := os.Remove(filePath); err != nil {
			ErrorDialogWithError(i18n.Text("Unable to remove ")+fs.BaseName(filePath), err)
			return "", false
		}
	}
	return filePath, true
}
