// Copyright (c) 2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package main

import (
	"os"
	"strconv"

	"github.com/richardwilkes/toolbox/atexit"
	"github.com/richardwilkes/toolbox/cmdline"
	"github.com/richardwilkes/toolbox/xio/fs"
	"github.com/richardwilkes/unison/upack/packager"
)

func main() {
	cmdline.AppName = "Unison Packager"
	cmdline.AppCmdName = "upack"
	cmdline.License = "Mozilla Public License, version 2.0"
	cmdline.CopyrightStartYear = "2025"
	cmdline.CopyrightHolder = "Richard A. Wilkes"
	cmdline.AppIdentifier = "com.trollworks.unison.packager"
	cl := cmdline.New(false)
	cl.Description = "A tool for packaging Unison apps for distribution."
	cl.UsageSuffix = "<config-file>"
	var release string
	var createDist bool
	cl.NewGeneralOption(&release).SetName("release").SetSingle('r').SetArg("version").
		SetUsage(`The release version to package (e.g. "1.2.3") to package.`)
	cl.NewGeneralOption(&createDist).SetName("dist").SetSingle('d').
		SetUsage(`Enable creation of a distribution package.`)
	args := cl.Parse(os.Args[1:])
	if len(args) != 1 {
		cl.FatalMsg("Expected a single argument specifying the configuration file to use, got " +
			strconv.Itoa(len(args)))
	}
	if release == "" {
		cl.FatalMsg("A release version must be specified.")
	}
	var cfg packager.Config
	cl.FatalIfError(fs.LoadYAML(args[0], &cfg))
	cl.FatalIfError(packager.Package(&cfg, release, createDist))
	atexit.Exit(0)
}
