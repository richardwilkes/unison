// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package main

import (
	"flag"

	"github.com/richardwilkes/toolbox/v2/xflag"
	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/toolbox/v2/xslog"
	"github.com/richardwilkes/toolbox/v2/xyaml"
	"github.com/richardwilkes/unison/cmd/upack/packager"
)

func main() {
	xos.AppName = "Unison Packager"
	xos.AppCmdName = "upack"
	xos.License = "Mozilla Public License, version 2.0"
	xos.CopyrightStartYear = "2021"
	xos.CopyrightHolder = "Richard A. Wilkes"
	xos.AppIdentifier = "com.trollworks.unison.packager"
	xflag.SetUsage(nil, "A tool for packaging Unison apps for distribution.", "<config-file>")
	release := flag.String("release", "", "The release `version` to package (e.g. '1.2.3') to package")
	flag.StringVar(release, "r", "", "Short `version` of -release")
	createDist := flag.Bool("dist", false, "Enable creation of a distribution package")
	flag.BoolVar(createDist, "d", false, "Short version of -dist")
	logCfg := xslog.Config{Console: true}
	logCfg.AddFlags()
	xflag.Parse()
	args := flag.Args()
	if len(args) != 1 {
		xos.ExitWithMsg("must provide exactly one argument specifying the configuration file")
	}
	if *release == "" {
		xos.ExitWithMsg("must specify a release version")
	}
	var cfg packager.Config
	xos.ExitIfErr(xyaml.Load(args[0], &cfg))
	xos.ExitIfErr(packager.Package(&cfg, *release, *createDist))
	xos.Exit(0)
}
