// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

// OpenDialog represents a dialog that permits a user to select one or more files or directories.
type OpenDialog interface {
	FileDialog
	// CanChooseFiles returns true if the open dialog is permitted to select files.
	CanChooseFiles() bool
	// SetCanChooseFiles sets whether the open dialog is permitted to select files.
	SetCanChooseFiles(canChoose bool)
	// CanChooseDirectories returns true if the open dialog is permitted to select directories.
	CanChooseDirectories() bool
	// SetCanChooseDirectories sets whether the open dialog is permitted to select directories.
	SetCanChooseDirectories(canChoose bool)
	// ResolvesAliases returns whether the returned paths have been resolved in the case where the selection was an
	// alias.
	ResolvesAliases() bool
	// SetResolvesAliases sets whether the returned paths will be resolved in the case where the selection was an alias.
	SetResolvesAliases(resolves bool)
	// AllowsMultipleSelection returns true if more than one item can be selected.
	AllowsMultipleSelection() bool
	// SetAllowsMultipleSelection sets whether more than one item can be selected.
	SetAllowsMultipleSelection(allow bool)
	// Paths returns the paths that were chosen.
	Paths() []string
}

// NewOpenDialog creates a new open dialog using native support where possible.
func NewOpenDialog() OpenDialog {
	return platformNewOpenDialog()
}
