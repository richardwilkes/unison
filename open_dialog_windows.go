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
	"strings"

	"github.com/richardwilkes/toolbox/i18n"
	"github.com/richardwilkes/toolbox/log/jot"
	"github.com/richardwilkes/unison/internal/w32"
)

type winOpenDialog struct {
	fileCommon
}

func platformNewOpenDialog() OpenDialog {
	d := &winOpenDialog{}
	d.initialize()
	return d
}

func (d *winOpenDialog) RunModal() bool {
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
		jot.Error("unable to create open dialog")
		return false
	}
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
	openDialog.SetFileTypes(createFileFilters(d.extensions))
	d.paths = nil
	if !openDialog.Show() {
		return false
	}
	d.paths = openDialog.GetResults()
	switch len(d.paths) {
	case 0:
		return false
	case 1:
		lastWorkingDir = filepath.Dir(d.paths[0])
	default:
		if d.allowMultipleSelection {
			paths := make([]string, len(d.paths)-1)
			for i, p := range d.paths {
				if i != 0 {
					paths[i-1] = filepath.Join(d.paths[0], p)
				}
			}
			d.paths = paths
			lastWorkingDir = d.paths[0]
		} else {
			paths := make([]string, 1)
			paths[0] = d.paths[1]
			d.paths = paths
			lastWorkingDir = filepath.Dir(d.paths[0])
		}
	}
	return true
}

func createFileFilters(extensions []string) []w32.FileFilter {
	filters := make([]w32.FileFilter, 0, len(extensions)+1)
	readable := make([]string, 0, len(extensions))
	for _, ext := range extensions {
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
	for _, ext := range extensions {
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
	if len(extensions) == 0 {
		filters = append(filters, w32.FileFilter{
			Name:    i18n.Text("All Files"),
			Pattern: "*.*",
		})
	}
	return filters
}
