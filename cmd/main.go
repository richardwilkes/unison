package main

import (
	"strconv"
	"strings"

	"github.com/richardwilkes/toolbox/v2/xflag"
	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/toolbox/v2/xslog"
	"github.com/richardwilkes/unison/internal/x11"
)

var (
	x11Conn         *x11.Conn
	x11ContentScale float32 = 1
)

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
	if x11ContentScale, err = x11GetContentScale(); err != nil {
		return err
	}
	x11Conn.SetClipboardText("Yo!")
	x11Conn.Close()
	return nil
}

func x11GetContentScale() (float32, error) {
	format, actualPropertyType, value, err := x11Conn.GetProperty(x11Conn.RootWindow(), x11.AtomResourceManager,
		x11.AtomString, 0, 100_000_000, false)
	if err != nil {
		return 1, err
	}
	if format == 8 && actualPropertyType == x11.AtomString {
		for _, line := range strings.Split(string(value), "\n") {
			const xftDPI = "Xft.dpi:"
			if strings.HasPrefix(line, xftDPI) {
				var dpi int
				if dpi, err = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, xftDPI))); err == nil {
					return float32(dpi) / 96, nil
				}
			}
		}
	}
	return 1, nil
}
