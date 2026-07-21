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
	"path/filepath"
	"strings"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/i18n"
	"github.com/richardwilkes/unison/internal/w32"
)

var _ OpenDialog = &w32OpenDialog{}

type w32OpenDialog struct {
	fileCommon
}

func apiNewOpenDialog() OpenDialog {
	d := &w32OpenDialog{}
	d.initialize()
	return d
}

func (d *w32OpenDialog) RunModal() bool {
	active := ActiveWindow()
	if active != nil {
		active.restoreHiddenCursor()
	}
	defer func() {
		if active != nil && active.IsVisible() {
			active.ToFront()
		}
	}()

	openDialog := w32.NewOpenDialog()
	if openDialog == nil {
		errs.Log(errs.New("unable to create open dialog"))
		return false
	}
	// NewOpenDialog hands over a +1 COM reference; without this release, every RunModal leaked the dialog object and
	// its shell state for the life of the process.
	defer openDialog.Release()
	if d.initialDir != "" {
		openDialog.SetFolder(filepath.Clean(d.initialDir))
	}
	options := w32.FOSPathMustExist | w32.FOSFileMustExist
	if d.canChooseDirs {
		options |= w32.FOSPickFolders
	}
	if d.allowMultipleSelection {
		options |= w32.FOSAllowMultiSelect
	}
	if !d.resolvesAliases {
		options |= w32.FOSNoDereferenceLinks
	}
	openDialog.SetOptions(options)
	openDialog.SetFileTypes(d.w32CreateFilters())
	d.paths = nil
	if !openDialog.Show(w32OwnerHWND()) {
		return false
	}
	var dir string
	d.paths, dir = w32FinalizeOpenPaths(openDialog.GetResults(), d.allowMultipleSelection)
	if len(d.paths) == 0 {
		return false
	}
	lastWorkingDir = dir
	return true
}

// w32FinalizeOpenPaths normalizes the results of IFileOpenDialog.GetResults, which yields one full filesystem path
// (SIGDN_FILESYSPATH) per selected item -- unlike the legacy GetOpenFileName API, whose multi-select buffer held the
// directory followed by bare file names. If multiple selection is not enabled, only the first path is kept. It returns
// the paths to report along with the directory to record as the last working dir, or (nil, "") if there were no
// results.
func w32FinalizeOpenPaths(paths []string, allowMultipleSelection bool) (finalPaths []string, workingDir string) {
	if len(paths) == 0 {
		return nil, ""
	}
	if !allowMultipleSelection && len(paths) > 1 {
		paths = paths[:1]
	}
	return paths, filepath.Dir(paths[0])
}

func (d *w32OpenDialog) w32CreateFilters() []w32.FileFilter {
	filters := make([]w32.FileFilter, 0, len(d.extensions)+1)
	readable := make([]string, 0, len(d.extensions))
	for _, ext := range d.extensions {
		if ext != "*" {
			readable = append(readable, "*."+ext)
		}
	}
	if len(readable) != 0 {
		filters = append(filters, w32.FileFilter{
			Name:    i18n.Text("All Readable Files"),
			Pattern: strings.Join(readable, ";"),
		})
	}
	for _, ext := range d.extensions {
		if ext == "*" {
			filters = append(filters, w32.FileFilter{
				Name:    i18n.Text("All Files"),
				Pattern: "*.*",
			})
		} else {
			filters = append(filters, w32.FileFilter{
				Name:    ext + i18n.Text(" Files"),
				Pattern: "*." + ext,
			})
		}
	}
	if len(d.extensions) == 0 {
		filters = append(filters, w32.FileFilter{
			Name:    i18n.Text("All Files"),
			Pattern: "*.*",
		})
	}
	return filters
}
