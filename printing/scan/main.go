package main

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/richardwilkes/toolbox/atexit"
	"github.com/richardwilkes/toolbox/cmdline"
	"github.com/richardwilkes/toolbox/log/jot"
	"github.com/richardwilkes/toolbox/xio"
	"github.com/richardwilkes/unison/printing"
)

func main() {
	cl := cmdline.New(true)
	duration := time.Second
	cl.NewGeneralOption(&duration).SetName("duration").SetSingle('d').SetUsage("The amount of time to scan for printers as well as the amount of time to wait for a response when querying for attributes")
	output := "scan-results.txt"
	cl.NewGeneralOption(&output).SetName("output").SetSingle('o').SetUsage("The file to write to")
	cl.Parse(os.Args[1:])
	scan(duration, output)
	atexit.Exit(0)
}

func scan(duration time.Duration, output string) {
	f, err := os.Create(output)
	jot.FatalIfErr(err)
	jot.SetWriter(&xio.TeeWriter{Writers: []io.Writer{f, os.Stdout}})
	pm := &printing.PrintManager{}
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	printers := make(chan *printing.Printer, 128)
	pm.ScanForPrinters(ctx, printers)
	needDivider := false
	for printer := range printers {
		if needDivider {
			jot.Info("=====")
		} else {
			needDivider = true
		}
		jot.Infof("Found Printer: %s", printer.Name)
		var a *printing.PrinterAttributes
		if a, err = printer.Attributes(duration, true); err != nil {
			jot.Error(err)
			continue
		}
		for k, v := range a.Attributes {
			jot.Infof("  %s: %v", k, v)
		}
	}
}
