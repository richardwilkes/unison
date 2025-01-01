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
	"io"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/richardwilkes/toolbox/atexit"
	"github.com/richardwilkes/toolbox/cmdline"
	"github.com/richardwilkes/toolbox/errs"
	"github.com/richardwilkes/toolbox/fatal"
	"github.com/richardwilkes/toolbox/xio"
	"github.com/richardwilkes/unison/printing"
)

func main() {
	cl := cmdline.New(true)
	duration := 5 * time.Second
	cl.NewGeneralOption(&duration).SetName("duration").SetSingle('d').SetUsage("The amount of time to scan for printers as well as the amount of time to wait for a response when querying for attributes")
	output := "scan-results.txt"
	cl.NewGeneralOption(&output).SetName("output").SetSingle('o').SetUsage("The file to write to")
	cl.Parse(os.Args[1:])
	scan(duration, output)
	atexit.Exit(0)
}

func scan(duration time.Duration, output string) {
	f, err := os.Create(output)
	fatal.IfErr(err)
	log.SetOutput(&xio.TeeWriter{Writers: []io.Writer{f, os.Stdout}})
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
			slog.Info("attribute", "key", k, "value", v)
		}
	}
}
