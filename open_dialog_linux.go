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
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/i18n"
)

type linuxOpenDialog struct {
	fallback OpenDialog
}

func platformNewOpenDialog() OpenDialog {
	return &linuxOpenDialog{fallback: NewCommonOpenDialog()}
}

func (d *linuxOpenDialog) InitialDirectory() string {
	return d.fallback.InitialDirectory()
}

func (d *linuxOpenDialog) SetInitialDirectory(dir string) {
	d.fallback.SetInitialDirectory(dir)
}

func (d *linuxOpenDialog) AllowedExtensions() []string {
	return d.fallback.AllowedExtensions()
}

func (d *linuxOpenDialog) SetAllowedExtensions(extensions ...string) {
	d.fallback.SetAllowedExtensions(extensions...)
}

func (d *linuxOpenDialog) Path() string {
	return d.fallback.Path()
}

func (d *linuxOpenDialog) CanChooseFiles() bool {
	return d.fallback.CanChooseFiles()
}

func (d *linuxOpenDialog) SetCanChooseFiles(canChoose bool) {
	d.fallback.SetCanChooseFiles(canChoose)
}

func (d *linuxOpenDialog) CanChooseDirectories() bool {
	return d.fallback.CanChooseDirectories()
}

func (d *linuxOpenDialog) SetCanChooseDirectories(canChoose bool) {
	d.fallback.SetCanChooseDirectories(canChoose)
}

func (d *linuxOpenDialog) ResolvesAliases() bool {
	return d.fallback.ResolvesAliases()
}

func (d *linuxOpenDialog) SetResolvesAliases(resolves bool) {
	d.fallback.SetResolvesAliases(resolves)
}

func (d *linuxOpenDialog) AllowsMultipleSelection() bool {
	return d.fallback.AllowsMultipleSelection()
}

func (d *linuxOpenDialog) SetAllowsMultipleSelection(allow bool) {
	d.fallback.SetAllowsMultipleSelection(allow)
}

func (d *linuxOpenDialog) Paths() []string {
	return d.fallback.Paths()
}

func (d *linuxOpenDialog) RunModal() bool {
	kdialog, err := exec.LookPath("kdialog")
	if err != nil {
		kdialog = ""
	}
	if os.Getenv("KDE_FULL_SESSION") != "" && kdialog != "" {
		return d.runKDialog(kdialog)
	}

	var zenity string
	if zenity, err = exec.LookPath("zenity"); err != nil {
		zenity = ""
	}
	if zenity != "" {
		return d.runZenity(zenity)
	}
	if kdialog != "" {
		return d.runKDialog(kdialog)
	}
	return d.fallback.RunModal()
}

func (d *linuxOpenDialog) runKDialog(kdialog string) bool {
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
		allowed := d.prepExt()
		if len(allowed) != 0 {
			cmd.Args = append(cmd.Args, fmt.Sprintf(i18n.Text("Readable Files (%s)"), strings.Join(allowed, " ")))
		}
	}
	return d.runModal(cmd, "\n")
}

func (d *linuxOpenDialog) runZenity(zenity string) bool {
	cmd := exec.Command(zenity, "--file-selection", "--filename="+d.InitialDirectory()+"/")
	if d.AllowsMultipleSelection() {
		cmd.Args = append(cmd.Args, "--multiple")
	}
	if d.CanChooseDirectories() {
		cmd.Args = append(cmd.Args, "--directory")
	} else {
		allowed := d.prepExt()
		if len(allowed) != 0 {
			cmd.Args = append(cmd.Args, "--file-filter="+strings.Join(allowed, " "))
		}
	}
	return d.runModal(cmd, "|")
}

func (d *linuxOpenDialog) prepExt() []string {
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

func (d *linuxOpenDialog) runModal(cmd *exec.Cmd, splitOn string) bool {
	wnd, err := NewWindow("", FloatingWindowOption(), UndecoratedWindowOption(), NotResizableWindowOption())
	if err != nil {
		errs.Log(err)
	}
	wnd.SetFrameRect(NewRect(-10000, -10000, 1, 1))
	InvokeTaskAfter(func() { go d.runCmd(wnd, cmd, splitOn) }, time.Millisecond)
	return wnd.RunModal() == ModalResponseOK
}

func (d *linuxOpenDialog) runCmd(wnd *Window, cmd *exec.Cmd, splitOn string) {
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
	d.fallback.(*fileDialog).paths = strings.Split(strings.TrimSuffix(string(out), "\n"), splitOn)
	code = ModalResponseOK
}
