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
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type linuxFileDialog struct {
	fallback SaveDialog
	results  []string
}

func platformNewSaveDialog() SaveDialog {
	return &linuxFileDialog{fallback: NewCommonSaveDialog()}
}

func (d *linuxFileDialog) InitialDirectory() string {
	return d.fallback.InitialDirectory()
}

func (d *linuxFileDialog) SetInitialDirectory(dir string) {
	d.fallback.SetInitialDirectory(dir)
}

func (d *linuxFileDialog) AllowedExtensions() []string {
	return d.fallback.AllowedExtensions()
}

func (d *linuxFileDialog) SetAllowedExtensions(extensions ...string) {
	d.fallback.SetAllowedExtensions(extensions...)
}

func (d *linuxFileDialog) RunModal() bool {
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

func (d *linuxFileDialog) runKDialog(kdialog string) bool {
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
	cmd := exec.Command(kdialog, "--getsavefilename", d.InitialDirectory()+"/untitled"+ext)
	if len(allowed) != 0 {
		list := strings.Join(allowed, " ")
		cmd.Args = append(cmd.Args, fmt.Sprintf("%[1]s (%[1]s)", list))
	}
	out, _ := cmd.Output()
	if cmd.ProcessState.ExitCode() != 0 {
		return false
	}
	d.fallback.(*fileDialog).paths = strings.Split(string(out), "\n")
	return true
}

func (d *linuxFileDialog) runZenity(zenity string) bool {
	return true
}

func (d *linuxFileDialog) Path() string {
	return d.fallback.Path()
}
