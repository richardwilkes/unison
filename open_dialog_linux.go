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
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/i18n"
)

var _ OpenDialog = &x11OpenDialog{}

type x11OpenDialog struct {
	fallback OpenDialog
}

func apiNewOpenDialog() OpenDialog {
	return &x11OpenDialog{fallback: NewCommonOpenDialog()}
}

func (d *x11OpenDialog) InitialDirectory() string {
	return d.fallback.InitialDirectory()
}

func (d *x11OpenDialog) SetInitialDirectory(dir string) {
	d.fallback.SetInitialDirectory(dir)
}

func (d *x11OpenDialog) AllowedExtensions() []string {
	return d.fallback.AllowedExtensions()
}

func (d *x11OpenDialog) SetAllowedExtensions(extensions ...string) {
	d.fallback.SetAllowedExtensions(extensions...)
}

func (d *x11OpenDialog) RunModal() bool {
	kdialog, err := exec.LookPath("kdialog")
	if err != nil {
		kdialog = ""
	}
	if os.Getenv("KDE_FULL_SESSION") != "" && kdialog != "" {
		return d.x11RunKDialog(kdialog)
	}

	var zenity string
	if zenity, err = exec.LookPath("zenity"); err != nil {
		zenity = ""
	}
	if zenity != "" {
		return d.x11RunZenity(zenity)
	}
	if kdialog != "" {
		return d.x11RunKDialog(kdialog)
	}
	return d.fallback.RunModal()
}

func (d *x11OpenDialog) Path() string {
	return d.fallback.Path()
}

func (d *x11OpenDialog) CanChooseFiles() bool {
	return d.fallback.CanChooseFiles()
}

func (d *x11OpenDialog) SetCanChooseFiles(canChoose bool) {
	d.fallback.SetCanChooseFiles(canChoose)
}

func (d *x11OpenDialog) CanChooseDirectories() bool {
	return d.fallback.CanChooseDirectories()
}

func (d *x11OpenDialog) SetCanChooseDirectories(canChoose bool) {
	d.fallback.SetCanChooseDirectories(canChoose)
}

func (d *x11OpenDialog) ResolvesAliases() bool {
	return d.fallback.ResolvesAliases()
}

func (d *x11OpenDialog) SetResolvesAliases(resolves bool) {
	d.fallback.SetResolvesAliases(resolves)
}

func (d *x11OpenDialog) AllowsMultipleSelection() bool {
	return d.fallback.AllowsMultipleSelection()
}

func (d *x11OpenDialog) SetAllowsMultipleSelection(allow bool) {
	d.fallback.SetAllowsMultipleSelection(allow)
}

func (d *x11OpenDialog) Paths() []string {
	return d.fallback.Paths()
}

func (d *x11OpenDialog) x11RunKDialog(kdialog string) bool {
	cmd := exec.Command(kdialog)
	if d.CanChooseDirectories() {
		cmd.Args = append(cmd.Args, "--getexistingdirectory")
	} else {
		cmd.Args = append(cmd.Args, "--getopenfilename")
	}
	if d.AllowsMultipleSelection() {
		cmd.Args = append(cmd.Args, "--multiple", "--separate-output")
	}
	cmd.Args = append(cmd.Args, d.InitialDirectory()+"/")
	if d.CanChooseFiles() {
		allowed := d.x11PrepExt()
		if len(allowed) != 0 {
			cmd.Args = append(cmd.Args, fmt.Sprintf(i18n.Text("Readable Files (%s)"), strings.Join(allowed, " ")))
		}
	}
	return d.x11RunModal(cmd, "\n")
}

func (d *x11OpenDialog) x11RunZenity(zenity string) bool {
	cmd := exec.Command(zenity, "--file-selection", "--filename="+d.InitialDirectory()+"/")
	if d.AllowsMultipleSelection() {
		cmd.Args = append(cmd.Args, "--multiple")
	}
	if d.CanChooseDirectories() {
		cmd.Args = append(cmd.Args, "--directory")
	} else {
		allowed := d.x11PrepExt()
		if len(allowed) != 0 {
			cmd.Args = append(cmd.Args, "--file-filter="+strings.Join(allowed, " "))
		}
	}
	return d.x11RunModal(cmd, "|")
}

func (d *x11OpenDialog) x11PrepExt() []string {
	allowed := d.fallback.AllowedExtensions()
	if len(allowed) != 0 {
		revised := make([]string, len(allowed))
		for i, one := range allowed {
			revised[i] = "*." + one
		}
		allowed = revised
	}
	return allowed
}

func (d *x11OpenDialog) x11RunModal(cmd *exec.Cmd, splitOn string) bool {
	wnd, err := NewWindow("")
	if err != nil {
		errs.Log(err)
	}
	// This window exists only to run a modal event loop, blocking input to this app's windows while the external
	// dialog process runs, so it is never shown. Showing it off-screen instead does not work under Wayland, which
	// ignores client-requested window positions and places it on-screen as a tiny "phantom" window.
	wnd.keepHidden = true
	InvokeTaskAfter(func() { go d.x11RunCmd(wnd, cmd, splitOn) }, time.Millisecond)
	return wnd.RunModal() == ModalResponseOK
}

func (d *x11OpenDialog) x11RunCmd(wnd *Window, cmd *exec.Cmd, splitOn string) {
	code := ModalResponseCancel
	defer func() { InvokeTask(func() { wnd.StopModal(code) }) }()
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return
		}
		errs.Log(err)
		return
	}
	if cmd.ProcessState.ExitCode() != 0 {
		return
	}
	if dialog, ok := d.fallback.(*fileDialog); ok {
		dialog.paths = strings.Split(strings.TrimSuffix(string(out), "\n"), splitOn)
	} else {
		slog.Error("unable to access dialog to store path")
	}
	code = ModalResponseOK
}
