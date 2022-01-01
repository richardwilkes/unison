// Copyright ©2021 by Richard A. Wilkes. All rights reserved.
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
	"runtime"

	"github.com/richardwilkes/toolbox/errs"
	"github.com/richardwilkes/toolbox/log/jot"
	"github.com/richardwilkes/unison/internal/ns"
)

type macSaveDialog struct {
	dialog ns.SavePanel
}

func platformNewSaveDialog() SaveDialog {
	d := &macSaveDialog{dialog: ns.NewSavePanel()}
	runtime.SetFinalizer(d, func(obj *macSaveDialog) {
		ReleaseOnUIThread(func() {
			obj.dialog.Release()
		})
	})
	return d
}

func (d *macSaveDialog) InitialDirectory() string {
	u, err := url.Parse(d.dialog.DirectoryURL().AbsoluteString())
	if err != nil {
		jot.Warn(errs.NewWithCause("unable to parse directory URL", err))
		return ""
	}
	return u.Path
}

func (d *macSaveDialog) SetInitialDirectory(dir string) {
	dirURL := ns.NewFileURL(dir)
	defer dirURL.Release()
	d.dialog.SetDirectoryURL(dirURL)
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
	u, err := url.Parse(d.dialog.URL().AbsoluteString())
	if err != nil {
		jot.Warn(errs.NewWithCause("unable to convert url to path", err))
		return ""
	}
	return u.Path
}

func (d *macSaveDialog) RunModal() bool {
	active := ActiveWindow()
	result := d.dialog.RunModal()
	if active != nil && active.IsVisible() {
		active.ToFront()
	}
	return result
}
