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
	"context"
	"flag"
	"io"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/xflag"
	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/toolbox/v2/xslog"
	"github.com/richardwilkes/unison/printing"
)

func main() {
	xos.AppName = "Printer Scan"
	xos.AppCmdName = "printerscan"
	xos.License = "Mozilla Public License, version 2.0"
	xos.CopyrightStartYear = "2021"
	xos.CopyrightHolder = "Richard A. Wilkes"
	xos.AppIdentifier = "com.trollworks.unison.printer.scanner"
	xflag.SetUsage(nil, "A tool for scanning for printers on the network.", "")
	duration := flag.Duration("duration", 5*time.Second,
		"The amount of `time` to scan for printers as well as the amount of time to wait for a response when querying for attributes")
	defOutput := "scan-results.txt"
	outputDesc := "The file to write to"
	output := flag.String("output", defOutput, outputDesc)
	flag.StringVar(output, "o", defOutput, outputDesc)
	logCfg := xslog.Config{Console: true}
	logCfg.AddFlags()
	xflag.Parse()
	scan(*duration, *output)
	xos.Exit(0)
}

func scan(duration time.Duration, output string) {
	f, err := os.Create(output)
	xos.ExitIfErr(err)
	log.SetOutput(io.MultiWriter(f, os.Stdout))
	pm := &printing.PrintManager{}
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	printers := make(chan *printing.Printer, 128)
	pm.ScanForPrinters(ctx, printers)
	needDivider := false
	for printer := range printers {
		if needDivider {
			slog.Info("=====")
		} else {
			needDivider = true
		}
		slog.Info("found printer", "name", printer.Name, "host", printer.Host, "port", printer.Port)
		var a *printing.PrinterAttributes
		if a, err = printer.Attributes(duration, true); err != nil {
			errs.Log(err)
			continue
		}
		for k, v := range a.Attributes {
			slog.Info("attribute", k, v)
		}
	}
}
