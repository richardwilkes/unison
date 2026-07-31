// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package main

import (
	_ "embed"
	"flag"

	"github.com/richardwilkes/toolbox/v2/i18n"
	"github.com/richardwilkes/toolbox/v2/xflag"
	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/toolbox/v2/xslog"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/cmd/example/demo"
)

func main() {
	xos.AppName = "Example"
	xos.AppCmdName = "example"
	xos.CopyrightStartYear = "2021"
	xos.CopyrightHolder = "Richard A. Wilkes"
	xos.AppIdentifier = "com.trollworks.unison.example"
	xflag.SetUsage(nil, "Demo of some of the features of Unison", "")
	noFileDialogs := flag.Bool("no-file-dialogs", false, i18n.Text("Use internal file dialogs rather than the platform's"))
	noGlobalMenuBar := flag.Bool("no-menu-bar", false, i18n.Text("Disable the global menu bar on platforms that support it"))

	unison.AttachConsole()

	logCfg := xslog.Config{Console: true}
	logCfg.AddFlags()
	xflag.Parse()

	var options []unison.StartupOption
	options = append(options, unison.StartupFinishedCallback(func() {
		_, err := demo.NewDemoWindow()
		xos.ExitIfErr(err)
	}))
	if *noFileDialogs {
		options = append(options, unison.NoPlatformFileDialogs())
	}
	if *noGlobalMenuBar {
		options = append(options, unison.NoGlobalMenuBar())
	}
	unison.Start(options...) // Never returns
}
