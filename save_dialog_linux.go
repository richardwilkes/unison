// Copyright (c) 2021-2024 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/richardwilkes/toolbox/errs"
	"github.com/richardwilkes/toolbox/xio/fs"
)

type linuxSaveDialog struct {
	fallback SaveDialog
}

func platformNewSaveDialog() SaveDialog {
	return &linuxSaveDialog{fallback: NewCommonSaveDialog()}
}

func (d *linuxSaveDialog) InitialDirectory() string {
	return d.fallback.InitialDirectory()
}

func (d *linuxSaveDialog) SetInitialDirectory(dir string) {
	d.fallback.SetInitialDirectory(dir)
}

func (d *linuxSaveDialog) InitialFileName() string {
	return d.fallback.InitialFileName()
}

func (d *linuxSaveDialog) SetInitialFileName(name string) {
	d.fallback.SetInitialFileName(name)
}

func (d *linuxSaveDialog) AllowedExtensions() []string {
	return d.fallback.AllowedExtensions()
}

func (d *linuxSaveDialog) SetAllowedExtensions(extensions ...string) {
	d.fallback.SetAllowedExtensions(extensions...)
}

func (d *linuxSaveDialog) Path() string {
	return d.fallback.Path()
}

func (d *linuxSaveDialog) RunModal() bool {
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

func (d *linuxSaveDialog) runKDialog(kdialog string) bool {
	ext, allowed := d.prepExt()
	cmd := exec.Command(kdialog, "--getsavefilename", d.InitialDirectory()+"/"+fs.TrimExtension(d.InitialFileName())+ext)
	if len(allowed) != 0 {
		list := strings.Join(allowed, " ")
		cmd.Args = append(cmd.Args, fmt.Sprintf("%[1]s (%[1]s)", list))
	}
	return d.runModal(cmd, "\n")
}

func (d *linuxSaveDialog) runZenity(zenity string) bool {
	cmd := exec.Command(zenity, "--help-file-selection")
	output, err := cmd.CombinedOutput()
	if err != nil {
		errs.Log(err, "cmd", cmd.String())
		return false
	}
	cmd = exec.Command(zenity, "--file-selection", "--save")
	if bytes.Contains(output, []byte("confirm-overwrite")) {
		cmd.Args = append(cmd.Args, "--confirm-overwrite")
	}
	ext, allowed := d.prepExt()
	cmd.Args = append(cmd.Args, "--filename="+d.InitialDirectory()+"/"+fs.TrimExtension(d.InitialFileName())+ext)
	if len(allowed) != 0 {
		cmd.Args = append(cmd.Args, "--file-filter="+strings.Join(allowed, " "))
	}
	return d.runModal(cmd, "|")
}

func (d *linuxSaveDialog) prepExt() (string, []string) {
	ext := ""
	allowed := d.fallback.AllowedExtensions()
	if len(allowed) != 0 {
		ext = "." + allowed[0]
		revised := make([]string, len(allowed))
		for i, one := range allowed {
			revised[i] = "*." + one
		}
		allowed = revised
	}
	return ext, allowed
}

func (d *linuxSaveDialog) runModal(cmd *exec.Cmd, splitOn string) bool {
	wnd, err := NewWindow("", FloatingWindowOption(), UndecoratedWindowOption(), NotResizableWindowOption())
	if err != nil {
		errs.Log(err)
	}
	wnd.SetFrameRect(NewRect(-10000, -10000, 1, 1))
	InvokeTaskAfter(func() { go d.runCmd(wnd, cmd, splitOn) }, time.Millisecond)
	return wnd.RunModal() == ModalResponseOK
}

func (d *linuxSaveDialog) runCmd(wnd *Window, cmd *exec.Cmd, splitOn string) {
	code := ModalResponseCancel
	defer func() { InvokeTask(func() { wnd.StopModal(code) }) }()
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return
		}
		errs.Log(err, "cmd", cmd.String())
		return
	}
	if cmd.ProcessState.ExitCode() != 0 {
		return
	}
	d.fallback.(*fileDialog).paths = strings.Split(strings.TrimSuffix(string(out), "\n"), splitOn)
	code = ModalResponseOK
}
