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
	"runtime"
	"strconv"
	"sync/atomic"
)

// uiGoroutineID holds the ID of the goroutine that called Start(). Because init() locks the main goroutine to the main
// OS thread, that goroutine is the one and only UI thread, so goroutine identity is a valid stand-in for thread
// identity. This remains true inside platform callbacks made on the main thread, since those nest within the main
// goroutine's outstanding call into C. A zero value means Start() has not been reached yet.
var uiGoroutineID atomic.Uint64

// onUIThread returns true if the caller is running on the UI thread. Platform windowing APIs (Cocoa, Win32, X11) are
// thread-affine and must only be touched from the UI thread; code that can be entered from an arbitrary goroutine (such
// as the signal handler goroutine that xos installs) uses this to decide whether it must marshal its work over via
// InvokeTask instead of performing it directly.
func onUIThread() bool {
	id := uiGoroutineID.Load()
	return id != 0 && id == currentGoroutineID()
}

// currentGoroutineID returns the ID of the calling goroutine, or 0 if it could not be determined. The runtime provides
// no API for this, so the ID is scraped from the header line of the goroutine's own stack trace, which always has the
// form "goroutine N [state]:". This is only used for the UI thread check, which treats 0 as "not the UI thread", so a
// parse failure degrades to the safe (marshaling) behavior rather than an incorrect one.
func currentGoroutineID() uint64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	const prefix = "goroutine "
	s := string(buf[:n])
	if len(s) < len(prefix) || s[:len(prefix)] != prefix {
		return 0
	}
	s = s[len(prefix):]
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	id, err := strconv.ParseUint(s[:i], 10, 64)
	if err != nil {
		return 0
	}
	return id
}
