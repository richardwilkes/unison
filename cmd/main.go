package main

import (
	"log/slog"

	"github.com/richardwilkes/toolbox/v2/xflag"
	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/toolbox/v2/xslog"
	"github.com/richardwilkes/unison/internal/x11"
)

var x11Conn *x11.Conn

func main() {
	logCfg := xslog.Config{Console: true}
	logCfg.AddFlags()
	xflag.Parse()
	xos.ExitIfErr(start())
	xos.Exit(0)
}

func start() error {
	var err error
	if x11Conn, err = x11.NewConn(); err != nil {
		return err
	}

	slog.Info("RANDR",
		"available", x11Conn.ExtRandr.Present,
		"major", x11Conn.ExtRandr.MajorVersion,
		"minor", x11Conn.ExtRandr.MinorVersion,
	)

	x11Conn.SetClipboardText("test text")

	var monitors []x11.Monitor
	if monitors, err = x11Conn.ExtRandr.GetMonitors(x11Conn.RootWindow(), true); err != nil {
		return err
	}
	for i := range monitors {
		slog.Info("monitor",
			"index", i,
			"name", monitors[i].Name,
			"primary", monitors[i].Primary,
			"automatic", monitors[i].Automatic,
			"x", monitors[i].X,
			"y", monitors[i].Y,
			"width", monitors[i].Width,
			"height", monitors[i].Height,
			"widthMM", monitors[i].WidthMM,
			"heightMM", monitors[i].HeightMM,
		)
	}

	x11Conn.Close()
	return nil
}
