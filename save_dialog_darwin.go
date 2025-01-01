// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
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
	"github.com/richardwilkes/unison/internal/ns"
)

type macSaveDialog struct {
	dialog ns.SavePanel
}

func platformNewSaveDialog() SaveDialog {
	return &macSaveDialog{dialog: ns.NewSavePanel()}
}

func (d *macSaveDialog) InitialDirectory() string {
	urlStr := d.dialog.DirectoryURL().AbsoluteString()
	u, err := url.Parse(urlStr)
	if err != nil {
		errs.Log(errs.NewWithCause("unable to parse directory URL", err), "url", urlStr)
		return ""
	}
	return u.Path
}

func (d *macSaveDialog) SetInitialDirectory(dir string) {
	dirURL := ns.NewFileURL(dir)
	defer dirURL.Release()
	d.dialog.SetDirectoryURL(dirURL)
}

func (d *macSaveDialog) InitialFileName() string {
	return d.dialog.InitialFileName()
}

func (d *macSaveDialog) SetInitialFileName(name string) {
	d.dialog.SetInitialFileName(name)
}

func (d *macSaveDialog) AllowedExtensions() []string {
	allowed := d.dialog.AllowedFileTypes()
	defer allowed.Release()
	return allowed.ArrayOfStringToStringSlice()
}

func (d *macSaveDialog) SetAllowedExtensions(types ...string) {
	types = SanitizeExtensionList(types)
	if len(types) != 0 {
		d.dialog.SetAllowedFileTypes(ns.NewArrayFromStringSlice(types))
	} else {
		d.dialog.SetAllowedFileTypes(0)
	}
}

func (d *macSaveDialog) Path() string {
	urlStr := d.dialog.URL().AbsoluteString()
	u, err := url.Parse(urlStr)
	if err != nil {
		errs.Log(errs.NewWithCause("unable to convert url to path", err), "url", urlStr)
		return ""
	}
	return u.Path
}

func (d *macSaveDialog) RunModal() bool {
	active := ActiveWindow()
	if active != nil {
		active.restoreHiddenCursor()
	}
	defer func() {
		if active != nil && active.IsVisible() {
			active.ToFront()
		}
	}()
	return d.dialog.RunModal()
}
