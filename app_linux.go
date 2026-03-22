// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"strconv"
	"strings"
	"time"

	"github.com/richardwilkes/unison/internal/x11"
)

var (
	x11Conn         *x11.Conn
	x11ContentScale = float32(1)
)

func apiBeginStartup() error {
	var err error
	if x11Conn, err = x11.NewConn(); err != nil {
		return err
	}
	if x11ContentScale, err = x11GetContentScale(); err != nil {
		return err
	}
	apiFillKeyCodes()
	// TODO: Need additional implementation?
	return nil
}

func apiLateInit() {
	// TODO: Need implementation?
}

func apiFinalFinishStartup() {
	// TODO: Need implementation?
}

func apiTerminate() error {
	if x11Conn != nil {
		x11Conn.PushClipboardToManager()
		x11Conn.Close()
		x11Conn = nil
	}
	return nil
}

func apiBeep() {
	x11Conn.Bell(0)
}

func apiIsColorModeTrackingPossible() bool {
	return false
}

func apiIsDarkModeEnabled() bool {
	return false
}

func apiDoubleClickInterval() time.Duration {
	return 500 * time.Millisecond
}

func apiPollEvents() {
	x11Conn.PollEvents()
}

func apiWaitEvents() {
	x11Conn.WaitEvents()
}

func apiPostEmptyEvent() {
	x11Conn.PostEmptyEvent()
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
