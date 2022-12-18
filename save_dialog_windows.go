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
	"path/filepath"

	"github.com/richardwilkes/toolbox/i18n"
	"github.com/richardwilkes/toolbox/log/jot"
	"github.com/richardwilkes/unison/internal/w32"
)

type winSaveDialog struct {
	fileCommon
}

func platformNewSaveDialog() SaveDialog {
	d := &winSaveDialog{}
	d.initialize()
	return d
}

func (d *winSaveDialog) RunModal() bool {
	active := ActiveWindow()
	if active != nil {
		active.restoreHiddenCursor()
	}
	defer func() {
		if active != nil && active.IsVisible() {
			active.ToFront()
		}
	}()

	saveDialog := w32.NewSaveDialog()
	if saveDialog == nil {
		jot.Error("unable to create save dialog")
		return false
	}
	if d.initialDir != "" {
		saveDialog.SetFolder(filepath.Clean(d.initialDir))
	}
	options := w32.FOSOverwritePrompt | w32.FOSPathMustExist | w32.FOSNoTestFileCreate
	if d.canChooseDirs {
		options |= w32.FOSPickFolders
	}
	if d.allowMultipleSelection {
		options |= w32.FOSAllowMultiSelect
	}
	if !d.resolvesAliases {
		options |= w32.FOSNoDereferenceLinks
	}
	saveDialog.SetOptions(options)
	saveDialog.SetFileTypes(d.createFilters())
	for _, ext := range d.extensions {
		if ext != "*" {
			saveDialog.SetDefaultExtension(ext)
			break
		}
	}
	d.paths = nil
	if !saveDialog.Show() {
		return false
	}
	result := saveDialog.GetResult()
	if result == "" {
		return false
	}
	d.paths = []string{result}
	lastWorkingDir = filepath.Dir(d.paths[0])
	return true
}

func (d *winSaveDialog) createFilters() []w32.FileFilter {
	filters := make([]w32.FileFilter, 0, len(d.extensions))
	for _, ext := range d.extensions {
		filters = append(filters, w32.FileFilter{
			Name:    ext + i18n.Text(" Files"),
			Pattern: "*." + ext,
		})
	}
	return filters
}
