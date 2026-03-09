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
	"fmt"
	"time"

	"github.com/richardwilkes/unison/internal/x11"
)

var xconn *x11.Conn

func apiBeginStartup() error {
	var err error
	if xconn, err = x11.NewConn(); err != nil {
		return err
	}
	var prop *x11.GetPropertyReply
	if prop, err = x11.GetProperty(xconn, xconn.RootWindow(), x11.AtomResourceManager, x11.AtomString, 0, 100_000_000, false); err != nil {
		return err
	}
	if prop.Format == 8 && prop.Type == x11.AtomString {
		fmt.Printf("\nX11 Resource Manager:\n\n%s\n\n====\n", string(prop.Value))
	}

	apiFillKeyCodes()
	return nil
}

func apiLateInit() {
	// TODO: Need implementation
}

func apiFinalFinishStartup() {
	// TODO: Need implementation
}

func apiTerminate() error {
	// TODO: Need implementation
	return nil
}

func apiBeep() {
	// TODO: Need implementation
}

func apiIsColorModeTrackingPossible() bool {
	// TODO: Need implementation
	return false
}

func apiIsDarkModeEnabled() bool {
	// TODO: Need implementation
	return false
}

func apiDoubleClickInterval() time.Duration {
	return 500 * time.Millisecond
}

func apiPollEvents() {
	// TODO: Need implementation
}

func apiWaitEvents() {
	// TODO: Need implementation
}

func apiPostEmptyEvent() {
	// TODO: Need implementation
}
