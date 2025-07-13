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
	_ "embed"

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

	unison.AttachConsole()

	logCfg := xslog.Config{Console: true}
	logCfg.AddFlags()
	xflag.Parse()

	unison.Start(unison.StartupFinishedCallback(func() {
		_, err := demo.NewDemoWindow(unison.PrimaryDisplay().Usable.Point)
		xos.ExitIfErr(err)
	})) // Never returns
}
