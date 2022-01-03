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
	"net/url"

	"github.com/richardwilkes/toolbox/errs"
	"github.com/richardwilkes/toolbox/log/jot"
	"github.com/richardwilkes/unison/internal/ns"
)

type macOpenDialog struct {
	dialog ns.OpenPanel
}

func platformNewOpenDialog() OpenDialog {
	return &macOpenDialog{dialog: ns.NewOpenPanel()}
}

func (d *macOpenDialog) InitialDirectory() string {
	u, err := url.Parse(d.dialog.DirectoryURL().AbsoluteString())
	if err != nil {
		jot.Warn(errs.NewWithCause("unable to parse directory URL", err))
		return ""
	}
	return u.Path
}

func (d *macOpenDialog) SetInitialDirectory(dir string) {
	dirURL := ns.NewFileURL(dir)
	defer dirURL.Release()
	d.dialog.SetDirectoryURL(dirURL)
}

func (d *macOpenDialog) AllowedExtensions() []string {
	allowed := d.dialog.AllowedFileTypes()
	defer allowed.Release()
	return allowed.ArrayOfStringToStringSlice()
}

func (d *macOpenDialog) SetAllowedExtensions(types ...string) {
	types = SanitizeExtensionList(types)
	if len(types) != 0 {
		d.dialog.SetAllowedFileTypes(ns.NewArrayFromStringSlice(types))
	} else {
		d.dialog.SetAllowedFileTypes(0)
	}
}

func (d *macOpenDialog) CanChooseFiles() bool {
	return d.dialog.CanChooseFiles()
}

func (d *macOpenDialog) SetCanChooseFiles(canChoose bool) {
	d.dialog.SetCanChooseFiles(canChoose)
}

func (d *macOpenDialog) CanChooseDirectories() bool {
	return d.dialog.CanChooseDirectories()
}

func (d *macOpenDialog) SetCanChooseDirectories(canChoose bool) {
	d.dialog.SetCanChooseDirectories(canChoose)
}

func (d *macOpenDialog) ResolvesAliases() bool {
	return d.dialog.ResolvesAliases()
}

func (d *macOpenDialog) SetResolvesAliases(resolves bool) {
	d.dialog.SetResolvesAliases(resolves)
}

func (d *macOpenDialog) AllowsMultipleSelection() bool {
	return d.dialog.AllowsMultipleSelection()
}

func (d *macOpenDialog) SetAllowsMultipleSelection(allow bool) {
	d.dialog.SetAllowsMultipleSelection(allow)
}

func (d *macOpenDialog) Path() string {
	paths := d.Paths()
	if len(paths) == 0 {
		return ""
	}
	return paths[0]
}

func (d *macOpenDialog) Paths() []string {
	return d.dialog.URLs().ArrayOfURLToStringSlice()
}

func (d *macOpenDialog) RunModal() bool {
	active := ActiveWindow()
	result := d.dialog.RunModal()
	if active != nil && active.IsVisible() {
		active.ToFront()
	}
	return result
}
