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
	"sync/atomic"
	"time"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/toolbox/v2/xreflect"
	"github.com/richardwilkes/unison/enums/thememode"
	"github.com/richardwilkes/unison/internal/x11"
)

var (
	// x11Conn is UI-thread-only state; everything that touches it must run on the UI thread. The one exception is
	// waking the event loop from another goroutine, which goes through x11PostConn below instead.
	x11Conn *x11.Conn
	// x11PostConn mirrors x11Conn for use by apiPostEmptyEvent, which may be called from any goroutine and therefore
	// cannot read x11Conn without racing apiTerminate's teardown of it on the UI thread.
	x11PostConn             atomic.Pointer[x11.Conn]
	linuxColorModeTrackable atomic.Bool
	linuxDarkModeEnabled    atomic.Bool
	linuxPortalHasValue     atomic.Bool
	linuxPortalValue        atomic.Uint32 // 0 = no preference, 1 = prefer dark, 2 = prefer light
)

func apiBeginStartup() error {
	var err error
	if x11Conn, err = x11.NewConn(); err != nil {
		return err
	}
	x11PostConn.Store(x11Conn)
	apiFillKeyCodes()
	return nil
}

func apiLateInit() {
	// Dark mode is detected from two sources, in priority order:
	//   1. The XDG Desktop Portal "color-scheme" appearance setting (GNOME 42+, KDE Plasma 5.23+).
	//   2. XSETTINGS, the GTK theme published over X11 (Cinnamon, MATE, XFCE, Budgie, GNOME on X11, ...).
	// The portal is the modern cross-desktop standard; XSETTINGS covers desktops that do not implement it.
	x11Conn.InitXSettings()
	if value, ok := x11.ReadColorScheme(); ok {
		linuxPortalValue.Store(value)
		linuxPortalHasValue.Store(true)
	}
	linuxRecomputeDarkMode()
	// The dynamic colors have already been built assuming light mode (RebuildDynamicColors runs before apiLateInit), so
	// if we detected dark mode at launch, trigger a rebuild now, before the first frame is shown.
	if linuxDarkModeEnabled.Load() && CurrentThemeMode() == thememode.Auto {
		ThemeChanged()
	}
	x11.WatchColorScheme(func(value uint32) {
		InvokeTask(func() {
			linuxPortalValue.Store(value)
			linuxPortalHasValue.Store(true)
			if linuxRecomputeDarkMode() {
				ThemeChanged()
			}
		})
	})
}

// linuxRecomputeDarkMode recombines the portal and XSETTINGS sources into the cached dark-mode state, returning whether
// either the tracking-possible or dark-mode value changed. It must be called on the main thread.
func linuxRecomputeDarkMode() bool {
	var dark, trackable bool
	if linuxPortalHasValue.Load() {
		switch linuxPortalValue.Load() {
		case 1: // prefer dark
			dark = true
			trackable = true
		case 2: // prefer light
			dark = false
			trackable = true
		}
	}
	if !trackable { // The portal gave no definite answer; fall back to the GTK theme via XSETTINGS.
		if d, ok := x11Conn.XSettingsDark(); ok {
			dark = d
			trackable = true
		}
	}
	trackableChanged := linuxColorModeTrackable.Swap(trackable) != trackable
	darkChanged := linuxDarkModeEnabled.Swap(dark) != dark
	return trackableChanged || darkChanged
}

// linuxXSettingsChanged is invoked from the X11 event loop when the XSETTINGS manager's property changes.
func linuxXSettingsChanged() {
	if x11Conn.RefreshXSettings() && linuxRecomputeDarkMode() {
		ThemeChanged()
	}
}

func apiFinalFinishStartup() {
}

func apiTerminate() error {
	if x11Conn != nil {
		// Withdraw the connection from apiPostEmptyEvent before closing it. A goroutine that loaded the pointer just
		// before the swap may still call PostEmptyEvent concurrently with (or after) Close, which is safe: it becomes
		// a no-op once the connection's event channel shuts down.
		x11PostConn.Store(nil)
		x11Conn.Close()
		x11Conn = nil
	}
	return nil
}

func apiBeep() {
	x11Conn.Bell(0)
}

func apiIsColorModeTrackingPossible() bool {
	return linuxColorModeTrackable.Load()
}

func apiIsDarkModeEnabled() bool {
	return linuxDarkModeEnabled.Load()
}

func apiDoubleClickInterval() time.Duration {
	return 500 * time.Millisecond
}

func apiPollEvents() {
	x11ProcessEvent(x11Conn.PollEvents(nil))
}

func apiWaitEvents() {
	// Block until at least one event is available so the event loop idles instead of spinning. Then process that event
	// along with any others that are already pending. They are handled one at a time rather than pulling them all at
	// once, so that a nested event loop started by a handler (such as the one used for the source side of drag & drop)
	// is able to see the events that are still pending.
	x11ProcessEvent(x11Conn.WaitEvents(nil))
	if x11Conn.Dead() {
		// The connection to the X server was lost (server exit, dropped remote session, etc.). No further events can
		// ever arrive and every request now fails immediately, so returning would leave the main loop spinning at full
		// speed on a dead connection. Do what Xlib's fatal IO error handler does and exit, but through xos.Exit so the
		// registered exit hooks (including the normal quit path) still get a chance to clean up. An orderly quit never
		// reaches this point, since it tears the connection down from within xos.Exit and the process terminates
		// before the main loop runs again.
		errs.Log(errs.New("connection to X server lost"))
		xos.Exit(1)
	}
	for {
		e := x11Conn.PollEvents(nil)
		if xreflect.IsNil(e) {
			return
		}
		x11ProcessEvent(e)
	}
}

func apiPostEmptyEvent() {
	// This runs on arbitrary goroutines, so it must use the atomic x11PostConn handle rather than x11Conn, whose
	// non-atomic teardown in apiTerminate would race the check here.
	if conn := x11PostConn.Load(); conn != nil {
		conn.PostEmptyEvent()
	}
}

// apiWithAutoreleasePool runs f directly: autorelease pools are a macOS concept with no X11 counterpart.
func apiWithAutoreleasePool(f func()) {
	f()
}
